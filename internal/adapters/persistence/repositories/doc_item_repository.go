package repositories

import (
	"context"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// DocItemRepository handles doc_items data access
type DocItemRepository struct {
	db *gorm.DB
}

// NewDocItemRepository creates a new doc item repository
func NewDocItemRepository(db *gorm.DB) *DocItemRepository {
	return &DocItemRepository{db: db}
}

// Create creates a new doc item
func (r *DocItemRepository) Create(ctx context.Context, docItem *models.DocItem) error {
	return r.db.WithContext(ctx).Create(docItem).Error
}

// GetByID gets a doc item by ID
func (r *DocItemRepository) GetByID(ctx context.Context, id uint) (*models.DocItem, error) {
	var docItem models.DocItem
	err := r.db.WithContext(ctx).Preload("LoanType").First(&docItem, id).Error
	return &docItem, err
}

// ListByLoanType lists active doc items for a specific loan type
func (r *DocItemRepository) ListByLoanType(ctx context.Context, loanTypeID uint) ([]*models.DocItem, error) {
	var docItems []*models.DocItem
	err := r.db.WithContext(ctx).
		Where("loan_type_id = ? AND is_active = ?", loanTypeID, true).
		Order("sort_order ASC, id ASC").
		Find(&docItems).Error
	return docItems, err
}

// List lists all active doc items
func (r *DocItemRepository) List(ctx context.Context) ([]*models.DocItem, error) {
	var docItems []*models.DocItem
	err := r.db.WithContext(ctx).
		Preload("LoanType").
		Where("is_active = ?", true).
		Order("loan_type_id ASC, sort_order ASC, id ASC").
		Find(&docItems).Error
	return docItems, err
}

// ListAll lists all doc items including inactive
func (r *DocItemRepository) ListAll(ctx context.Context) ([]*models.DocItem, error) {
	var docItems []*models.DocItem
	err := r.db.WithContext(ctx).
		Preload("LoanType").
		Order("loan_type_id ASC, sort_order ASC, id ASC").
		Find(&docItems).Error
	return docItems, err
}

// Update updates a doc item
func (r *DocItemRepository) Update(ctx context.Context, docItem *models.DocItem) error {
	return r.db.WithContext(ctx).Save(docItem).Error
}

// Delete soft deletes a doc item
func (r *DocItemRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.DocItem{}, id).Error
}
