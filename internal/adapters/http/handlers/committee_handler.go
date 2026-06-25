package handlers

import (
	"errors"
	"strconv"
	"time"

	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// CommitteeHandler handles คณะกรรมการ designation and borrower-list endpoints
type CommitteeHandler struct {
	committeeService *services.CommitteeService
}

// NewCommitteeHandler creates a new committee handler
func NewCommitteeHandler(committeeService *services.CommitteeService) *CommitteeHandler {
	return &CommitteeHandler{committeeService: committeeService}
}

// AddMemberRequest represents the request to designate a member as committee
type AddMemberRequest struct {
	MembNo    string `json:"memb_no"`
	TermLabel string `json:"term_label"`
}

// AddMember designates a member as an active committee member (Officer/Admin)
// @Summary Add committee member
// @Description Designate an existing member as an active committee member for a term
// @Tags Committee
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body AddMemberRequest true "Committee member data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Router /admin/committee/members [post]
func (h *CommitteeHandler) AddMember(c *fiber.Ctx) error {
	var req AddMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if req.MembNo == "" {
		return response.BadRequest(c, "Member number is required")
	}
	if req.TermLabel == "" {
		return response.BadRequest(c, "Term label is required")
	}

	userID, _ := c.Locals("userID").(uint)

	cm, err := h.committeeService.Add(c.Context(), &services.AddCommitteeMemberInput{
		MembNo:    req.MembNo,
		TermLabel: req.TermLabel,
	}, userID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrFlommastMemberNotFound):
			return response.NotFound(c, "Member not found")
		case errors.Is(err, services.ErrAlreadyActiveCommittee):
			return response.Conflict(c, "Member is already an active committee member")
		default:
			return response.InternalServerError(c, "Failed to add committee member")
		}
	}

	return response.Created(c, "Committee member added successfully", fiber.Map{
		"committee_member": cm,
	})
}

// ListMembers lists all committee member designations (Officer/Admin)
// @Summary List committee members
// @Tags Committee
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} response.Response
// @Router /admin/committee/members [get]
func (h *CommitteeHandler) ListMembers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	result, err := h.committeeService.List(c.Context(), page, limit)
	if err != nil {
		return response.InternalServerError(c, "Failed to list committee members")
	}

	return response.Success(c, "Committee members retrieved successfully", result)
}

// RemoveMember revokes a committee member designation (Officer/Admin)
// @Summary Remove committee member
// @Tags Committee
// @Produce json
// @Security BearerAuth
// @Param id path int true "Committee member ID"
// @Success 200 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /admin/committee/members/{id} [delete]
func (h *CommitteeHandler) RemoveMember(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid committee member ID")
	}

	userID, _ := c.Locals("userID").(uint)

	if err := h.committeeService.Remove(c.Context(), uint(id), userID); err != nil {
		if errors.Is(err, services.ErrCommitteeMemberNotFound) {
			return response.NotFound(c, "Committee member not found")
		}
		return response.InternalServerError(c, "Failed to remove committee member")
	}

	return response.Success(c, "Committee member removed successfully", nil)
}

// IsCommitteeMember reports whether the logged-in member is an active committee member
// @Summary Check my committee status
// @Tags Committee
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Router /committee/me [get]
func (h *CommitteeHandler) IsCommitteeMember(c *fiber.Ctx) error {
	membNo, _ := c.Locals("membNo").(string)

	isActive, err := h.committeeService.IsActiveMember(c.Context(), membNo)
	if err != nil {
		return response.InternalServerError(c, "Failed to check committee status")
	}

	return response.Success(c, "OK", fiber.Map{
		"is_committee_member": isActive,
	})
}

// GetVisibility returns the current borrower-field visibility settings (Officer/Admin)
// @Summary Get committee borrower-field visibility settings
// @Tags Committee
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Router /admin/committee/visibility [get]
func (h *CommitteeHandler) GetVisibility(c *fiber.Ctx) error {
	setting, err := h.committeeService.GetVisibility(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Failed to get visibility settings")
	}
	return response.Success(c, "OK", setting)
}

// UpdateVisibilityRequest represents the request to update borrower-field visibility
type UpdateVisibilityRequest struct {
	ShowBorrowerName bool `json:"show_borrower_name"`
	ShowMembNo       bool `json:"show_memb_no"`
	ShowAmount       bool `json:"show_amount"`
	ShowLoanStatus   bool `json:"show_loan_status"`
}

// UpdateVisibility updates which borrower fields committee viewers can see (Officer/Admin)
// @Summary Update committee borrower-field visibility settings
// @Tags Committee
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body UpdateVisibilityRequest true "Visibility settings"
// @Success 200 {object} response.Response
// @Router /admin/committee/visibility [put]
func (h *CommitteeHandler) UpdateVisibility(c *fiber.Ctx) error {
	var req UpdateVisibilityRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	setting, err := h.committeeService.UpdateVisibility(c.Context(), &services.UpdateVisibilityInput{
		ShowBorrowerName: req.ShowBorrowerName,
		ShowMembNo:       req.ShowMembNo,
		ShowAmount:       req.ShowAmount,
		ShowLoanStatus:   req.ShowLoanStatus,
	})
	if err != nil {
		return response.InternalServerError(c, "Failed to update visibility settings")
	}
	return response.Success(c, "Visibility settings updated successfully", setting)
}

// ListBorrowersByMonth returns all loan applicants for a given month/year (committee members only)
// @Summary List borrowers by month
// @Tags Committee
// @Produce json
// @Security BearerAuth
// @Param year query int true "Year"
// @Param month query int true "Month (1-12)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /committee/borrowers [get]
func (h *CommitteeHandler) ListBorrowersByMonth(c *fiber.Ctx) error {
	membNo, _ := c.Locals("membNo").(string)

	now := time.Now()
	year, _ := strconv.Atoi(c.Query("year", strconv.Itoa(now.Year())))
	month, _ := strconv.Atoi(c.Query("month", strconv.Itoa(int(now.Month()))))
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	result, err := h.committeeService.ListBorrowersByMonth(c.Context(), membNo, year, month, page, limit)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrNotCommitteeMember):
			return response.Forbidden(c, "You are not an active committee member")
		case errors.Is(err, services.ErrInvalidMonth):
			return response.BadRequest(c, "Month must be between 1 and 12")
		default:
			return response.InternalServerError(c, "Failed to list borrowers")
		}
	}

	return response.Success(c, "Borrowers retrieved successfully", result)
}
