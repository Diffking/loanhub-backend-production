package repositories

import (
	"fmt"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// QueueRepository handles queue-related database operations
type QueueRepository struct {
	db *gorm.DB
}

// NewQueueRepository creates a new queue repository
func NewQueueRepository(db *gorm.DB) *QueueRepository {
	return &QueueRepository{db: db}
}

// ============================================================
// Branch Queries
// ============================================================

// GetActiveBranches returns all active branches
func (r *QueueRepository) GetActiveBranches() ([]models.Branch, error) {
	var branches []models.Branch
	err := r.db.Where("is_active = ?", true).Order("id ASC").Find(&branches).Error
	return branches, err
}

// GetBranchByID returns a branch by ID
func (r *QueueRepository) GetBranchByID(id uint) (*models.Branch, error) {
	var branch models.Branch
	err := r.db.First(&branch, id).Error
	return &branch, err
}

// ============================================================
// ServiceType Queries
// ============================================================

// GetActiveServiceTypes returns all active service types
func (r *QueueRepository) GetActiveServiceTypes() ([]models.ServiceType, error) {
	var types []models.ServiceType
	err := r.db.Where("is_active = ?", true).Order("display_order ASC").Find(&types).Error
	return types, err
}

// GetServiceTypeByID returns a service type by ID
func (r *QueueRepository) GetServiceTypeByID(id uint) (*models.ServiceType, error) {
	var st models.ServiceType
	err := r.db.First(&st, id).Error
	return &st, err
}

// ============================================================
// ServiceCounter Queries
// ============================================================

// GetCountersByBranch returns all active counters for a branch with preloaded relations
func (r *QueueRepository) GetCountersByBranch(branchID uint) ([]models.ServiceCounter, error) {
	var counters []models.ServiceCounter
	err := r.db.
		Preload("ServiceType").
		Preload("StaffUser").
		Where("branch_id = ? AND is_active = ?", branchID, true).
		Order("counter_number ASC").
		Find(&counters).Error
	return counters, err
}

// GetCounterByID returns a counter by ID with preloaded relations
func (r *QueueRepository) GetCounterByID(id uint) (*models.ServiceCounter, error) {
	var counter models.ServiceCounter
	err := r.db.Preload("ServiceType").Preload("Branch").First(&counter, id).Error
	return &counter, err
}

// UpdateCounterStatus updates counter status and optionally staff
func (r *QueueRepository) UpdateCounterStatus(counterID uint, status string, staffUserID *uint) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if staffUserID != nil {
		updates["staff_user_id"] = *staffUserID
	}
	return r.db.Model(&models.ServiceCounter{}).Where("id = ?", counterID).Updates(updates).Error
}

// ============================================================
// QueueTicket Queries
// ============================================================

// CreateTicket creates a new queue ticket
func (r *QueueRepository) CreateTicket(ticket *models.QueueTicket) error {
	return r.db.Create(ticket).Error
}

// GetTicketByID returns a ticket by ID with all relations
func (r *QueueRepository) GetTicketByID(id uint) (*models.QueueTicket, error) {
	var ticket models.QueueTicket
	err := r.db.
		Preload("Branch").
		Preload("ServiceType").
		Preload("Counter").
		Preload("User").
		First(&ticket, id).Error
	return &ticket, err
}

// GetTicketByNumber returns a ticket by ticket number for today
func (r *QueueRepository) GetTicketByNumber(ticketNumber string, queueDate time.Time) (*models.QueueTicket, error) {
	var ticket models.QueueTicket
	err := r.db.
		Preload("Branch").
		Preload("ServiceType").
		Preload("Counter").
		Where("ticket_number = ? AND queue_date = ?", ticketNumber, queueDate).
		First(&ticket).Error
	return &ticket, err
}

// GetMyTicketsToday returns all tickets for a user today
func (r *QueueRepository) GetMyTicketsToday(userID uint, queueDate time.Time) ([]models.QueueTicket, error) {
	var tickets []models.QueueTicket
	err := r.db.
		Preload("Branch").
		Preload("ServiceType").
		Preload("Counter").
		Where("user_id = ? AND queue_date = ?", userID, queueDate).
		Order("issued_at DESC").
		Find(&tickets).Error
	return tickets, err
}

// GetActiveTicketByUser checks if user already has an active (WAITING/CALLING/SERVING) ticket
// for the same branch+service today
func (r *QueueRepository) GetActiveTicketByUser(userID uint, branchID uint, serviceTypeID uint, queueDate time.Time) (*models.QueueTicket, error) {
	var ticket models.QueueTicket
	err := r.db.
		Where("user_id = ? AND branch_id = ? AND service_type_id = ? AND queue_date = ? AND status IN ?",
			userID, branchID, serviceTypeID, queueDate, []string{"WAITING", "CALLING", "SERVING"}).
		First(&ticket).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &ticket, err
}

// GetNextTicketNumber generates the next ticket number for today
// Walk-in: Q-001 ~ Q-499, Booking: B-001 ~ B-499
func (r *QueueRepository) GetNextTicketNumber(branchID uint, queueType string, queueDate time.Time) (string, error) {
	var count int64
	prefix := "Q"
	if queueType == "BOOKING" {
		prefix = "B"
	}

	r.db.Model(&models.QueueTicket{}).
		Where("branch_id = ? AND queue_type = ? AND queue_date = ?", branchID, queueType, queueDate).
		Count(&count)

	number := int(count) + 1
	return fmt.Sprintf("%s-%03d", prefix, number), nil
}

// GetWaitingCount returns the number of waiting tickets ahead of a given ticket
func (r *QueueRepository) GetWaitingCount(branchID uint, serviceTypeID uint, queueDate time.Time, issuedBefore time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.QueueTicket{}).
		Where("branch_id = ? AND service_type_id = ? AND queue_date = ? AND status = ? AND issued_at < ?",
			branchID, serviceTypeID, queueDate, "WAITING", issuedBefore).
		Count(&count).Error
	return count, err
}

// GetBranchQueueStatus returns queue counts by status for a branch today
func (r *QueueRepository) GetBranchQueueStatus(branchID uint, queueDate time.Time) (map[string]int64, error) {
	type Result struct {
		Status string
		Count  int64
	}
	var results []Result

	err := r.db.Model(&models.QueueTicket{}).
		Select("status, COUNT(*) as count").
		Where("branch_id = ? AND queue_date = ?", branchID, queueDate).
		Group("status").
		Find(&results).Error

	statusMap := map[string]int64{
		"WAITING":   0,
		"CALLING":   0,
		"SERVING":   0,
		"COMPLETED": 0,
		"SKIPPED":   0,
		"CANCELLED": 0,
	}
	for _, r := range results {
		statusMap[r.Status] = r.Count
	}
	return statusMap, err
}

// GetNextCallableTicket finds the next ticket to call based on priority rules:
// 1. Booking with check-in (WAITING, priority desc)
// 2. Walk-in by priority desc, then issued_at asc (FIFO)
func (r *QueueRepository) GetNextCallableTicket(branchID uint, serviceTypeID uint, queueDate time.Time) (*models.QueueTicket, error) {
	var ticket models.QueueTicket
	err := r.db.
		Preload("Branch").
		Preload("ServiceType").
		Preload("User").
		Where("branch_id = ? AND service_type_id = ? AND queue_date = ? AND status = ?",
			branchID, serviceTypeID, queueDate, "WAITING").
		Order("priority DESC, issued_at ASC").
		First(&ticket).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &ticket, err
}

// UpdateTicketStatus updates ticket status and related timestamps
func (r *QueueRepository) UpdateTicketStatus(ticketID uint, updates map[string]interface{}) error {
	return r.db.Model(&models.QueueTicket{}).Where("id = ?", ticketID).Updates(updates).Error
}

// GetWaitingTicketsByBranch returns all waiting tickets for a branch today (for dashboard)
func (r *QueueRepository) GetWaitingTicketsByBranch(branchID uint, queueDate time.Time) ([]models.QueueTicket, error) {
	var tickets []models.QueueTicket
	err := r.db.
		Preload("ServiceType").
		Preload("User").
		Where("branch_id = ? AND queue_date = ? AND status IN ?",
			branchID, queueDate, []string{"WAITING", "CALLING", "SERVING"}).
		Order("priority DESC, issued_at ASC").
		Find(&tickets).Error
	return tickets, err
}

// GetCompletedTicketsByBranch returns completed/skipped tickets for a branch today (history)
func (r *QueueRepository) GetCompletedTicketsByBranch(branchID uint, queueDate time.Time) ([]models.QueueTicket, error) {
	var tickets []models.QueueTicket
	err := r.db.
		Preload("ServiceType").
		Preload("User").
		Preload("Counter").
		Where("branch_id = ? AND queue_date = ? AND status IN ?",
			branchID, queueDate, []string{"COMPLETED", "SKIPPED", "CANCELLED"}).
		Order("completed_at DESC").
		Find(&tickets).Error
	return tickets, err
}

// GetCurrentServingByCounter returns the ticket currently being served at a counter
func (r *QueueRepository) GetCurrentServingByCounter(counterID uint, queueDate time.Time) (*models.QueueTicket, error) {
	var ticket models.QueueTicket
	err := r.db.
		Preload("User").
		Preload("ServiceType").
		Where("counter_id = ? AND queue_date = ? AND status IN ?",
			counterID, queueDate, []string{"CALLING", "SERVING"}).
		First(&ticket).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &ticket, err
}

// ============================================================
// QueueConfig Queries
// ============================================================

// GetConfigByBranch returns all config for a branch
func (r *QueueRepository) GetConfigByBranch(branchID uint) ([]models.QueueConfig, error) {
	var configs []models.QueueConfig
	err := r.db.Where("branch_id = ?", branchID).Find(&configs).Error
	return configs, err
}

// GetConfigValue returns a specific config value
func (r *QueueRepository) GetConfigValue(branchID uint, key string) (string, error) {
	var config models.QueueConfig
	err := r.db.Where("branch_id = ? AND config_key = ?", branchID, key).First(&config).Error
	if err != nil {
		return "", err
	}
	return config.ConfigValue, nil
}

// UpdateConfig updates or creates a config entry
func (r *QueueRepository) UpdateConfig(branchID uint, key string, value string) error {
	var config models.QueueConfig
	err := r.db.Where("branch_id = ? AND config_key = ?", branchID, key).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		config = models.QueueConfig{
			BranchID:    branchID,
			ConfigKey:   key,
			ConfigValue: value,
		}
		return r.db.Create(&config).Error
	}
	return r.db.Model(&config).Update("config_value", value).Error
}
