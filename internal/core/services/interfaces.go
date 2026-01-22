package services

import (
	"context"
	"spsc-loaneasy/internal/core/domain"
)

// Note: AuthService implementation is in auth_service.go
// Note: UserService implementation is in user_service.go

// MemberService defines member service interface (Phase 3)
type MemberService interface {
	GetByMembNo(ctx context.Context, membNo string) (*domain.Member, error)
	ValidateMember(ctx context.Context, membNo string) (bool, error)
}

// LoanService defines loan service interface (Phase 4)
type LoanService interface {
	Create(ctx context.Context, input CreateLoanInput) (*domain.Loan, error)
	GetByID(ctx context.Context, id uint) (*domain.Loan, error)
	Update(ctx context.Context, id uint, input UpdateLoanInput) (*domain.Loan, error)
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, page, limit int) ([]*domain.Loan, int64, error)
	GetByMembNo(ctx context.Context, membNo string) ([]*domain.Loan, error)
}

// Input DTOs for Phase 4

// CreateLoanInput for creating loan
type CreateLoanInput struct {
	MembNo      string
	Amount      float64
	Description string
}

// UpdateLoanInput for updating loan
type UpdateLoanInput struct {
	Amount      *float64
	Status      *string
	Description *string
}
