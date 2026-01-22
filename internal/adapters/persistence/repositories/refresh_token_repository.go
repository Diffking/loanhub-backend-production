package repositories

import (
	"context"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// refreshTokenRepository implements RefreshTokenRepository interface
type refreshTokenRepository struct {
	db *gorm.DB
}

// NewRefreshTokenRepository creates a new refresh token repository
func NewRefreshTokenRepository(db *gorm.DB) RefreshTokenRepository {
	return &refreshTokenRepository{db: db}
}

// Create creates a new refresh token
func (r *refreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// GetByTokenHash gets a refresh token by its hash
func (r *refreshTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	err := r.db.WithContext(ctx).
		Where("token_hash = ?", tokenHash).
		Where("revoked_at IS NULL").
		First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// GetByUserID gets all refresh tokens for a user
func (r *refreshTokenRepository) GetByUserID(ctx context.Context, userID uint) ([]*models.RefreshToken, error) {
	var tokens []*models.RefreshToken
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Where("revoked_at IS NULL").
		Find(&tokens).Error
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

// Revoke revokes a refresh token by ID
func (r *refreshTokenRepository) Revoke(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("id = ?", id).
		Update("revoked_at", &now).Error
}

// RevokeByTokenHash revokes a refresh token by its hash
func (r *refreshTokenRepository) RevokeByTokenHash(ctx context.Context, tokenHash string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("token_hash = ?", tokenHash).
		Update("revoked_at", &now).Error
}

// RevokeAllByUserID revokes all refresh tokens for a user
func (r *refreshTokenRepository) RevokeAllByUserID(ctx context.Context, userID uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("user_id = ?", userID).
		Where("revoked_at IS NULL").
		Update("revoked_at", &now).Error
}

// DeleteExpired deletes all expired tokens (cleanup job)
func (r *refreshTokenRepository) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&models.RefreshToken{}).Error
}

// CountActiveByUserID counts active tokens for a user
func (r *refreshTokenRepository) CountActiveByUserID(ctx context.Context, userID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("user_id = ?", userID).
		Where("revoked_at IS NULL").
		Where("expires_at > ?", time.Now()).
		Count(&count).Error
	return count, err
}
