package handlers

import (
	"strconv"

	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// QueueHandler handles user-facing queue endpoints
type QueueHandler struct {
	queueService *services.QueueService
}

// NewQueueHandler creates a new queue handler
func NewQueueHandler(queueService *services.QueueService) *QueueHandler {
	return &QueueHandler{
		queueService: queueService,
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
