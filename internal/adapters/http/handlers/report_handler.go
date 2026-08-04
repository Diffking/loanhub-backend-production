package handlers

import (
	"errors"
	"strconv"
	"time"

	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// ReportHandler serves the Officer/Admin "รายงาน" page.
type ReportHandler struct {
	reportService *services.ReportService
}

func NewReportHandler(reportService *services.ReportService) *ReportHandler {
	return &ReportHandler{reportService: reportService}
}

// GetMonthlyStepReport returns the monthly step-by-step report
// @Summary Monthly step report
// @Description รายงานประจำเดือน แยกตามขั้นตอนทั้ง 6 (Officer/Admin)
// @Tags Report
// @Produce json
// @Security BearerAuth
// @Param year query int false "Year (ค.ศ.)"
// @Param month query int false "Month (1-12)"
// @Param officer_id query int false "Filter by officer"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /reports/monthly-steps [get]
func (h *ReportHandler) GetMonthlyStepReport(c *fiber.Ctx) error {
	now := time.Now()
	year, err := strconv.Atoi(c.Query("year", strconv.Itoa(now.Year())))
	if err != nil {
		return response.BadRequest(c, "Invalid year")
	}
	month, err := strconv.Atoi(c.Query("month", strconv.Itoa(int(now.Month()))))
	if err != nil {
		return response.BadRequest(c, "Invalid month")
	}

	var officerID *uint
	if raw := c.Query("officer_id"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return response.BadRequest(c, "Invalid officer_id")
		}
		oid := uint(id)
		officerID = &oid
	}

	data, err := h.reportService.GetMonthlyStepReport(c.Context(), year, month, officerID)
	if err != nil {
		if errors.Is(err, services.ErrInvalidReportMonth) {
			return response.BadRequest(c, "Month must be between 1 and 12")
		}
		return response.InternalServerError(c, "Failed to build monthly report")
	}

	return response.Success(c, "Monthly report retrieved successfully", data)
}
