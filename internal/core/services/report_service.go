package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// ErrInvalidReportMonth is returned when the requested month is out of range.
var ErrInvalidReportMonth = errors.New("month must be between 1 and 12")

// ReportService produces the monthly step-by-step report (รายงานประจำเดือน)
// used by the Officer/Admin "รายงาน" page.
type ReportService struct {
	db *gorm.DB
}

func NewReportService(db *gorm.DB) *ReportService {
	return &ReportService{db: db}
}

// ============================================================
// DTOs
// ============================================================

// ReportStep is one of the 6 master steps, echoed back so the frontend can
// build its columns from the DB instead of hard-coding step codes.
type ReportStep struct {
	ID        uint   `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	StepOrder int    `json:"step_order"`
	IsFinal   bool   `json:"is_final"`
}

// ReportStepSummary is the month total for one step.
//
//	Entered      = จำนวนครั้งที่คำขอ "เข้าสู่" ขั้นตอนนี้ระหว่างเดือน (นับจาก transactions)
//	MonthCurrent = คำขอที่ยื่นในเดือนนี้ และ ณ ตอนนี้ค้างอยู่ขั้นตอนนี้
//	CurrentAll   = คำขอทั้งระบบ (ทุกเดือน) ที่ ณ ตอนนี้ค้างอยู่ขั้นตอนนี้
type ReportStepSummary struct {
	StepID       uint   `json:"step_id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Color        string `json:"color"`
	StepOrder    int    `json:"step_order"`
	Entered      int64  `json:"entered"`
	MonthCurrent int64  `json:"month_current"`
	CurrentAll   int64  `json:"current_all"`
}

// ReportDailyRow is one calendar day of the month.
// Steps is keyed by step code, e.g. {"RECEIVED": 3, "APPROVED": 1}.
type ReportDailyRow struct {
	Day         int              `json:"day"`
	Date        string           `json:"date"`
	NewRequests int64            `json:"new_requests"`
	NewAmount   float64          `json:"new_amount"`
	Steps       map[string]int64 `json:"steps"`
	Total       int64            `json:"total"`
}

// ReportOfficerRow is the per-officer breakdown for the month.
type ReportOfficerRow struct {
	OfficerID   uint    `json:"officer_id"`
	Username    string  `json:"username"`
	FullName    string  `json:"full_name"`
	NewRequests int64   `json:"new_requests"`
	Amount      float64 `json:"amount"`
	Approved    int64   `json:"approved"`
	Rejected    int64   `json:"rejected"`
	Completed   int64   `json:"completed"`
	InProgress  int64   `json:"in_progress"`
}

// ReportSummary is the headline block at the top of the report.
type ReportSummary struct {
	NewRequests     int64   `json:"new_requests"`
	NewAmount       float64 `json:"new_amount"`
	ApprovedCount   int64   `json:"approved_count"`
	ApprovedAmount  float64 `json:"approved_amount"`
	RejectedCount   int64   `json:"rejected_count"`
	CompletedCount  int64   `json:"completed_count"`
	InProgressCount int64   `json:"in_progress_count"`
	TotalMovements  int64   `json:"total_movements"`
}

// MonthlyStepReport is the whole payload for one month.
type MonthlyStepReport struct {
	Year        int                 `json:"year"`
	Month       int                 `json:"month"`
	MonthLabel  string              `json:"month_label"`
	PeriodStart string              `json:"period_start"`
	PeriodEnd   string              `json:"period_end"`
	DaysInMonth int                 `json:"days_in_month"`
	OfficerID   *uint               `json:"officer_id,omitempty"`
	Steps       []ReportStep        `json:"steps"`
	Summary     ReportSummary       `json:"summary"`
	StepSummary []ReportStepSummary `json:"step_summary"`
	Daily       []ReportDailyRow    `json:"daily"`
	ByOfficer   []ReportOfficerRow  `json:"by_officer"`
	GeneratedAt time.Time           `json:"generated_at"`
}

var thaiMonths = [...]string{
	"มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน", "พฤษภาคม", "มิถุนายน",
	"กรกฎาคม", "สิงหาคม", "กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม",
}

// ============================================================
// Main query
// ============================================================

// GetMonthlyStepReport builds the report for year/month, optionally narrowed
// to a single officer. Day boundaries follow the DB's own clock, same as the
// committee borrower-list view, so both screens agree on "this month".
func (s *ReportService) GetMonthlyStepReport(ctx context.Context, year, month int, officerID *uint) (*MonthlyStepReport, error) {
	if month < 1 || month > 12 {
		return nil, ErrInvalidReportMonth
	}
	if year < 2000 || year > 2200 {
		return nil, ErrInvalidReportMonth
	}

	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	daysInMonth := first.AddDate(0, 1, -1).Day()

	report := &MonthlyStepReport{
		Year:        year,
		Month:       month,
		MonthLabel:  fmt.Sprintf("%s %d", thaiMonths[month-1], year+543),
		PeriodStart: first.Format("2006-01-02"),
		PeriodEnd:   first.AddDate(0, 1, -1).Format("2006-01-02"),
		DaysInMonth: daysInMonth,
		OfficerID:   officerID,
		GeneratedAt: time.Now(),
	}

	// --- 1. Master steps (columns of the report) ---
	var steps []models.LoanStep
	if err := s.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("step_order ASC").
		Find(&steps).Error; err != nil {
		return nil, err
	}

	report.Steps = make([]ReportStep, len(steps))
	stepCodeByID := make(map[uint]string, len(steps))
	for i, st := range steps {
		report.Steps[i] = ReportStep{
			ID:        st.ID,
			Code:      st.Code,
			Name:      st.Name,
			Color:     st.Color,
			StepOrder: st.StepOrder,
			IsFinal:   st.IsFinal,
		}
		stepCodeByID[st.ID] = st.Code
	}

	// --- 2. Movements into each step, per day ---
	// One row per (day, target step). A mortgage that moves RECEIVED -> SURVEY
	// on the 5th counts once under SURVEY on day 5; the CREATE transaction
	// counts under RECEIVED on the day it was filed.
	type movementRow struct {
		Day    int
		StepID uint
		Cnt    int64
	}
	var movements []movementRow

	movQ := s.db.WithContext(ctx).Table("transactions AS t").
		Select("DAY(t.created_at) AS day, t.to_step_id AS step_id, COUNT(*) AS cnt").
		Joins("JOIN mortgages m ON m.id = t.mortgage_id").
		Where("m.deleted_at IS NULL").
		Where("t.to_step_id IS NOT NULL").
		Where("YEAR(t.created_at) = ? AND MONTH(t.created_at) = ?", year, month).
		Group("DAY(t.created_at), t.to_step_id")
	if officerID != nil {
		movQ = movQ.Where("m.officer_id = ?", *officerID)
	}
	if err := movQ.Scan(&movements).Error; err != nil {
		return nil, err
	}

	// --- 3. New requests filed, per day ---
	type newRow struct {
		Day int
		Cnt int64
		Amt float64
	}
	var newRows []newRow

	newQ := s.db.WithContext(ctx).Table("mortgages").
		Select("DAY(created_at) AS day, COUNT(*) AS cnt, COALESCE(SUM(amount), 0) AS amt").
		Where("deleted_at IS NULL").
		Where("YEAR(created_at) = ? AND MONTH(created_at) = ?", year, month).
		Group("DAY(created_at)")
	if officerID != nil {
		newQ = newQ.Where("officer_id = ?", *officerID)
	}
	if err := newQ.Scan(&newRows).Error; err != nil {
		return nil, err
	}

	// --- 4. Where this month's requests stand right now ---
	type currentRow struct {
		StepID uint
		Cnt    int64
	}
	var monthCurrent []currentRow

	monthCurQ := s.db.WithContext(ctx).Table("mortgages").
		Select("current_step_id AS step_id, COUNT(*) AS cnt").
		Where("deleted_at IS NULL").
		Where("YEAR(created_at) = ? AND MONTH(created_at) = ?", year, month).
		Group("current_step_id")
	if officerID != nil {
		monthCurQ = monthCurQ.Where("officer_id = ?", *officerID)
	}
	if err := monthCurQ.Scan(&monthCurrent).Error; err != nil {
		return nil, err
	}

	// --- 5. Whole-system backlog per step (ทุกเดือน, สถานะ ณ ปัจจุบัน) ---
	var currentAll []currentRow
	curAllQ := s.db.WithContext(ctx).Table("mortgages").
		Select("current_step_id AS step_id, COUNT(*) AS cnt").
		Where("deleted_at IS NULL").
		Group("current_step_id")
	if officerID != nil {
		curAllQ = curAllQ.Where("officer_id = ?", *officerID)
	}
	if err := curAllQ.Scan(&currentAll).Error; err != nil {
		return nil, err
	}

	// --- 6. Approved amount within the month ---
	var approvedAmount float64
	apprQ := s.db.WithContext(ctx).Table("mortgages").
		Select("COALESCE(SUM(COALESCE(approved_amount, amount)), 0)").
		Where("deleted_at IS NULL").
		Where("approved_at IS NOT NULL").
		Where("YEAR(approved_at) = ? AND MONTH(approved_at) = ?", year, month)
	if officerID != nil {
		apprQ = apprQ.Where("officer_id = ?", *officerID)
	}
	if err := apprQ.Row().Scan(&approvedAmount); err != nil {
		return nil, err
	}

	// --- 7. Per-officer breakdown (คำขอที่ยื่นในเดือนนี้) ---
	var officerRows []ReportOfficerRow
	offQ := s.db.WithContext(ctx).Table("mortgages AS m").
		Select(`
			m.officer_id AS officer_id,
			COALESCE(u.username, '') AS username,
			COALESCE(u.full_name, '') AS full_name,
			COUNT(*) AS new_requests,
			COALESCE(SUM(m.amount), 0) AS amount,
			SUM(CASE WHEN s.code = 'APPROVED' THEN 1 ELSE 0 END) AS approved,
			SUM(CASE WHEN s.code = 'REJECTED' THEN 1 ELSE 0 END) AS rejected,
			SUM(CASE WHEN s.code = 'COMPLETED' THEN 1 ELSE 0 END) AS completed,
			SUM(CASE WHEN s.is_final = 0 THEN 1 ELSE 0 END) AS in_progress
		`).
		Joins("LEFT JOIN users u ON u.id = m.officer_id").
		Joins("LEFT JOIN loan_steps s ON s.id = m.current_step_id").
		Where("m.deleted_at IS NULL").
		Where("YEAR(m.created_at) = ? AND MONTH(m.created_at) = ?", year, month).
		Group("m.officer_id, u.username, u.full_name").
		Order("new_requests DESC")
	if officerID != nil {
		offQ = offQ.Where("m.officer_id = ?", *officerID)
	}
	if err := offQ.Scan(&officerRows).Error; err != nil {
		return nil, err
	}
	if officerRows == nil {
		officerRows = []ReportOfficerRow{}
	}
	report.ByOfficer = officerRows

	// ============================================================
	// Assemble
	// ============================================================

	// Daily grid: every day of the month is present, even with zero activity,
	// so the table lines up 1..31 without the frontend filling gaps.
	newByDay := make(map[int]newRow, len(newRows))
	for _, r := range newRows {
		newByDay[r.Day] = r
	}
	movementsByDay := make(map[int]map[string]int64, daysInMonth)
	enteredByStep := make(map[string]int64, len(steps))
	var totalMovements int64
	for _, m := range movements {
		code, ok := stepCodeByID[m.StepID]
		if !ok {
			continue // step was deactivated/deleted — skip rather than mislabel
		}
		if movementsByDay[m.Day] == nil {
			movementsByDay[m.Day] = make(map[string]int64, len(steps))
		}
		movementsByDay[m.Day][code] += m.Cnt
		enteredByStep[code] += m.Cnt
		totalMovements += m.Cnt
	}

	report.Daily = make([]ReportDailyRow, 0, daysInMonth)
	for d := 1; d <= daysInMonth; d++ {
		row := ReportDailyRow{
			Day:   d,
			Date:  time.Date(year, time.Month(month), d, 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
			Steps: make(map[string]int64, len(steps)),
		}
		if n, ok := newByDay[d]; ok {
			row.NewRequests = n.Cnt
			row.NewAmount = n.Amt
		}
		for _, st := range steps {
			c := movementsByDay[d][st.Code]
			row.Steps[st.Code] = c
			row.Total += c
		}
		report.Daily = append(report.Daily, row)
	}

	monthCurByStep := make(map[uint]int64, len(monthCurrent))
	for _, r := range monthCurrent {
		monthCurByStep[r.StepID] = r.Cnt
	}
	curAllByStep := make(map[uint]int64, len(currentAll))
	for _, r := range currentAll {
		curAllByStep[r.StepID] = r.Cnt
	}

	report.StepSummary = make([]ReportStepSummary, len(steps))
	for i, st := range steps {
		report.StepSummary[i] = ReportStepSummary{
			StepID:       st.ID,
			Code:         st.Code,
			Name:         st.Name,
			Color:        st.Color,
			StepOrder:    st.StepOrder,
			Entered:      enteredByStep[st.Code],
			MonthCurrent: monthCurByStep[st.ID],
			CurrentAll:   curAllByStep[st.ID],
		}
	}

	summary := ReportSummary{
		ApprovedAmount: approvedAmount,
		TotalMovements: totalMovements,
	}
	for _, r := range report.Daily {
		summary.NewRequests += r.NewRequests
		summary.NewAmount += r.NewAmount
	}
	for _, st := range report.StepSummary {
		switch st.Code {
		case "APPROVED":
			summary.ApprovedCount = st.Entered
		case "REJECTED":
			summary.RejectedCount = st.Entered
		case "COMPLETED":
			summary.CompletedCount = st.Entered
		}
	}
	for _, o := range report.ByOfficer {
		summary.InProgressCount += o.InProgress
	}
	report.Summary = summary

	return report, nil
}
