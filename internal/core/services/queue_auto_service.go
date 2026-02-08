package services

import (
	"log"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/repositories"
)

// ============================================================
// Phase 4: Auto-cancel booking + Reminder cron
// ============================================================

// QueueAutoService runs background goroutines for queue automation
type QueueAutoService struct {
	queueRepo     *repositories.QueueRepository
	notifyService *QueueNotifyService
	stopChan      chan struct{}
}

// NewQueueAutoService creates a new auto service
func NewQueueAutoService(queueRepo *repositories.QueueRepository, notifyService *QueueNotifyService) *QueueAutoService {
	return &QueueAutoService{
		queueRepo:     queueRepo,
		notifyService: notifyService,
		stopChan:      make(chan struct{}),
	}
}

// Start launches all background goroutines
func (s *QueueAutoService) Start() {
	log.Println("🚀 QueueAutoService started")

	// Auto-cancel: check every 5 minutes
	go s.runAutoCancelLoop()

	// Daily reminder: check every 30 minutes (fires at ~18:00)
	go s.runReminderLoop()

	// Nearly-turn check: every 30 seconds
	go s.runNearlyTurnLoop()
}

// Stop gracefully stops all goroutines
func (s *QueueAutoService) Stop() {
	close(s.stopChan)
	log.Println("🛑 QueueAutoService stopped")
}

// ============================================================
// Auto-cancel: Booking ที่เลย 30 นาทีไม่ check-in → CANCELLED
// ============================================================

func (s *QueueAutoService) runAutoCancelLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.autoCancelExpiredBookings()
		case <-s.stopChan:
			return
		}
	}
}

func (s *QueueAutoService) autoCancelExpiredBookings() {
	today := time.Now().Truncate(24 * time.Hour)
	now := time.Now()

	// Find BOOKING tickets that are WAITING + booking_date = today + slot_time + 30min < now
	tickets, err := s.queueRepo.GetExpiredBookingTickets(today, now, 30)
	if err != nil {
		log.Printf("❌ Auto-cancel query error: %v", err)
		return
	}

	for _, ticket := range tickets {
		updates := map[string]interface{}{
			"status":       "CANCELLED",
			"completed_at": now,
		}
		if err := s.queueRepo.UpdateTicketStatus(ticket.ID, updates); err != nil {
			log.Printf("❌ Auto-cancel ticket %s error: %v", ticket.TicketNumber, err)
			continue
		}

		// Update booking slot: decrement current_bookings
		if ticket.BookingDate != nil && ticket.BookingSlot != "" {
			s.queueRepo.DecrementBookingSlot(ticket.BranchID, ticket.ServiceTypeID, *ticket.BookingDate, ticket.BookingSlot)
		}

		// Notify user
		s.notifyService.NotifyBookingCancelled(ticket.UserID, ticket.TicketNumber)

		// Notify branch (queue update)
		s.notifyService.NotifyQueueUpdate(ticket.BranchID, "queue_update", map[string]interface{}{
			"action":        "auto_cancel",
			"ticket_number": ticket.TicketNumber,
		})

		log.Printf("🗑️ Auto-cancelled booking ticket: %s (no check-in after 30min)", ticket.TicketNumber)
	}

	if len(tickets) > 0 {
		log.Printf("🗑️ Auto-cancelled %d expired booking tickets", len(tickets))
	}
}

// ============================================================
// Reminder: Booking พรุ่งนี้ → LINE Notify เวลา ~18:00
// ============================================================

func (s *QueueAutoService) runReminderLoop() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	var lastReminderDate string

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			todayStr := now.Format("2006-01-02")
			hour := now.Hour()

			// Send reminder once per day at 18:00 (between 17:30 - 18:30)
			if hour == 18 && lastReminderDate != todayStr {
				s.sendBookingReminders()
				lastReminderDate = todayStr
			}
		case <-s.stopChan:
			return
		}
	}
}

func (s *QueueAutoService) sendBookingReminders() {
	tomorrow := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)

	tickets, err := s.queueRepo.GetBookingTicketsForDate(tomorrow)
	if err != nil {
		log.Printf("❌ Reminder query error: %v", err)
		return
	}

	for _, ticket := range tickets {
		if ticket.Status != "WAITING" {
			continue
		}

		branchName := ""
		if ticket.Branch.Name != "" {
			branchName = ticket.Branch.Name
		}
		slotDate := ""
		if ticket.BookingDate != nil {
			slotDate = ticket.BookingDate.Format("02/01/2006")
		}

		s.notifyService.NotifyBookingReminder(
			ticket.TicketNumber,
			branchName,
			slotDate,
			ticket.BookingSlot,
		)
	}

	if len(tickets) > 0 {
		log.Printf("📅 Sent %d booking reminders for tomorrow", len(tickets))
	}
}

// ============================================================
// Nearly-turn: เหลืออีก ~3 คิว → แจ้ง user
// ============================================================

func (s *QueueAutoService) runNearlyTurnLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkNearlyTurn()
		case <-s.stopChan:
			return
		}
	}
}

func (s *QueueAutoService) checkNearlyTurn() {
	today := time.Now().Truncate(24 * time.Hour)

	// Get threshold from config (default 3)
	nearlyThreshold := int64(3)

	// Get all WAITING tickets that haven't been notified
	tickets, err := s.queueRepo.GetUnnotifiedWaitingTickets(today)
	if err != nil {
		return
	}

	for _, ticket := range tickets {
		// Count how many ahead
		ahead, err := s.queueRepo.GetWaitingCount(ticket.BranchID, ticket.ServiceTypeID, today, ticket.IssuedAt)
		if err != nil {
			continue
		}

		if ahead <= nearlyThreshold && ahead > 0 {
			// Send notification
			s.notifyService.NotifyNearlyTurn(ticket.UserID, ticket.TicketNumber, ahead)

			// Mark as notified
			s.queueRepo.UpdateTicketStatus(ticket.ID, map[string]interface{}{
				"notify_sent": true,
			})
		}
	}
}
