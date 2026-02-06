package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// CacheControl sets cache headers for responses
func CacheControl(maxAge time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Process request first
		err := c.Next()

		// Set cache headers only for successful GET requests
		if c.Method() == "GET" && c.Response().StatusCode() == 200 {
			c.Set("Cache-Control", "public, max-age="+string(rune(int(maxAge.Seconds()))))
			c.Set("Cache-Control", formatCacheControl(maxAge))
		}

		return err
	}
}

// formatCacheControl formats cache control header value
func formatCacheControl(maxAge time.Duration) string {
	seconds := int(maxAge.Seconds())
	return "public, max-age=" + itoa(seconds)
}

// itoa converts int to string (simple implementation)
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	
	var b [20]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	
	if neg {
		pos--
		b[pos] = '-'
	}
	
	return string(b[pos:])
}

// MasterDataCache returns cache middleware for master data (1 hour cache)
func MasterDataCache() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		// Set cache headers only for successful GET requests
		if c.Method() == "GET" && c.Response().StatusCode() == 200 {
			// Cache for 1 hour
			c.Set("Cache-Control", "public, max-age=3600")
			// Add ETag based on response body hash (optional)
		}

		return err
	}
}

// NoCacheHeaders sets no-cache headers
func NoCacheHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Set("Pragma", "no-cache")
		c.Set("Expires", "0")
		return c.Next()
	}
}

// PrivateCacheHeaders sets private cache headers (for user-specific data)
func PrivateCacheHeaders(maxAge time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		if c.Method() == "GET" && c.Response().StatusCode() == 200 {
			seconds := int(maxAge.Seconds())
			c.Set("Cache-Control", "private, max-age="+itoa(seconds))
		}

		return err
	}
}
