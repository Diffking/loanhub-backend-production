package services

import (
	"context"
	"errors"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"

	"gorm.io/gorm"
)

// Mortgage service errors
var (
	ErrMortgageNotFound         = errors.New("mortgage not found")
	ErrLoanTypeNotFound         = errors.New("loan type not found")
	ErrLoanStepNotFound         = errors.New("loan step not found")
	ErrLoanDocNotFound          = errors.New("loan doc not found")
	ErrLoanApptNotFound         = errors.New("loan appt not found")
	ErrMemberNotFoundMortgage   = errors.New("member not found")
	ErrOfficerNotFound          = errors.New("officer not found")
	ErrNotAuthorized            = errors.New("not authorized")
	ErrInvalidStep              = errors.New("invalid step transition")
	ErrAlreadyApproved          = errors.New("mortgage already approved")
	ErrApptNotFound             = errors.New("appointment not found")
)

// MortgageService handles mortgage business logic
type MortgageService struct {
	mortgageRepo     *repositories.MortgageRepository
	transactionRepo  *repositories.TransactionRepository
	loanTypeRepo     *repositories.LoanTypeRepository
	loanStepRepo     *repositories.LoanStepRepository
	loanDocRepo      *repositories.LoanDocRepository
	loanApptRepo     *repositories.LoanApptRepository
	loanDocCurrRepo  *repositories.LoanDocCurrentRepository
	loanApptCurrRepo *repositories.LoanApptCurrentRepository
	memberRepo       repositories.MemberRepository
	userRepo         repositories.UserRepository
	notifyService    *NotificationService
}

// NewMortgageService creates a new mortgage service
func NewMortgageService(
	mortgageRepo *repositories.MortgageRepository,
	transactionRepo *repositories.TransactionRepository,
	loanTypeRepo *repositories.LoanTypeRepository,
	loanStepRepo *repositories.LoanStepRepository,
	loanDocRepo *repositories.LoanDocRepository,
	loanApptRepo *repositories.LoanApptRepository,
	loanDocCurrRepo *repositories.LoanDocCurrentRepository,
	loanApptCurrRepo *repositories.LoanApptCurrentRepository,
	memberRepo repositories.MemberRepository,
	userRepo repositories.UserRepository,
	notifyService *NotificationService,
) *MortgageService {
	return &MortgageService{
		mortgageRepo:     mortgageRepo,
		transactionRepo:  transactionRepo,
		loanTypeRepo:     loanTypeRepo,
		loanStepRepo:     loanStepRepo,
		loanDocRepo:      loanDocRepo,
		loanApptRepo:     loanApptRepo,
		loanDocCurrRepo:  loanDocCurrRepo,
		loanApptCurrRepo: loanApptCurrRepo,
		memberRepo:       memberRepo,
		userRepo:         userRepo,
		notifyService:    notifyService,
	}
}

// CreateMortgageInput represents create mortgage input
type CreateMortgageInput struct {
	MembNo          string  `json:"memb_no" validate:"required"`
	LoanTypeID      uint    `json:"loan_type_id" validate:"required"`
	Amount          float64 `json:"amount" validate:"required,gt=0"`
	Collateral      string  `json:"collateral,omitempty"`
	Purpose         string  `json:"purpose,omitempty"`
	GuarantorMembNo string  `json:"guarantor_memb_no,omitempty"`
	Remark          string  `json:"remark,omitempty"`
}

// Create creates a new mortgage
func (s *MortgageService) Create(ctx context.Context, input *CreateMortgageInput, officerID uint, ipAddress string) (*models.Mortgage, error) {
	// Validate member exists
	member, err := s.memberRepo.GetByMembNo(ctx, input.MembNo)
	if err != nil || member == nil {
		return nil, ErrMemberNotFoundMortgage
	}

	// Validate loan type exists
	loanType, err := s.loanTypeRepo.GetByID(ctx, input.LoanTypeID)
	if err != nil {
		return nil, ErrLoanTypeNotFound
	}

	// Get first step
	firstStep, err := s.loanStepRepo.GetFirstStep(ctx)
	if err != nil {
		return nil, ErrLoanStepNotFound
	}

	// Create mortgage
	mortgage := &models.Mortgage{
		MembNo:        input.MembNo,
		OfficerID:     officerID,
		UserID:        officerID,
		Amount:        input.Amount,
		Collateral:    input.Collateral,
		Purpose:       input.Purpose,
		LoanTypeID:    input.LoanTypeID,
		InterestRate:  loanType.InterestRate,
		CurrentStepID: firstStep.ID,
		Remark:        input.Remark,
	}

	if input.GuarantorMembNo != "" {
		mortgage.GuarantorMembNo = &input.GuarantorMembNo
	}

	if err := s.mortgageRepo.Create(ctx, mortgage); err != nil {
		return nil, err
	}

	// Create transaction (History)
	tx := &models.Transaction{
		MortgageID:      mortgage.ID,
		TransactionType: models.TxTypeCreate,
		ToStepID:        &firstStep.ID,
		ToTypeID:        &loanType.ID,
		Amount:          &input.Amount,
		Description:     "สร้างคำขอสินเชื่อใหม่",
		PerformedBy:     officerID,
		IPAddress:       ipAddress,
	}
	s.transactionRepo.Create(ctx, tx)

	// Create loan doc currents (checklist)
	docs, _ := s.loanDocRepo.List(ctx)
	for _, doc := range docs {
		docCurr := &models.LoanDocCurrent{
			MortgageID:  mortgage.ID,
			LoanDocID:   doc.ID,
			IsSubmitted: false,
		}
		s.loanDocCurrRepo.Create(ctx, docCurr)
	}

	// Send LINE notification
	if s.notifyService != nil {
		s.notifyService.NotifyNewMortgage(mortgage, member.FullName)
	}

	return mortgage, nil
}

// GetByID gets a mortgage by ID
func (s *MortgageService) GetByID(ctx context.Context, id uint) (*models.Mortgage, error) {
	mortgage, err := s.mortgageRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMortgageNotFound
		}
		return nil, err
	}
	return mortgage, nil
}

// GetByMembNo gets mortgages by member number (for member to view)
func (s *MortgageService) GetByMembNo(ctx context.Context, membNo string) ([]*models.Mortgage, error) {
	return s.mortgageRepo.GetByMembNo(ctx, membNo)
}

// ListInput represents list input
type ListInput struct {
	Page      int
	Limit     int
	OfficerID *uint
	StepID    *uint
}

// ListOutput represents list output
type ListOutput struct {
	Mortgages  []*models.Mortgage `json:"mortgages"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
	TotalPages int                `json:"total_pages"`
}

// List lists mortgages
func (s *MortgageService) List(ctx context.Context, input *ListInput) (*ListOutput, error) {
	if input.Page < 1 {
		input.Page = 1
	}
	if input.Limit < 1 {
		input.Limit = 10
	}
	if input.Limit > 100 {
		input.Limit = 100
	}

	offset := (input.Page - 1) * input.Limit
	var mortgages []*models.Mortgage
	var total int64
	var err error

	if input.OfficerID != nil {
		mortgages, total, err = s.mortgageRepo.ListByOfficer(ctx, *input.OfficerID, offset, input.Limit)
	} else if input.StepID != nil {
		mortgages, total, err = s.mortgageRepo.ListByStep(ctx, *input.StepID, offset, input.Limit)
	} else {
		mortgages, total, err = s.mortgageRepo.List(ctx, offset, input.Limit)
	}

	if err != nil {
		return nil, err
	}

	totalPages := int(total) / input.Limit
	if int(total)%input.Limit > 0 {
		totalPages++
	}

	return &ListOutput{
		Mortgages:  mortgages,
		Total:      total,
		Page:       input.Page,
		Limit:      input.Limit,
		TotalPages: totalPages,
	}, nil
}

// ChangeStepInput represents change step input
type ChangeStepInput struct {
	StepID uint   `json:"step_id" validate:"required"`
	Remark string `json:"remark,omitempty"`
}

// ChangeStep changes mortgage step
func (s *MortgageService) ChangeStep(ctx context.Context, mortgageID uint, input *ChangeStepInput, userID uint, ipAddress string) (*models.Mortgage, error) {
	mortgage, err := s.mortgageRepo.GetByID(ctx, mortgageID)
	if err != nil {
		return nil, ErrMortgageNotFound
	}

	// Validate new step exists
	newStep, err := s.loanStepRepo.GetByID(ctx, input.StepID)
	if err != nil {
		return nil, ErrLoanStepNotFound
	}

	oldStepID := mortgage.CurrentStepID

	// Update mortgage
	mortgage.CurrentStepID = newStep.ID
	if err := s.mortgageRepo.Update(ctx, mortgage); err != nil {
		return nil, err
	}

	// Create transaction
	tx := &models.Transaction{
		MortgageID:      mortgageID,
		TransactionType: models.TxTypeStatusChange,
		FromStepID:      &oldStepID,
		ToStepID:        &newStep.ID,
		Description:     input.Remark,
		PerformedBy:     userID,
		IPAddress:       ipAddress,
	}
	s.transactionRepo.Create(ctx, tx)

	// Create loan step current
	now := time.Now()
	stepCurr := &models.LoanStepCurrent{
		TransactionID: tx.ID,
		MortgageID:    mortgageID,
		LoanStepID:    newStep.ID,
		FromStepID:    &oldStepID,
		ChangedBy:     userID,
		ChangedAt:     now,
		Remark:        input.Remark,
	}
	// Note: Need to add LoanStepCurrentRepository if needed

	_ = stepCurr // placeholder

	// Send LINE notification
	if s.notifyService != nil {
		s.notifyService.NotifyStatusChange(mortgage, newStep.Name)
	}

	return mortgage, nil
}

// ApproveInput represents approve input
type ApproveInput struct {
	ContractNo string `json:"contract_no" validate:"required"`
	Remark     string `json:"remark,omitempty"`
}

// Approve approves a mortgage
func (s *MortgageService) Approve(ctx context.Context, mortgageID uint, input *ApproveInput, approverID uint, ipAddress string) (*models.Mortgage, error) {
	mortgage, err := s.mortgageRepo.GetByID(ctx, mortgageID)
	if err != nil {
		return nil, ErrMortgageNotFound
	}

	if mortgage.ApprovedAt != nil {
		return nil, ErrAlreadyApproved
	}

	// Get approved step
	approvedStep, err := s.loanStepRepo.GetByCode(ctx, "APPROVED")
	if err != nil {
		return nil, ErrLoanStepNotFound
	}

	oldStepID := mortgage.CurrentStepID
	now := time.Now()

	// Update mortgage
	mortgage.ContractNo = &input.ContractNo
	mortgage.ApprovedBy = &approverID
	mortgage.ApprovedAt = &now
	mortgage.CurrentStepID = approvedStep.ID
	mortgage.Remark = input.Remark

	if err := s.mortgageRepo.Update(ctx, mortgage); err != nil {
		return nil, err
	}

	// Create transaction
	tx := &models.Transaction{
		MortgageID:      mortgageID,
		TransactionType: models.TxTypeApprove,
		FromStepID:      &oldStepID,
		ToStepID:        &approvedStep.ID,
		Description:     "อนุมัติสินเชื่อ: " + input.Remark,
		PerformedBy:     approverID,
		IPAddress:       ipAddress,
	}
	s.transactionRepo.Create(ctx, tx)

	// Send LINE notification
	if s.notifyService != nil {
		s.notifyService.NotifyApproved(mortgage)
	}

	return mortgage, nil
}

// RejectInput represents reject input
type RejectInput struct {
	Remark string `json:"remark" validate:"required"`
}

// Reject rejects a mortgage
func (s *MortgageService) Reject(ctx context.Context, mortgageID uint, input *RejectInput, userID uint, ipAddress string) (*models.Mortgage, error) {
	mortgage, err := s.mortgageRepo.GetByID(ctx, mortgageID)
	if err != nil {
		return nil, ErrMortgageNotFound
	}

	// Get rejected step
	rejectedStep, err := s.loanStepRepo.GetByCode(ctx, "REJECTED")
	if err != nil {
		return nil, ErrLoanStepNotFound
	}

	oldStepID := mortgage.CurrentStepID

	// Update mortgage
	mortgage.CurrentStepID = rejectedStep.ID
	mortgage.Remark = input.Remark

	if err := s.mortgageRepo.Update(ctx, mortgage); err != nil {
		return nil, err
	}

	// Create transaction
	tx := &models.Transaction{
		MortgageID:      mortgageID,
		TransactionType: models.TxTypeReject,
		FromStepID:      &oldStepID,
		ToStepID:        &rejectedStep.ID,
		Description:     "ปฏิเสธสินเชื่อ: " + input.Remark,
		PerformedBy:     userID,
		IPAddress:       ipAddress,
	}
	s.transactionRepo.Create(ctx, tx)

	// Send LINE notification
	if s.notifyService != nil {
		s.notifyService.NotifyRejected(mortgage, input.Remark)
	}

	return mortgage, nil
}

// GetHistory gets mortgage history
func (s *MortgageService) GetHistory(ctx context.Context, mortgageID uint) ([]*models.Transaction, error) {
	// Verify mortgage exists
	_, err := s.mortgageRepo.GetByID(ctx, mortgageID)
	if err != nil {
		return nil, ErrMortgageNotFound
	}

	return s.transactionRepo.GetByMortgageID(ctx, mortgageID)
}

// UpdateDocInput represents update doc input
type UpdateDocInput struct {
	DocID       uint   `json:"doc_id" validate:"required"`
	IsSubmitted bool   `json:"is_submitted"`
	Remark      string `json:"remark,omitempty"`
}

// UpdateDoc updates document status
func (s *MortgageService) UpdateDoc(ctx context.Context, mortgageID uint, input *UpdateDocInput, userID uint, ipAddress string) error {
	// Get doc current
	doc, err := s.loanDocCurrRepo.GetByID(ctx, input.DocID)
	if err != nil {
		return ErrLoanDocNotFound
	}

	if doc.MortgageID != mortgageID {
		return ErrNotAuthorized
	}

	now := time.Now()
	doc.IsSubmitted = input.IsSubmitted
	doc.CheckedBy = &userID
	doc.CheckedAt = &now
	doc.Remark = input.Remark

	if err := s.loanDocCurrRepo.Update(ctx, doc); err != nil {
		return err
	}

	// Create transaction
	tx := &models.Transaction{
		MortgageID:      mortgageID,
		TransactionType: models.TxTypeDocCheck,
		ToDocID:         &doc.LoanDocID,
		Description:     input.Remark,
		PerformedBy:     userID,
		IPAddress:       ipAddress,
	}
	s.transactionRepo.Create(ctx, tx)

	return nil
}

// GetDocs gets mortgage documents
func (s *MortgageService) GetDocs(ctx context.Context, mortgageID uint) ([]*models.LoanDocCurrent, error) {
	return s.loanDocCurrRepo.GetByMortgageID(ctx, mortgageID)
}

// CreateApptInput represents create appointment input
type CreateApptInput struct {
	LoanApptID uint   `json:"loan_appt_id" validate:"required"`
	ApptDate   string `json:"appt_date" validate:"required"`
	ApptTime   string `json:"appt_time,omitempty"`
	Location   string `json:"location,omitempty"`
	Remark     string `json:"remark,omitempty"`
}

// CreateAppt creates an appointment
func (s *MortgageService) CreateAppt(ctx context.Context, mortgageID uint, input *CreateApptInput, userID uint, ipAddress string) (*models.LoanApptCurrent, error) {
	mortgage, err := s.mortgageRepo.GetByID(ctx, mortgageID)
	if err != nil {
		return nil, ErrMortgageNotFound
	}

	// Validate loan appt exists
	loanAppt, err := s.loanApptRepo.GetByID(ctx, input.LoanApptID)
	if err != nil {
		return nil, ErrLoanApptNotFound
	}

	// Parse date
	apptDate, err := time.Parse("2006-01-02", input.ApptDate)
	if err != nil {
		return nil, errors.New("invalid date format, use YYYY-MM-DD")
	}

	// Create transaction first
	tx := &models.Transaction{
		MortgageID:      mortgageID,
		TransactionType: models.TxTypeApptCreate,
		ToApptID:        &loanAppt.ID,
		Description:     input.Remark,
		PerformedBy:     userID,
		IPAddress:       ipAddress,
	}
	s.transactionRepo.Create(ctx, tx)

	// Use default location if not provided
	location := input.Location
	if location == "" {
		location = loanAppt.DefaultLocation
	}

	// Create appt current
	apptCurr := &models.LoanApptCurrent{
		TransactionID: &tx.ID,
		MortgageID:    mortgageID,
		LoanApptID:    input.LoanApptID,
		ApptDate:      apptDate,
		Location:      location,
		ApptBy:        mortgage.OfficerID, // Use officer from mortgage
		Status:        models.ApptStatusPending,
		Remark:        input.Remark,
	}

	if input.ApptTime != "" {
		apptCurr.ApptTime = &input.ApptTime
	}

	if err := s.loanApptCurrRepo.Create(ctx, apptCurr); err != nil {
		return nil, err
	}

	// Update mortgage current appt
	mortgage.CurrentApptID = &apptCurr.ID
	s.mortgageRepo.Update(ctx, mortgage)

	// Send LINE notification
	if s.notifyService != nil {
		s.notifyService.NotifyNewAppointment(mortgage, loanAppt.Name, input.ApptDate)
	}

	return apptCurr, nil
}

// CompleteAppt completes an appointment
func (s *MortgageService) CompleteAppt(ctx context.Context, mortgageID uint, apptID uint, userID uint, ipAddress string) error {
	appt, err := s.loanApptCurrRepo.GetByID(ctx, apptID)
	if err != nil {
		return ErrApptNotFound
	}

	if appt.MortgageID != mortgageID {
		return ErrNotAuthorized
	}

	now := time.Now()
	appt.Status = models.ApptStatusCompleted
	appt.CompletedAt = &now

	if err := s.loanApptCurrRepo.Update(ctx, appt); err != nil {
		return err
	}

	// Create transaction
	tx := &models.Transaction{
		MortgageID:      mortgageID,
		TransactionType: models.TxTypeApptComplete,
		ToApptID:        &appt.LoanApptID,
		Description:     "นัดหมายเสร็จสิ้น",
		PerformedBy:     userID,
		IPAddress:       ipAddress,
	}
	s.transactionRepo.Create(ctx, tx)

	return nil
}

// GetAppts gets mortgage appointments
func (s *MortgageService) GetAppts(ctx context.Context, mortgageID uint) ([]*models.LoanApptCurrent, error) {
	return s.loanApptCurrRepo.GetByMortgageID(ctx, mortgageID)
}

// ChangeOfficerInput represents change officer input
type ChangeOfficerInput struct {
	OfficerID uint   `json:"officer_id" validate:"required"`
	Remark    string `json:"remark,omitempty"`
}

// ChangeOfficer changes the responsible officer
func (s *MortgageService) ChangeOfficer(ctx context.Context, mortgageID uint, input *ChangeOfficerInput, userID uint, ipAddress string) (*models.Mortgage, error) {
	mortgage, err := s.mortgageRepo.GetByID(ctx, mortgageID)
	if err != nil {
		return nil, ErrMortgageNotFound
	}

	// Validate officer exists and is OFFICER or ADMIN
	officer, err := s.userRepo.GetByID(ctx, input.OfficerID)
	if err != nil || officer == nil {
		return nil, ErrOfficerNotFound
	}

	if officer.Role != "OFFICER" && officer.Role != "ADMIN" {
		return nil, errors.New("user is not an officer")
	}

	oldOfficerID := mortgage.OfficerID
	mortgage.OfficerID = input.OfficerID

	if err := s.mortgageRepo.Update(ctx, mortgage); err != nil {
		return nil, err
	}

	// Create transaction
	tx := &models.Transaction{
		MortgageID:      mortgageID,
		TransactionType: models.TxTypeOfficerChange,
		Description:     input.Remark,
		PerformedBy:     userID,
		IPAddress:       ipAddress,
	}
	_ = oldOfficerID // Can store in description if needed
	s.transactionRepo.Create(ctx, tx)

	return mortgage, nil
}
