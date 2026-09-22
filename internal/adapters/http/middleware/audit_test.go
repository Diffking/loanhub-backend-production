package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"unicode/utf8"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"github.com/gofiber/fiber/v2"
)

func TestBuildAuditEntry(t *testing.T) {
	var got *models.AuditLog
	app := fiber.New()
	app.Get("/members/:memb_no",
		func(c *fiber.Ctx) error { // stands in for AuthMiddleware
			c.Locals("userID", uint(7))
			c.Locals("username", "officer1")
			c.Locals("role", "OFFICER")
			c.Locals("membNo", "01234")
			return c.Next()
		},
		func(c *fiber.Ctx) error {
			c.Status(fiber.StatusOK)
			got = buildAuditEntry(c, "member.view", c.Response().StatusCode())
			return nil
		},
	)

	req := httptest.NewRequest("GET", "/members/07337?q=สมชาย&token=secret&year=2026", nil)
	req.Header.Set("User-Agent", "test-agent")
	if _, err := app.Test(req); err != nil {
		t.Fatal(err)
	}

	if got == nil {
		t.Fatal("entry not built")
	}
	if got.Action != "member.view" || got.Method != "GET" || got.StatusCode != 200 {
		t.Errorf("unexpected action/method/status: %+v", got)
	}
	if got.UserID == nil || *got.UserID != 7 || got.Role != "OFFICER" || got.MembNo != "01234" || got.Username != "officer1" {
		t.Errorf("actor not captured: %+v", got)
	}
	if got.TargetID != "07337" {
		t.Errorf("target = %q, want 07337", got.TargetID)
	}
	if got.UserAgent != "test-agent" {
		t.Errorf("user agent = %q", got.UserAgent)
	}

	var detail map[string]string
	if err := json.Unmarshal([]byte(got.Detail), &detail); err != nil {
		t.Fatalf("detail not JSON: %v (%q)", err, got.Detail)
	}
	if detail["memb_no"] != "07337" || detail["query.q"] != "สมชาย" || detail["query.year"] != "2026" {
		t.Errorf("detail = %v", detail)
	}
	if _, leaked := detail["query.token"]; leaked {
		t.Error("non-whitelisted query key (token) must not be recorded")
	}
}

func TestBuildAuditEntryAnonymous(t *testing.T) {
	var got *models.AuditLog
	app := fiber.New()
	app.Post("/auth/liff/login", func(c *fiber.Ctx) error {
		c.Status(fiber.StatusUnauthorized)
		got = buildAuditEntry(c, "auth.liff_login", c.Response().StatusCode())
		return nil
	})
	if _, err := app.Test(httptest.NewRequest("POST", "/auth/liff/login", nil)); err != nil {
		t.Fatal(err)
	}
	if got.UserID != nil || got.Role != "" || got.StatusCode != 401 || got.Detail != "" {
		t.Errorf("anonymous entry = %+v", got)
	}
}

func TestTruncateKeepsValidUTF8(t *testing.T) {
	s := "ทดสอบภาษาไทย" // 3 bytes per rune
	for max := 0; max <= len(s); max++ {
		out := truncate(s, max)
		if len(out) > max {
			t.Fatalf("max %d: len %d", max, len(out))
		}
		if !utf8.ValidString(out) {
			t.Fatalf("max %d: invalid UTF-8 %q", max, out)
		}
	}
}
