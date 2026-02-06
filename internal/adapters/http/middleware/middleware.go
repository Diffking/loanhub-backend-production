package middleware

import (
	"time"

	"spsc-loaneasy/internal/config"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// Setup configures all middlewares for the application
func Setup(app *fiber.App, cfg *config.Config) {
	// Recover middleware - catches panics
	app.Use(recover.New())

	// Gzip Compression middleware - ลด response size 60-70%
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed, // เร็วที่สุด เหมาะกับ API
	}))

	// Security Headers middleware (Helmet)
	app.Use(helmet.New(helmet.Config{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "SAMEORIGIN",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		CrossOriginEmbedderPolicy: "require-corp",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
		PermissionPolicy:          "geolocation=(), microphone=(), camera=()",
	}))

	// Rate Limiter middleware - General API (100 requests per minute per IP)
	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"error":   "Too many requests",
				"message": "คุณส่ง request มากเกินไป กรุณารอสักครู่",
			})
		},
		SkipFailedRequests:     false,
		SkipSuccessfulRequests: false,
	}))

	// Logger middleware
	if cfg.IsDev() {
		app.Use(logger.New(logger.Config{
			Format: "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path}\n",
		}))
	} else {
		app.Use(logger.New(logger.Config{
			Format:     "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path} | ${error}\n",
			TimeFormat: "2006-01-02 15:04:05",
		}))
	}

	// CORS middleware
	if cfg.IsDev() {
		// Development: Allow all origins
		app.Use(cors.New(cors.Config{
			AllowOrigins:     "*",
			AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
			AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
			AllowCredentials: false, // Cannot be true with AllowOrigins: "*"
		}))
	} else {
		// Production: Restrict origins
		app.Use(cors.New(cors.Config{
			AllowOrigins:     cfg.GetAllowedOrigins(), // From config
			AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
			AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
			AllowCredentials: true,
		}))
	}
}

// AuthRateLimiter creates a stricter rate limiter for auth endpoints
// 5 requests per minute per IP (for login, register, etc.)
func AuthRateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        5,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP() + "-auth"
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"error":   "Too many login attempts",
				"message": "คุณพยายาม login มากเกินไป กรุณารอ 1 นาที",
			})
		},
	})
}

// StrictRateLimiter creates an even stricter rate limiter for sensitive operations
// 3 requests per minute per IP (for password reset, etc.)
func StrictRateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        3,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP() + "-strict"
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"error":   "Rate limit exceeded",
				"message": "กรุณารอสักครู่ก่อนลองใหม่",
			})
		},
	})
}

// CustomErrorHandler handles errors globally
func CustomErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"error":   true,
		"message": message,
	})
}
