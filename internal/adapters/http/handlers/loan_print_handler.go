package handlers

import (
	"errors"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// LoanPrintHandler handles endpoints for the "Print Loan Application" feature.
// Officers/Admins can search eligible members, fetch full FLOMMAST data,
// and list loan purposes (FLOPRESN) — all read-only.
type LoanPrintHandler struct {
	memberRepo      repositories.MemberRepository
	loanPurposeRepo *repositories.LoanPurposeRepository
}

// NewLoanPrintHandler creates a new loan print handler
func NewLoanPrintHandler(
	memberRepo repositories.MemberRepository,
	loanPurposeRepo *repositories.LoanPurposeRepository,
) *LoanPrintHandler {
	return &LoanPrintHandler{
		memberRepo:      memberRepo,
		loanPurposeRepo: loanPurposeRepo,
	}
}

// ============================================================
// Member endpoints
// ============================================================

// MemberListItem — slim payload for the search panel
type MemberListItem struct {
	MastMembNo  string `json:"mast_memb_no"`
	FullName    string `json:"full_name"`
	DeptName    string `json:"dept_name"`
	StsTypeDesc string `json:"sts_type_desc"`
}

// MemberFullResponse — payload for the form (data dropped into the printable form)
type MemberFullResponse struct {
	MastMembNo   string  `json:"mast_memb_no"`
	FullName     string  `json:"full_name"`
	Age          int     `json:"age"` // computed from MAST_BIRTH_YMD
	MastBirthYmd string  `json:"mast_birth_ymd"`
	MastCardId   string  `json:"mast_card_id"`
	StsTypeDesc  string  `json:"sts_type_desc"`
	MastPosition string  `json:"mast_position"`
	DeptName     string  `json:"dept_name"`
	Addr         string  `json:"addr"`
	MastSalary   float64 `json:"mast_salary"`
	MastMobile   string  `json:"mast_mobile"`
	MastAccNo    string  `json:"mast_acc_no"`
	MastBankAcno string  `json:"mast_bank_acno"`
}

func toMemberFullResponse(m *models.Flommast) *MemberFullResponse {
	return &MemberFullResponse{
		MastMembNo:   m.MastMembNo,
		FullName:     m.FullName,
		Age:          m.Age(),
		MastBirthYmd: m.MastBirthYmd,
		MastCardId:   m.MastCardId,
		StsTypeDesc:  m.StsTypeDesc,
		MastPosition: m.MastPosition,
		DeptName:     m.DeptName,
		Addr:         m.Addr,
		MastSalary:   m.MastSalary,
		MastMobile:   m.MastMobile,
		MastAccNo:    m.MastAccNo,
		MastBankAcno: m.MastBankAcno,
	}
}

// SearchMembers searches eligible members for the loan print form.
//
//	GET /api/v1/loan-print/members/search?q=<keyword>&limit=20
//
// Auth: OfficerOrAdmin
func (h *LoanPrintHandler) SearchMembers(c *fiber.Ctx) error {
	q := c.Query("q", "")
	limit := c.QueryInt("limit", 20)
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	members, err := h.memberRepo.SearchActive(c.Context(), q, limit)
	if err != nil {
		return response.InternalServerError(c, "Failed to search members")
	}

	items := make([]MemberListItem, 0, len(members))
	for _, m := range members {
		items = append(items, MemberListItem{
			MastMembNo:  m.MastMembNo,
			FullName:    m.FullName,
			DeptName:    m.DeptName,
			StsTypeDesc: m.StsTypeDesc,
		})
	}

	return response.Success(c, "Members retrieved", fiber.Map{
		"members": items,
		"count":   len(items),
	})
}

// GetMember returns full member data for filling the form.
//
//	GET /api/v1/loan-print/members/:memb_no
//
// Auth: OfficerOrAdmin
// Returns 404 if memb_no doesn't exist OR is ineligible (สมทบ/บุตร/etc).
func (h *LoanPrintHandler) GetMember(c *fiber.Ctx) error {
	membNo := c.Params("memb_no")
	if membNo == "" {
		return response.BadRequest(c, "memb_no is required")
	}

	member, err := h.memberRepo.GetFullByMembNo(c.Context(), membNo)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบสมาชิก หรือ สมาชิกประเภทนี้ไม่สามารถยื่นคำขอกู้สามัญได้")
		}
		return response.InternalServerError(c, "Failed to fetch member")
	}

	return response.Success(c, "Member retrieved", fiber.Map{
		"member": toMemberFullResponse(member),
	})
}

// ============================================================
// Loan Purpose endpoints
// ============================================================

// ListPurposes returns all active loan purposes (for dropdown in form).
//
//	GET /api/v1/loan-print/purposes
//
// Auth: OfficerOrAdmin
func (h *LoanPrintHandler) ListPurposes(c *fiber.Ctx) error {
	purposes, err := h.loanPurposeRepo.List(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Failed to list loan purposes")
	}
	return response.Success(c, "Loan purposes retrieved", fiber.Map{
		"purposes": purposes,
		"count":    len(purposes),
	})
}
