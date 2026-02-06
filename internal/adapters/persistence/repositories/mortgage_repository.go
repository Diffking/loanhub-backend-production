package repositories

import (
	"context"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// MortgageRepository handles mortgage data access
type MortgageRepository struct {
	db *gorm.DB
}

// NewMortgageRepository creates a new mortgage repository
func NewMortgageRepository(db *gorm.DB) *MortgageRepository {
	return &MortgageRepository{db: db}
}

// Create creates a new mortgage
func (r *MortgageRepository) Create(ctx context.Context, mortgage *models.Mortgage) error {
	return r.db.WithContext(ctx).Create(mortgage).Error
}

// GetByID gets a mortgage by ID with relations
func (r *MortgageRepository) GetByID(ctx context.Context, id uint) (*models.Mortgage, error) {
	var mortgage models.Mortgage
	err := r.db.WithContext(ctx).
		Preload("Officer").
		Preload("Creator").
		Preload("LoanType").
		Preload("CurrentStep").
		Preload("CurrentAppt"). // FK to loan_appts master table
		Preload("CurrentDoc").  // FK to loan_docs master table
		Preload("Approver").
		First(&mortgage, id).Error
	return &mortgage, err
}

// GetByMembNo gets mortgages by member number
func (r *MortgageRepository) GetByMembNo(ctx context.Context, membNo string) ([]*models.Mortgage, error) {
	var mortgages []*models.Mortgage
	err := r.db.WithContext(ctx).
		Preload("LoanType").
		Preload("CurrentStep").
		Preload("CurrentAppt").
		Where("memb_no = ?", membNo).
		Order("created_at DESC").
		Find(&mortgages).Error
	return mortgages, err
}

// List lists all mortgages with pagination
func (r *MortgageRepository) List(ctx context.Context, offset, limit int) ([]*models.Mortgage, int64, error) {
	var mortgages []*models.Mortgage
	var total int64

	r.db.WithContext(ctx).Model(&models.Mortgage{}).Count(&total)

	err := r.db.WithContext(ctx).
		Preload("Officer").
		Preload("LoanType").
		Preload("CurrentStep").
		Preload("CurrentAppt").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&mortgages).Error

	return mortgages, total, err
}

// ListByOfficer lists mortgages by officer
func (r *MortgageRepository) ListByOfficer(ctx context.Context, officerID uint, offset, limit int) ([]*models.Mortgage, int64, error) {
	var mortgages []*models.Mortgage
	var total int64

	r.db.WithContext(ctx).Model(&models.Mortgage{}).Where("officer_id = ?", officerID).Count(&total)

	err := r.db.WithContext(ctx).
		Preload("LoanType").
		Preload("CurrentStep").
		Preload("CurrentAppt").
		Where("officer_id = ?", officerID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&mortgages).Error

	return mortgages, total, err
}

// ListByStep lists mortgages by step
func (r *MortgageRepository) ListByStep(ctx context.Context, stepID uint, offset, limit int) ([]*models.Mortgage, int64, error) {
	var mortgages []*models.Mortgage
	var total int64

	r.db.WithContext(ctx).Model(&models.Mortgage{}).Where("current_step_id = ?", stepID).Count(&total)

	err := r.db.WithContext(ctx).
		Preload("Officer").
		Preload("LoanType").
		Preload("CurrentStep").
		Preload("CurrentAppt").
		Where("current_step_id = ?", stepID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&mortgages).Error

	return mortgages, total, err
}

// Update updates a mortgage
func (r *MortgageRepository) Update(ctx context.Context, mortgage *models.Mortgage) error {
	return r.db.WithContext(ctx).Model(&models.Mortgage{}).Where("id = ?", mortgage.ID).Updates(map[string]interface{}{
		"contract_no":       mortgage.ContractNo,
		"officer_id":        mortgage.OfficerID,
		"amount":            mortgage.Amount,
		"collateral":        mortgage.Collateral,
		"purpose":           mortgage.Purpose,
		"guarantor_memb_no": mortgage.GuarantorMembNo,
		"loan_type_id":      mortgage.LoanTypeID,
		"interest_rate":     mortgage.InterestRate,
		"current_step_id":   mortgage.CurrentStepID,
		"current_appt_id":   mortgage.CurrentApptID,
		"current_doc_id":    mortgage.CurrentDocID,
		"appt_date":         mortgage.ApptDate,
		"appt_time":         mortgage.ApptTime,
		"appt_location":     mortgage.ApptLocation,
		"approved_by":       mortgage.ApprovedBy,
		"approved_at":       mortgage.ApprovedAt,
		"remark":            mortgage.Remark,
	}).Error
}

// Delete soft deletes a mortgage
func (r *MortgageRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Mortgage{}, id).Error
}

// TransactionRepository handles transaction data access
type TransactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository creates a new transaction repository
func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Create creates a new transaction
func (r *TransactionRepository) Create(ctx context.Context, tx *models.Transaction) error {
	return r.db.WithContext(ctx).Create(tx).Error
}

// GetByMortgageID gets transactions by mortgage ID (History)
func (r *TransactionRepository) GetByMortgageID(ctx context.Context, mortgageID uint) ([]*models.Transaction, error) {
	var transactions []*models.Transaction
	err := r.db.WithContext(ctx).
		Preload("Performer").
		Preload("FromStep").
		Preload("ToStep").
		Where("mortgage_id = ?", mortgageID).
		Order("created_at DESC").
		Find(&transactions).Error
	return transactions, err
}
