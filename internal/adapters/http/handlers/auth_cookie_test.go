package handlers

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRefreshTokenFromRequest(t *testing.T) {
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendString(refreshTokenFromRequest(c))
	})

	cases := []struct {
		name   string
		cookie string
		body   string
		want   string
	}{
		{"cookie only", "from-cookie", "", "from-cookie"},
		{"cookie wins over body", "from-cookie", `{"refresh_token":"from-body"}`, "from-cookie"},
		{"body fallback (old frontend)", "", `{"refresh_token":" from-body "}`, "from-body"},
		{"nothing", "", "", ""},
		{"bad body", "", `not-json`, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			if tc.cookie != "" {
				req.Header.Set("Cookie", "refresh_token="+tc.cookie)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := io.ReadAll(resp.Body)
			if string(got) != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
