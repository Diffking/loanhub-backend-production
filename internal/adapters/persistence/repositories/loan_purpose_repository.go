package repositories

import (
	"context"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// LoanPurposeRepository handles loan_purposes (เหตุผลการกู้) data access.
// Data source: FLOPRESN.txt seeded via cmd/seed-loan-purposes.
type LoanPurposeRepository struct {
	db *gorm.DB
}

// NewLoanPurposeRepository creates a new loan purpose repository
func NewLoanPurposeRepository(db *gorm.DB) *LoanPurposeRepository {
	return &LoanPurposeRepository{db: db}
}

// Create creates a new loan purpose
func (r *LoanPurposeRepository) Create(ctx context.Context, p *models.LoanPurpose) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// GetByID gets a loan purpose by ID
func (r *LoanPurposeRepository) GetByID(ctx context.Context, id uint) (*models.LoanPurpose, error) {
	var p models.LoanPurpose
	err := r.db.WithContext(ctx).First(&p, id).Error
	return &p, err
}

// GetByCode gets a loan purpose by code (e.g. "001")
func (r *LoanPurposeRepository) GetByCode(ctx context.Context, code string) (*models.LoanPurpose, error) {
	var p models.LoanPurpose
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&p).Error
	return &p, err
}

// List lists all active loan purposes, ordered by code
func (r *LoanPurposeRepository) List(ctx context.Context) ([]*models.LoanPurpose, error) {
	var list []*models.LoanPurpose
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("code ASC").
		Find(&list).Error
	return list, err
}

// ListAll lists every loan purpose (incl. inactive)
func (r *LoanPurposeRepository) ListAll(ctx context.Context) ([]*models.LoanPurpose, error) {
	var list []*models.LoanPurpose
	err := r.db.WithContext(ctx).Order("code ASC").Find(&list).Error
	return list, err
}

// UpsertByCode inserts or updates by code (used by seeder)
func (r *LoanPurposeRepository) UpsertByCode(ctx context.Context, p *models.LoanPurpose) error {
	var existing models.LoanPurpose
	err := r.db.WithContext(ctx).Where("code = ?", p.Code).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(p).Error
	}
	if err != nil {
		return err
	}
	existing.Name = p.Name
	existing.IsActive = true
	return r.db.WithContext(ctx).Save(&existing).Error
}

// Update updates a loan purpose
func (r *LoanPurposeRepository) Update(ctx context.Context, p *models.LoanPurpose) error {
	return r.db.WithContext(ctx).Save(p).Error
}

// Delete soft-deletes a loan purpose
func (r *LoanPurposeRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.LoanPurpose{}, id).Error
}

// Count returns active count
func (r *LoanPurposeRepository) Count(ctx context.Context) (int64, error) {
	var c int64
	err := r.db.WithContext(ctx).Model(&models.LoanPurpose{}).
		Where("is_active = ?", true).Count(&c).Error
	return c, err
}
