package handlers

import (
	"errors"
	"strconv"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"
	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// DocCheckHandler handles document checklist endpoints
type DocCheckHandler struct {
	docCheckService *services.DocCheckService
	docItemRepo     *repositories.DocItemRepository
}

// NewDocCheckHandler creates a new doc check handler
func NewDocCheckHandler(
	docCheckService *services.DocCheckService,
	docItemRepo *repositories.DocItemRepository,
) *DocCheckHandler {
	return &DocCheckHandler{
		docCheckService: docCheckService,
		docItemRepo:     docItemRepo,
	}
}

// ============================================================
// Master: Doc Items CRUD
// ============================================================

// ListDocItems lists all doc items
func (h *DocCheckHandler) ListDocItems(c *fiber.Ctx) error {
	includeInactive := c.Query("all") == "true"
	loanTypeID := c.Query("loan_type_id")

	var docItems []*models.DocItem
	var err error

	if includeInactive {
		docItems, err = h.docItemRepo.ListAll(c.Context())
	} else {
		docItems, err = h.docItemRepo.List(c.Context())
	}

	if err != nil {
		return response.InternalServerError(c, "Failed to list doc items")
	}

	if loanTypeID != "" {
		ltID, _ := strconv.ParseUint(loanTypeID, 10, 32)
		if ltID > 0 {
			var filtered []*models.DocItem
			for _, di := range docItems {
				if di.LoanTypeID == uint(ltID) {
					filtered = append(filtered, di)
				}
			}
			docItems = filtered
		}
	}

	return response.Success(c, "Doc items retrieved successfully", fiber.Map{
		"doc_items": docItems,
	})
}

// GetDocItem gets a doc item by ID
func (h *DocCheckHandler) GetDocItem(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid doc item ID")
	}

	docItem, err := h.docItemRepo.GetByID(c.Context(), uint(id))
	if err != nil {
		return response.NotFound(c, "Doc item not found")
	}

	return response.Success(c, "Doc item retrieved successfully", fiber.Map{
		"doc_item": docItem,
	})
}

// CreateDocItemRequest represents create doc item request
type CreateDocItemRequest struct {
	LoanTypeID uint   `json:"loan_type_id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	IsRequired bool   `json:"is_required"`
	SortOrder  int    `json:"sort_order"`
}

// CreateDocItem creates a new doc item
func (h *DocCheckHandler) CreateDocItem(c *fiber.Ctx) error {
	var req CreateDocItemRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.LoanTypeID == 0 {
		return response.BadRequest(c, "Loan type ID is required")
	}
	if req.Code == "" {
		return response.BadRequest(c, "Code is required")
	}
	if req.Name == "" {
		return response.BadRequest(c, "Name is required")
	}

	docItem := &models.DocItem{
		LoanTypeID: req.LoanTypeID,
		Code:       req.Code,
		Name:       req.Name,
		IsRequired: req.IsRequired,
		SortOrder:  req.SortOrder,
		IsActive:   true,
	}

	if err := h.docItemRepo.Create(c.Context(), docItem); err != nil {
		return response.InternalServerError(c, "Failed to create doc item")
	}

	return response.Created(c, "Doc item created successfully", fiber.Map{
		"doc_item": docItem,
	})
}

// UpdateDocItemRequest represents update doc item request
type UpdateDocItemRequest struct {
	LoanTypeID *uint   `json:"loan_type_id"`
	Code       *string `json:"code"`
	Name       *string `json:"name"`
	IsRequired *bool   `json:"is_required"`
	SortOrder  *int    `json:"sort_order"`
	IsActive   *bool   `json:"is_active"`
}

// UpdateDocItem updates a doc item
func (h *DocCheckHandler) UpdateDocItem(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid doc item ID")
	}

	docItem, err := h.docItemRepo.GetByID(c.Context(), uint(id))
	if err != nil {
		return response.NotFound(c, "Doc item not found")
	}

	var req UpdateDocItemRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.LoanTypeID != nil {
		docItem.LoanTypeID = *req.LoanTypeID
	}
	if req.Code != nil {
		docItem.Code = *req.Code
	}
	if req.Name != nil {
		docItem.Name = *req.Name
	}
	if req.IsRequired != nil {
		docItem.IsRequired = *req.IsRequired
	}
	if req.SortOrder != nil {
		docItem.SortOrder = *req.SortOrder
	}
	if req.IsActive != nil {
		docItem.IsActive = *req.IsActive
	}

	if err := h.docItemRepo.Update(c.Context(), docItem); err != nil {
		return response.InternalServerError(c, "Failed to update doc item")
	}

	return response.Success(c, "Doc item updated successfully", fiber.Map{
		"doc_item": docItem,
	})
}

// DeleteDocItem soft deletes a doc item
func (h *DocCheckHandler) DeleteDocItem(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid doc item ID")
	}

	if err := h.docItemRepo.Delete(c.Context(), uint(id)); err != nil {
		return response.InternalServerError(c, "Failed to delete doc item")
	}

	return response.Success(c, "Doc item deleted successfully", nil)
}

// ============================================================
// Mortgage: Doc Checks (Checklist)
// ============================================================

// GetDocChecks ดึงเช็คลิสต์เอกสารทั้งหมดของ mortgage
func (h *DocCheckHandler) GetDocChecks(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	checks, err := h.docCheckService.GetChecklist(c.Context(), uint(id))
	if err != nil {
		if errors.Is(err, services.ErrMortgageNotFound) {
			return response.NotFound(c, "Mortgage not found")
		}
		return response.InternalServerError(c, "Failed to get doc checks")
	}

	return response.Success(c, "Doc checks retrieved successfully", fiber.Map{
		"doc_checks": checks,
	})
}

// UpdateDocChecks อัพเดทเช็คลิสต์ (batch)
func (h *DocCheckHandler) UpdateDocChecks(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	var req struct {
		Items []services.UpdateDocCheckItem `json:"items"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if len(req.Items) == 0 {
		return response.BadRequest(c, "Items are required")
	}

	userID, _ := c.Locals("userID").(uint)
	input := &services.UpdateDocCheckInput{Items: req.Items}

	if err := h.docCheckService.UpdateChecklist(c.Context(), uint(id), input, userID); err != nil {
		if errors.Is(err, services.ErrMortgageNotFound) {
			return response.NotFound(c, "Mortgage not found")
		}
		return response.InternalServerError(c, "Failed to update doc checks")
	}

	return response.Success(c, "Doc checks updated successfully", nil)
}

// GetIncompleteDoc ดึงรายการเอกสารไม่ครบ → Frontend แสดง Toast
func (h *DocCheckHandler) GetIncompleteDoc(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	result, err := h.docCheckService.GetIncompleteItems(c.Context(), uint(id))
	if err != nil {
		if errors.Is(err, services.ErrMortgageNotFound) {
			return response.NotFound(c, "Mortgage not found")
		}
		return response.InternalServerError(c, "Failed to get incomplete items")
	}

	return response.Success(c, "Incomplete doc items retrieved", fiber.Map{
		"result": result,
	})
}

// NotifyLineIncompleteDoc ส่ง LINE แจ้งสมาชิกว่าเอกสารไม่ครบ
func (h *DocCheckHandler) NotifyLineIncompleteDoc(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid mortgage ID")
	}

	err = h.docCheckService.NotifyLineIncompleteDoc(c.Context(), uint(id))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrMortgageNotFound):
			return response.NotFound(c, "Mortgage not found")
		case errors.Is(err, services.ErrNoLineUserID):
			return response.BadRequest(c, "สมาชิกยังไม่ได้เชื่อมต่อ LINE")
		case errors.Is(err, services.ErrNoIncompleteItems):
			return response.BadRequest(c, "เอกสารครบถ้วนแล้ว ไม่มีรายการที่ต้องแจ้งเตือน")
		default:
			return response.InternalServerError(c, "ส่ง LINE ไม่สำเร็จ: "+err.Error())
		}
	}

	return response.Success(c, "ส่งแจ้งเตือน LINE สำเร็จ", nil)
}
