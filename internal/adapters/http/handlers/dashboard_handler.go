package handlers

import (
	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// DashboardHandler handles dashboard endpoints
type DashboardHandler struct {
	dashboardService *services.DashboardService
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(dashboardService *services.DashboardService) *DashboardHandler {
	return &DashboardHandler{
		dashboardService: dashboardService,
	}
}

// GetAdminDashboard returns admin dashboard data
// @Summary Admin Dashboard
// @Description Get admin dashboard with system overview (Admin only)
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /dashboard/admin [get]
func (h *DashboardHandler) GetAdminDashboard(c *fiber.Ctx) error {
	data, err := h.dashboardService.GetAdminDashboard(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Failed to get admin dashboard")
	}

	return response.Success(c, "Admin dashboard retrieved successfully", data)
}

// GetOfficerDashboard returns officer dashboard data
// @Summary Officer Dashboard
// @Description Get officer dashboard with assigned cases and tasks (Officer only)
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /dashboard/officer [get]
func (h *DashboardHandler) GetOfficerDashboard(c *fiber.Ctx) error {
	// Get officer ID from context
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}

	data, err := h.dashboardService.GetOfficerDashboard(c.Context(), userID)
	if err != nil {
		return response.InternalServerError(c, "Failed to get officer dashboard")
	}

	return response.Success(c, "Officer dashboard retrieved successfully", data)
}

// GetUserDashboard returns user dashboard data
// @Summary User Dashboard
// @Description Get user dashboard with mortgage status and appointments
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /dashboard/user [get]
func (h *DashboardHandler) GetUserDashboard(c *fiber.Ctx) error {
	// Get member number from context
	membNo, ok := c.Locals("membNo").(string)
	if !ok || membNo == "" {
		return response.Unauthorized(c, "Unauthorized")
	}

	data, err := h.dashboardService.GetUserDashboard(c.Context(), membNo)
	if err != nil {
		return response.InternalServerError(c, "Failed to get user dashboard")
	}

	return response.Success(c, "User dashboard retrieved successfully", data)
}

// GetMyDashboard returns dashboard based on user role
// @Summary My Dashboard
// @Description Get dashboard based on current user's role (auto-detect)
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /dashboard [get]
func (h *DashboardHandler) GetMyDashboard(c *fiber.Ctx) error {
	// Get role from context
	role, _ := c.Locals("role").(string)
	userID, _ := c.Locals("userID").(uint)
	membNo, _ := c.Locals("membNo").(string)

	var data interface{}
	var err error

	switch role {
	case "ADMIN":
		data, err = h.dashboardService.GetAdminDashboard(c.Context())
	case "OFFICER":
		data, err = h.dashboardService.GetOfficerDashboard(c.Context(), userID)
	default:
		data, err = h.dashboardService.GetUserDashboard(c.Context(), membNo)
	}

	if err != nil {
		return response.InternalServerError(c, "Failed to get dashboard")
	}

	return response.Success(c, "Dashboard retrieved successfully", fiber.Map{
		"role": role,
		"data": data,
	})
}
