package middleware

import (
	"crypto/subtle"

	"spsc-loaneasy/internal/config"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// APIKeyAuth creates a service-to-service authentication middleware
// that verifies the X-API-Key header against config.Sync.APIKey.
//
// Used for the MSSQL sync agent → POST /api/v1/admin/flommast/sync.
// Constant-time comparison prevents timing-side-channel attacks.
//
// Sets c.Locals("auth_type") = "api_key" + c.Locals("source") = "mssql-agent"
// so downstream handlers can audit accordingly.
func APIKeyAuth(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if cfg.Sync.APIKey == "" {
			return response.InternalServerError(c,
				"Sync API key not configured (set SYNC_API_KEY in .env)")
		}

		provided := c.Get("X-API-Key")
		if provided == "" {
			return response.Unauthorized(c, "API key required (X-API-Key header)")
		}

		if subtle.ConstantTimeCompare(
			[]byte(provided),
			[]byte(cfg.Sync.APIKey),
		) != 1 {
			return response.Unauthorized(c, "Invalid API key")
		}

		c.Locals("auth_type", "api_key")
		c.Locals("source", "mssql-agent")
		return c.Next()
	}
}
