package handlers

import (
	"bufio"
	"fmt"
	"log"
	"strconv"
	"time"

	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// QueueHandler handles user-facing queue endpoints
type QueueHandler struct {
	queueService  *services.QueueService
	notifyService *services.QueueNotifyService
}

// NewQueueHandler creates a new queue handler
func NewQueueHandler(queueService *services.QueueService, notifyService *services.QueueNotifyService) *QueueHandler {
	return &QueueHandler{
		queueService:  queueService,
		notifyService: notifyService,
	}
}

// ============================================================
// GET /api/v1/queue/branches — ดูจุดบริการทั้งหมด
// ============================================================
func (h *QueueHandler) GetBranches(c *fiber.Ctx) error {
	branches, err := h.queueService.GetBranches()
	if err != nil {
		return response.InternalServerError(c, "Failed to get branches")
	}
	return response.Success(c, "Branches retrieved", branches)
}

// ============================================================
// GET /api/v1/queue/branches/:id/services — ดูบริการ + ช่องที่เปิด
// ============================================================
func (h *QueueHandler) GetBranchServices(c *fiber.Ctx) error {
	branchID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid branch ID")
	}

	result, err := h.queueService.GetBranchServices(uint(branchID))
	if err != nil {
		if err == services.ErrBranchNotFound {
			return response.NotFound(c, "Branch not found")
		}
		return response.InternalServerError(c, "Failed to get branch services")
	}
	return response.Success(c, "Branch services retrieved", result)
}

// ============================================================
// GET /api/v1/queue/branches/:id/status — สถานะคิวปัจจุบัน
// ============================================================
func (h *QueueHandler) GetBranchStatus(c *fiber.Ctx) error {
	branchID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid branch ID")
	}

	result, err := h.queueService.GetBranchStatus(uint(branchID))
	if err != nil {
		if err == services.ErrBranchNotFound {
			return response.NotFound(c, "Branch not found")
		}
		return response.InternalServerError(c, "Failed to get branch status")
	}
	return response.Success(c, "Branch status retrieved", result)
}

// ============================================================
// POST /api/v1/queue/walkin — กดคิว Walk-in
// ============================================================
func (h *QueueHandler) CreateWalkin(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "User not authenticated")
	}

	var input services.WalkinInput
	if err := c.BodyParser(&input); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if input.BranchID == 0 || input.ServiceTypeID == 0 {
		return response.BadRequest(c, "branch_id and service_type_id are required")
	}

	result, err := h.queueService.CreateWalkin(userID, &input)
	if err != nil {
		switch err {
		case services.ErrBranchNotFound:
			return response.NotFound(c, "Branch not found")
		case services.ErrBranchClosed:
			return response.BadRequest(c, "Branch is not active")
		case services.ErrServiceTypeNotFound:
			return response.NotFound(c, "Service type not found")
		case services.ErrDuplicateQueue:
			return response.Conflict(c, err.Error())
		default:
			return response.InternalServerError(c, "Failed to create walk-in ticket")
		}
	}
	return response.Created(c, "Walk-in ticket created", result)
}

// ============================================================
// GET /api/v1/queue/my-tickets — คิวของฉันวันนี้
// ============================================================
func (h *QueueHandler) GetMyTickets(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "User not authenticated")
	}

	tickets, err := h.queueService.GetMyTicketsToday(userID)
	if err != nil {
		return response.InternalServerError(c, "Failed to get tickets")
	}
	return response.Success(c, "My tickets retrieved", tickets)
}

// ============================================================
// GET /api/v1/queue/my-tickets/:id — รายละเอียดคิว
// ============================================================
func (h *QueueHandler) GetMyTicketByID(c *fiber.Ctx) error {
	ticketID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ticket ID")
	}

	ticket, err := h.queueService.GetTicketByID(uint(ticketID))
	if err != nil {
		return response.NotFound(c, "Ticket not found")
	}
	return response.Success(c, "Ticket retrieved", ticket)
}

// ============================================================
// GET /api/v1/queue/track/:ticket_number — ติดตามจากเลขคิว
// ============================================================
func (h *QueueHandler) TrackTicket(c *fiber.Ctx) error {
	ticketNumber := c.Params("ticket_number")
	if ticketNumber == "" {
		return response.BadRequest(c, "Ticket number is required")
	}

	result, err := h.queueService.TrackTicket(ticketNumber)
	if err != nil {
		if err == services.ErrTicketNotFound {
			return response.NotFound(c, "Ticket not found")
		}
		return response.InternalServerError(c, "Failed to track ticket")
	}
	return response.Success(c, "Ticket tracked", result)
}

// ============================================================
// Phase 4: GET /api/v1/queue/events — SSE real-time สำหรับ user
// ============================================================
func (h *QueueHandler) SSEEvents(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Get branch_id from query (required for filtering events)
	branchID, _ := strconv.ParseUint(c.Query("branch_id", "0"), 10, 32)
	if branchID == 0 {
		return response.BadRequest(c, "branch_id query parameter is required")
	}

	clientID := fmt.Sprintf("user-%d-%d", userID, time.Now().UnixNano())

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("Access-Control-Allow-Origin", "*")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		client := &services.SSEClient{
			ID:       clientID,
			UserID:   userID,
			BranchID: uint(branchID),
			Channel:  make(chan services.SSEEvent, 50),
			IsTV:     false,
		}

		h.notifyService.Hub.Register(client)
		defer h.notifyService.Hub.Unregister(clientID)

		// Send initial connection event
		fmt.Fprintf(w, "event: connected\ndata: {\"client_id\":\"%s\",\"user_id\":%d}\n\n", clientID, userID)
		w.Flush()

		// Heartbeat ticker
		heartbeat := time.NewTicker(30 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case event, ok := <-client.Channel:
				if !ok {
					return
				}
				writeSSEEventUser(w, event)
				w.Flush()

			case <-heartbeat.C:
				fmt.Fprintf(w, ": heartbeat\n\n")
				if err := w.Flush(); err != nil {
					log.Printf("📡 User SSE client disconnected: %s", clientID)
					return
				}
			}
		}
	})

	return nil
}

// writeSSEEventUser writes a formatted SSE event for user stream
func writeSSEEventUser(w *bufio.Writer, event services.SSEEvent) {
	fmt.Fprintf(w, "event: %s\n", event.Event)

	switch data := event.Data.(type) {
	case map[string]interface{}:
		jsonStr := "{"
		first := true
		for k, v := range data {
			if !first {
				jsonStr += ","
			}
			switch val := v.(type) {
			case string:
				jsonStr += fmt.Sprintf(`"%s":"%s"`, k, val)
			case int:
				jsonStr += fmt.Sprintf(`"%s":%d`, k, val)
			case int64:
				jsonStr += fmt.Sprintf(`"%s":%d`, k, val)
			case uint:
				jsonStr += fmt.Sprintf(`"%s":%d`, k, val)
			case float64:
				jsonStr += fmt.Sprintf(`"%s":%f`, k, val)
			case bool:
				jsonStr += fmt.Sprintf(`"%s":%v`, k, val)
			default:
				jsonStr += fmt.Sprintf(`"%s":"%v"`, k, val)
			}
			first = false
		}
		jsonStr += "}"
		fmt.Fprintf(w, "data: %s\n\n", jsonStr)
	default:
		fmt.Fprintf(w, "data: %v\n\n", data)
	}
}

// ============================================================
// Phase 5: Booking endpoints สำหรับ User
// ============================================================

// GET /api/v1/queue/booking/slots?branch_id=X&service_type_id=X&date=X
func (h *QueueHandler) GetBookingSlots(c *fiber.Ctx) error {
	branchID, _ := strconv.ParseUint(c.Query("branch_id", "0"), 10, 32)
	serviceTypeID, _ := strconv.ParseUint(c.Query("service_type_id", "0"), 10, 32)
	dateStr := c.Query("date")

	if branchID == 0 || serviceTypeID == 0 || dateStr == "" {
		return response.BadRequest(c, "branch_id, service_type_id, and date are required")
	}

	slots, err := h.queueService.GetAvailableSlots(uint(branchID), uint(serviceTypeID), dateStr)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Available slots retrieved", slots)
}

// POST /api/v1/queue/booking — จองล่วงหน้า
func (h *QueueHandler) CreateBooking(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "User not authenticated")
	}

	var input services.BookingInput
	if err := c.BodyParser(&input); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if input.BranchID == 0 || input.ServiceTypeID == 0 || input.SlotDate == "" || input.SlotTime == "" {
		return response.BadRequest(c, "branch_id, service_type_id, slot_date, and slot_time are required")
	}

	result, err := h.queueService.CreateBooking(userID, &input)
	if err != nil {
		switch err {
		case services.ErrBranchNotFound:
			return response.NotFound(c, "Branch not found")
		case services.ErrBranchClosed:
			return response.BadRequest(c, "Branch is not active")
		case services.ErrServiceTypeNotFound:
			return response.NotFound(c, "Service type not found")
		case services.ErrSlotNotAvailable:
			return response.NotFound(c, "Booking slot not available")
		case services.ErrSlotFull:
			return response.Conflict(c, "Booking slot is full")
		case services.ErrDuplicateBooking:
			return response.Conflict(c, err.Error())
		default:
			return response.BadRequest(c, err.Error())
		}
	}
	return response.Created(c, "Booking created", result)
}

// DELETE /api/v1/queue/booking/:id — ยกเลิกจอง
func (h *QueueHandler) CancelBooking(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "User not authenticated")
	}

	ticketID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid booking ID")
	}

	if err := h.queueService.CancelBooking(uint(ticketID), userID); err != nil {
		switch err {
		case services.ErrBookingNotFound:
			return response.NotFound(c, "Booking not found")
		case services.ErrBookingNotWaiting:
			return response.BadRequest(c, "Booking is not in WAITING status")
		default:
			return response.InternalServerError(c, "Failed to cancel booking")
		}
	}
	return response.Success(c, "Booking cancelled", nil)
}
