package repositories

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// ============================================================
// Flommast Import — รับ .sql จาก MS SQL Server (BACKUP from FLOMMAST query)
// แปลงเป็น MySQL upsert + diff preview
// ============================================================

// FlommastImportRepository handles flommast bulk import operations
type FlommastImportRepository struct {
	db *gorm.DB
}

// NewFlommastImportRepository creates a new import repository
func NewFlommastImportRepository(db *gorm.DB) *FlommastImportRepository {
	return &FlommastImportRepository{db: db}
}

// ImportRow represents one parsed row from the uploaded .sql
// Phase 3c: 16 columns including MastTel.
// Fix (perd-swap): SQL export ใหม่ใช้ลำดับ col 5 = MAST_PAID_PERD (int, → mast_paid_time)
//                  col 6 = MAST_PAID_AMT (decimal). ก่อนหน้านี้ regex สลับลำดับทำให้ทุก row fail
//                  เพราะ \d+ ไม่ match decimal point ใน 1037070.0000
type ImportRow struct {
	MastMembNo   string  `json:"mast_memb_no"`
	FullName     string  `json:"full_name"`
	MastBirthYmd string  `json:"mast_birth_ymd"`
	MastCardId   string  `json:"mast_card_id"`
	MastPaidAmt  float64 `json:"mast_paid_amt"`
	MastPaidTime int     `json:"mast_paid_time"`
	MastSalary   float64 `json:"mast_salary"`
	MastMembDept string  `json:"mast_memb_dept"`
	StsTypeDesc  string  `json:"sts_type_desc"`
	MastPosition string  `json:"mast_position"`
	DeptName     string  `json:"dept_name"`
	Addr         string  `json:"addr"`
	MastTel      string  `json:"mast_tel"`
	MastMobile   string  `json:"mast_mobile"`
	MastAccNo    string  `json:"mast_acc_no"`
	MastBankAcno string  `json:"mast_bank_acno"`
}

// ToFlommast converts an ImportRow to Flommast model
func (r *ImportRow) ToFlommast() *models.Flommast {
	return &models.Flommast{
		MastMembNo:   r.MastMembNo,
		FullName:     r.FullName,
		MastBirthYmd: r.MastBirthYmd,
		MastCardId:   r.MastCardId,
		MastPaidAmt:  r.MastPaidAmt,
		MastPaidTime: r.MastPaidTime,
		MastSalary:   r.MastSalary,
		MastMembDept: r.MastMembDept,
		StsTypeDesc:  r.StsTypeDesc,
		MastPosition: r.MastPosition,
		DeptName:     r.DeptName,
		Addr:         r.Addr,
		MastTel:      r.MastTel,
		MastMobile:   r.MastMobile,
		MastAccNo:    r.MastAccNo,
		MastBankAcno: r.MastBankAcno,
	}
}

// DiffEntry represents one row in the diff preview
type DiffEntry struct {
	MastMembNo string        `json:"mast_memb_no"`
	FullName   string        `json:"full_name"`         // current name (or new if NEW)
	Status     string        `json:"status"`            // NEW | UPDATED | UNCHANGED | MISSING
	Changes    []FieldChange `json:"changes,omitempty"` // only for UPDATED
}

// FieldChange shows old → new for a single column
type FieldChange struct {
	Field string `json:"field"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

// DiffSummary aggregates counts for the preview UI
type DiffSummary struct {
	TotalUploaded  int          `json:"total_uploaded"`
	TotalInDB      int64        `json:"total_in_db"`
	NewCount       int          `json:"new_count"`
	UpdatedCount   int          `json:"updated_count"`
	UnchangedCount int          `json:"unchanged_count"`
	MissingCount   int          `json:"missing_count"` // in DB but not in uploaded file
	Entries        []*DiffEntry `json:"entries"`       // capped to MaxPreviewEntries
	Truncated      bool         `json:"truncated"`     // true if entries was capped
}

// MaxPreviewEntries caps the entries returned to the UI
const MaxPreviewEntries = 200

// ============================================================
// SQL Parser — รองรับ MS SQL Server INSERT format
// ============================================================

// insertRowPattern matches the 16-column SQL dump.
// SQL column order (จาก export ใหม่ของ MS SQL Server):
//
//	1: MAST_MEMB_NO       'string'
//	2: Full_Name          'string'
//	3: MAST_BIRTH_YMD     'string'
//	4: MAST_CARD_ID       'string'
//	5: MAST_PAID_PERD     int        → struct.MastPaidTime  (จำนวนงวด)
//	6: MAST_PAID_AMT      decimal    → struct.MastPaidAmt   (ทุนเรือนหุ้นสะสม)
//	7: MAST_SALARY        decimal
//	8: MAST_MEMB_DEPT     'string'
//	9: STS_TYPE_DESC      'string'
//	10: MAST_POSITION     'string'
//	11: DEPT_NAME         'string'
//	12: ADDR              'string'
//	13: MAST_TEL          'string'
//	14: MAST_MOBILE       'string'
//	15: MAST_ACC_NO       'string' or NULL
//	16: MAST_BANK_ACNO    'string' or NULL
var insertRowPattern = regexp.MustCompile(
	`(?s)VALUES\s*\(` +
		`\s*'([^']*)'` + // 1: MAST_MEMB_NO
		`\s*,\s*'((?:[^']|'')*)'` + // 2: Full_Name
		`\s*,\s*'([^']*)'` + // 3: MAST_BIRTH_YMD
		`\s*,\s*'([^']*)'` + // 4: MAST_CARD_ID
		`\s*,\s*(\d+|NULL)` + // 5: MAST_PAID_PERD (int) → mast_paid_time
		`\s*,\s*([\d.]+|NULL)` + // 6: MAST_PAID_AMT (decimal) → mast_paid_amt
		`\s*,\s*([\d.]+|NULL)` + // 7: MAST_SALARY
		`\s*,\s*'([^']*)'` + // 8: MAST_MEMB_DEPT
		`\s*,\s*'((?:[^']|'')*)'` + // 9: STS_TYPE_DESC
		`\s*,\s*'((?:[^']|'')*)'` + // 10: MAST_POSITION
		`\s*,\s*'((?:[^']|'')*)'` + // 11: DEPT_NAME
		`\s*,\s*'((?:[^']|'')*)'` + // 12: ADDR
		`\s*,\s*'([^']*)'` + // 13: MAST_TEL
		`\s*,\s*'([^']*)'` + // 14: MAST_MOBILE
		`\s*,\s*('[^']*'|NULL)` + // 15: MAST_ACC_NO
		`\s*,\s*('[^']*'|NULL)` + // 16: MAST_BANK_ACNO
		`\s*\)`,
)

// ParseSQL parses uploaded SQL bytes into a slice of ImportRow.
// Robust to MS SQL syntax: brackets, "GO" lines, NULL values, doubled single quotes inside strings.
func ParseSQL(sql string) ([]*ImportRow, []string, error) {
	matches := insertRowPattern.FindAllStringSubmatch(sql, -1)
	if len(matches) == 0 {
		return nil, nil, fmt.Errorf("no INSERT rows found — รองรับเฉพาะรูปแบบ flommast3.sql (16 columns)")
	}

	rows := make([]*ImportRow, 0, len(matches))
	warnings := []string{}
	seen := make(map[string]bool)

	for i, m := range matches {
		// Unescape doubled single quotes
		fullName := strings.ReplaceAll(m[2], "''", "'")
		stsType := strings.ReplaceAll(m[9], "''", "'")
		position := strings.ReplaceAll(m[10], "''", "'")
		deptName := strings.ReplaceAll(m[11], "''", "'")
		addr := strings.ReplaceAll(m[12], "''", "'")

		// Parse MAST_PAID_PERD (int/NULL) — SQL col 5 → mast_paid_time
		var paidTime int
		if m[5] != "NULL" {
			v, err := strconv.Atoi(m[5])
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("row %d (%s): invalid paid_perd '%s'", i+1, m[1], m[5]))
				continue
			}
			paidTime = v
		}

		// Parse MAST_PAID_AMT (decimal/NULL) — SQL col 6 → mast_paid_amt
		var paidAmt float64
		if m[6] != "NULL" {
			v, err := strconv.ParseFloat(m[6], 64)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("row %d (%s): invalid paid_amt '%s'", i+1, m[1], m[6]))
				continue
			}
			paidAmt = v
		}

		// Parse MAST_SALARY (decimal/NULL)
		var salary float64
		if m[7] != "NULL" {
			v, err := strconv.ParseFloat(m[7], 64)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("row %d (%s): invalid salary '%s'", i+1, m[1], m[7]))
				continue
			}
			salary = v
		}

		// Parse acc_no (NULL allowed)
		accNo := ""
		if m[15] != "NULL" {
			accNo = strings.Trim(m[15], "'")
		}

		// Parse bank account (NULL allowed)
		bankAcno := ""
		if m[16] != "NULL" {
			bankAcno = strings.Trim(m[16], "'")
		}

		row := &ImportRow{
			MastMembNo:   m[1],
			FullName:     strings.TrimSpace(fullName),
			MastBirthYmd: m[3],
			MastCardId:   m[4],
			MastPaidAmt:  paidAmt,
			MastPaidTime: paidTime,
			MastSalary:   salary,
			MastMembDept: strings.TrimSpace(m[8]),
			StsTypeDesc:  strings.TrimSpace(stsType),
			MastPosition: strings.TrimSpace(position),
			DeptName:     strings.TrimSpace(deptName),
			Addr:         strings.TrimSpace(addr),
			MastTel:      m[13],
			MastMobile:   m[14],
			MastAccNo:    accNo,
			MastBankAcno: bankAcno,
		}

		// Skip duplicates within the same file (MS SQL export quirk)
		if seen[row.MastMembNo] {
			warnings = append(warnings, fmt.Sprintf("duplicate memb_no in file: %s (kept first)", row.MastMembNo))
			continue
		}
		seen[row.MastMembNo] = true
		rows = append(rows, row)
	}

	return rows, warnings, nil
}

// ============================================================
// Diff: compare uploaded rows vs current DB
// ============================================================

// BuildDiff produces a summary by comparing uploaded rows vs the current flommast table.
func (r *FlommastImportRepository) BuildDiff(ctx context.Context, rows []*ImportRow) (*DiffSummary, error) {
	// Load all current flommast records keyed by memb_no
	var currentRows []models.Flommast
	if err := r.db.WithContext(ctx).Find(&currentRows).Error; err != nil {
		return nil, fmt.Errorf("query current flommast: %w", err)
	}
	currentMap := make(map[string]*models.Flommast, len(currentRows))
	for i := range currentRows {
		currentMap[currentRows[i].MastMembNo] = &currentRows[i]
	}

	// Index uploaded rows
	uploadedMap := make(map[string]*ImportRow, len(rows))
	for _, r := range rows {
		uploadedMap[r.MastMembNo] = r
	}

	summary := &DiffSummary{
		TotalUploaded: len(rows),
		TotalInDB:     int64(len(currentRows)),
		Entries:       make([]*DiffEntry, 0, MaxPreviewEntries),
	}

	// Walk uploaded — classify each as NEW / UPDATED / UNCHANGED
	for _, up := range rows {
		current, exists := currentMap[up.MastMembNo]
		if !exists {
			summary.NewCount++
			if !summary.Truncated && len(summary.Entries) < MaxPreviewEntries {
				summary.Entries = append(summary.Entries, &DiffEntry{
					MastMembNo: up.MastMembNo,
					FullName:   up.FullName,
					Status:     "NEW",
				})
			}
			continue
		}

		changes := compareRows(current, up)
		if len(changes) == 0 {
			summary.UnchangedCount++
		} else {
			summary.UpdatedCount++
			if !summary.Truncated && len(summary.Entries) < MaxPreviewEntries {
				summary.Entries = append(summary.Entries, &DiffEntry{
					MastMembNo: up.MastMembNo,
					FullName:   up.FullName,
					Status:     "UPDATED",
					Changes:    changes,
				})
			}
		}
	}

	// Walk current DB — find MISSING (exists in DB but not in upload)
	for membNo, cur := range currentMap {
		if _, exists := uploadedMap[membNo]; !exists {
			summary.MissingCount++
			if !summary.Truncated && len(summary.Entries) < MaxPreviewEntries {
				summary.Entries = append(summary.Entries, &DiffEntry{
					MastMembNo: cur.MastMembNo,
					FullName:   cur.FullName,
					Status:     "MISSING",
				})
			}
		}
	}

	if summary.NewCount+summary.UpdatedCount+summary.MissingCount > MaxPreviewEntries {
		summary.Truncated = true
	}

	return summary, nil
}

// compareRows returns a list of FieldChange for differing fields
func compareRows(current *models.Flommast, up *ImportRow) []FieldChange {
	var changes []FieldChange
	add := func(field, oldV, newV string) {
		if oldV != newV {
			changes = append(changes, FieldChange{Field: field, Old: oldV, New: newV})
		}
	}
	add("full_name", current.FullName, up.FullName)
	add("mast_birth_ymd", current.MastBirthYmd, up.MastBirthYmd)
	add("mast_card_id", current.MastCardId, up.MastCardId)
	add("mast_memb_dept", current.MastMembDept, up.MastMembDept)
	if current.MastPaidAmt != up.MastPaidAmt {
		changes = append(changes, FieldChange{
			Field: "mast_paid_amt",
			Old:   strconv.FormatFloat(current.MastPaidAmt, 'f', 2, 64),
			New:   strconv.FormatFloat(up.MastPaidAmt, 'f', 2, 64),
		})
	}
	if current.MastPaidTime != up.MastPaidTime {
		changes = append(changes, FieldChange{
			Field: "mast_paid_time",
			Old:   strconv.Itoa(current.MastPaidTime),
			New:   strconv.Itoa(up.MastPaidTime),
		})
	}
	add("sts_type_desc", current.StsTypeDesc, up.StsTypeDesc)
	add("mast_position", current.MastPosition, up.MastPosition)
	add("dept_name", current.DeptName, up.DeptName)
	add("addr", current.Addr, up.Addr)
	if current.MastSalary != up.MastSalary {
		changes = append(changes, FieldChange{
			Field: "mast_salary",
			Old:   strconv.FormatFloat(current.MastSalary, 'f', 2, 64),
			New:   strconv.FormatFloat(up.MastSalary, 'f', 2, 64),
		})
	}
	add("mast_mobile", current.MastMobile, up.MastMobile)
	add("mast_acc_no", current.MastAccNo, up.MastAccNo)
	add("mast_bank_acno", current.MastBankAcno, up.MastBankAcno)
	return changes
}

// ============================================================
// Apply: bulk upsert rows + (optionally) delete missing
// ============================================================

// ApplyOptions — control what to apply
type ApplyOptions struct {
	DeleteMissing bool // ลบสมาชิกที่ไม่มีในไฟล์ออกจาก DB หรือไม่
	BatchSize     int  // default 500
}

// ApplyResult summarizes the result of Apply()
type ApplyResult struct {
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
	Deleted  int `json:"deleted"`
}

// Apply persists rows to the flommast table.
// Uses ON DUPLICATE KEY UPDATE (MySQL upsert) inside batched transactions.
func (r *FlommastImportRepository) Apply(ctx context.Context, rows []*ImportRow, opts ApplyOptions) (*ApplyResult, error) {
	if opts.BatchSize == 0 {
		opts.BatchSize = 500
	}

	result := &ApplyResult{}

	// Build set of uploaded memb_nos for the missing-delete pass
	uploadedSet := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		uploadedSet[r.MastMembNo] = struct{}{}
	}

	// Run inside a single transaction for atomicity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) Count what's NEW vs UPDATED for the result (cheap, optional —
		//    we could rely on RowsAffected per batch but GORM upsert doesn't split it)
		//    Strategy: get list of existing memb_nos first.
		var existing []string
		if err := tx.Model(&models.Flommast{}).
			Pluck("mast_memb_no", &existing).Error; err != nil {
			return err
		}
		existingSet := make(map[string]struct{}, len(existing))
		for _, e := range existing {
			existingSet[e] = struct{}{}
		}
		for _, row := range rows {
			if _, exists := existingSet[row.MastMembNo]; exists {
				result.Updated++
			} else {
				result.Inserted++
			}
		}

		// 2) Batched upsert
		batch := make([]*models.Flommast, 0, opts.BatchSize)
		flush := func() error {
			if len(batch) == 0 {
				return nil
			}
			// GORM upsert via ON CONFLICT (MySQL: ON DUPLICATE KEY UPDATE)
			err := tx.Save(&batch).Error
			batch = batch[:0]
			return err
		}
		for _, row := range rows {
			batch = append(batch, row.ToFlommast())
			if len(batch) >= opts.BatchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
		if err := flush(); err != nil {
			return err
		}

		// 3) Optional: delete missing
		if opts.DeleteMissing {
			toDelete := make([]string, 0)
			for _, e := range existing {
				if _, exists := uploadedSet[e]; !exists {
					toDelete = append(toDelete, e)
				}
			}
			if len(toDelete) > 0 {
				// Hard delete (no soft-delete column on Flommast)
				res := tx.Where("mast_memb_no IN ?", toDelete).Delete(&models.Flommast{})
				if res.Error != nil {
					return res.Error
				}
				result.Deleted = int(res.RowsAffected)
				// Recompute Updated (rows we counted as existing but were going to delete still count as updated since they were in the upload)
				// (no adjustment needed — toDelete are NOT in uploadedSet, so they weren't in result.Updated anyway)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}
