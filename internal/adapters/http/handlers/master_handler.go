package handlers

import (
	"strconv"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// MasterHandler handles master data endpoints
type MasterHandler struct {
	loanTypeRepo *repositories.LoanTypeRepository
	loanStepRepo *repositories.LoanStepRepository
	loanDocRepo  *repositories.LoanDocRepository
	loanApptRepo *repositories.LoanApptRepository
}

// NewMasterHandler creates a new master handler
func NewMasterHandler(
	loanTypeRepo *repositories.LoanTypeRepository,
	loanStepRepo *repositories.LoanStepRepository,
	loanDocRepo *repositories.LoanDocRepository,
	loanApptRepo *repositories.LoanApptRepository,
) *MasterHandler {
	return &MasterHandler{
		loanTypeRepo: loanTypeRepo,
		loanStepRepo: loanStepRepo,
		loanDocRepo:  loanDocRepo,
		loanApptRepo: loanApptRepo,
	}
}

// ============================================================
// Loan Type
// ============================================================

// ListLoanTypes lists all loan types
// @Summary List loan types
// @Description Get all loan types (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param all query bool false "Include inactive"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /master/loan-types [get]
func (h *MasterHandler) ListLoanTypes(c *fiber.Ctx) error {
	includeInactive := c.Query("all") == "true"

	var loanTypes []*models.LoanType
	var err error

	if includeInactive {
		loanTypes, err = h.loanTypeRepo.ListAll(c.Context())
	} else {
		loanTypes, err = h.loanTypeRepo.List(c.Context())
	}

	if err != nil {
		return response.InternalServerError(c, "Failed to list loan types")
	}

	return response.Success(c, "Loan types retrieved successfully", fiber.Map{
		"loan_types": loanTypes,
	})
}

// GetLoanType gets a loan type by ID
// @Summary Get loan type
// @Description Get a loan type by ID (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan Type ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /master/loan-types/{id} [get]
func (h *MasterHandler) GetLoanType(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}

	loanType, err := h.loanTypeRepo.GetByID(c.Context(), uint(id))
	if err != nil {
		return response.NotFound(c, "Loan type not found")
	}

	return response.Success(c, "Loan type retrieved successfully", fiber.Map{
		"loan_type": loanType,
	})
}

// CreateLoanTypeRequest represents create loan type request
type CreateLoanTypeRequest struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	InterestRate float64 `json:"interest_rate"`
}

// CreateLoanType creates a new loan type
// @Summary Create loan type
// @Description Create a new loan type (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateLoanTypeRequest true "Loan type data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /master/loan-types [post]
func (h *MasterHandler) CreateLoanType(c *fiber.Ctx) error {
	var req CreateLoanTypeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Code == "" || req.Name == "" {
		return response.BadRequest(c, "Code and name are required")
	}

	loanType := &models.LoanType{
		Code:         req.Code,
		Name:         req.Name,
		Description:  req.Description,
		InterestRate: req.InterestRate,
		IsActive:     true,
	}

	if err := h.loanTypeRepo.Create(c.Context(), loanType); err != nil {
		return response.InternalServerError(c, "Failed to create loan type")
	}

	return response.Created(c, "Loan type created successfully", fiber.Map{
		"loan_type": loanType,
	})
}

// UpdateLoanType updates a loan type
// @Summary Update loan type
// @Description Update a loan type (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan Type ID"
// @Param body body CreateLoanTypeRequest true "Loan type data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /master/loan-types/{id} [put]
func (h *MasterHandler) UpdateLoanType(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}

	loanType, err := h.loanTypeRepo.GetByID(c.Context(), uint(id))
	if err != nil {
		return response.NotFound(c, "Loan type not found")
	}

	var req CreateLoanTypeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Code != "" {
		loanType.Code = req.Code
	}
	if req.Name != "" {
		loanType.Name = req.Name
	}
	if req.Description != "" {
		loanType.Description = req.Description
	}
	if req.InterestRate > 0 {
		loanType.InterestRate = req.InterestRate
	}

	if err := h.loanTypeRepo.Update(c.Context(), loanType); err != nil {
		return response.InternalServerError(c, "Failed to update loan type")
	}

	return response.Success(c, "Loan type updated successfully", fiber.Map{
		"loan_type": loanType,
	})
}

// DeleteLoanType deletes a loan type
// @Summary Delete loan type
// @Description Delete a loan type (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan Type ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /master/loan-types/{id} [delete]
func (h *MasterHandler) DeleteLoanType(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}

	if err := h.loanTypeRepo.Delete(c.Context(), uint(id)); err != nil {
		return response.InternalServerError(c, "Failed to delete loan type")
	}

	return response.Success(c, "Loan type deleted successfully", nil)
}

// ============================================================
// Loan Step
// ============================================================

// ListLoanSteps lists all loan steps
// @Summary List loan steps
// @Description Get all loan steps (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param all query bool false "Include inactive"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /master/loan-steps [get]
func (h *MasterHandler) ListLoanSteps(c *fiber.Ctx) error {
	includeInactive := c.Query("all") == "true"

	var loanSteps []*models.LoanStep
	var err error

	if includeInactive {
		loanSteps, err = h.loanStepRepo.ListAll(c.Context())
	} else {
		loanSteps, err = h.loanStepRepo.List(c.Context())
	}

	if err != nil {
		return response.InternalServerError(c, "Failed to list loan steps")
	}

	return response.Success(c, "Loan steps retrieved successfully", fiber.Map{
		"loan_steps": loanSteps,
	})
}

// GetLoanStep gets a loan step by ID
// @Summary Get loan step
// @Description Get a loan step by ID (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan Step ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /master/loan-steps/{id} [get]
func (h *MasterHandler) GetLoanStep(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}

	loanStep, err := h.loanStepRepo.GetByID(c.Context(), uint(id))
	if err != nil {
		return response.NotFound(c, "Loan step not found")
	}

	return response.Success(c, "Loan step retrieved successfully", fiber.Map{
		"loan_step": loanStep,
	})
}

// CreateLoanStepRequest represents create loan step request
type CreateLoanStepRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	StepOrder   int    `json:"step_order"`
	Color       string `json:"color,omitempty"`
	IsFinal     bool   `json:"is_final"`
}

// CreateLoanStep creates a new loan step
// @Summary Create loan step
// @Description Create a new loan step (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateLoanStepRequest true "Loan step data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /master/loan-steps [post]
func (h *MasterHandler) CreateLoanStep(c *fiber.Ctx) error {
	var req CreateLoanStepRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Code == "" || req.Name == "" {
		return response.BadRequest(c, "Code and name are required")
	}

	loanStep := &models.LoanStep{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		StepOrder:   req.StepOrder,
		Color:       req.Color,
		IsFinal:     req.IsFinal,
		IsActive:    true,
	}

	if err := h.loanStepRepo.Create(c.Context(), loanStep); err != nil {
		return response.InternalServerError(c, "Failed to create loan step")
	}

	return response.Created(c, "Loan step created successfully", fiber.Map{
		"loan_step": loanStep,
	})
}

// UpdateLoanStep updates a loan step
// @Summary Update loan step
// @Description Update a loan step (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan Step ID"
// @Param body body CreateLoanStepRequest true "Loan step data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /master/loan-steps/{id} [put]
func (h *MasterHandler) UpdateLoanStep(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}

	loanStep, err := h.loanStepRepo.GetByID(c.Context(), uint(id))
	if err != nil {
		return response.NotFound(c, "Loan step not found")
	}

	var req CreateLoanStepRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Code != "" {
		loanStep.Code = req.Code
	}
	if req.Name != "" {
		loanStep.Name = req.Name
	}
	if req.Description != "" {
		loanStep.Description = req.Description
	}
	if req.StepOrder > 0 {
		loanStep.StepOrder = req.StepOrder
	}
	if req.Color != "" {
		loanStep.Color = req.Color
	}
	loanStep.IsFinal = req.IsFinal

	if err := h.loanStepRepo.Update(c.Context(), loanStep); err != nil {
		return response.InternalServerError(c, "Failed to update loan step")
	}

	return response.Success(c, "Loan step updated successfully", fiber.Map{
		"loan_step": loanStep,
	})
}

// DeleteLoanStep deletes a loan step
// @Summary Delete loan step
// @Description Delete a loan step (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan Step ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /master/loan-steps/{id} [delete]
func (h *MasterHandler) DeleteLoanStep(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}

	if err := h.loanStepRepo.Delete(c.Context(), uint(id)); err != nil {
		return response.InternalServerError(c, "Failed to delete loan step")
	}

	return response.Success(c, "Loan step deleted successfully", nil)
}

// ============================================================
// Loan Doc
// ============================================================

// ListLoanDocs lists all loan docs
// @Summary List loan docs
// @Description Get all loan documents (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param all query bool false "Include inactive"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /master/loan-docs [get]
func (h *MasterHandler) ListLoanDocs(c *fiber.Ctx) error {
	includeInactive := c.Query("all") == "true"

	var loanDocs []*models.LoanDoc
	var err error

	if includeInactive {
		loanDocs, err = h.loanDocRepo.ListAll(c.Context())
	} else {
		loanDocs, err = h.loanDocRepo.List(c.Context())
	}

	if err != nil {
		return response.InternalServerError(c, "Failed to list loan docs")
	}

	return response.Success(c, "Loan docs retrieved successfully", fiber.Map{
		"loan_docs": loanDocs,
	})
}

// GetLoanDoc gets a loan doc by ID
// @Summary Get loan doc
// @Description Get a loan document by ID (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan Doc ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /master/loan-docs/{id} [get]
func (h *MasterHandler) GetLoanDoc(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}

	loanDoc, err := h.loanDocRepo.GetByID(c.Context(), uint(id))
	if err != nil {
		return response.NotFound(c, "Loan doc not found")
	}

	return response.Success(c, "Loan doc retrieved successfully", fiber.Map{
		"loan_doc": loanDoc,
	})
}

// CreateLoanDocRequest represents create loan doc request
type CreateLoanDocRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CreateLoanDoc creates a new loan doc
// @Summary Create loan doc
// @Description Create a new loan document (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateLoanDocRequest true "Loan doc data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /master/loan-docs [post]
func (h *MasterHandler) CreateLoanDoc(c *fiber.Ctx) error {
	var req CreateLoanDocRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Code == "" || req.Name == "" {
		return response.BadRequest(c, "Code and name are required")
	}

	loanDoc := &models.LoanDoc{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    true,
	}

	if err := h.loanDocRepo.Create(c.Context(), loanDoc); err != nil {
		return response.InternalServerError(c, "Failed to create loan doc")
	}

	return response.Created(c, "Loan doc created successfully", fiber.Map{
		"loan_doc": loanDoc,
	})
}

// UpdateLoanDoc updates a loan doc
// @Summary Update loan doc
// @Description Update a loan document (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan Doc ID"
// @Param body body CreateLoanDocRequest true "Loan doc data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /master/loan-docs/{id} [put]
func (h *MasterHandler) UpdateLoanDoc(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}

	loanDoc, err := h.loanDocRepo.GetByID(c.Context(), uint(id))
	if err != nil {
		return response.NotFound(c, "Loan doc not found")
	}

	var req CreateLoanDocRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Code != "" {
		loanDoc.Code = req.Code
	}
	if req.Name != "" {
		loanDoc.Name = req.Name
	}
	if req.Description != "" {
		loanDoc.Description = req.Description
	}

	if err := h.loanDocRepo.Update(c.Context(), loanDoc); err != nil {
		return response.InternalServerError(c, "Failed to update loan doc")
	}

	return response.Success(c, "Loan doc updated successfully", fiber.Map{
		"loan_doc": loanDoc,
	})
}

// DeleteLoanDoc deletes a loan doc
// @Summary Delete loan doc
// @Description Delete a loan document (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan Doc ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /master/loan-docs/{id} [delete]
func (h *MasterHandler) DeleteLoanDoc(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}

	if err := h.loanDocRepo.Delete(c.Context(), uint(id)); err != nil {
		return response.InternalServerError(c, "Failed to delete loan doc")
	}

	return response.Success(c, "Loan doc deleted successfully", nil)
}

// ============================================================
// Loan Appt
// ============================================================

// ListLoanAppts lists all loan appts
// @Summary List loan appointments
// @Description Get all loan appointment types (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param all query bool false "Include inactive"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /master/loan-appts [get]
func (h *MasterHandler) ListLoanAppts(c *fiber.Ctx) error {
	includeInactive := c.Query("all") == "true"

	var loanAppts []*models.LoanAppt
	var err error

	if includeInactive {
		loanAppts, err = h.loanApptRepo.ListAll(c.Context())
	} else {
		loanAppts, err = h.loanApptRepo.List(c.Context())
	}

	if err != nil {
		return response.InternalServerError(c, "Failed to list loan appts")
	}

	return response.Success(c, "Loan appts retrieved successfully", fiber.Map{
		"loan_appts": loanAppts,
	})
}

// GetLoanAppt gets a loan appt by ID
// @Summary Get loan appointment
// @Description Get a loan appointment type by ID (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan Appt ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /master/loan-appts/{id} [get]
func (h *MasterHandler) GetLoanAppt(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}

	loanAppt, err := h.loanApptRepo.GetByID(c.Context(), uint(id))
	if err != nil {
		return response.NotFound(c, "Loan appt not found")
	}

	return response.Success(c, "Loan appt retrieved successfully", fiber.Map{
		"loan_appt": loanAppt,
	})
}

// CreateLoanApptRequest represents create loan appt request
type CreateLoanApptRequest struct {
	Code            string `json:"code"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	DefaultLocation string `json:"default_location,omitempty"`
}

// CreateLoanAppt creates a new loan appt
// @Summary Create loan appointment
// @Description Create a new loan appointment type (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateLoanApptRequest true "Loan appt data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /master/loan-appts [post]
func (h *MasterHandler) CreateLoanAppt(c *fiber.Ctx) error {
	var req CreateLoanApptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Code == "" || req.Name == "" {
		return response.BadRequest(c, "Code and name are required")
	}

	loanAppt := &models.LoanAppt{
		Code:            req.Code,
		Name:            req.Name,
		Description:     req.Description,
		DefaultLocation: req.DefaultLocation,
		IsActive:        true,
	}

	if err := h.loanApptRepo.Create(c.Context(), loanAppt); err != nil {
		return response.InternalServerError(c, "Failed to create loan appt")
	}

	return response.Created(c, "Loan appt created successfully", fiber.Map{
		"loan_appt": loanAppt,
	})
}

// UpdateLoanAppt updates a loan appt
// @Summary Update loan appointment
// @Description Update a loan appointment type (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan Appt ID"
// @Param body body CreateLoanApptRequest true "Loan appt data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /master/loan-appts/{id} [put]
func (h *MasterHandler) UpdateLoanAppt(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}

	loanAppt, err := h.loanApptRepo.GetByID(c.Context(), uint(id))
	if err != nil {
		return response.NotFound(c, "Loan appt not found")
	}

	var req CreateLoanApptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Code != "" {
		loanAppt.Code = req.Code
	}
	if req.Name != "" {
		loanAppt.Name = req.Name
	}
	if req.Description != "" {
		loanAppt.Description = req.Description
	}
	if req.DefaultLocation != "" {
		loanAppt.DefaultLocation = req.DefaultLocation
	}

	if err := h.loanApptRepo.Update(c.Context(), loanAppt); err != nil {
		return response.InternalServerError(c, "Failed to update loan appt")
	}

	return response.Success(c, "Loan appt updated successfully", fiber.Map{
		"loan_appt": loanAppt,
	})
}

// DeleteLoanAppt deletes a loan appt
// @Summary Delete loan appointment
// @Description Delete a loan appointment type (Admin only)
// @Tags Master
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan Appt ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /master/loan-appts/{id} [delete]
func (h *MasterHandler) DeleteLoanAppt(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}

	if err := h.loanApptRepo.Delete(c.Context(), uint(id)); err != nil {
		return response.InternalServerError(c, "Failed to delete loan appt")
	}

	return response.Success(c, "Loan appt deleted successfully", nil)
}
