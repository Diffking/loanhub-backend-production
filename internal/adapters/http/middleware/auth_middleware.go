package middleware

import (
	"strings"

	"spsc-loaneasy/internal/config"
	"spsc-loaneasy/internal/pkg/jwt"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// AuthMiddleware creates authentication middleware
func AuthMiddleware(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var accessToken string

		// 1. Try to get token from cookie first
		accessToken = c.Cookies("access_token")

		// 2. If not in cookie, try Authorization header
		if accessToken == "" {
			authHeader := c.Get("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				accessToken = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// 3. No token found
		if accessToken == "" {
			return response.Unauthorized(c, "Access token required")
		}

		// 4. Validate token
		claims, err := jwt.ValidateAccessToken(accessToken, cfg.JWT.Secret)
		if err != nil {
			if err == jwt.ErrTokenExpired {
				return response.Unauthorized(c, "Access token expired")
			}
			return response.Unauthorized(c, "Invalid access token")
		}

		// 5. Set user info in context
		c.Locals("userID", claims.UserID)
		c.Locals("membNo", claims.MembNo)
		c.Locals("username", claims.Username)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}

// RoleMiddleware creates role-based authorization middleware
func RoleMiddleware(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals("role").(string)
		if !ok {
			return response.Unauthorized(c, "Unauthorized")
		}

		// Check if user's role is in allowed roles
		for _, allowedRole := range allowedRoles {
			if role == allowedRole {
				return c.Next()
			}
		}

		return response.Forbidden(c, "You don't have permission to access this resource")
	}
}

// AdminOnly middleware allows only ADMIN role
func AdminOnly() fiber.Handler {
	return RoleMiddleware("ADMIN")
}

// OfficerOrAdmin middleware allows OFFICER or ADMIN roles
func OfficerOrAdmin() fiber.Handler {
	return RoleMiddleware("OFFICER", "ADMIN")
}

// OptionalAuth middleware - doesn't require auth but sets user info if token present
func OptionalAuth(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var accessToken string

		// Try to get token from cookie
		accessToken = c.Cookies("access_token")

		// If not in cookie, try Authorization header
		if accessToken == "" {
			authHeader := c.Get("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				accessToken = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// If token exists, validate and set user info
		if accessToken != "" {
			claims, err := jwt.ValidateAccessToken(accessToken, cfg.JWT.Secret)
			if err == nil {
				c.Locals("userID", claims.UserID)
				c.Locals("membNo", claims.MembNo)
				c.Locals("username", claims.Username)
				c.Locals("role", claims.Role)
			}
		}

		return c.Next()
	}
}
