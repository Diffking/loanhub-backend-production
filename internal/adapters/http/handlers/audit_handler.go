package handlers

import (
	"strconv"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/repositories"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// AuditHandler exposes the audit log to Admins
type AuditHandler struct {
	repo *repositories.AuditLogRepository
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(repo *repositories.AuditLogRepository) *AuditHandler {
	return &AuditHandler{repo: repo}
}

// List returns audit log entries, newest first (Admin only)
// Query: page, limit (max 200), user_id, action ("member." = prefix), target_id, ip,
// from, to (YYYY-MM-DD, Asia/Bangkok; "to" is inclusive)
func (h *AuditHandler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}

	f := repositories.AuditLogFilter{
		Action:   c.Query("action"),
		TargetID: c.Query("target_id"),
		IP:       c.Query("ip"),
		Offset:   (page - 1) * limit,
		Limit:    limit,
	}
	if uid, err := strconv.ParseUint(c.Query("user_id"), 10, 32); err == nil {
		f.UserID = uint(uid)
	}

	loc, _ := time.LoadLocation("Asia/Bangkok")
	if loc == nil {
		loc = time.Local
	}
	if v := c.Query("from"); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, loc)
		if err != nil {
			return response.BadRequest(c, "from must be YYYY-MM-DD")
		}
		f.From = t
	}
	if v := c.Query("to"); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, loc)
		if err != nil {
			return response.BadRequest(c, "to must be YYYY-MM-DD")
		}
		f.To = t.AddDate(0, 0, 1)
	}

	rows, total, err := h.repo.List(c.Context(), f)
	if err != nil {
		return response.InternalError(c, "Failed to list audit logs", err)
	}

	return response.Success(c, "Audit logs retrieved successfully", fiber.Map{
		"items": rows,
		"page":  page,
		"limit": limit,
		"total": total,
	})
}
