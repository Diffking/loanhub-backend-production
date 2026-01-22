package handlers

import (
	"errors"
	"strconv"

	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// MortgageHandler handles mortgage endpoints
type MortgageHandler struct {
	mortgageService *services.MortgageService
}

// NewMortgageHandler creates a new mortgage handler
func NewMortgageHandler(mortgageService *services.MortgageService) *MortgageHandler {
	return &MortgageHandler{
		mortgageService: mortgageService,
	}
}

// getClientIP gets client IP address
func getClientIP(c *fiber.Ctx) string {
	ip := c.Get("X-Real-IP")
	if ip == "" {
		ip = c.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip = c.IP()
	}
	return ip
}

// CreateRequest represents create mortgage request
type CreateMortgageRequest struct {
	MembNo          string  `json:"memb_no"`
	LoanTypeID      uint    `json:"loan_type_id"`
	Amount          float64 `json:"amount"`
	Collateral      string  `json:"collateral,omitempty"`
	Purpose         string  `json:"purpose,omitempty"`
	GuarantorMembNo string  `json:"guarantor_memb_no,omitempty"`
	Remark          string  `json:"remark,omitempty"`
}

// Create creates a new mortgage
// @Summary Create mortgage
// @Description Create a new mortgage (Officer only)
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateMortgageRequest true "Mortgage data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /mortgages [post]
func (h *MortgageHandler) Create(c *fiber.Ctx) error {
	var req CreateMortgageRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate required fields
	if req.MembNo == "" {
		return response.BadRequest(c, "Member number is required")
	}
	if req.LoanTypeID == 0 {
		return response.BadRequest(c, "Loan type is required")
	}
	if req.Amount <= 0 {
		return response.BadRequest(c, "Amount must be greater than 0")
	}

	userID, _ := c.Locals("userID").(uint)
	ipAddress := getClientIP(c)

	input := &services.CreateMortgageInput{
		MembNo:          req.MembNo,
		LoanTypeID:      req.LoanTypeID,
		Amount:          req.Amount,
		Collateral:      req.Collateral,
		Purpose:         req.Purpose,
		GuarantorMembNo: req.GuarantorMembNo,
		Remark:          req.Remark,
	}

	mortgage, err := h.mortgageService.Create(c.Context(), input, userID, ipAddress)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrMemberNotFoundMortgage):
			return response.NotFound(c, "Member not found")
		case errors.Is(err, services.ErrLoanTypeNotFound):
			return response.NotFound(c, "Loan type not found")
		default:
			return response.InternalServerError(c, "Failed to create mortgage")
		}
	}

	return response.Created(c, "Mortgage created successfully", fiber.Map{
		"mortgage": mortgage.ToResponse(),
	})
}

// List lists mortgages
// @Summary List mortgages
// @Description List all mortgages (Officer only)
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Param officer_id query int false "Filter by officer ID"
// @Param step_id query int false "Filter by step ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /mortgages [get]
func (h *MortgageHandler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	input := &services.ListInput{
		Page:  page,
		Limit: limit,
	}

	if officerID := c.Query("officer_id"); officerID != "" {
		id, _ := strconv.ParseUint(officerID, 10, 32)
		uid := uint(id)
		input.OfficerID = &uid
	}

	if stepID := c.Query("step_id"); stepID != "" {
		id, _ := strconv.ParseUint(stepID, 10, 32)
		uid := uint(id)
		input.StepID = &uid
	}

	result, err := h.mortgageService.List(c.Context(), input)
	if err != nil {
		return response.InternalServerError(c, "Failed to list mortgages")
	}

	return response.Success(c, "Mortgages retrieved successfully", result)
}

// GetByID gets a mortgage by ID
// @Summary Get mortgage by ID
// @Description Get a specific mortgage (Officer only)
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Mortgage ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /mortgages/{id} [get]
func (h *MortgageHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	mortgage, err := h.mortgageService.GetByID(c.Context(), uint(id))
	if err != nil {
		if errors.Is(err, services.ErrMortgageNotFound) {
			return response.NotFound(c, "Mortgage not found")
		}
		return response.InternalServerError(c, "Failed to get mortgage")
	}

	return response.Success(c, "Mortgage retrieved successfully", fiber.Map{
		"mortgage": mortgage.ToResponse(),
	})
}

// GetMyMortgages gets member's own mortgages
// @Summary Get my mortgages
// @Description Get current user's mortgages
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /mortgages/my [get]
func (h *MortgageHandler) GetMyMortgages(c *fiber.Ctx) error {
	membNo, ok := c.Locals("membNo").(string)
	if !ok || membNo == "" {
		return response.Unauthorized(c, "Unauthorized")
	}

	mortgages, err := h.mortgageService.GetByMembNo(c.Context(), membNo)
	if err != nil {
		return response.InternalServerError(c, "Failed to get mortgages")
	}

	// Convert to response format
	// Convert to response using ToResponse()
	var result []interface{}
	for _, m := range mortgages {
		result = append(result, m.ToResponse())
	}

	// Return empty array if no mortgages
	if result == nil {
		result = []interface{}{}
	}

	return response.Success(c, "Mortgages retrieved successfully", result)
}

// ChangeStepRequest represents change step request
type ChangeStepRequest struct {
	StepID uint   `json:"step_id"`
	Remark string `json:"remark,omitempty"`
}

// ChangeStep changes mortgage step
// @Summary Change mortgage step
// @Description Change mortgage step/status (Officer only)
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Mortgage ID"
// @Param body body ChangeStepRequest true "Step data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /mortgages/{id}/step [put]
func (h *MortgageHandler) ChangeStep(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	var req ChangeStepRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.StepID == 0 {
		return response.BadRequest(c, "Step ID is required")
	}

	userID, _ := c.Locals("userID").(uint)
	ipAddress := getClientIP(c)

	input := &services.ChangeStepInput{
		StepID: req.StepID,
		Remark: req.Remark,
	}

	mortgage, err := h.mortgageService.ChangeStep(c.Context(), uint(id), input, userID, ipAddress)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrMortgageNotFound):
			return response.NotFound(c, "Mortgage not found")
		case errors.Is(err, services.ErrLoanStepNotFound):
			return response.NotFound(c, "Step not found")
		default:
			return response.InternalServerError(c, "Failed to change step")
		}
	}

	return response.Success(c, "Step changed successfully", fiber.Map{
		"mortgage": mortgage.ToResponse(),
	})
}

// ApproveRequest represents approve request
type ApproveRequest struct {
	ContractNo string `json:"contract_no"`
	Remark     string `json:"remark,omitempty"`
}

// Approve approves a mortgage
// @Summary Approve mortgage
// @Description Approve a mortgage (Officer only)
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Mortgage ID"
// @Param body body ApproveRequest true "Approve data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /mortgages/{id}/approve [put]
func (h *MortgageHandler) Approve(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	var req ApproveRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.ContractNo == "" {
		return response.BadRequest(c, "Contract number is required")
	}

	userID, _ := c.Locals("userID").(uint)
	ipAddress := getClientIP(c)

	input := &services.ApproveInput{
		ContractNo: req.ContractNo,
		Remark:     req.Remark,
	}

	mortgage, err := h.mortgageService.Approve(c.Context(), uint(id), input, userID, ipAddress)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrMortgageNotFound):
			return response.NotFound(c, "Mortgage not found")
		case errors.Is(err, services.ErrAlreadyApproved):
			return response.BadRequest(c, "Mortgage already approved")
		default:
			return response.InternalServerError(c, "Failed to approve mortgage")
		}
	}

	return response.Success(c, "Mortgage approved successfully", fiber.Map{
		"mortgage": mortgage.ToResponse(),
	})
}

// RejectRequest represents reject request
type RejectRequest struct {
	Remark string `json:"remark"`
}

// Reject rejects a mortgage
// @Summary Reject mortgage
// @Description Reject a mortgage (Officer only)
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Mortgage ID"
// @Param body body RejectRequest true "Reject data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /mortgages/{id}/reject [put]
func (h *MortgageHandler) Reject(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	var req RejectRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Remark == "" {
		return response.BadRequest(c, "Reason is required")
	}

	userID, _ := c.Locals("userID").(uint)
	ipAddress := getClientIP(c)

	input := &services.RejectInput{
		Remark: req.Remark,
	}

	mortgage, err := h.mortgageService.Reject(c.Context(), uint(id), input, userID, ipAddress)
	if err != nil {
		if errors.Is(err, services.ErrMortgageNotFound) {
			return response.NotFound(c, "Mortgage not found")
		}
		return response.InternalServerError(c, "Failed to reject mortgage")
	}

	return response.Success(c, "Mortgage rejected successfully", fiber.Map{
		"mortgage": mortgage.ToResponse(),
	})
}

// GetHistory gets mortgage history
// @Summary Get mortgage history
// @Description Get mortgage transaction history
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Mortgage ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /mortgages/{id}/history [get]
func (h *MortgageHandler) GetHistory(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	transactions, err := h.mortgageService.GetHistory(c.Context(), uint(id))
	if err != nil {
		if errors.Is(err, services.ErrMortgageNotFound) {
			return response.NotFound(c, "Mortgage not found")
		}
		return response.InternalServerError(c, "Failed to get history")
	}

	return response.Success(c, "History retrieved successfully", fiber.Map{
		"transactions": transactions,
	})
}

// GetDocs gets mortgage documents
// @Summary Get mortgage documents
// @Description Get mortgage document checklist
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Mortgage ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /mortgages/{id}/docs [get]
func (h *MortgageHandler) GetDocs(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	docs, err := h.mortgageService.GetDocs(c.Context(), uint(id))
	if err != nil {
		return response.InternalServerError(c, "Failed to get documents")
	}

	return response.Success(c, "Documents retrieved successfully", fiber.Map{
		"documents": docs,
	})
}

// UpdateDocRequest represents update doc request
type UpdateDocRequest struct {
	DocID       uint   `json:"doc_id"`
	IsSubmitted bool   `json:"is_submitted"`
	Remark      string `json:"remark,omitempty"`
}

// UpdateDoc updates document status
// @Summary Update document status
// @Description Update mortgage document status (Officer only)
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Mortgage ID"
// @Param body body UpdateDocRequest true "Document data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /mortgages/{id}/docs [put]
func (h *MortgageHandler) UpdateDoc(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	var req UpdateDocRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.DocID == 0 {
		return response.BadRequest(c, "Document ID is required")
	}

	userID, _ := c.Locals("userID").(uint)
	ipAddress := getClientIP(c)

	input := &services.UpdateDocInput{
		DocID:       req.DocID,
		IsSubmitted: req.IsSubmitted,
		Remark:      req.Remark,
	}

	err = h.mortgageService.UpdateDoc(c.Context(), uint(id), input, userID, ipAddress)
	if err != nil {
		if errors.Is(err, services.ErrLoanDocNotFound) {
			return response.NotFound(c, "Document not found")
		}
		return response.InternalServerError(c, "Failed to update document")
	}

	return response.Success(c, "Document updated successfully", nil)
}

// CreateApptRequest represents create appointment request
type CreateApptRequest struct {
	LoanApptID uint   `json:"loan_appt_id"`
	ApptDate   string `json:"appt_date"`
	ApptTime   string `json:"appt_time,omitempty"`
	Location   string `json:"location,omitempty"`
	Remark     string `json:"remark,omitempty"`
}

// CreateAppt creates an appointment
// @Summary Create appointment
// @Description Create a new appointment (Officer only)
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Mortgage ID"
// @Param body body CreateApptRequest true "Appointment data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /mortgages/{id}/appts [post]
func (h *MortgageHandler) CreateAppt(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	var req CreateApptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.LoanApptID == 0 {
		return response.BadRequest(c, "Appointment type is required")
	}
	if req.ApptDate == "" {
		return response.BadRequest(c, "Appointment date is required")
	}

	userID, _ := c.Locals("userID").(uint)
	ipAddress := getClientIP(c)

	input := &services.CreateApptInput{
		LoanApptID: req.LoanApptID,
		ApptDate:   req.ApptDate,
		ApptTime:   req.ApptTime,
		Location:   req.Location,
		Remark:     req.Remark,
	}

	appt, err := h.mortgageService.CreateAppt(c.Context(), uint(id), input, userID, ipAddress)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrMortgageNotFound):
			return response.NotFound(c, "Mortgage not found")
		case errors.Is(err, services.ErrLoanApptNotFound):
			return response.NotFound(c, "Appointment type not found")
		default:
			return response.InternalServerError(c, "Failed to create appointment")
		}
	}

	return response.Created(c, "Appointment created successfully", fiber.Map{
		"appointment": appt,
	})
}

// GetAppts gets mortgage appointments
// @Summary Get appointments
// @Description Get mortgage appointments
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Mortgage ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /mortgages/{id}/appts [get]
func (h *MortgageHandler) GetAppts(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	appts, err := h.mortgageService.GetAppts(c.Context(), uint(id))
	if err != nil {
		return response.InternalServerError(c, "Failed to get appointments")
	}

	return response.Success(c, "Appointments retrieved successfully", fiber.Map{
		"appointments": appts,
	})
}

// CompleteAppt completes an appointment
// @Summary Complete appointment
// @Description Mark appointment as completed (Officer only)
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Mortgage ID"
// @Param appt_id path int true "Appointment ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /mortgages/{id}/appts/{appt_id}/complete [put]
func (h *MortgageHandler) CompleteAppt(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	apptID, err := strconv.ParseUint(c.Params("appt_id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid appointment ID")
	}

	userID, _ := c.Locals("userID").(uint)
	ipAddress := getClientIP(c)

	err = h.mortgageService.CompleteAppt(c.Context(), uint(id), uint(apptID), userID, ipAddress)
	if err != nil {
		if errors.Is(err, services.ErrApptNotFound) {
			return response.NotFound(c, "Appointment not found")
		}
		return response.InternalServerError(c, "Failed to complete appointment")
	}

	return response.Success(c, "Appointment completed successfully", nil)
}

// ChangeOfficerRequest represents change officer request
type ChangeOfficerRequest struct {
	OfficerID uint   `json:"officer_id"`
	Remark    string `json:"remark,omitempty"`
}

// ChangeOfficer changes the responsible officer
// @Summary Change officer
// @Description Change the responsible officer (Admin only)
// @Tags Mortgages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Mortgage ID"
// @Param body body ChangeOfficerRequest true "Officer data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /mortgages/{id}/officer [put]
func (h *MortgageHandler) ChangeOfficer(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	var req ChangeOfficerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.OfficerID == 0 {
		return response.BadRequest(c, "Officer ID is required")
	}

	userID, _ := c.Locals("userID").(uint)
	ipAddress := getClientIP(c)

	input := &services.ChangeOfficerInput{
		OfficerID: req.OfficerID,
		Remark:    req.Remark,
	}

	mortgage, err := h.mortgageService.ChangeOfficer(c.Context(), uint(id), input, userID, ipAddress)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrMortgageNotFound):
			return response.NotFound(c, "Mortgage not found")
		case errors.Is(err, services.ErrOfficerNotFound):
			return response.NotFound(c, "Officer not found")
		default:
			return response.InternalServerError(c, "Failed to change officer")
		}
	}

	return response.Success(c, "Officer changed successfully", fiber.Map{
		"mortgage": mortgage.ToResponse(),
	})
}
