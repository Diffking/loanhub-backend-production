package pagination

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// Params represents pagination parameters
type Params struct {
	Page    int `json:"page"`
	Limit   int `json:"limit"`
	Offset  int `json:"-"`
}

// Meta represents pagination metadata
type Meta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// DefaultLimit is the default number of items per page
const DefaultLimit = 20

// MaxLimit is the maximum number of items per page
const MaxLimit = 100

// GetParams extracts pagination parameters from request
func GetParams(c *fiber.Ctx) *Params {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", strconv.Itoa(DefaultLimit)))

	// Validate page
	if page < 1 {
		page = 1
	}

	// Validate limit
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	offset := (page - 1) * limit

	return &Params{
		Page:   page,
		Limit:  limit,
		Offset: offset,
	}
}

// GetMeta calculates pagination metadata
func GetMeta(params *Params, total int64) *Meta {
	totalPages := int(total) / params.Limit
	if int(total)%params.Limit > 0 {
		totalPages++
	}

	return &Meta{
		Page:       params.Page,
		Limit:      params.Limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    params.Page < totalPages,
		HasPrev:    params.Page > 1,
	}
}

// Response represents paginated response
type Response struct {
	Data interface{} `json:"data"`
	Meta *Meta       `json:"meta"`
}

// NewResponse creates a new paginated response
func NewResponse(data interface{}, params *Params, total int64) *Response {
	return &Response{
		Data: data,
		Meta: GetMeta(params, total),
	}
}
