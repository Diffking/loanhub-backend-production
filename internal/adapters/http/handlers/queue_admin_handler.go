package handlers

import (
	"strconv"

	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// QueueAdminHandler handles officer/admin queue endpoints
type QueueAdminHandler struct {
	queueService *services.QueueService
}

// NewQueueAdminHandler creates a new queue admin handler
func NewQueueAdminHandler(queueService *services.QueueService) *QueueAdminHandler {
	return &QueueAdminHandler{
		queueService: queueService,
	}
}

// ============================================================
// Counter Management
// ============================================================

// counterRequest is used for counter open/close/break
type counterRequest struct {
	CounterID uint `json:"counter_id"`
}

// POST /api/v1/admin/queue/counter/open — เปิดช่อง
func (h *QueueAdminHandler) OpenCounter(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "User not authenticated")
	}

	var req counterRequest
	if err := c.BodyParser(&req); err != nil || req.CounterID == 0 {
		return response.BadRequest(c, "counter_id is required")
	}

	if err := h.queueService.OpenCounter(req.CounterID, userID); err != nil {
		switch err {
		case services.ErrCounterNotFound:
			return response.NotFound(c, "Counter not found")
		case services.ErrCounterAlreadyOpen:
			return response.Conflict(c, "Counter is already open")
		default:
			return response.InternalServerError(c, "Failed to open counter")
		}
	}
	return response.Success(c, "Counter opened", nil)
}

// POST /api/v1/admin/queue/counter/close — ปิดช่อง
func (h *QueueAdminHandler) CloseCounter(c *fiber.Ctx) error {
	var req counterRequest
	if err := c.BodyParser(&req); err != nil || req.CounterID == 0 {
		return response.BadRequest(c, "counter_id is required")
	}

	if err := h.queueService.CloseCounter(req.CounterID); err != nil {
		if err == services.ErrCounterNotFound {
			return response.NotFound(c, "Counter not found")
		}
		return response.InternalServerError(c, "Failed to close counter")
	}
	return response.Success(c, "Counter closed", nil)
}

// POST /api/v1/admin/queue/counter/break — พักช่อง
func (h *QueueAdminHandler) BreakCounter(c *fiber.Ctx) error {
	var req counterRequest
	if err := c.BodyParser(&req); err != nil || req.CounterID == 0 {
		return response.BadRequest(c, "counter_id is required")
	}

	if err := h.queueService.BreakCounter(req.CounterID); err != nil {
		switch err {
		case services.ErrCounterNotFound:
			return response.NotFound(c, "Counter not found")
		case services.ErrCounterNotOpen:
			return response.BadRequest(c, "Counter is not open")
		default:
			return response.InternalServerError(c, "Failed to set counter to break")
		}
	}
	return response.Success(c, "Counter set to break", nil)
}

// ============================================================
// Call & Serve
// ============================================================

// callNextRequest is used for call-next
type callNextRequest struct {
	CounterID uint `json:"counter_id"`
}

// POST /api/v1/admin/queue/call-next — เรียกคิวถัดไป (auto-select)
func (h *QueueAdminHandler) CallNext(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "User not authenticated")
	}

	var req callNextRequest
	if err := c.BodyParser(&req); err != nil || req.CounterID == 0 {
		return response.BadRequest(c, "counter_id is required")
	}

	ticket, err := h.queueService.CallNextTicket(req.CounterID, userID)
	if err != nil {
		switch err {
		case services.ErrCounterNotFound:
			return response.NotFound(c, "Counter not found")
		case services.ErrCounterNotOpen:
			return response.BadRequest(c, "Counter is not open")
		case services.ErrNoWaitingTicket:
			return response.NotFound(c, "No waiting tickets")
		default:
			return response.BadRequest(c, err.Error())
		}
	}
	return response.Success(c, "Ticket called", ticket)
}

// callSpecificRequest is used for call-specific
type callSpecificRequest struct {
	CounterID uint `json:"counter_id"`
}

// POST /api/v1/admin/queue/call/:id — เรียกคิวเจาะจง
func (h *QueueAdminHandler) CallSpecific(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "User not authenticated")
	}

	ticketID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ticket ID")
	}

	var req callSpecificRequest
	if err := c.BodyParser(&req); err != nil || req.CounterID == 0 {
		return response.BadRequest(c, "counter_id is required")
	}

	ticket, err := h.queueService.CallSpecificTicket(uint(ticketID), req.CounterID, userID)
	if err != nil {
		switch err {
		case services.ErrTicketNotFound:
			return response.NotFound(c, "Ticket not found")
		case services.ErrInvalidTicketStatus:
			return response.BadRequest(c, "Ticket is not in WAITING status")
		case services.ErrCounterNotFound:
			return response.NotFound(c, "Counter not found")
		case services.ErrCounterNotOpen:
			return response.BadRequest(c, "Counter is not open")
		default:
			return response.InternalServerError(c, "Failed to call ticket")
		}
	}
	return response.Success(c, "Ticket called", ticket)
}

// POST /api/v1/admin/queue/recall/:id — เรียกซ้ำ
func (h *QueueAdminHandler) Recall(c *fiber.Ctx) error {
	ticketID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ticket ID")
	}

	ticket, err := h.queueService.RecallTicket(uint(ticketID))
	if err != nil {
		switch err {
		case services.ErrTicketNotFound:
			return response.NotFound(c, "Ticket not found")
		case services.ErrInvalidTicketStatus:
			return response.BadRequest(c, "Ticket is not in CALLING status")
		default:
			return response.InternalServerError(c, "Failed to recall ticket")
		}
	}
	return response.Success(c, "Ticket recalled", ticket)
}

// POST /api/v1/admin/queue/serve/:id — เริ่มให้บริการ
func (h *QueueAdminHandler) Serve(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "User not authenticated")
	}

	ticketID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ticket ID")
	}

	ticket, err := h.queueService.ServeTicket(uint(ticketID), userID)
	if err != nil {
		switch err {
		case services.ErrTicketNotFound:
			return response.NotFound(c, "Ticket not found")
		case services.ErrInvalidTicketStatus:
			return response.BadRequest(c, "Ticket is not in CALLING status")
		default:
			return response.InternalServerError(c, "Failed to serve ticket")
		}
	}
	return response.Success(c, "Now serving", ticket)
}

// POST /api/v1/admin/queue/complete/:id — เสร็จสิ้น
func (h *QueueAdminHandler) Complete(c *fiber.Ctx) error {
	ticketID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ticket ID")
	}

	ticket, err := h.queueService.CompleteTicket(uint(ticketID))
	if err != nil {
		switch err {
		case services.ErrTicketNotFound:
			return response.NotFound(c, "Ticket not found")
		case services.ErrInvalidTicketStatus:
			return response.BadRequest(c, "Ticket is not in SERVING or CALLING status")
		default:
			return response.InternalServerError(c, "Failed to complete ticket")
		}
	}
	return response.Success(c, "Ticket completed", ticket)
}

// POST /api/v1/admin/queue/skip/:id — ข้ามคิว
func (h *QueueAdminHandler) Skip(c *fiber.Ctx) error {
	ticketID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ticket ID")
	}

	ticket, err := h.queueService.SkipTicket(uint(ticketID))
	if err != nil {
		switch err {
		case services.ErrTicketNotFound:
			return response.NotFound(c, "Ticket not found")
		case services.ErrInvalidTicketStatus:
			return response.BadRequest(c, "Ticket is not in CALLING status")
		default:
			return response.InternalServerError(c, "Failed to skip ticket")
		}
	}
	return response.Success(c, "Ticket skipped", ticket)
}

// transferRequest is used for transfer
type transferRequest struct {
	NewCounterID uint `json:"new_counter_id"`
}

// POST /api/v1/admin/queue/transfer/:id — โอนคิวไปช่องอื่น
func (h *QueueAdminHandler) Transfer(c *fiber.Ctx) error {
	ticketID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ticket ID")
	}

	var req transferRequest
	if err := c.BodyParser(&req); err != nil || req.NewCounterID == 0 {
		return response.BadRequest(c, "new_counter_id is required")
	}

	ticket, err := h.queueService.TransferTicket(uint(ticketID), req.NewCounterID)
	if err != nil {
		switch err {
		case services.ErrTicketNotFound:
			return response.NotFound(c, "Ticket not found")
		case services.ErrInvalidTicketStatus:
			return response.BadRequest(c, "Ticket cannot be transferred in current status")
		case services.ErrCounterNotFound:
			return response.NotFound(c, "Target counter not found")
		default:
			return response.InternalServerError(c, "Failed to transfer ticket")
		}
	}
	return response.Success(c, "Ticket transferred", ticket)
}

// ============================================================
// Dashboard & History
// ============================================================

// GET /api/v1/admin/queue/dashboard?branch_id=X — สรุปคิวทั้งหมด
func (h *QueueAdminHandler) Dashboard(c *fiber.Ctx) error {
	branchID, err := strconv.ParseUint(c.Query("branch_id", "0"), 10, 32)
	if err != nil || branchID == 0 {
		return response.BadRequest(c, "branch_id query parameter is required")
	}

	result, err := h.queueService.GetAdminDashboard(uint(branchID))
	if err != nil {
		if err == services.ErrBranchNotFound {
			return response.NotFound(c, "Branch not found")
		}
		return response.InternalServerError(c, "Failed to get dashboard")
	}
	return response.Success(c, "Dashboard retrieved", result)
}

// GET /api/v1/admin/queue/history?branch_id=X — ประวัติคิว
func (h *QueueAdminHandler) History(c *fiber.Ctx) error {
	branchID, err := strconv.ParseUint(c.Query("branch_id", "0"), 10, 32)
	if err != nil || branchID == 0 {
		return response.BadRequest(c, "branch_id query parameter is required")
	}

	tickets, err := h.queueService.GetQueueHistory(uint(branchID))
	if err != nil {
		return response.InternalServerError(c, "Failed to get history")
	}
	return response.Success(c, "Queue history retrieved", tickets)
}

// ============================================================
// Config
// ============================================================

// GET /api/v1/admin/queue/config?branch_id=X — ดูค่าตั้ง
func (h *QueueAdminHandler) GetConfig(c *fiber.Ctx) error {
	branchID, err := strconv.ParseUint(c.Query("branch_id", "0"), 10, 32)
	if err != nil || branchID == 0 {
		return response.BadRequest(c, "branch_id query parameter is required")
	}

	configs, err := h.queueService.GetConfig(uint(branchID))
	if err != nil {
		return response.InternalServerError(c, "Failed to get config")
	}
	return response.Success(c, "Config retrieved", configs)
}

// PUT /api/v1/admin/queue/config — แก้ค่าตั้ง
func (h *QueueAdminHandler) UpdateConfig(c *fiber.Ctx) error {
	var input services.UpdateConfigInput
	if err := c.BodyParser(&input); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if input.BranchID == 0 || input.Key == "" || input.Value == "" {
		return response.BadRequest(c, "branch_id, key, and value are required")
	}

	if err := h.queueService.UpdateConfig(&input); err != nil {
		return response.InternalServerError(c, "Failed to update config")
	}
	return response.Success(c, "Config updated", nil)
}

// ============================================================
// Phase 5: Booking Admin endpoints
// ============================================================

// GET /api/v1/admin/queue/bookings?branch_id=X — การจองทั้งหมด
func (h *QueueAdminHandler) GetBookings(c *fiber.Ctx) error {
	branchID, err := strconv.ParseUint(c.Query("branch_id", "0"), 10, 32)
	if err != nil || branchID == 0 {
		return response.BadRequest(c, "branch_id query parameter is required")
	}

	bookings, err := h.queueService.GetBookingsByBranch(uint(branchID))
	if err != nil {
		return response.InternalServerError(c, "Failed to get bookings")
	}
	return response.Success(c, "Bookings retrieved", bookings)
}

// POST /api/v1/admin/queue/booking/:id/checkin — Check-in Booking
func (h *QueueAdminHandler) CheckinBooking(c *fiber.Ctx) error {
	ticketID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid booking ID")
	}

	ticket, err := h.queueService.CheckinBooking(uint(ticketID))
	if err != nil {
		switch err {
		case services.ErrBookingNotFound:
			return response.NotFound(c, "Booking not found")
		case services.ErrBookingNotWaiting:
			return response.BadRequest(c, "Booking is not in WAITING status")
		default:
			return response.InternalServerError(c, "Failed to check-in booking")
		}
	}
	return response.Success(c, "Booking checked in", ticket)
}

// POST /api/v1/admin/queue/slots/generate — สร้าง booking slots
func (h *QueueAdminHandler) GenerateSlots(c *fiber.Ctx) error {
	var input services.GenerateSlotsInput
	if err := c.BodyParser(&input); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if input.BranchID == 0 || input.ServiceTypeID == 0 || input.StartDate == "" || input.EndDate == "" {
		return response.BadRequest(c, "branch_id, service_type_id, start_date, and end_date are required")
	}

	count, err := h.queueService.GenerateBookingSlots(&input)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.Success(c, "Booking slots generated", map[string]interface{}{
		"slots_created": count,
	})
}
