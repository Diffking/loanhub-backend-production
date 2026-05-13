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
	savingsRepo     repositories.SavingsRepository
}

// NewLoanPrintHandler creates a new loan print handler.
func NewLoanPrintHandler(
	memberRepo repositories.MemberRepository,
	loanPurposeRepo *repositories.LoanPurposeRepository,
	appCounterRepo *repositories.AppCounterRepository,
	savingsRepo     repositories.SavingsRepository,
) *LoanPrintHandler {
	return &LoanPrintHandler{
		memberRepo:      memberRepo,
		loanPurposeRepo: loanPurposeRepo,
		appCounterRepo:  appCounterRepo,
		savingsRepo:     savingsRepo,
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
	MastPrindAmt   float64 `json:"mast_prind_amt"`
	MemberTypeCode string  `json:"member_type_code"`
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
		MastPrindAmt:   m.MastPrindAmt,
		MemberTypeCode: m.MemberTypeCode,
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
		"formatted":  requestNo,
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
		"formatted":  requestNo,
		"request_no": requestNo,
		"seq":        seq,
		"year":       year,
	})
}

// ============================================================
// Phase 3b: Collateral endpoint (shares + savings + 95% cap)
// ============================================================

// CollateralCapPct — สัดส่วนสูงสุดที่ใช้ค้ำได้ (95% ของมูลค่าหุ้นหรือเงินฝาก).
const CollateralCapPct = 0.95

// ShareCollateralInfo — ทุนเรือนหุ้น
type ShareCollateralInfo struct {
	Value         float64 `json:"value"`           // ทุนเรือนหุ้น (mast_prind_amt)
	MaxCollateral float64 `json:"max_collateral"`  // 95% ของ Value
}

// SavingsCollateralInfo — บัญชีเงินฝากต่อรายการ
type SavingsCollateralInfo struct {
	AccountNo     string  `json:"account_no"`
	FullName      string  `json:"full_name"`
	Balance       float64 `json:"balance"`
	MaxCollateral float64 `json:"max_collateral"` // 95% ของ Balance
}

// CollateralResponse — รวมข้อมูลค้ำประกันทั้งหมดของสมาชิก
type CollateralResponse struct {
	MembNo             string                  `json:"memb_no"`
	Shares             ShareCollateralInfo     `json:"shares"`
	Savings            []SavingsCollateralInfo `json:"savings"`
	TotalSavings       float64                 `json:"total_savings"`
	TotalMaxCollateral float64                 `json:"total_max_collateral"`
	CapPct             float64                 `json:"cap_pct"`
}

// GetCollateral — GET /api/v1/loan-print/collateral/:memb_no
// คืนข้อมูลค้ำประกัน (ทุนเรือนหุ้น + บัญชีเงินฝาก) พร้อมเพดาน 95% ที่ใช้ค้ำได้
func (h *LoanPrintHandler) GetCollateral(c *fiber.Ctx) error {
	membNo := c.Params("memb_no")
	if membNo == "" {
		return response.BadRequest(c, "memb_no is required")
	}

	ctx := c.Context()

	// 1. Get member (for mast_prind_amt)
	m, err := h.memberRepo.GetFullByMembNo(ctx, membNo)
	if err != nil {
		return response.InternalServerError(c, "failed to fetch member: "+err.Error())
	}
	if m == nil {
		return response.NotFound(c, "member not found")
	}

	// 2. Get savings accounts
	accounts, err := h.savingsRepo.GetByMembNo(ctx, membNo)
	if err != nil {
		return response.InternalServerError(c, "failed to fetch savings: "+err.Error())
	}

	// 3. Build response with 95% caps
	shares := ShareCollateralInfo{
		Value:         m.MastPrindAmt,
		MaxCollateral: roundFloat(m.MastPrindAmt*CollateralCapPct, 2),
	}

	savings := make([]SavingsCollateralInfo, 0, len(accounts))
	var totalBalance float64
	var totalMaxCollateral float64
	totalMaxCollateral += shares.MaxCollateral

	for _, a := range accounts {
		maxC := roundFloat(a.Balance*CollateralCapPct, 2)
		savings = append(savings, SavingsCollateralInfo{
			AccountNo:     a.AccountNo,
			FullName:      a.FullName,
			Balance:       a.Balance,
			MaxCollateral: maxC,
		})
		totalBalance += a.Balance
		totalMaxCollateral += maxC
	}

	return response.Success(c, "Collateral fetched", CollateralResponse{
		MembNo:             membNo,
		Shares:             shares,
		Savings:            savings,
		TotalSavings:       roundFloat(totalBalance, 2),
		TotalMaxCollateral: roundFloat(totalMaxCollateral, 2),
		CapPct:             CollateralCapPct,
	})
}

// roundFloat rounds a float to N decimal places
func roundFloat(val float64, precision int) float64 {
	pow := 1.0
	for i := 0; i < precision; i++ {
		pow *= 10
	}
	return float64(int(val*pow+0.5)) / pow
}
