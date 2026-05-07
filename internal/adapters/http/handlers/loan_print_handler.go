package handlers

import (
	"github.com/gofiber/fiber/v2"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"
	"spsc-loaneasy/internal/pkg/response"
)

// LoanPrintHandler handles the printable loan-application workflow.
//
// Phase 1 endpoints:
//   - GET  /api/v1/loan-print/members/search?q=&limit=
//   - GET  /api/v1/loan-print/members/:memb_no
//   - GET  /api/v1/loan-print/purposes
//
// Phase 3a endpoints:
//   - GET  /api/v1/loan-print/next-number    (peek; does not consume)
//   - POST /api/v1/loan-print/issue-number   (atomic increment)
type LoanPrintHandler struct {
	memberRepo      repositories.MemberRepository
	loanPurposeRepo *repositories.LoanPurposeRepository
	appCounterRepo  *repositories.AppCounterRepository
}

// NewLoanPrintHandler creates a new loan print handler.
func NewLoanPrintHandler(
	memberRepo repositories.MemberRepository,
	loanPurposeRepo *repositories.LoanPurposeRepository,
	appCounterRepo *repositories.AppCounterRepository,
) *LoanPrintHandler {
	return &LoanPrintHandler{
		memberRepo:      memberRepo,
		loanPurposeRepo: loanPurposeRepo,
		appCounterRepo:  appCounterRepo,
	}
}

// ============================================================
// Member endpoints
// ============================================================

// MemberListItem is the slim payload returned by the search endpoint.
type MemberListItem struct {
	MastMembNo  string `json:"mast_memb_no"`
	FullName    string `json:"full_name"`
	DeptName    string `json:"dept_name"`
	StsTypeDesc string `json:"sts_type_desc"`
}

// MemberFullResponse is the payload returned when an officer selects a member.
// Field names mirror Flommast model + JSON tags match what the frontend expects.
type MemberFullResponse struct {
	MastMembNo   string  `json:"mast_memb_no"`
	FullName     string  `json:"full_name"`
	Age          int     `json:"age"`
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

func toMemberFullResponse(m *models.Flommast) MemberFullResponse {
	return MemberFullResponse{
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

// SearchMembers — GET /api/v1/loan-print/members/search?q=&limit=
//
// Auth: OfficerOrAdmin.
//
// SearchActive filters out associate members
// (สมทบ / บุตรสมาชิก / คู่สมรส / บิดา มารดาสมาชิก) automatically.
func (h *LoanPrintHandler) SearchMembers(c *fiber.Ctx) error {
	query := c.Query("q", "")
	limit := c.QueryInt("limit", 20)
	if limit < 1 || limit > 100 {
		limit = 20
	}

	members, err := h.memberRepo.SearchActive(c.Context(), query, limit)
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

	return response.Success(c, "Members found", fiber.Map{
		"items": items,
		"total": len(items),
	})
}

// GetMember — GET /api/v1/loan-print/members/:memb_no
//
// Auth: OfficerOrAdmin.
//
// Returns full Flommast data for the form. Returns 404 if the member
// is not found OR is an associate type (cannot apply for a regular loan).
func (h *LoanPrintHandler) GetMember(c *fiber.Ctx) error {
	membNo := c.Params("memb_no")
	if membNo == "" {
		return response.BadRequest(c, "Member number required")
	}

	m, err := h.memberRepo.GetFullByMembNo(c.Context(), membNo)
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch member")
	}
	if m == nil {
		return response.NotFound(c, "ไม่พบสมาชิก หรือ สมาชิกประเภทนี้ไม่สามารถยื่นคำขอกู้สามัญได้")
	}

	return response.Success(c, "Member found", toMemberFullResponse(m))
}

// ListPurposes — GET /api/v1/loan-print/purposes
//
// Auth: OfficerOrAdmin.
//
// Returns all active loan purposes (FLOPRESN codes) sorted by code.
func (h *LoanPrintHandler) ListPurposes(c *fiber.Ctx) error {
	purposes, err := h.loanPurposeRepo.ListAll(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Failed to list purposes")
	}
	return response.Success(c, "OK", fiber.Map{
		"purposes": purposes,
		"total":    len(purposes),
	})
}

// ============================================================
// Phase 3a: Auto-numbering endpoints
// ============================================================

// PeekNextNumber — GET /api/v1/loan-print/next-number
//
// Auth: OfficerOrAdmin.
//
// Returns what the next request number WOULD be without consuming it.
// Used by the form to display "เลขที่คำขอ: 00042/2569" preview.
//
// Response:
//
//	{
//	  "request_no": "00042/2569",
//	  "seq": 42,
//	  "year": 2569
//	}
func (h *LoanPrintHandler) PeekNextNumber(c *fiber.Ctx) error {
	requestNo, seq, year, err := h.appCounterRepo.PeekNext(c.Context(), models.AppCounterKindLoanPrint)
	if err != nil {
		return response.InternalServerError(c, "Failed to peek next request number")
	}

	return response.Success(c, "Next number", fiber.Map{
		"request_no": requestNo,
		"seq":        seq,
		"year":       year,
	})
}

// IssueNextNumber — POST /api/v1/loan-print/issue-number
//
// Auth: OfficerOrAdmin.
//
// Atomically increments the counter and returns the issued number.
// Called when officer clicks "Print" — guarantees no duplicate numbers
// even with concurrent users.
//
// Response:
//
//	{
//	  "request_no": "00043/2569",
//	  "seq": 43,
//	  "year": 2569
//	}
func (h *LoanPrintHandler) IssueNextNumber(c *fiber.Ctx) error {
	requestNo, seq, year, err := h.appCounterRepo.IssueNext(c.Context(), models.AppCounterKindLoanPrint)
	if err != nil {
		return response.InternalServerError(c, "Failed to issue next request number")
	}

	return response.Success(c, "Number issued", fiber.Map{
		"request_no": requestNo,
		"seq":        seq,
		"year":       year,
	})
}
