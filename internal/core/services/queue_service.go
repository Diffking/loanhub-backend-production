package services

import (
	"errors"
	"fmt"
	"log"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"
)

// Queue errors
var (
	ErrBranchNotFound      = errors.New("branch not found")
	ErrBranchClosed        = errors.New("branch is not active")
	ErrServiceTypeNotFound = errors.New("service type not found")
	ErrCounterNotFound     = errors.New("counter not found")
	ErrTicketNotFound      = errors.New("ticket not found")
	ErrDuplicateQueue      = errors.New("you already have an active ticket for this service")
	ErrNoWaitingTicket     = errors.New("no waiting tickets")
	ErrInvalidTicketStatus = errors.New("invalid ticket status for this action")
	ErrCounterNotOpen      = errors.New("counter is not open")
	ErrCounterAlreadyOpen  = errors.New("counter is already open")
)

// QueueService handles queue business logic
type QueueService struct {
	queueRepo *repositories.QueueRepository
}

// NewQueueService creates a new queue service
func NewQueueService(queueRepo *repositories.QueueRepository) *QueueService {
	return &QueueService{
		queueRepo: queueRepo,
	}
}

// ============================================================
// USER — Walk-in & Status
// ============================================================

// WalkinInput represents walk-in queue request
type WalkinInput struct {
	BranchID      uint `json:"branch_id" validate:"required"`
	ServiceTypeID uint `json:"service_type_id" validate:"required"`
}

// WalkinResponse represents walk-in queue response
type WalkinResponse struct {
	Ticket       *models.QueueTicket `json:"ticket"`
	QueueAhead   int64               `json:"queue_ahead"`
	EstimatedMin int                 `json:"estimated_min"`
}

// CreateWalkin creates a new walk-in queue ticket
func (s *QueueService) CreateWalkin(userID uint, input *WalkinInput) (*WalkinResponse, error) {
	// 1. Validate branch
	branch, err := s.queueRepo.GetBranchByID(input.BranchID)
	if err != nil {
		return nil, ErrBranchNotFound
	}
	if !branch.IsActive {
		return nil, ErrBranchClosed
	}

	// 2. Validate service type
	_, err = s.queueRepo.GetServiceTypeByID(input.ServiceTypeID)
	if err != nil {
		return nil, ErrServiceTypeNotFound
	}

	// 3. Check duplicate active ticket
	today := time.Now().Truncate(24 * time.Hour)
	existing, err := s.queueRepo.GetActiveTicketByUser(userID, input.BranchID, input.ServiceTypeID, today)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrDuplicateQueue
	}

	// 4. Generate ticket number
	ticketNumber, err := s.queueRepo.GetNextTicketNumber(input.BranchID, "WALKIN", today)
	if err != nil {
		return nil, err
	}

	// 5. Create ticket
	now := time.Now()
	ticket := &models.QueueTicket{
		TicketNumber:  ticketNumber,
		QueueType:     "WALKIN",
		QueueDate:     today,
		BranchID:      input.BranchID,
		ServiceTypeID: input.ServiceTypeID,
		UserID:        userID,
		Status:        "WAITING",
		IssuedAt:      now,
		Priority:      0, // TODO Phase 3+: check flommast for elderly → higher priority
	}

	if err := s.queueRepo.CreateTicket(ticket); err != nil {
		return nil, err
	}

	// 6. Count queue ahead
	ahead, _ := s.queueRepo.GetWaitingCount(input.BranchID, input.ServiceTypeID, today, now)

	// 7. Estimate wait time (avg 10 min per person from config, fallback)
	avgMin := 10
	configVal, err := s.queueRepo.GetConfigValue(input.BranchID, "avg_service_min")
	if err == nil {
		fmt.Sscanf(configVal, "%d", &avgMin)
	}
	estimatedMin := int(ahead) * avgMin

	// 8. Reload ticket with relations
	ticket, _ = s.queueRepo.GetTicketByID(ticket.ID)

	log.Printf("✅ Walk-in ticket created: %s (User: %d, Branch: %s)", ticketNumber, userID, branch.Code)

	return &WalkinResponse{
		Ticket:       ticket,
		QueueAhead:   ahead,
		EstimatedMin: estimatedMin,
	}, nil
}

// GetMyTicketsToday returns all tickets for a user today
func (s *QueueService) GetMyTicketsToday(userID uint) ([]models.QueueTicket, error) {
	today := time.Now().Truncate(24 * time.Hour)
	return s.queueRepo.GetMyTicketsToday(userID, today)
}

// GetTicketByID returns a ticket by ID
func (s *QueueService) GetTicketByID(ticketID uint) (*models.QueueTicket, error) {
	ticket, err := s.queueRepo.GetTicketByID(ticketID)
	if err != nil {
		return nil, ErrTicketNotFound
	}
	return ticket, nil
}

// TrackTicket returns ticket info + queue position by ticket number
func (s *QueueService) TrackTicket(ticketNumber string) (*WalkinResponse, error) {
	today := time.Now().Truncate(24 * time.Hour)
	ticket, err := s.queueRepo.GetTicketByNumber(ticketNumber, today)
	if err != nil {
		return nil, ErrTicketNotFound
	}

	var ahead int64
	if ticket.Status == "WAITING" {
		ahead, _ = s.queueRepo.GetWaitingCount(ticket.BranchID, ticket.ServiceTypeID, today, ticket.IssuedAt)
	}

	avgMin := 10
	configVal, err := s.queueRepo.GetConfigValue(ticket.BranchID, "avg_service_min")
	if err == nil {
		fmt.Sscanf(configVal, "%d", &avgMin)
	}

	return &WalkinResponse{
		Ticket:       ticket,
		QueueAhead:   ahead,
		EstimatedMin: int(ahead) * avgMin,
	}, nil
}

// GetBranches returns all active branches
func (s *QueueService) GetBranches() ([]models.Branch, error) {
	return s.queueRepo.GetActiveBranches()
}

// GetBranchServices returns service types + open counters for a branch
func (s *QueueService) GetBranchServices(branchID uint) (map[string]interface{}, error) {
	branch, err := s.queueRepo.GetBranchByID(branchID)
	if err != nil {
		return nil, ErrBranchNotFound
	}

	serviceTypes, _ := s.queueRepo.GetActiveServiceTypes()
	counters, _ := s.queueRepo.GetCountersByBranch(branchID)

	return map[string]interface{}{
		"branch":        branch,
		"service_types": serviceTypes,
		"counters":      counters,
	}, nil
}

// GetBranchStatus returns current queue status for a branch
func (s *QueueService) GetBranchStatus(branchID uint) (map[string]interface{}, error) {
	_, err := s.queueRepo.GetBranchByID(branchID)
	if err != nil {
		return nil, ErrBranchNotFound
	}

	today := time.Now().Truncate(24 * time.Hour)
	statusMap, _ := s.queueRepo.GetBranchQueueStatus(branchID, today)
	counters, _ := s.queueRepo.GetCountersByBranch(branchID)

	// Count open counters
	var openCounters int
	for _, c := range counters {
		if c.Status == "OPEN" {
			openCounters++
		}
	}

	return map[string]interface{}{
		"queue_date":    today.Format("2006-01-02"),
		"status":        statusMap,
		"total_waiting": statusMap["WAITING"],
		"total_serving": statusMap["SERVING"] + statusMap["CALLING"],
		"total_done":    statusMap["COMPLETED"],
		"open_counters": openCounters,
	}, nil
}

// ============================================================
// OFFICER/ADMIN — Counter Management
// ============================================================

// CounterActionInput represents counter open/close request
type CounterActionInput struct {
	CounterID uint `json:"counter_id" validate:"required"`
}

// OpenCounter opens a service counter and assigns staff
func (s *QueueService) OpenCounter(counterID uint, staffUserID uint) error {
	counter, err := s.queueRepo.GetCounterByID(counterID)
	if err != nil {
		return ErrCounterNotFound
	}
	if counter.Status == "OPEN" {
		return ErrCounterAlreadyOpen
	}

	err = s.queueRepo.UpdateCounterStatus(counterID, "OPEN", &staffUserID)
	if err != nil {
		return err
	}

	log.Printf("✅ Counter %d opened by staff %d", counterID, staffUserID)
	return nil
}

// CloseCounter closes a service counter
func (s *QueueService) CloseCounter(counterID uint) error {
	_, err := s.queueRepo.GetCounterByID(counterID)
	if err != nil {
		return ErrCounterNotFound
	}

	return s.queueRepo.UpdateCounterStatus(counterID, "CLOSED", nil)
}

// BreakCounter sets counter to break status
func (s *QueueService) BreakCounter(counterID uint) error {
	counter, err := s.queueRepo.GetCounterByID(counterID)
	if err != nil {
		return ErrCounterNotFound
	}
	if counter.Status != "OPEN" {
		return ErrCounterNotOpen
	}

	return s.queueRepo.UpdateCounterStatus(counterID, "BREAK", nil)
}

// ============================================================
// OFFICER/ADMIN — Call & Serve
// ============================================================

// CallNextTicket calls the next ticket in queue (auto-select based on priority)
func (s *QueueService) CallNextTicket(counterID uint, calledByUserID uint) (*models.QueueTicket, error) {
	// 1. Get counter info
	counter, err := s.queueRepo.GetCounterByID(counterID)
	if err != nil {
		return nil, ErrCounterNotFound
	}
	if counter.Status != "OPEN" {
		return nil, ErrCounterNotOpen
	}

	// 2. Check if counter already has an active ticket
	today := time.Now().Truncate(24 * time.Hour)
	serving, err := s.queueRepo.GetCurrentServingByCounter(counterID, today)
	if err != nil {
		return nil, err
	}
	if serving != nil {
		return nil, fmt.Errorf("counter still has active ticket: %s", serving.TicketNumber)
	}

	// 3. Find next ticket
	ticket, err := s.queueRepo.GetNextCallableTicket(counter.BranchID, counter.ServiceTypeID, today)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, ErrNoWaitingTicket
	}

	// 4. Update ticket → CALLING
	now := time.Now()
	updates := map[string]interface{}{
		"status":     "CALLING",
		"counter_id": counterID,
		"called_at":  now,
		"called_by":  calledByUserID,
	}
	if err := s.queueRepo.UpdateTicketStatus(ticket.ID, updates); err != nil {
		return nil, err
	}

	// Reload
	ticket, _ = s.queueRepo.GetTicketByID(ticket.ID)

	log.Printf("✅ Ticket %s called to counter %d by staff %d", ticket.TicketNumber, counterID, calledByUserID)
	return ticket, nil
}

// CallSpecificTicket calls a specific ticket by ID
func (s *QueueService) CallSpecificTicket(ticketID uint, counterID uint, calledByUserID uint) (*models.QueueTicket, error) {
	ticket, err := s.queueRepo.GetTicketByID(ticketID)
	if err != nil {
		return nil, ErrTicketNotFound
	}
	if ticket.Status != "WAITING" {
		return nil, ErrInvalidTicketStatus
	}

	counter, err := s.queueRepo.GetCounterByID(counterID)
	if err != nil {
		return nil, ErrCounterNotFound
	}
	if counter.Status != "OPEN" {
		return nil, ErrCounterNotOpen
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":     "CALLING",
		"counter_id": counterID,
		"called_at":  now,
		"called_by":  calledByUserID,
	}
	if err := s.queueRepo.UpdateTicketStatus(ticketID, updates); err != nil {
		return nil, err
	}

	ticket, _ = s.queueRepo.GetTicketByID(ticketID)
	log.Printf("✅ Ticket %s called (specific) to counter %d", ticket.TicketNumber, counterID)
	return ticket, nil
}

// RecallTicket re-calls a ticket that was already called
func (s *QueueService) RecallTicket(ticketID uint) (*models.QueueTicket, error) {
	ticket, err := s.queueRepo.GetTicketByID(ticketID)
	if err != nil {
		return nil, ErrTicketNotFound
	}
	if ticket.Status != "CALLING" {
		return nil, ErrInvalidTicketStatus
	}

	// Just update called_at to re-trigger notification
	now := time.Now()
	updates := map[string]interface{}{
		"called_at": now,
	}
	if err := s.queueRepo.UpdateTicketStatus(ticketID, updates); err != nil {
		return nil, err
	}

	ticket, _ = s.queueRepo.GetTicketByID(ticketID)
	log.Printf("✅ Ticket %s recalled", ticket.TicketNumber)
	return ticket, nil
}

// ServeTicket starts serving a called ticket
func (s *QueueService) ServeTicket(ticketID uint, servedByUserID uint) (*models.QueueTicket, error) {
	ticket, err := s.queueRepo.GetTicketByID(ticketID)
	if err != nil {
		return nil, ErrTicketNotFound
	}
	if ticket.Status != "CALLING" {
		return nil, ErrInvalidTicketStatus
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":     "SERVING",
		"serving_at": now,
		"served_by":  servedByUserID,
	}
	if err := s.queueRepo.UpdateTicketStatus(ticketID, updates); err != nil {
		return nil, err
	}

	ticket, _ = s.queueRepo.GetTicketByID(ticketID)
	log.Printf("✅ Ticket %s now serving", ticket.TicketNumber)
	return ticket, nil
}

// CompleteTicket marks a ticket as completed
func (s *QueueService) CompleteTicket(ticketID uint) (*models.QueueTicket, error) {
	ticket, err := s.queueRepo.GetTicketByID(ticketID)
	if err != nil {
		return nil, ErrTicketNotFound
	}
	if ticket.Status != "SERVING" && ticket.Status != "CALLING" {
		return nil, ErrInvalidTicketStatus
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":       "COMPLETED",
		"completed_at": now,
	}
	if err := s.queueRepo.UpdateTicketStatus(ticketID, updates); err != nil {
		return nil, err
	}

	ticket, _ = s.queueRepo.GetTicketByID(ticketID)
	log.Printf("✅ Ticket %s completed", ticket.TicketNumber)
	return ticket, nil
}

// SkipTicket skips a ticket (patient didn't show up)
func (s *QueueService) SkipTicket(ticketID uint) (*models.QueueTicket, error) {
	ticket, err := s.queueRepo.GetTicketByID(ticketID)
	if err != nil {
		return nil, ErrTicketNotFound
	}
	if ticket.Status != "CALLING" {
		return nil, ErrInvalidTicketStatus
	}

	now := time.Now()
	newSkipCount := ticket.SkipCount + 1

	// If skipped 3 times → auto cancel
	newStatus := "WAITING"
	if newSkipCount >= 3 {
		newStatus = "CANCELLED"
	}

	updates := map[string]interface{}{
		"status":       newStatus,
		"skip_count":   newSkipCount,
		"counter_id":   nil,
		"called_at":    nil,
		"called_by":    nil,
		"completed_at": nil,
	}
	if newStatus == "CANCELLED" {
		updates["completed_at"] = now
	}

	if err := s.queueRepo.UpdateTicketStatus(ticketID, updates); err != nil {
		return nil, err
	}

	ticket, _ = s.queueRepo.GetTicketByID(ticketID)
	log.Printf("✅ Ticket %s skipped (count: %d, new status: %s)", ticket.TicketNumber, newSkipCount, newStatus)
	return ticket, nil
}

// TransferTicket transfers a ticket to a different counter
func (s *QueueService) TransferTicket(ticketID uint, newCounterID uint) (*models.QueueTicket, error) {
	ticket, err := s.queueRepo.GetTicketByID(ticketID)
	if err != nil {
		return nil, ErrTicketNotFound
	}
	if ticket.Status != "WAITING" && ticket.Status != "CALLING" {
		return nil, ErrInvalidTicketStatus
	}

	newCounter, err := s.queueRepo.GetCounterByID(newCounterID)
	if err != nil {
		return nil, ErrCounterNotFound
	}

	updates := map[string]interface{}{
		"status":          "WAITING",
		"counter_id":      nil,
		"service_type_id": newCounter.ServiceTypeID,
		"called_at":       nil,
		"called_by":       nil,
	}
	if err := s.queueRepo.UpdateTicketStatus(ticketID, updates); err != nil {
		return nil, err
	}

	ticket, _ = s.queueRepo.GetTicketByID(ticketID)
	log.Printf("✅ Ticket %s transferred to counter %d", ticket.TicketNumber, newCounterID)
	return ticket, nil
}

// ============================================================
// OFFICER/ADMIN — Dashboard & History
// ============================================================

// DashboardResponse represents the admin queue dashboard
type DashboardResponse struct {
	QueueDate    string                 `json:"queue_date"`
	Status       map[string]int64       `json:"status"`
	WaitingList  []models.QueueTicket   `json:"waiting_list"`
	Counters     []models.ServiceCounter `json:"counters"`
	OpenCounters int                    `json:"open_counters"`
}

// GetAdminDashboard returns queue dashboard for a branch
func (s *QueueService) GetAdminDashboard(branchID uint) (*DashboardResponse, error) {
	_, err := s.queueRepo.GetBranchByID(branchID)
	if err != nil {
		return nil, ErrBranchNotFound
	}

	today := time.Now().Truncate(24 * time.Hour)
	statusMap, _ := s.queueRepo.GetBranchQueueStatus(branchID, today)
	waitingList, _ := s.queueRepo.GetWaitingTicketsByBranch(branchID, today)
	counters, _ := s.queueRepo.GetCountersByBranch(branchID)

	var openCounters int
	for _, c := range counters {
		if c.Status == "OPEN" {
			openCounters++
		}
	}

	return &DashboardResponse{
		QueueDate:    today.Format("2006-01-02"),
		Status:       statusMap,
		WaitingList:  waitingList,
		Counters:     counters,
		OpenCounters: openCounters,
	}, nil
}

// GetQueueHistory returns completed tickets for a branch today
func (s *QueueService) GetQueueHistory(branchID uint) ([]models.QueueTicket, error) {
	today := time.Now().Truncate(24 * time.Hour)
	return s.queueRepo.GetCompletedTicketsByBranch(branchID, today)
}

// ============================================================
// OFFICER/ADMIN — Config
// ============================================================

// GetConfig returns all config for a branch
func (s *QueueService) GetConfig(branchID uint) ([]models.QueueConfig, error) {
	return s.queueRepo.GetConfigByBranch(branchID)
}

// UpdateConfigInput represents config update request
type UpdateConfigInput struct {
	BranchID uint   `json:"branch_id" validate:"required"`
	Key      string `json:"key" validate:"required"`
	Value    string `json:"value" validate:"required"`
}

// UpdateConfig updates a config entry
func (s *QueueService) UpdateConfig(input *UpdateConfigInput) error {
	return s.queueRepo.UpdateConfig(input.BranchID, input.Key, input.Value)
}
