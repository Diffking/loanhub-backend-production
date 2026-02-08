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
	ErrSlotNotAvailable    = errors.New("booking slot not available")
	ErrSlotFull            = errors.New("booking slot is full")
	ErrDuplicateBooking    = errors.New("you already have an active booking for this service on this date")
	ErrBookingNotFound     = errors.New("booking not found")
	ErrBookingNotWaiting   = errors.New("booking is not in WAITING status")
)

// QueueService handles queue business logic
type QueueService struct {
	queueRepo     *repositories.QueueRepository
	notifyService *QueueNotifyService
}

// NewQueueService creates a new queue service
func NewQueueService(queueRepo *repositories.QueueRepository) *QueueService {
	return &QueueService{
		queueRepo: queueRepo,
	}
}

// SetNotifyService injects the notification service (called after both are created)
func (s *QueueService) SetNotifyService(ns *QueueNotifyService) {
	s.notifyService = ns
	log.Println("✅ QueueService: NotifyService injected")
}

// notify is a helper that safely sends notifications (nil-safe)
func (s *QueueService) notify() *QueueNotifyService {
	return s.notifyService
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

	// Phase 4: SSE broadcast — new ticket in queue
	if s.notify() != nil {
		s.notify().NotifyQueueUpdate(input.BranchID, "queue_update", map[string]interface{}{
			"action":        "new_walkin",
			"ticket_number": ticketNumber,
		})
	}

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

// GetBranchByID returns a branch by ID (public helper for display handler)
func (s *QueueService) GetBranchByID(branchID uint) (*models.Branch, error) {
	branch, err := s.queueRepo.GetBranchByID(branchID)
	if err != nil {
		return nil, ErrBranchNotFound
	}
	return branch, nil
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

	// Phase 4: SSE broadcast
	if s.notify() != nil {
		s.notify().NotifyQueueUpdate(counter.BranchID, "counter_update", map[string]interface{}{
			"action":       "counter_open",
			"counter_id":   counterID,
			"counter_name": counter.CounterName,
		})
	}
	return nil
}

// CloseCounter closes a service counter
func (s *QueueService) CloseCounter(counterID uint) error {
	counter, err := s.queueRepo.GetCounterByID(counterID)
	if err != nil {
		return ErrCounterNotFound
	}

	err = s.queueRepo.UpdateCounterStatus(counterID, "CLOSED", nil)
	if err != nil {
		return err
	}

	// Phase 4: SSE broadcast
	if s.notify() != nil {
		s.notify().NotifyQueueUpdate(counter.BranchID, "counter_update", map[string]interface{}{
			"action":       "counter_close",
			"counter_id":   counterID,
			"counter_name": counter.CounterName,
		})
	}
	return nil
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

	err = s.queueRepo.UpdateCounterStatus(counterID, "BREAK", nil)
	if err != nil {
		return err
	}

	// Phase 4: SSE broadcast
	if s.notify() != nil {
		s.notify().NotifyQueueUpdate(counter.BranchID, "counter_update", map[string]interface{}{
			"action":       "counter_break",
			"counter_id":   counterID,
			"counter_name": counter.CounterName,
		})
	}
	return nil
}

// ============================================================
// OFFICER/ADMIN — Call & Serve (with Phase 4 notifications)
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

	// Phase 4: Notify user + broadcast
	if s.notify() != nil {
		s.notify().NotifyTicketCalled(counter.BranchID, ticket.UserID, ticket.TicketNumber, counter.CounterName)
	}

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

	// Phase 4: Notify user + broadcast
	if s.notify() != nil {
		s.notify().NotifyTicketCalled(counter.BranchID, ticket.UserID, ticket.TicketNumber, counter.CounterName)
	}

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

	// Phase 4: Re-notify user
	if s.notify() != nil && ticket.Counter != nil {
		s.notify().NotifyTicketCalled(ticket.BranchID, ticket.UserID, ticket.TicketNumber, ticket.Counter.CounterName)
	}

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

	// Phase 4: SSE broadcast
	if s.notify() != nil {
		s.notify().NotifyQueueUpdate(ticket.BranchID, "queue_update", map[string]interface{}{
			"action":        "serve",
			"ticket_number": ticket.TicketNumber,
		})
	}

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

	// Phase 4: SSE broadcast
	if s.notify() != nil {
		s.notify().NotifyQueueUpdate(ticket.BranchID, "queue_update", map[string]interface{}{
			"action":        "complete",
			"ticket_number": ticket.TicketNumber,
		})
	}

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

	// Phase 4: SSE broadcast
	if s.notify() != nil {
		s.notify().NotifyQueueUpdate(ticket.BranchID, "queue_update", map[string]interface{}{
			"action":        "skip",
			"ticket_number": ticket.TicketNumber,
			"new_status":    newStatus,
		})
	}

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

	// Phase 4: SSE broadcast
	if s.notify() != nil {
		s.notify().NotifyQueueUpdate(ticket.BranchID, "queue_update", map[string]interface{}{
			"action":        "transfer",
			"ticket_number": ticket.TicketNumber,
		})
	}

	return ticket, nil
}

// ============================================================
// OFFICER/ADMIN — Dashboard & History
// ============================================================

// DashboardResponse represents the admin queue dashboard
type DashboardResponse struct {
	QueueDate    string                  `json:"queue_date"`
	Status       map[string]int64        `json:"status"`
	WaitingList  []models.QueueTicket    `json:"waiting_list"`
	Counters     []models.ServiceCounter `json:"counters"`
	OpenCounters int                     `json:"open_counters"`
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

// ============================================================
// Phase 5: Booking Online
// ============================================================

// BookingInput represents booking request
type BookingInput struct {
	BranchID      uint   `json:"branch_id" validate:"required"`
	ServiceTypeID uint   `json:"service_type_id" validate:"required"`
	SlotDate      string `json:"slot_date" validate:"required"` // YYYY-MM-DD
	SlotTime      string `json:"slot_time" validate:"required"` // HH:MM
	Note          string `json:"note"`
}

// CreateBooking creates a new booking ticket
func (s *QueueService) CreateBooking(userID uint, input *BookingInput) (*WalkinResponse, error) {
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

	// 3. Parse date
	slotDate, err := time.Parse("2006-01-02", input.SlotDate)
	if err != nil {
		return nil, fmt.Errorf("invalid date format, use YYYY-MM-DD")
	}

	// 4. Check duplicate booking
	existing, err := s.queueRepo.GetActiveBookingByUser(userID, input.BranchID, input.ServiceTypeID, slotDate)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrDuplicateBooking
	}

	// 5. Check slot availability
	slot, err := s.queueRepo.GetBookingSlot(input.BranchID, input.ServiceTypeID, slotDate, input.SlotTime)
	if err != nil {
		return nil, err
	}
	if slot == nil {
		return nil, ErrSlotNotAvailable
	}
	if !slot.IsAvailable || slot.CurrentBookings >= slot.MaxBookings {
		return nil, ErrSlotFull
	}

	// 6. Generate ticket number (B-series)
	ticketNumber, err := s.queueRepo.GetNextTicketNumber(input.BranchID, "BOOKING", slotDate)
	if err != nil {
		return nil, err
	}

	// 7. Create ticket
	now := time.Now()
	ticket := &models.QueueTicket{
		TicketNumber:  ticketNumber,
		QueueType:     "BOOKING",
		QueueDate:     slotDate,
		BranchID:      input.BranchID,
		ServiceTypeID: input.ServiceTypeID,
		UserID:        userID,
		Status:        "WAITING",
		BookingDate:   &slotDate,
		BookingSlot:   input.SlotTime,
		BookingNote:   input.Note,
		IssuedAt:      now,
		Priority:      10, // Booking priority higher than walk-in
	}

	if err := s.queueRepo.CreateTicket(ticket); err != nil {
		return nil, err
	}

	// 8. Increment slot bookings
	if err := s.queueRepo.IncrementBookingSlot(slot.ID); err != nil {
		log.Printf("⚠️ Failed to increment booking slot: %v", err)
	}

	// 9. Reload with relations
	ticket, _ = s.queueRepo.GetTicketByID(ticket.ID)

	log.Printf("✅ Booking ticket created: %s (User: %d, Branch: %s, Date: %s %s)",
		ticketNumber, userID, branch.Code, input.SlotDate, input.SlotTime)

	return &WalkinResponse{
		Ticket:       ticket,
		QueueAhead:   0,
		EstimatedMin: 0,
	}, nil
}

// CancelBooking cancels a booking ticket
func (s *QueueService) CancelBooking(ticketID uint, userID uint) error {
	ticket, err := s.queueRepo.GetTicketByID(ticketID)
	if err != nil {
		return ErrBookingNotFound
	}

	// Verify ownership
	if ticket.UserID != userID {
		return ErrBookingNotFound
	}
	if ticket.QueueType != "BOOKING" {
		return ErrBookingNotFound
	}
	if ticket.Status != "WAITING" {
		return ErrBookingNotWaiting
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":       "CANCELLED",
		"completed_at": now,
	}
	if err := s.queueRepo.UpdateTicketStatus(ticketID, updates); err != nil {
		return err
	}

	// Decrement slot
	if ticket.BookingDate != nil && ticket.BookingSlot != "" {
		s.queueRepo.DecrementBookingSlot(ticket.BranchID, ticket.ServiceTypeID, *ticket.BookingDate, ticket.BookingSlot)
	}

	log.Printf("✅ Booking %s cancelled by user %d", ticket.TicketNumber, userID)
	return nil
}

// GetAvailableSlots returns available booking slots
func (s *QueueService) GetAvailableSlots(branchID uint, serviceTypeID uint, dateStr string) ([]models.BookingSlot, error) {
	slotDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date format, use YYYY-MM-DD")
	}
	return s.queueRepo.GetAvailableSlots(branchID, serviceTypeID, slotDate)
}

// CheckinBooking checks in a booking ticket (admin action)
func (s *QueueService) CheckinBooking(ticketID uint) (*models.QueueTicket, error) {
	ticket, err := s.queueRepo.GetTicketByID(ticketID)
	if err != nil {
		return nil, ErrBookingNotFound
	}
	if ticket.QueueType != "BOOKING" {
		return nil, ErrBookingNotFound
	}
	if ticket.Status != "WAITING" {
		return nil, ErrBookingNotWaiting
	}

	// Check-in: keep WAITING but set priority high so it gets called next
	updates := map[string]interface{}{
		"priority": 10,
	}
	if err := s.queueRepo.UpdateTicketStatus(ticketID, updates); err != nil {
		return nil, err
	}

	ticket, _ = s.queueRepo.GetTicketByID(ticketID)
	log.Printf("✅ Booking %s checked in (priority boosted)", ticket.TicketNumber)

	// Phase 4: SSE broadcast
	if s.notify() != nil {
		s.notify().NotifyQueueUpdate(ticket.BranchID, "queue_update", map[string]interface{}{
			"action":        "booking_checkin",
			"ticket_number": ticket.TicketNumber,
		})
	}

	return ticket, nil
}

// GetBookingsByBranch returns all booking tickets for a branch today (admin)
func (s *QueueService) GetBookingsByBranch(branchID uint) ([]models.QueueTicket, error) {
	today := time.Now().Truncate(24 * time.Hour)
	return s.queueRepo.GetBookingsByBranch(branchID, today)
}

// GenerateBookingSlots creates booking slots for a date range
type GenerateSlotsInput struct {
	BranchID      uint   `json:"branch_id" validate:"required"`
	ServiceTypeID uint   `json:"service_type_id" validate:"required"`
	StartDate     string `json:"start_date" validate:"required"` // YYYY-MM-DD
	EndDate       string `json:"end_date" validate:"required"`   // YYYY-MM-DD
}

func (s *QueueService) GenerateBookingSlots(input *GenerateSlotsInput) (int, error) {
	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		return 0, fmt.Errorf("invalid start_date format")
	}
	endDate, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		return 0, fmt.Errorf("invalid end_date format")
	}
	if endDate.Before(startDate) {
		return 0, fmt.Errorf("end_date must be after start_date")
	}

	// Get config values
	maxPerSlot := 5
	configVal, err := s.queueRepo.GetConfigValue(input.BranchID, "max_booking_per_slot")
	if err == nil {
		fmt.Sscanf(configVal, "%d", &maxPerSlot)
	}

	// Default time slots: every 30 min from 08:30 to 15:30
	timeSlots := []string{
		"08:30", "09:00", "09:30", "10:00", "10:30",
		"11:00", "11:30", "13:00", "13:30", "14:00",
		"14:30", "15:00", "15:30",
	}

	var slots []models.BookingSlot
	current := startDate
	for !current.After(endDate) {
		// Skip weekends (Saturday=6, Sunday=0)
		if current.Weekday() != time.Saturday && current.Weekday() != time.Sunday {
			for _, t := range timeSlots {
				// Check if slot already exists
				existing, _ := s.queueRepo.GetBookingSlot(input.BranchID, input.ServiceTypeID, current, t)
				if existing != nil {
					continue // Skip existing slots
				}
				slots = append(slots, models.BookingSlot{
					BranchID:        input.BranchID,
					ServiceTypeID:   input.ServiceTypeID,
					SlotDate:        current,
					SlotTime:        t,
					MaxBookings:     maxPerSlot,
					CurrentBookings: 0,
					IsAvailable:     true,
				})
			}
		}
		current = current.AddDate(0, 0, 1)
	}

	if len(slots) == 0 {
		return 0, nil
	}

	if err := s.queueRepo.GenerateBookingSlots(slots); err != nil {
		return 0, err
	}

	log.Printf("✅ Generated %d booking slots for branch %d, service %d (%s to %s)",
		len(slots), input.BranchID, input.ServiceTypeID, input.StartDate, input.EndDate)
	return len(slots), nil
}

// ============================================================
// Phase 6: Display Data (for TV screens)
// ============================================================

// DisplayData represents the TV display data structure
type DisplayData struct {
	Branch         *models.Branch   `json:"branch"`
	CurrentCalling []DisplayTicket  `json:"current_calling"`
	WaitingCount   int64            `json:"waiting_count"`
	CompletedCount int64            `json:"completed_count"`
	Counters       []DisplayCounter `json:"counters"`
}

// DisplayTicket represents a ticket on the display
type DisplayTicket struct {
	TicketNumber string `json:"ticket_number"`
	CounterName  string `json:"counter_name"`
	ServiceType  string `json:"service_type"`
}

// DisplayCounter represents a counter on the display
type DisplayCounter struct {
	CounterName   string `json:"counter_name"`
	Status        string `json:"status"`
	CurrentTicket string `json:"current_ticket"`
}

// GetDisplayData returns data for the TV display
func (s *QueueService) GetDisplayData(branchID uint) (*DisplayData, error) {
	branch, err := s.queueRepo.GetBranchByID(branchID)
	if err != nil {
		return nil, ErrBranchNotFound
	}

	today := time.Now().Truncate(24 * time.Hour)

	// Get currently calling/serving tickets
	callingTickets, _ := s.queueRepo.GetCurrentCallingTickets(branchID, today)
	var currentCalling []DisplayTicket
	for _, t := range callingTickets {
		counterName := ""
		if t.Counter != nil {
			counterName = t.Counter.CounterName
		}
		serviceType := ""
		if t.ServiceType.Code != "" {
			serviceType = t.ServiceType.Code
		}
		currentCalling = append(currentCalling, DisplayTicket{
			TicketNumber: t.TicketNumber,
			CounterName:  counterName,
			ServiceType:  serviceType,
		})
	}

	// Get status counts
	statusMap, _ := s.queueRepo.GetBranchQueueStatus(branchID, today)

	// Get counters
	counters, _ := s.queueRepo.GetCountersByBranch(branchID)
	var displayCounters []DisplayCounter
	for _, c := range counters {
		currentTicket := ""
		// Find if counter has an active ticket
		serving, _ := s.queueRepo.GetCurrentServingByCounter(c.ID, today)
		if serving != nil {
			currentTicket = serving.TicketNumber
		}
		displayCounters = append(displayCounters, DisplayCounter{
			CounterName:   c.CounterName,
			Status:        c.Status,
			CurrentTicket: currentTicket,
		})
	}

	return &DisplayData{
		Branch:         branch,
		CurrentCalling: currentCalling,
		WaitingCount:   statusMap["WAITING"],
		CompletedCount: statusMap["COMPLETED"],
		Counters:       displayCounters,
	}, nil
}
