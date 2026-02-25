package services

import (
	"context"
	"errors"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"
)

var (
	ErrDocItemNotFound   = errors.New("doc item not found")
	ErrDocCheckNotFound  = errors.New("doc check not found")
	ErrChecklistEmpty    = errors.New("checklist is empty")
	ErrNoIncompleteItems = errors.New("no incomplete items")
)

// DocCheckService handles document checklist business logic
type DocCheckService struct {
	docItemRepo  *repositories.DocItemRepository
	docCheckRepo *repositories.MortgageDocCheckRepository
	mortgageRepo *repositories.MortgageRepository
}

// NewDocCheckService creates a new doc check service
func NewDocCheckService(
	docItemRepo *repositories.DocItemRepository,
	docCheckRepo *repositories.MortgageDocCheckRepository,
	mortgageRepo *repositories.MortgageRepository,
) *DocCheckService {
	return &DocCheckService{
		docItemRepo:  docItemRepo,
		docCheckRepo: docCheckRepo,
		mortgageRepo: mortgageRepo,
	}
}

// ============================================================
// Get Checklist — ดึงเช็คลิสต์ (สร้างอัตโนมัติถ้ายังไม่มี)
// ============================================================

func (s *DocCheckService) GetChecklist(ctx context.Context, mortgageID uint) ([]*models.MortgageDocCheckResponse, error) {
	mortgage, err := s.mortgageRepo.GetByID(ctx, mortgageID)
	if err != nil {
		return nil, ErrMortgageNotFound
	}

	checks, err := s.docCheckRepo.GetByMortgageID(ctx, mortgageID)
	if err != nil {
		return nil, err
	}

	// ถ้ายังไม่มี → สร้างจาก doc_items ตาม loan_type
	if len(checks) == 0 {
		if err := s.docCheckRepo.InitChecklist(ctx, mortgageID, mortgage.LoanTypeID); err != nil {
			return nil, err
		}
		checks, err = s.docCheckRepo.GetByMortgageID(ctx, mortgageID)
		if err != nil {
			return nil, err
		}
	}

	var result []*models.MortgageDocCheckResponse
	for _, c := range checks {
		result = append(result, c.ToResponse())
	}
	if result == nil {
		result = []*models.MortgageDocCheckResponse{}
	}

	return result, nil
}

// ============================================================
// Update Checklist — อัพเดทเช็คลิสต์ (batch)
// ============================================================

type UpdateDocCheckItem struct {
	ID            uint `json:"id"`
	IsChecked     bool `json:"is_checked"`
	IsRecommended bool `json:"is_recommended"`
}

type UpdateDocCheckInput struct {
	Items []UpdateDocCheckItem `json:"items"`
}

func (s *DocCheckService) UpdateChecklist(ctx context.Context, mortgageID uint, input *UpdateDocCheckInput, userID uint) error {
	_, err := s.mortgageRepo.GetByID(ctx, mortgageID)
	if err != nil {
		return ErrMortgageNotFound
	}

	now := time.Now()

	for _, item := range input.Items {
		check, err := s.docCheckRepo.GetByID(ctx, item.ID)
		if err != nil {
			continue
		}
		if check.MortgageID != mortgageID {
			continue
		}

		check.IsChecked = item.IsChecked
		check.IsRecommended = item.IsRecommended

		if item.IsChecked {
			check.CheckedBy = &userID
			check.CheckedAt = &now
		} else {
			check.CheckedBy = nil
			check.CheckedAt = nil
		}

		s.docCheckRepo.UpdateCheck(ctx, check)
	}

	return nil
}

// ============================================================
// Get Incomplete — ดึงรายการเอกสารที่ยังไม่ครบ (สำหรับ toast)
// ============================================================

type IncompleteDocResult struct {
	MortgageID   uint                               `json:"mortgage_id"`
	TotalItems   int                                `json:"total_items"`
	CheckedItems int                                `json:"checked_items"`
	MissingItems []*models.MortgageDocCheckResponse `json:"missing_items"`
}

func (s *DocCheckService) GetIncompleteItems(ctx context.Context, mortgageID uint) (*IncompleteDocResult, error) {
	_, err := s.mortgageRepo.GetByID(ctx, mortgageID)
	if err != nil {
		return nil, ErrMortgageNotFound
	}

	allChecks, err := s.docCheckRepo.GetByMortgageID(ctx, mortgageID)
	if err != nil {
		return nil, err
	}

	incompleteChecks, err := s.docCheckRepo.GetIncomplete(ctx, mortgageID)
	if err != nil {
		return nil, err
	}

	checkedCount := 0
	for _, c := range allChecks {
		if c.IsChecked {
			checkedCount++
		}
	}

	var missingItems []*models.MortgageDocCheckResponse
	for _, c := range incompleteChecks {
		missingItems = append(missingItems, c.ToResponse())
	}
	if missingItems == nil {
		missingItems = []*models.MortgageDocCheckResponse{}
	}

	return &IncompleteDocResult{
		MortgageID:   mortgageID,
		TotalItems:   len(allChecks),
		CheckedItems: checkedCount,
		MissingItems: missingItems,
	}, nil
}
