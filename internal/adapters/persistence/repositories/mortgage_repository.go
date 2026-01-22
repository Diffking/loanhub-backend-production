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
		Where("current_step_id = ?", stepID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&mortgages).Error

	return mortgages, total, err
}

// Update updates a mortgage
func (r *MortgageRepository) Update(ctx context.Context, mortgage *models.Mortgage) error {
	return r.db.WithContext(ctx).Save(mortgage).Error
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

// LoanDocCurrentRepository handles loan doc current data access
type LoanDocCurrentRepository struct {
	db *gorm.DB
}

// NewLoanDocCurrentRepository creates a new loan doc current repository
func NewLoanDocCurrentRepository(db *gorm.DB) *LoanDocCurrentRepository {
	return &LoanDocCurrentRepository{db: db}
}

// Create creates a new loan doc current
func (r *LoanDocCurrentRepository) Create(ctx context.Context, doc *models.LoanDocCurrent) error {
	return r.db.WithContext(ctx).Create(doc).Error
}

// CreateBatch creates multiple loan doc currents
func (r *LoanDocCurrentRepository) CreateBatch(ctx context.Context, docs []*models.LoanDocCurrent) error {
	return r.db.WithContext(ctx).Create(docs).Error
}

// GetByMortgageID gets loan doc currents by mortgage ID
func (r *LoanDocCurrentRepository) GetByMortgageID(ctx context.Context, mortgageID uint) ([]*models.LoanDocCurrent, error) {
	var docs []*models.LoanDocCurrent
	err := r.db.WithContext(ctx).
		Preload("LoanDoc").
		Preload("Checker").
		Where("mortgage_id = ?", mortgageID).
		Find(&docs).Error
	return docs, err
}

// Update updates a loan doc current
func (r *LoanDocCurrentRepository) Update(ctx context.Context, doc *models.LoanDocCurrent) error {
	return r.db.WithContext(ctx).Save(doc).Error
}

// GetByID gets a loan doc current by ID
func (r *LoanDocCurrentRepository) GetByID(ctx context.Context, id uint) (*models.LoanDocCurrent, error) {
	var doc models.LoanDocCurrent
	err := r.db.WithContext(ctx).First(&doc, id).Error
	return &doc, err
}

// LoanApptCurrentRepository handles loan appt current data access
type LoanApptCurrentRepository struct {
	db *gorm.DB
}

// NewLoanApptCurrentRepository creates a new loan appt current repository
func NewLoanApptCurrentRepository(db *gorm.DB) *LoanApptCurrentRepository {
	return &LoanApptCurrentRepository{db: db}
}

// Create creates a new loan appt current
func (r *LoanApptCurrentRepository) Create(ctx context.Context, appt *models.LoanApptCurrent) error {
	return r.db.WithContext(ctx).Create(appt).Error
}

// GetByID gets a loan appt current by ID
func (r *LoanApptCurrentRepository) GetByID(ctx context.Context, id uint) (*models.LoanApptCurrent, error) {
	var appt models.LoanApptCurrent
	err := r.db.WithContext(ctx).
		Preload("LoanAppt").
		Preload("Officer").
		First(&appt, id).Error
	return &appt, err
}

// GetByMortgageID gets loan appt currents by mortgage ID
func (r *LoanApptCurrentRepository) GetByMortgageID(ctx context.Context, mortgageID uint) ([]*models.LoanApptCurrent, error) {
	var appts []*models.LoanApptCurrent
	err := r.db.WithContext(ctx).
		Preload("LoanAppt").
		Preload("Officer").
		Where("mortgage_id = ?", mortgageID).
		Order("created_at DESC").
		Find(&appts).Error
	return appts, err
}

// Update updates a loan appt current
func (r *LoanApptCurrentRepository) Update(ctx context.Context, appt *models.LoanApptCurrent) error {
	return r.db.WithContext(ctx).Save(appt).Error
}
