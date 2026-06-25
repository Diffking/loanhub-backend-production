package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"

	"gorm.io/gorm"
)

var (
	ErrDocItemNotFound   = errors.New("doc item not found")
	ErrDocCheckNotFound  = errors.New("doc check not found")
	ErrChecklistEmpty    = errors.New("checklist is empty")
	ErrNoIncompleteItems = errors.New("no incomplete items")
	ErrNoLineUserID      = errors.New("member has no LINE account linked")
)

// DocCheckService handles document checklist business logic
type DocCheckService struct {
	docItemRepo  *repositories.DocItemRepository
	docCheckRepo *repositories.MortgageDocCheckRepository
	mortgageRepo *repositories.MortgageRepository
	lineService  *LINEService
	db           *gorm.DB
}

// NewDocCheckService creates a new doc check service
func NewDocCheckService(
	docItemRepo *repositories.DocItemRepository,
	docCheckRepo *repositories.MortgageDocCheckRepository,
	mortgageRepo *repositories.MortgageRepository,
	lineService *LINEService,
	db *gorm.DB,
) *DocCheckService {
	return &DocCheckService{
		docItemRepo:  docItemRepo,
		docCheckRepo: docCheckRepo,
		mortgageRepo: mortgageRepo,
		lineService:  lineService,
		db:           db,
	}
}

// ============================================================
// GetChecklist — ดึงเช็คลิสต์ (สร้างอัตโนมัติถ้ายังไม่มี)
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
// UpdateChecklist — อัพเดทเช็คลิสต์ (batch)
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
// GetIncompleteItems — ดึงรายการเอกสารไม่ครบ (สำหรับ Toast)
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

// ============================================================
// NotifyLineIncompleteDoc — ส่ง LINE แจ้งสมาชิกว่าเอกสารไม่ครบ
// ============================================================

func (s *DocCheckService) NotifyLineIncompleteDoc(ctx context.Context, mortgageID uint) error {
	// 1. ดึง mortgage
	mortgage, err := s.mortgageRepo.GetByID(ctx, mortgageID)
	if err != nil {
		return ErrMortgageNotFound
	}

	// 2. ดึงรายการเอกสารที่ยังไม่ครบ
	incompleteChecks, err := s.docCheckRepo.GetIncomplete(ctx, mortgageID)
	if err != nil {
		return err
	}
	if len(incompleteChecks) == 0 {
		return ErrNoIncompleteItems
	}

	// 3. ดึง line_user_id จาก users (ไม่มีใน Go model → ใช้ raw SQL)
	var lineUserID string
	err = s.db.Raw("SELECT line_user_id FROM users WHERE memb_no = ? AND line_user_id IS NOT NULL AND line_user_id != '' LIMIT 1", mortgage.MembNo).Scan(&lineUserID).Error
	if err != nil || lineUserID == "" {
		return ErrNoLineUserID
	}

	// 4. สร้างข้อความ
	message := fmt.Sprintf("📋 แจ้งเตือนเอกสารสินเชื่อ\nคำขอ #%d\n\n", mortgage.ID)
	message += "❌ เอกสารที่ยังไม่ได้ส่ง:\n"
	for i, check := range incompleteChecks {
		docName := ""
		if check.DocItem != nil {
			docName = check.DocItem.Name
		}
		message += fmt.Sprintf("%d. %s\n", i+1, docName)
	}
	message += "\nกรุณาจัดเตรียมเอกสารข้างต้นและติดต่อสหกรณ์ฯ"

	// 5. ส่ง LINE push message
	channelAccessToken := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if channelAccessToken == "" {
		log.Println("⚠️ LINE_CHANNEL_ACCESS_TOKEN not set")
		return fmt.Errorf("LINE channel access token not configured")
	}

	if s.lineService == nil {
		return fmt.Errorf("LINE service not available")
	}

	return s.lineService.SendPushMessage(lineUserID, message, channelAccessToken)
}
