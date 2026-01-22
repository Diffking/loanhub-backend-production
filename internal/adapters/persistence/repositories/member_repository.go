package repositories

import (
	"context"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// memberRepository implements MemberRepository interface
// This is READ-ONLY access to the legacy flommast table
type memberRepository struct {
	db *gorm.DB
}

// NewMemberRepository creates a new member repository
func NewMemberRepository(db *gorm.DB) MemberRepository {
	return &memberRepository{db: db}
}

// GetByMembNo gets a member by member number from flommast
func (r *memberRepository) GetByMembNo(ctx context.Context, membNo string) (*models.Flommast, error) {
	var member models.Flommast
	err := r.db.WithContext(ctx).
		Where("MAST_MEMB_NO = ?", membNo).
		First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// Exists checks if a member exists in flommast
func (r *memberRepository) Exists(ctx context.Context, membNo string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Flommast{}).
		Where("MAST_MEMB_NO = ?", membNo).
		Count(&count).Error
	return count > 0, err
}

// Search searches for members by name or member number
func (r *memberRepository) Search(ctx context.Context, query string, limit int) ([]*models.Flommast, error) {
	var members []*models.Flommast
	searchQuery := "%" + query + "%"
	err := r.db.WithContext(ctx).
		Where("MAST_MEMB_NO LIKE ? OR Full_Name LIKE ?", searchQuery, searchQuery).
		Limit(limit).
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}
