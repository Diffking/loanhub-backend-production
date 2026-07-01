package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// FlommastSyncHandler — endpoints for MSSQL sync agent + admin UI history.
//
//	POST   /api/v1/admin/flommast/sync           (API key)   — agent push
//	GET    /api/v1/admin/flommast/sync-history   (JWT/Admin) — recent runs for UI
//	GET    /api/v1/admin/flommast/sync-status    (JWT/Admin) — latest + next scheduled
//	GET    /api/v1/admin/flommast/missing        (JWT/Admin) — members in DB but missing from latest sync
//	DELETE /api/v1/admin/flommast/missing        (JWT/Admin) — remove selected missing members
type FlommastSyncHandler struct {
	importRepo *repositories.FlommastImportRepository
	db         *gorm.DB
}

// NewFlommastSyncHandler wires a new sync handler.
func NewFlommastSyncHandler(
	importRepo *repositories.FlommastImportRepository,
	db *gorm.DB,
) *FlommastSyncHandler {
	return &FlommastSyncHandler{importRepo: importRepo, db: db}
}

// MaxSyncRows guards against accidental floods (real data ~6k rows).
const MaxSyncRows = 50000

// ──────────────────────── Payload types ────────────────────────

// SyncRequest matches the JSON body posted by sync-agent (Phase 1 + Phase 3A extras).
type SyncRequest struct {
	Source        string               `json:"source"`
	AgentVersion  string               `json:"agent_version"`
	AgentHost     string               `json:"agent_host"`
	PublicIP      string               `json:"public_ip"`
	DeleteMissing bool                 `json:"delete_missing"`
	RowCount      int                  `json:"row_count"`
	Rows          []*ExtendedImportRow `json:"rows"`
}

// ExtendedImportRow = ImportRow (16 fields used by Apply) + 2 fields for Loan Print.
// Phase 3A: agent ships member_type_code (= MAST_MEMB_TYPE) per Loan Print spec.
// This is NOT in Apply's upsert SQL → must be UPDATEd after Apply.
type ExtendedImportRow struct {
	repositories.ImportRow
	MemberTypeCode string `json:"member_type_code"`
}

// DeleteMissingRequest — body of DELETE /missing
type DeleteMissingRequest struct {
	MembNos []string `json:"memb_nos"` // memb_nos to remove from flommast table
}

// MissingMember — return shape for GET /missing (JOIN flommast)
type MissingMember struct {
	MastMembNo   string    `json:"mast_memb_no"   gorm:"column:mast_memb_no"`
	FullName     string    `json:"full_name"      gorm:"column:full_name"`
	MastMembDept string    `json:"mast_memb_dept" gorm:"column:mast_memb_dept"`
	DeptName     string    `json:"dept_name"      gorm:"column:dept_name"`
	StsTypeDesc  string    `json:"sts_type_desc"  gorm:"column:sts_type_desc"`
	MastTel      string    `json:"mast_tel"       gorm:"column:mast_tel"`
	MastMobile   string    `json:"mast_mobile"    gorm:"column:mast_mobile"`
	UpdatedAt    time.Time `json:"updated_at"     gorm:"column:updated_at"`
}

// ──────────────────────── Endpoints ────────────────────────

// Sync receives a push from the MSSQL sync agent, runs diff+apply,
// stores missing memb_nos as JSON, writes audit row.
//
//	POST /api/v1/admin/flommast/sync
//	Auth: X-API-Key (NOT JWT — this is service-to-service)
func (h *FlommastSyncHandler) Sync(c *fiber.Ctx) error {
	req := new(SyncRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Invalid JSON: "+err.Error())
	}
	if len(req.Rows) == 0 {
		return response.BadRequest(c, "No rows provided")
	}
	if len(req.Rows) > MaxSyncRows {
		return response.BadRequest(c,
			fmt.Sprintf("Too many rows (max %d)", MaxSyncRows))
	}

	// ─── Convert ExtendedImportRow → ImportRow (16 fields) for Apply ───
	// Apply repository expects []*ImportRow; we have []*ExtendedImportRow.
	// The 2 extra fields are applied separately after Apply via post-UPDATE.
	importRows := make([]*repositories.ImportRow, len(req.Rows))
	for i, r := range req.Rows {
		ir := r.ImportRow // copy embedded struct
		importRows[i] = &ir
	}

	// ─── Open audit log row (status=running) ───
	started := time.Now()
	totalPtr := len(req.Rows)
	auditLog := &models.FlommastSyncLog{
		StartedAt:    started,
		Source:       defaultStr(req.Source, "mssql-agent"),
		AgentVersion: req.AgentVersion,
		AgentHost:    req.AgentHost,
		PublicIP:     defaultStr(req.PublicIP, c.IP()),
		TotalRows:    &totalPtr,
		Status:       "running",
	}
	if err := h.db.WithContext(c.Context()).Create(auditLog).Error; err != nil {
		return response.InternalServerError(c,
			"Failed to create audit log: "+err.Error())
	}

	// ─── Build diff for accurate UNCHANGED/MISSING counts ───
	summary, err := h.importRepo.BuildDiff(c.Context(), importRows)
	if err != nil {
		h.markFailed(auditLog, started, "diff failed: "+err.Error())
		return response.InternalServerError(c, "Diff failed: "+err.Error())
	}

	// ─── Compute full list of missing memb_nos (not capped by 200-entry preview) ───
	missingMembNos, err := h.computeMissingMembNos(c, importRows)
	if err != nil {
		// non-fatal: log warning but continue
		fmt.Println("WARN: computeMissingMembNos:", err)
		missingMembNos = nil
	}

	// ─── Apply (upsert + optional delete missing) ───
	result, err := h.importRepo.Apply(c.Context(), importRows,
		repositories.ApplyOptions{
			DeleteMissing: req.DeleteMissing,
			BatchSize:     500,
		},
	)
	if err != nil {
		h.markFailed(auditLog, started, "apply failed: "+err.Error())
		return response.InternalServerError(c, "Apply failed: "+err.Error())
	}

	// ─── Phase 3A: post-Apply UPDATE for member_type_code ───
	// Apply's upsert doesn't know about member_type_code,
	// so we UPDATE it in a separate transaction (batched).
	extraUpdated, extraErr := h.updateExtraFields(c, req.Rows)
	if extraErr != nil {
		// Non-fatal: log warning. Apply succeeded; just the 2 extras failed.
		fmt.Println("WARN: updateExtraFields:", extraErr)
	}

	// ─── Finalize audit log ───
	finished := time.Now()
	durMs := int(finished.Sub(started).Milliseconds())
	unchanged := summary.UnchangedCount
	missing := summary.MissingCount

	auditLog.FinishedAt = &finished
	auditLog.Inserted = &result.Inserted
	auditLog.Updated = &result.Updated
	auditLog.Deleted = &result.Deleted
	auditLog.Unchanged = &unchanged
	auditLog.Missing = &missing
	auditLog.DurationMs = &durMs
	auditLog.Status = "success"

	// Store missing list as JSON (only if Apply did NOT delete them — otherwise no missing left)
	if len(missingMembNos) > 0 && !req.DeleteMissing {
		jsonBytes, _ := json.Marshal(missingMembNos)
		s := string(jsonBytes)
		auditLog.MissingMembNos = &s
	}

	if saveErr := h.db.WithContext(c.Context()).Save(auditLog).Error; saveErr != nil {
		fmt.Println("WARN: save audit failed:", saveErr)
	}

	return response.Success(c, "Sync complete", fiber.Map{
		"sync_id":         auditLog.ID,
		"inserted":        result.Inserted,
		"updated":         result.Updated,
		"unchanged":       summary.UnchangedCount,
		"missing":         summary.MissingCount,
		"deleted":         result.Deleted,
		"duration_ms":     durMs,
		"total_in_db":     summary.TotalInDB,
		"total_synced":    summary.TotalUploaded,
		"extras_updated":  extraUpdated,
	})
}

// History returns the most recent 30 sync runs (newest first).
//
//	GET /api/v1/admin/flommast/sync-history
//	Auth: JWT (Admin)
func (h *FlommastSyncHandler) History(c *fiber.Ctx) error {
	var logs []models.FlommastSyncLog
	if err := h.db.WithContext(c.Context()).
		Order("started_at DESC").
		Limit(30).
		Find(&logs).Error; err != nil {
		return response.InternalServerError(c, "Query failed: "+err.Error())
	}
	return response.Success(c, "ok", fiber.Map{
		"logs":  logs,
		"count": len(logs),
	})
}

// Status returns latest sync + computed next scheduled run for the UI.
//
//	GET /api/v1/admin/flommast/sync-status
//	Auth: JWT (Admin)
func (h *FlommastSyncHandler) Status(c *fiber.Ctx) error {
	var latest models.FlommastSyncLog
	err := h.db.WithContext(c.Context()).
		Order("started_at DESC").
		First(&latest).Error

	var latestPtr *models.FlommastSyncLog
	if err == nil {
		latestPtr = &latest
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return response.InternalServerError(c, "Query failed: "+err.Error())
	}

	return response.Success(c, "ok", fiber.Map{
		"schedule_human": "ทุกวัน 09:00 (Asia/Bangkok)",
		"schedule_cron":  "0 9 * * *",
		"next_run_at":    nextScheduledRun(time.Now()),
		"last_sync":      latestPtr,
	})
}

// Missing returns members in DB but missing from the LATEST successful sync.
// Reads the missing_memb_nos JSON column then JOINs flommast for display details.
//
//	GET /api/v1/admin/flommast/missing
//	Auth: JWT (Admin)
func (h *FlommastSyncHandler) Missing(c *fiber.Ctx) error {
	// Find the latest successful sync with missing_memb_nos set
	var latest models.FlommastSyncLog
	err := h.db.WithContext(c.Context()).
		Where("status = ? AND missing_memb_nos IS NOT NULL", "success").
		Order("started_at DESC").
		First(&latest).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return response.Success(c, "ok", fiber.Map{
			"members":   []MissingMember{},
			"sync_id":   nil,
			"sync_date": nil,
			"count":     0,
			"note":      "ยังไม่มี sync ที่บันทึก missing list — รอ sync ครั้งถัดไป",
		})
	}
	if err != nil {
		return response.InternalServerError(c, "Query failed: "+err.Error())
	}

	// Decode JSON list
	var membNos []string
	if latest.MissingMembNos != nil && *latest.MissingMembNos != "" {
		if jsonErr := json.Unmarshal([]byte(*latest.MissingMembNos), &membNos); jsonErr != nil {
			return response.InternalServerError(c,
				"Failed to decode missing list: "+jsonErr.Error())
		}
	}

	// No memb_nos → return empty
	if len(membNos) == 0 {
		return response.Success(c, "ok", fiber.Map{
			"members":   []MissingMember{},
			"sync_id":   latest.ID,
			"sync_date": latest.StartedAt,
			"count":     0,
		})
	}

	// JOIN flommast for display details
	var members []MissingMember
	if err := h.db.WithContext(c.Context()).
		Table("flommast").
		Select("mast_memb_no, full_name, mast_memb_dept, dept_name, sts_type_desc, mast_tel, mast_mobile, updated_at").
		Where("mast_memb_no IN ?", membNos).
		Order("mast_memb_no").
		Find(&members).Error; err != nil {
		return response.InternalServerError(c, "Member detail query failed: "+err.Error())
	}

	return response.Success(c, "ok", fiber.Map{
		"members":   members,
		"sync_id":   latest.ID,
		"sync_date": latest.StartedAt,
		"count":     len(members),
	})
}

// DeleteMissing removes the selected memb_nos from flommast table.
// Admin-only safety: only memb_nos listed in the LATEST sync's missing list can be deleted.
//
//	DELETE /api/v1/admin/flommast/missing
//	Body: {"memb_nos": ["00007", "00012"]}
//	Auth: JWT (Admin)
func (h *FlommastSyncHandler) DeleteMissing(c *fiber.Ctx) error {
	req := new(DeleteMissingRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Invalid JSON: "+err.Error())
	}
	if len(req.MembNos) == 0 {
		return response.BadRequest(c, "No memb_nos provided")
	}
	if len(req.MembNos) > 1000 {
		return response.BadRequest(c, "Too many memb_nos (max 1000)")
	}

	// Safety check: only allow deletion of memb_nos in the latest sync's missing list
	var latest models.FlommastSyncLog
	err := h.db.WithContext(c.Context()).
		Where("status = ? AND missing_memb_nos IS NOT NULL", "success").
		Order("started_at DESC").
		First(&latest).Error
	if err != nil {
		return response.BadRequest(c, "ไม่พบ sync log ที่มี missing list — ทำ sync ก่อน")
	}

	allowedSet := map[string]bool{}
	if latest.MissingMembNos != nil {
		var allowed []string
		_ = json.Unmarshal([]byte(*latest.MissingMembNos), &allowed)
		for _, m := range allowed {
			allowedSet[m] = true
		}
	}

	// Validate every requested memb_no is in the allowed set
	var notAllowed []string
	for _, m := range req.MembNos {
		if !allowedSet[m] {
			notAllowed = append(notAllowed, m)
		}
	}
	if len(notAllowed) > 0 {
		return response.BadRequest(c, fmt.Sprintf(
			"memb_nos %s ไม่อยู่ในรายการ missing — ปฏิเสธลบเพื่อความปลอดภัย",
			strings.Join(notAllowed, ","),
		))
	}

	// Execute DELETE
	tx := h.db.WithContext(c.Context()).
		Table("flommast").
		Where("mast_memb_no IN ?", req.MembNos).
		Delete(nil)
	if tx.Error != nil {
		return response.InternalServerError(c, "Delete failed: "+tx.Error.Error())
	}

	// Update the audit log: remove deleted memb_nos from missing_memb_nos JSON
	if latest.MissingMembNos != nil {
		var current []string
		_ = json.Unmarshal([]byte(*latest.MissingMembNos), &current)
		removed := map[string]bool{}
		for _, m := range req.MembNos {
			removed[m] = true
		}
		var remaining []string
		for _, m := range current {
			if !removed[m] {
				remaining = append(remaining, m)
			}
		}
		newJSON, _ := json.Marshal(remaining)
		s := string(newJSON)
		latest.MissingMembNos = &s
		newMissingCount := len(remaining)
		latest.Missing = &newMissingCount
		newDeleted := 0
		if latest.Deleted != nil {
			newDeleted = *latest.Deleted
		}
		newDeleted += int(tx.RowsAffected)
		latest.Deleted = &newDeleted
		_ = h.db.WithContext(c.Context()).Save(&latest).Error
	}

	return response.Success(c, "Deleted", fiber.Map{
		"deleted":   tx.RowsAffected,
		"memb_nos":  req.MembNos,
		"sync_id":   latest.ID,
		"remaining": (func() int {
			if latest.Missing != nil {
				return *latest.Missing
			}
			return 0
		})(),
	})
}

// ──────────────────────── Helpers ────────────────────────

// updateExtraFields — Phase 3A: UPDATE member_type_code for every row,
// since Apply's upsert doesn't include this column.
// Batched in transaction; non-fatal on individual row errors (logs WARN).
func (h *FlommastSyncHandler) updateExtraFields(
	c *fiber.Ctx,
	rows []*ExtendedImportRow,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	updated := 0
	err := h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			if r == nil || r.MastMembNo == "" {
				continue
			}
			res := tx.Exec(
				"UPDATE flommast SET member_type_code = ? WHERE mast_memb_no = ?",
				r.MemberTypeCode, r.MastMembNo,
			)
			if res.Error != nil {
				// Don't abort whole batch — single row error is non-fatal
				fmt.Printf("WARN: update extras for %s: %v\n", r.MastMembNo, res.Error)
				continue
			}
			updated += int(res.RowsAffected)
		}
		return nil
	})
	return updated, err
}

// computeMissingMembNos returns memb_nos in DB but not in the uploaded rows.
// Direct SQL — faster than iterating BuildDiff entries.
func (h *FlommastSyncHandler) computeMissingMembNos(
	c *fiber.Ctx,
	rows []*repositories.ImportRow,
) ([]string, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	uploadedMembNos := make([]string, 0, len(rows))
	for _, r := range rows {
		if r != nil && r.MastMembNo != "" {
			uploadedMembNos = append(uploadedMembNos, r.MastMembNo)
		}
	}
	if len(uploadedMembNos) == 0 {
		return nil, nil
	}

	var missing []string
	err := h.db.WithContext(c.Context()).
		Table("flommast").
		Where("mast_memb_no NOT IN ?", uploadedMembNos).
		Pluck("mast_memb_no", &missing).Error
	return missing, err
}

func (h *FlommastSyncHandler) markFailed(
	auditLog *models.FlommastSyncLog,
	started time.Time,
	msg string,
) {
	finished := time.Now()
	durMs := int(finished.Sub(started).Milliseconds())
	auditLog.FinishedAt = &finished
	auditLog.Status = "failed"
	auditLog.ErrorMessage = msg
	auditLog.DurationMs = &durMs
	_ = h.db.Save(auditLog).Error
}

func defaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// nextScheduledRun returns the next daily 09:00 in Asia/Bangkok timezone.
// Used by Status endpoint so admin UI can show countdown.
func nextScheduledRun(now time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		loc = time.UTC
	}
	n := now.In(loc)
	candidate := time.Date(n.Year(), n.Month(), n.Day(), 9, 0, 0, 0, loc)
	if !candidate.After(n) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}
