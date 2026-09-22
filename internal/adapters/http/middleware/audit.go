package middleware

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"

	"github.com/gofiber/fiber/v2"
)

// auditQueryKeys are the query params worth recording (who searched/filtered for what).
var auditQueryKeys = []string{"q", "search", "keyword", "memb_no", "year", "month", "step_id", "officer_id", "role", "delete_missing"}

// auditTargetParams are route params that identify the record being touched, in priority order.
var auditTargetParams = []string{"memb_no", "id"}

// Audit records who did what to which record, after the handler has run.
// Put it after AuthMiddleware so the actor (userID/role/membNo) is known.
// The DB write is async so it never slows down or breaks the request.
func Audit(repo *repositories.AuditLogRepository, action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		status := c.Response().StatusCode()
		if err != nil {
			status = fiber.StatusInternalServerError
			if fe, ok := err.(*fiber.Error); ok {
				status = fe.Code
			}
		}

		entry := buildAuditEntry(c, action, status)

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if werr := repo.Create(ctx, entry); werr != nil {
				log.Printf("⚠️ audit log write failed (%s %s): %v", entry.Action, entry.Path, werr)
			}
		}()

		return err
	}
}

// buildAuditEntry copies everything needed out of the Fiber context
// (it is reused after the handler returns, so nothing may reference its buffers).
func buildAuditEntry(c *fiber.Ctx, action string, status int) *models.AuditLog {
	entry := &models.AuditLog{
		CreatedAt:  time.Now(),
		Action:     action,
		Method:     strings.Clone(c.Method()),
		Path:       truncate(strings.Clone(c.Path()), 255),
		StatusCode: status,
		IP:         strings.Clone(c.IP()),
		UserAgent:  truncate(strings.Clone(c.Get(fiber.HeaderUserAgent)), 255),
	}
	if uid, ok := c.Locals("userID").(uint); ok && uid > 0 {
		entry.UserID = &uid
	}
	entry.Username, _ = c.Locals("username").(string)
	entry.Role, _ = c.Locals("role").(string)
	entry.MembNo, _ = c.Locals("membNo").(string)

	detail := map[string]string{}
	for k, v := range c.AllParams() {
		detail[k] = strings.Clone(v)
	}
	for _, name := range auditTargetParams {
		if v := detail[name]; v != "" {
			entry.TargetID = truncate(v, 50)
			break
		}
	}
	for _, k := range auditQueryKeys {
		if v := c.Query(k); v != "" {
			detail["query."+k] = truncate(strings.Clone(v), 100)
		}
	}
	if len(detail) > 0 {
		if b, jerr := json.Marshal(detail); jerr == nil {
			entry.Detail = string(b)
		}
	}
	return entry
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	// avoid cutting a multi-byte (Thai) rune in half
	for max > 0 && (s[max]&0xC0) == 0x80 {
		max--
	}
	return s[:max]
}
