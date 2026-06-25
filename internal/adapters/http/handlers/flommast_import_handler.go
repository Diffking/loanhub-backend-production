package handlers

import (
	"fmt"
	"io"

	"spsc-loaneasy/internal/adapters/persistence/repositories"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// FlommastImportHandler — admin-only endpoints to upload + apply flommast .sql files.
//
//	POST /api/v1/admin/flommast/preview  →  upload SQL, get diff summary
//	POST /api/v1/admin/flommast/apply    →  upload SQL again, apply changes
//
// Both endpoints accept multipart/form-data with field name "file".
type FlommastImportHandler struct {
	importRepo *repositories.FlommastImportRepository
}

// NewFlommastImportHandler creates a new flommast import handler
func NewFlommastImportHandler(importRepo *repositories.FlommastImportRepository) *FlommastImportHandler {
	return &FlommastImportHandler{importRepo: importRepo}
}

// MaxUploadSize — maximum SQL upload size (10 MB)
const MaxUploadSize = 10 * 1024 * 1024

// Preview parses the uploaded SQL and returns a diff summary.
// Does NOT modify the database.
//
//	POST /api/v1/admin/flommast/preview
//	Content-Type: multipart/form-data
//	field "file": .sql file
//
// Auth: AdminOnly
func (h *FlommastImportHandler) Preview(c *fiber.Ctx) error {
	rows, warnings, err := h.parseUpload(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	summary, err := h.importRepo.BuildDiff(c.Context(), rows)
	if err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Build diff failed: %v", err))
	}

	return response.Success(c, "Preview generated", fiber.Map{
		"summary":  summary,
		"warnings": warnings,
	})
}

// ApplyRequest — query params for /apply
type ApplyRequest struct {
	DeleteMissing bool `query:"delete_missing"`
}

// Apply parses + applies the uploaded SQL changes to the flommast table.
//
//	POST /api/v1/admin/flommast/apply?delete_missing=false
//	Content-Type: multipart/form-data
//	field "file": .sql file
//
// Auth: AdminOnly
//
// Note: re-uploads the file (the preview's parsed result is not cached server-side).
// This is intentional: ensures the user is applying exactly what they see.
func (h *FlommastImportHandler) Apply(c *fiber.Ctx) error {
	rows, warnings, err := h.parseUpload(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	deleteMissing := c.QueryBool("delete_missing", false)

	result, err := h.importRepo.Apply(c.Context(), rows, repositories.ApplyOptions{
		DeleteMissing: deleteMissing,
		BatchSize:     500,
	})
	if err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Apply failed: %v", err))
	}

	return response.Success(c, "Flommast updated successfully", fiber.Map{
		"result":   result,
		"warnings": warnings,
	})
}

// ============================================================
// Helpers
// ============================================================

// parseUpload extracts the .sql file from the request and parses it.
// Returns the parsed rows + warnings, or an error message suitable for BadRequest.
func (h *FlommastImportHandler) parseUpload(c *fiber.Ctx) ([]*repositories.ImportRow, []string, error) {
	fh, err := c.FormFile("file")
	if err != nil {
		return nil, nil, fmt.Errorf("กรุณาแนบไฟล์ .sql ในฟิลด์ชื่อ \"file\"")
	}

	if fh.Size > MaxUploadSize {
		return nil, nil, fmt.Errorf("ไฟล์ใหญ่เกิน %d MB", MaxUploadSize/(1024*1024))
	}

	f, err := fh.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("เปิดไฟล์ไม่ได้: %v", err)
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, fmt.Errorf("อ่านไฟล์ไม่ได้: %v", err)
	}

	rows, warnings, err := repositories.ParseSQL(string(bytes))
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("ไม่พบข้อมูลในไฟล์ — โปรดตรวจสอบรูปแบบ")
	}

	return rows, warnings, nil
}
