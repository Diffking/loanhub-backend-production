package services

import (
	"context"
	"errors"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"

	"gorm.io/gorm"
)

var (
	ErrFlommastMemberNotFound  = errors.New("member not found in flommast")
	ErrAlreadyActiveCommittee  = errors.New("member is already an active committee member")
	ErrCommitteeMemberNotFound = errors.New("committee member not found")
	ErrNotCommitteeMember      = errors.New("not an active committee member")
	ErrInvalidMonth            = errors.New("month must be between 1 and 12")
)

// CommitteeService handles คณะกรรมการ designations and the borrower-list
// aggregate view they're granted access to.
type CommitteeService struct {
	committeeRepo  repositories.CommitteeRepository
	memberRepo     repositories.MemberRepository
	mortgageRepo   *repositories.MortgageRepository
	visibilityRepo repositories.CommitteeVisibilityRepository
}

func NewCommitteeService(
	committeeRepo repositories.CommitteeRepository,
	memberRepo repositories.MemberRepository,
	mortgageRepo *repositories.MortgageRepository,
	visibilityRepo repositories.CommitteeVisibilityRepository,
) *CommitteeService {
	return &CommitteeService{
		committeeRepo:  committeeRepo,
		memberRepo:     memberRepo,
		mortgageRepo:   mortgageRepo,
		visibilityRepo: visibilityRepo,
	}
}

// GetVisibility returns the current borrower-field visibility settings.
func (s *CommitteeService) GetVisibility(ctx context.Context) (*models.CommitteeVisibilitySetting, error) {
	return s.visibilityRepo.Get(ctx)
}

type UpdateVisibilityInput struct {
	ShowBorrowerName bool `json:"show_borrower_name"`
	ShowMembNo       bool `json:"show_memb_no"`
	ShowAmount       bool `json:"show_amount"`
	ShowLoanStatus   bool `json:"show_loan_status"`
}

// UpdateVisibility saves new borrower-field visibility settings.
func (s *CommitteeService) UpdateVisibility(ctx context.Context, input *UpdateVisibilityInput) (*models.CommitteeVisibilitySetting, error) {
	setting := &models.CommitteeVisibilitySetting{
		ShowBorrowerName: input.ShowBorrowerName,
		ShowMembNo:       input.ShowMembNo,
		ShowAmount:       input.ShowAmount,
		ShowLoanStatus:   input.ShowLoanStatus,
	}
	if err := s.visibilityRepo.Update(ctx, setting); err != nil {
		return nil, err
	}
	return setting, nil
}

type AddCommitteeMemberInput struct {
	MembNo    string `json:"memb_no" validate:"required"`
	TermLabel string `json:"term_label" validate:"required"`
}

// Add designates a member as an active committee member for the given term.
func (s *CommitteeService) Add(ctx context.Context, input *AddCommitteeMemberInput, addedBy uint) (*CommitteeMemberView, error) {
	member, err := s.memberRepo.GetByMembNo(ctx, input.MembNo)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFlommastMemberNotFound
		}
		return nil, err
	}

	_, err = s.committeeRepo.GetActiveByMembNo(ctx, input.MembNo)
	if err == nil {
		return nil, ErrAlreadyActiveCommittee
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	cm := &models.CommitteeMember{
		MembNo:    input.MembNo,
		TermLabel: input.TermLabel,
		IsActive:  true,
		AddedBy:   addedBy,
	}
	if err := s.committeeRepo.Create(ctx, cm); err != nil {
		return nil, err
	}
	return &CommitteeMemberView{CommitteeMember: cm, FullName: member.FullName}, nil
}

// CommitteeMemberView enriches a CommitteeMember with the designated member's
// own full name (from Flommast), for display in admin UIs.
type CommitteeMemberView struct {
	*models.CommitteeMember
	FullName string `json:"full_name"`
}

func (s *CommitteeService) enrich(ctx context.Context, members []*models.CommitteeMember) []*CommitteeMemberView {
	views := make([]*CommitteeMemberView, 0, len(members))
	for _, m := range members {
		view := &CommitteeMemberView{CommitteeMember: m}
		if member, err := s.memberRepo.GetByMembNo(ctx, m.MembNo); err == nil {
			view.FullName = member.FullName
		}
		views = append(views, view)
	}
	return views
}

type CommitteeListOutput struct {
	Members    []*CommitteeMemberView `json:"members"`
	Total      int64                  `json:"total"`
	Page       int                    `json:"page"`
	Limit      int                    `json:"limit"`
	TotalPages int                    `json:"total_pages"`
}

// List lists all committee member designations (active and revoked), newest first.
func (s *CommitteeService) List(ctx context.Context, page, limit int) (*CommitteeListOutput, error) {
	page, limit = clampPage(page, limit)
	offset := (page - 1) * limit

	members, total, err := s.committeeRepo.List(ctx, offset, limit)
	if err != nil {
		return nil, err
	}

	return &CommitteeListOutput{
		Members:    s.enrich(ctx, members),
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages(total, limit),
	}, nil
}

// Remove revokes a committee member designation, preserving history.
func (s *CommitteeService) Remove(ctx context.Context, id uint, removedBy uint) error {
	if _, err := s.committeeRepo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCommitteeMemberNotFound
		}
		return err
	}
	return s.committeeRepo.Deactivate(ctx, id, removedBy)
}

// IsActiveMember reports whether membNo currently holds an active committee designation.
func (s *CommitteeService) IsActiveMember(ctx context.Context, membNo string) (bool, error) {
	return s.committeeRepo.IsActiveMember(ctx, membNo)
}

// BorrowerRow's optional fields are pointers so that fields hidden by
// CommitteeVisibilitySetting serialize as null instead of a misleading
// zero value (empty string / 0).
type BorrowerRow struct {
	MortgageID   uint      `json:"mortgage_id"`
	MembNo       *string   `json:"memb_no"`
	BorrowerName *string   `json:"borrower_name"`
	Amount       *float64  `json:"amount"`
	LoanTypeName *string   `json:"loan_type_name"`
	StepName     *string   `json:"step_name"`
	CreatedAt    time.Time `json:"created_at"`
}

type ListBorrowersOutput struct {
	Borrowers  []*BorrowerRow `json:"borrowers"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
	TotalPages int            `json:"total_pages"`
	Year       int            `json:"year"`
	Month      int            `json:"month"`
}

// ListBorrowersByMonth returns all loan applicants for the given year/month,
// gated on requesterMembNo currently being an active committee member.
func (s *CommitteeService) ListBorrowersByMonth(ctx context.Context, requesterMembNo string, year, month, page, limit int) (*ListBorrowersOutput, error) {
	isActive, err := s.committeeRepo.IsActiveMember(ctx, requesterMembNo)
	if err != nil {
		return nil, err
	}
	if !isActive {
		return nil, ErrNotCommitteeMember
	}

	if month < 1 || month > 12 {
		return nil, ErrInvalidMonth
	}

	page, limit = clampPage(page, limit)
	offset := (page - 1) * limit

	mortgages, total, err := s.mortgageRepo.ListByMonth(ctx, year, month, offset, limit)
	if err != nil {
		return nil, err
	}

	visibility, err := s.visibilityRepo.Get(ctx)
	if err != nil {
		return nil, err
	}

	borrowers := make([]*BorrowerRow, 0, len(mortgages))
	for _, m := range mortgages {
		row := &BorrowerRow{
			MortgageID: m.ID,
			CreatedAt:  m.CreatedAt,
		}
		if visibility.ShowMembNo {
			row.MembNo = &m.MembNo
		}
		if visibility.ShowAmount {
			row.Amount = &m.Amount
		}
		if visibility.ShowBorrowerName {
			if member, err := s.memberRepo.GetByMembNo(ctx, m.MembNo); err == nil {
				row.BorrowerName = &member.FullName
			}
		}
		if visibility.ShowLoanStatus {
			if m.LoanType != nil {
				row.LoanTypeName = &m.LoanType.Name
			}
			if m.CurrentStep != nil {
				row.StepName = &m.CurrentStep.Name
			}
		}
		borrowers = append(borrowers, row)
	}

	return &ListBorrowersOutput{
		Borrowers:  borrowers,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages(total, limit),
		Year:       year,
		Month:      month,
	}, nil
}

func clampPage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

func totalPages(total int64, limit int) int {
	pages := int(total) / limit
	if int(total)%limit > 0 {
		pages++
	}
	return pages
}
