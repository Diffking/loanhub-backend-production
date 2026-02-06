package handlers

import (
	"spsc-loaneasy/internal/config"

	"github.com/gofiber/fiber/v2"
)

// HealthHandler handles health check endpoints
type HealthHandler struct{}

// NewHealthHandler creates a new health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Root handles root endpoint
// @Summary Root endpoint
// @Description Returns API status
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router / [get]
func (h *HealthHandler) Root(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "running",
		"message": "🚀 SPSC loanEasy API v2.2.2 is running",
		"mode":    config.AppConfig.AppMode,
		"docs":    "/swagger/index.html",
		"features": []string{
			"Gzip Compression",
			"Pagination",
			"Cache Headers",
			"Mobile APIs",
		},
	})
}

// HealthCheck handles health check
// @Summary Health check
// @Description Check API and database health
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /health [get]
func (h *HealthHandler) HealthCheck(c *fiber.Ctx) error {
	// Check database
	dbStatus := "healthy"
	if err := config.HealthCheck(); err != nil {
		dbStatus = "unhealthy"
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"checks": fiber.Map{
			"api":      "healthy",
			"database": dbStatus,
		},
	})
}

// APIInfo handles API v1 info
// @Summary API v1 Info
// @Description Returns API v1 information
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}

func (h *HealthHandler) APIInfo(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "SPSC loanEasy API v2.2.2",
		"version": "2.2.2",
		"v1":      "/api/v1 - Standard APIs",
		"v2":      "/api/v2 - Mobile Optimized APIs",
	})
}
