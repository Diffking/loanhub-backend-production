package repositories

import (
	"context"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// MortgageDocCheckRepository handles mortgage_doc_checks data access
type MortgageDocCheckRepository struct {
	db *gorm.DB
}

// NewMortgageDocCheckRepository creates a new repository
func NewMortgageDocCheckRepository(db *gorm.DB) *MortgageDocCheckRepository {
	return &MortgageDocCheckRepository{db: db}
}

// GetByMortgageID gets all doc checks for a mortgage (with DocItem preloaded)
func (r *MortgageDocCheckRepository) GetByMortgageID(ctx context.Context, mortgageID uint) ([]*models.MortgageDocCheck, error) {
	var checks []*models.MortgageDocCheck
	err := r.db.WithContext(ctx).
		Preload("DocItem").
		Where("mortgage_id = ?", mortgageID).
		Order("id ASC").
		Find(&checks).Error
	return checks, err
}

// InitChecklist creates checklist from doc_items for a mortgage
// ใช้ตอนเปิดเช็คลิสต์ครั้งแรก — สร้างจาก doc_items ตาม loan_type
func (r *MortgageDocCheckRepository) InitChecklist(ctx context.Context, mortgageID uint, loanTypeID uint) error {
	// ดึง doc_items ที่ active ตาม loan_type
	var docItems []*models.DocItem
	err := r.db.WithContext(ctx).
		Where("loan_type_id = ? AND is_active = ?", loanTypeID, true).
		Order("sort_order ASC, id ASC").
		Find(&docItems).Error
	if err != nil {
		return err
	}

	// สร้าง checklist
	for _, item := range docItems {
		check := &models.MortgageDocCheck{
			MortgageID:    mortgageID,
			DocItemID:     item.ID,
			IsChecked:     false,
			IsRecommended: false,
		}
		// ใช้ FirstOrCreate เพื่อไม่สร้างซ้ำ
		r.db.WithContext(ctx).
			Where("mortgage_id = ? AND doc_item_id = ?", mortgageID, item.ID).
			FirstOrCreate(check)
	}

	return nil
}

// UpdateCheck updates a single doc check
func (r *MortgageDocCheckRepository) UpdateCheck(ctx context.Context, check *models.MortgageDocCheck) error {
	return r.db.WithContext(ctx).Save(check).Error
}

// BatchUpdate updates multiple doc checks
func (r *MortgageDocCheckRepository) BatchUpdate(ctx context.Context, checks []*models.MortgageDocCheck) error {
	tx := r.db.WithContext(ctx).Begin()
	for _, check := range checks {
		if err := tx.Save(check).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// GetByID gets a single doc check by ID
func (r *MortgageDocCheckRepository) GetByID(ctx context.Context, id uint) (*models.MortgageDocCheck, error) {
	var check models.MortgageDocCheck
	err := r.db.WithContext(ctx).Preload("DocItem").First(&check, id).Error
	return &check, err
}

// GetIncomplete gets unchecked required/recommended items for a mortgage
func (r *MortgageDocCheckRepository) GetIncomplete(ctx context.Context, mortgageID uint) ([]*models.MortgageDocCheck, error) {
	var checks []*models.MortgageDocCheck
	err := r.db.WithContext(ctx).
		Preload("DocItem").
		Joins("JOIN doc_items ON doc_items.id = mortgage_doc_checks.doc_item_id").
		Where("mortgage_doc_checks.mortgage_id = ? AND mortgage_doc_checks.is_checked = ?", mortgageID, false).
		Where("(doc_items.is_required = ? OR mortgage_doc_checks.is_recommended = ?)", true, true).
		Order("doc_items.sort_order ASC").
		Find(&checks).Error
	return checks, err
}
