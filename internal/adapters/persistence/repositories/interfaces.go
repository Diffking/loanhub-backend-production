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

// MemberRepository defines member repository interface
// Read-only access to flommast table
type MemberRepository interface {
	GetByMembNo(ctx context.Context, membNo string) (*models.Flommast, error)
	Exists(ctx context.Context, membNo string) (bool, error)
	Search(ctx context.Context, query string, limit int) ([]*models.Flommast, error)
}
