#!/bin/bash
# Phase 3b Chunk 2 — apply patches to existing Go files
# Run from /var/www/loaneasy/

set -e
cd /var/www/loaneasy

TS=$(date +%Y%m%d-%H%M%S)
mkdir -p backups/phase3b-${TS}

echo "=== A. Backup files before patching ==="
cp internal/adapters/persistence/models/models.go            backups/phase3b-${TS}/models.go.bak
cp internal/adapters/persistence/repositories/interfaces.go  backups/phase3b-${TS}/interfaces.go.bak
cp internal/adapters/http/handlers/loan_print_handler.go     backups/phase3b-${TS}/loan_print_handler.go.bak
cp internal/adapters/http/routes/routes.go                   backups/phase3b-${TS}/routes.go.bak
ls -la backups/phase3b-${TS}/
echo ""

# ============================================================
# 1) models.go — add 2 fields to Flommast + new SavingsAccount + AutoMigrate
# ============================================================
echo "=== B. Patch models.go ==="
F=internal/adapters/persistence/models/models.go

# B1. Add MastPrindAmt + MemberTypeCode after MastBankAcno (before UpdatedAt)
# Use perl for multi-line aware insert
perl -i -pe 's|(MastBankAcno string\s+`gorm:"column:mast_bank_acno;size:30" json:"mast_bank_acno"`)|$1\n\t// 🆕 Phase 3b: Loan collateral\n\tMastPrindAmt   float64 `gorm:"column:mast_prind_amt;type:decimal(13,2);default:0.00" json:"mast_prind_amt"`\n\tMemberTypeCode string  `gorm:"column:member_type_code;size:10;default:'\'\'';index" json:"member_type_code"`|' $F

# B2. Add &SavingsAccount{} to AutoMigrate
perl -i -pe 's|(// Phase 3a: Auto-numbering\s*\n\s*&AppCounter\{\},)|$1\n\t\t// Phase 3b: Loan collateral (savings)\n\t\t\&SavingsAccount\{\},|' $F

# B3. Append SavingsAccount struct at end of file
cat >> $F << 'GOAPPEND'

// ============================================================
// Phase 3b: Savings accounts (for loan collateral)
// ============================================================

// SavingsAccount — บัญชีเงินฝากของสมาชิก ใช้สำหรับค้ำประกันเงินกู้.
// Phase 3b: ค้ำประกันด้วยเงินฝากออมทรัพย์ — ใช้ได้ไม่เกิน 95% ของยอดคงเหลือ.
//
// Indexes:
//   - mast_memb_no (lookup)
//   - (mast_memb_no, account_no) UNIQUE — ป้องกันบัญชีซ้ำ
type SavingsAccount struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	MastMembNo string    `gorm:"column:mast_memb_no;size:20;not null;index;uniqueIndex:uk_member_account,priority:1" json:"mast_memb_no"`
	FullName   string    `gorm:"column:full_name;size:255;not null;default:''" json:"full_name"`
	AccountNo  string    `gorm:"column:account_no;size:20;not null;uniqueIndex:uk_member_account,priority:2" json:"account_no"`
	Balance    float64   `gorm:"column:balance;type:decimal(15,4);not null;default:0" json:"balance"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SavingsAccount) TableName() string {
	return "savings_accounts"
}
GOAPPEND

echo "✓ models.go patched"

# ============================================================
# 2) interfaces.go — append SavingsRepository interface
# ============================================================
echo ""
echo "=== C. Patch interfaces.go ==="
F=internal/adapters/persistence/repositories/interfaces.go

cat >> $F << 'GOAPPEND'

// SavingsRepository defines savings_accounts repository interface.
// Phase 3b: ใช้ดึงข้อมูลบัญชีเงินฝากของสมาชิกเพื่อคำนวณค้ำประกันเงินกู้ (cap 95%).
type SavingsRepository interface {
	GetByMembNo(ctx context.Context, membNo string) ([]*models.SavingsAccount, error)
	CountByMembNo(ctx context.Context, membNo string) (int64, error)
	TotalBalance(ctx context.Context, membNo string) (float64, error)
}
GOAPPEND

echo "✓ interfaces.go patched"

# ============================================================
# 3) loan_print_handler.go — add savingsRepo field + GetCollateral method
# ============================================================
echo ""
echo "=== D. Patch loan_print_handler.go ==="
F=internal/adapters/http/handlers/loan_print_handler.go

# D1. Add savingsRepo field to LoanPrintHandler struct
perl -i -pe 's|(appCounterRepo \*repositories\.AppCounterRepository)|$1\n\tsavingsRepo     repositories.SavingsRepository|' $F

# D2. Add savingsRepo param to NewLoanPrintHandler signature + struct init
perl -i -pe 's|(appCounterRepo \*repositories\.AppCounterRepository,)\n(\) \*LoanPrintHandler \{)|$1\n\tsavingsRepo repositories.SavingsRepository,\n$2|' $F
perl -i -pe 's|(appCounterRepo:  appCounterRepo,)|$1\n\t\tsavingsRepo:     savingsRepo,|' $F

# D3. Add MastPrindAmt + MemberTypeCode to MemberFullResponse
perl -i -pe 's|(MastBankAcno string  `json:"mast_bank_acno"`)|$1\n\tMastPrindAmt   float64 `json:"mast_prind_amt"`\n\tMemberTypeCode string  `json:"member_type_code"`|' $F

# D4. Add 2 lines to MemberFullResponse mapping (after MastBankAcno mapping)
perl -i -pe 's|(MastBankAcno: m\.MastBankAcno,)|$1\n\t\tMastPrindAmt:   m.MastPrindAmt,\n\t\tMemberTypeCode: m.MemberTypeCode,|' $F

# D5. Append GetCollateral handler at end of file
cat >> $F << 'GOAPPEND'

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
		return response.InternalError(c, "failed to fetch member: "+err.Error())
	}
	if m == nil {
		return response.NotFound(c, "member not found")
	}

	// 2. Get savings accounts
	accounts, err := h.savingsRepo.GetByMembNo(ctx, membNo)
	if err != nil {
		return response.InternalError(c, "failed to fetch savings: "+err.Error())
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
GOAPPEND

echo "✓ loan_print_handler.go patched"

# ============================================================
# 4) routes.go — wire SavingsRepository + add /collateral route
# ============================================================
echo ""
echo "=== E. Patch routes.go ==="
F=internal/adapters/http/routes/routes.go

# E1. Insert savingsRepo init line BEFORE NewLoanPrintHandler call
perl -i -pe 's|(loanPrintHandler := handlers\.NewLoanPrintHandler\(memberRepo, loanPurposeRepo, appCounterRepo\))|savingsRepo := repositories.NewSavingsRepository(db)\n\tloanPrintHandler := handlers.NewLoanPrintHandler(memberRepo, loanPurposeRepo, appCounterRepo, savingsRepo)|' $F

# E2. Add /collateral route in setupLoanPrintRoutes
perl -i -pe 's|(router\.Post\("/issue-number", handler\.IssueNextNumber\))|$1\n\t// Phase 3b: Collateral endpoint\n\trouter.Get("/collateral/:memb_no", handler.GetCollateral)|' $F

echo "✓ routes.go patched"

# ============================================================
# 5) Copy new file
# ============================================================
echo ""
echo "=== F. Copy savings_repository.go (NEW) ==="
cp phase3b/chunk2/new_files/savings_repository.go internal/adapters/persistence/repositories/savings_repository.go
ls -la internal/adapters/persistence/repositories/savings_repository.go

echo ""
echo "=== G. Verify each patched file with grep ==="
echo "--- models.go ---"
grep -c "MastPrindAmt\|SavingsAccount\|MemberTypeCode" internal/adapters/persistence/models/models.go && echo "(should be ≥4)"
echo "--- interfaces.go ---"
grep -c "SavingsRepository" internal/adapters/persistence/repositories/interfaces.go && echo "(should be ≥2)"
echo "--- handler ---"
grep -c "savingsRepo\|GetCollateral\|CollateralCapPct" internal/adapters/http/handlers/loan_print_handler.go && echo "(should be ≥5)"
echo "--- routes ---"
grep -c "savingsRepo\|/collateral/" internal/adapters/http/routes/routes.go && echo "(should be ≥2)"

echo ""
echo "=== H. Build (1-3 นาที) ==="
docker compose build app 2>&1 | tail -10

