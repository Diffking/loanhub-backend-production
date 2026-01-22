package repositories

import (
	"context"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// LoanTypeRepository handles loan type data access
type LoanTypeRepository struct {
	db *gorm.DB
}

// NewLoanTypeRepository creates a new loan type repository
func NewLoanTypeRepository(db *gorm.DB) *LoanTypeRepository {
	return &LoanTypeRepository{db: db}
}

// Create creates a new loan type
func (r *LoanTypeRepository) Create(ctx context.Context, loanType *models.LoanType) error {
	return r.db.WithContext(ctx).Create(loanType).Error
}

// GetByID gets a loan type by ID
func (r *LoanTypeRepository) GetByID(ctx context.Context, id uint) (*models.LoanType, error) {
	var loanType models.LoanType
	err := r.db.WithContext(ctx).First(&loanType, id).Error
	return &loanType, err
}

// GetByCode gets a loan type by code
func (r *LoanTypeRepository) GetByCode(ctx context.Context, code string) (*models.LoanType, error) {
	var loanType models.LoanType
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&loanType).Error
	return &loanType, err
}

// List lists all active loan types
func (r *LoanTypeRepository) List(ctx context.Context) ([]*models.LoanType, error) {
	var loanTypes []*models.LoanType
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&loanTypes).Error
	return loanTypes, err
}

// ListAll lists all loan types including inactive
func (r *LoanTypeRepository) ListAll(ctx context.Context) ([]*models.LoanType, error) {
	var loanTypes []*models.LoanType
	err := r.db.WithContext(ctx).Find(&loanTypes).Error
	return loanTypes, err
}

// Update updates a loan type
func (r *LoanTypeRepository) Update(ctx context.Context, loanType *models.LoanType) error {
	return r.db.WithContext(ctx).Save(loanType).Error
}

// Delete soft deletes a loan type
func (r *LoanTypeRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.LoanType{}, id).Error
}

// LoanStepRepository handles loan step data access
type LoanStepRepository struct {
	db *gorm.DB
}

// NewLoanStepRepository creates a new loan step repository
func NewLoanStepRepository(db *gorm.DB) *LoanStepRepository {
	return &LoanStepRepository{db: db}
}

// Create creates a new loan step
func (r *LoanStepRepository) Create(ctx context.Context, loanStep *models.LoanStep) error {
	return r.db.WithContext(ctx).Create(loanStep).Error
}

// GetByID gets a loan step by ID
func (r *LoanStepRepository) GetByID(ctx context.Context, id uint) (*models.LoanStep, error) {
	var loanStep models.LoanStep
	err := r.db.WithContext(ctx).First(&loanStep, id).Error
	return &loanStep, err
}

// GetByCode gets a loan step by code
func (r *LoanStepRepository) GetByCode(ctx context.Context, code string) (*models.LoanStep, error) {
	var loanStep models.LoanStep
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&loanStep).Error
	return &loanStep, err
}

// GetFirstStep gets the first step (lowest order)
func (r *LoanStepRepository) GetFirstStep(ctx context.Context) (*models.LoanStep, error) {
	var loanStep models.LoanStep
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("step_order ASC").
		First(&loanStep).Error
	return &loanStep, err
}

// List lists all active loan steps ordered by step_order
func (r *LoanStepRepository) List(ctx context.Context) ([]*models.LoanStep, error) {
	var loanSteps []*models.LoanStep
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("step_order ASC").
		Find(&loanSteps).Error
	return loanSteps, err
}

// ListAll lists all loan steps including inactive
func (r *LoanStepRepository) ListAll(ctx context.Context) ([]*models.LoanStep, error) {
	var loanSteps []*models.LoanStep
	err := r.db.WithContext(ctx).Order("step_order ASC").Find(&loanSteps).Error
	return loanSteps, err
}

// Update updates a loan step
func (r *LoanStepRepository) Update(ctx context.Context, loanStep *models.LoanStep) error {
	return r.db.WithContext(ctx).Save(loanStep).Error
}

// Delete soft deletes a loan step
func (r *LoanStepRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.LoanStep{}, id).Error
}

// LoanDocRepository handles loan doc data access
type LoanDocRepository struct {
	db *gorm.DB
}

// NewLoanDocRepository creates a new loan doc repository
func NewLoanDocRepository(db *gorm.DB) *LoanDocRepository {
	return &LoanDocRepository{db: db}
}

// Create creates a new loan doc
func (r *LoanDocRepository) Create(ctx context.Context, loanDoc *models.LoanDoc) error {
	return r.db.WithContext(ctx).Create(loanDoc).Error
}

// GetByID gets a loan doc by ID
func (r *LoanDocRepository) GetByID(ctx context.Context, id uint) (*models.LoanDoc, error) {
	var loanDoc models.LoanDoc
	err := r.db.WithContext(ctx).First(&loanDoc, id).Error
	return &loanDoc, err
}

// GetByCode gets a loan doc by code
func (r *LoanDocRepository) GetByCode(ctx context.Context, code string) (*models.LoanDoc, error) {
	var loanDoc models.LoanDoc
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&loanDoc).Error
	return &loanDoc, err
}

// List lists all active loan docs
func (r *LoanDocRepository) List(ctx context.Context) ([]*models.LoanDoc, error) {
	var loanDocs []*models.LoanDoc
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&loanDocs).Error
	return loanDocs, err
}

// ListAll lists all loan docs including inactive
func (r *LoanDocRepository) ListAll(ctx context.Context) ([]*models.LoanDoc, error) {
	var loanDocs []*models.LoanDoc
	err := r.db.WithContext(ctx).Find(&loanDocs).Error
	return loanDocs, err
}

// Update updates a loan doc
func (r *LoanDocRepository) Update(ctx context.Context, loanDoc *models.LoanDoc) error {
	return r.db.WithContext(ctx).Save(loanDoc).Error
}

// Delete soft deletes a loan doc
func (r *LoanDocRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.LoanDoc{}, id).Error
}

// LoanApptRepository handles loan appt data access
type LoanApptRepository struct {
	db *gorm.DB
}

// NewLoanApptRepository creates a new loan appt repository
func NewLoanApptRepository(db *gorm.DB) *LoanApptRepository {
	return &LoanApptRepository{db: db}
}

// Create creates a new loan appt
func (r *LoanApptRepository) Create(ctx context.Context, loanAppt *models.LoanAppt) error {
	return r.db.WithContext(ctx).Create(loanAppt).Error
}

// GetByID gets a loan appt by ID
func (r *LoanApptRepository) GetByID(ctx context.Context, id uint) (*models.LoanAppt, error) {
	var loanAppt models.LoanAppt
	err := r.db.WithContext(ctx).First(&loanAppt, id).Error
	return &loanAppt, err
}

// GetByCode gets a loan appt by code
func (r *LoanApptRepository) GetByCode(ctx context.Context, code string) (*models.LoanAppt, error) {
	var loanAppt models.LoanAppt
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&loanAppt).Error
	return &loanAppt, err
}

// List lists all active loan appts
func (r *LoanApptRepository) List(ctx context.Context) ([]*models.LoanAppt, error) {
	var loanAppts []*models.LoanAppt
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&loanAppts).Error
	return loanAppts, err
}

// ListAll lists all loan appts including inactive
func (r *LoanApptRepository) ListAll(ctx context.Context) ([]*models.LoanAppt, error) {
	var loanAppts []*models.LoanAppt
	err := r.db.WithContext(ctx).Find(&loanAppts).Error
	return loanAppts, err
}

// Update updates a loan appt
func (r *LoanApptRepository) Update(ctx context.Context, loanAppt *models.LoanAppt) error {
	return r.db.WithContext(ctx).Save(loanAppt).Error
}

// Delete soft deletes a loan appt
func (r *LoanApptRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.LoanAppt{}, id).Error
}
