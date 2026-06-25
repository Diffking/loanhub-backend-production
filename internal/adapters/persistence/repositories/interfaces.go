package repositories

import (
	"context"

	"spsc-loaneasy/internal/adapters/persistence/models"
)

// UserRepository defines user repository interface
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uint) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByMembNo(ctx context.Context, membNo string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, offset, limit int) ([]*models.User, int64, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	ExistsByMembNo(ctx context.Context, membNo string) (bool, error)
}

// RefreshTokenRepository defines refresh token repository interface
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *models.RefreshToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	GetByUserID(ctx context.Context, userID uint) ([]*models.RefreshToken, error)
	Revoke(ctx context.Context, id uint) error
	RevokeByTokenHash(ctx context.Context, tokenHash string) error
	RevokeAllByUserID(ctx context.Context, userID uint) error
	DeleteExpired(ctx context.Context) error
	CountActiveByUserID(ctx context.Context, userID uint) (int64, error)
}

// MemberRepository defines member repository interface (flommast table).
// Read access (Get/Search) — managed by admin import for writes.
type MemberRepository interface {
	GetByMembNo(ctx context.Context, membNo string) (*models.Flommast, error)
	Exists(ctx context.Context, membNo string) (bool, error)
	Search(ctx context.Context, query string, limit int) ([]*models.Flommast, error)

	// 🆕 Phase 1 (Loan Print)
	SearchActive(ctx context.Context, query string, limit int) ([]*models.Flommast, error)
	GetFullByMembNo(ctx context.Context, membNo string) (*models.Flommast, error)
	CountActive(ctx context.Context) (int64, error)
}

// CommitteeRepository defines committee_members repository interface.
// Phase 7: คณะกรรมการ — designates Flommast members who may view the
// borrower-list aggregate view in the User app.
type CommitteeRepository interface {
	Create(ctx context.Context, cm *models.CommitteeMember) error
	GetByID(ctx context.Context, id uint) (*models.CommitteeMember, error)
	List(ctx context.Context, offset, limit int) ([]*models.CommitteeMember, int64, error)
	GetActiveByMembNo(ctx context.Context, membNo string) (*models.CommitteeMember, error)
	IsActiveMember(ctx context.Context, membNo string) (bool, error)
	Deactivate(ctx context.Context, id uint, removedBy uint) error
	ListActiveRecipients(ctx context.Context) ([]CommitteeRecipient, error)
}

// CommitteeRecipient is a LINE-push recipient resolved by joining active
// committee_members to users (line_user_id isn't mapped on the User model —
// see member_repository.go convention — so this is a raw-query result).
type CommitteeRecipient struct {
	MembNo     string
	FullName   string
	LineUserID string
}

// CommitteeVisibilityRepository manages the singleton committee_visibility_settings row.
type CommitteeVisibilityRepository interface {
	Get(ctx context.Context) (*models.CommitteeVisibilitySetting, error)
	Update(ctx context.Context, setting *models.CommitteeVisibilitySetting) error
}

// SavingsRepository defines savings_accounts repository interface.
// Phase 3b: ใช้ดึงข้อมูลบัญชีเงินฝากของสมาชิกเพื่อคำนวณค้ำประกันเงินกู้ (cap 95%).
type SavingsRepository interface {
	GetByMembNo(ctx context.Context, membNo string) ([]*models.SavingsAccount, error)
	CountByMembNo(ctx context.Context, membNo string) (int64, error)
	TotalBalance(ctx context.Context, membNo string) (float64, error)
}
