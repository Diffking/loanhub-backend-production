package repositories

import (
	"context"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// committeeRepository implements CommitteeRepository interface.
type committeeRepository struct {
	db *gorm.DB
}

// NewCommitteeRepository creates a new committee repository
func NewCommitteeRepository(db *gorm.DB) CommitteeRepository {
	return &committeeRepository{db: db}
}

// Create inserts a new committee member designation
func (r *committeeRepository) Create(ctx context.Context, cm *models.CommitteeMember) error {
	return r.db.WithContext(ctx).Create(cm).Error
}

// GetByID gets a committee member designation by ID
func (r *committeeRepository) GetByID(ctx context.Context, id uint) (*models.CommitteeMember, error) {
	var cm models.CommitteeMember
	err := r.db.WithContext(ctx).
		Preload("AddedByUser").
		First(&cm, id).Error
	if err != nil {
		return nil, err
	}
	return &cm, nil
}

// List lists all committee member designations, newest first
func (r *committeeRepository) List(ctx context.Context, offset, limit int) ([]*models.CommitteeMember, int64, error) {
	var members []*models.CommitteeMember
	var total int64

	r.db.WithContext(ctx).Model(&models.CommitteeMember{}).Count(&total)

	err := r.db.WithContext(ctx).
		Preload("AddedByUser").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&members).Error

	return members, total, err
}

// GetActiveByMembNo gets the active committee designation for a member, if any
func (r *committeeRepository) GetActiveByMembNo(ctx context.Context, membNo string) (*models.CommitteeMember, error) {
	var cm models.CommitteeMember
	err := r.db.WithContext(ctx).
		Where("memb_no = ? AND is_active = ?", membNo, true).
		First(&cm).Error
	if err != nil {
		return nil, err
	}
	return &cm, nil
}

// IsActiveMember checks whether a member is currently an active committee member
func (r *committeeRepository) IsActiveMember(ctx context.Context, membNo string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.CommitteeMember{}).
		Where("memb_no = ? AND is_active = ?", membNo, true).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Deactivate soft-revokes a committee member designation, preserving history
func (r *committeeRepository) Deactivate(ctx context.Context, id uint, removedBy uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.CommitteeMember{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_active":  false,
			"removed_by": removedBy,
			"removed_at": now,
		}).Error
}

// ListActiveRecipients resolves active committee members who have a linked
// LINE account, for push notifications.
func (r *committeeRepository) ListActiveRecipients(ctx context.Context) ([]CommitteeRecipient, error) {
	var recipients []CommitteeRecipient
	err := r.db.WithContext(ctx).Raw(`
		SELECT cm.memb_no AS memb_no,
		       COALESCE(f.full_name, u.username) AS full_name,
		       u.line_user_id AS line_user_id
		FROM committee_members cm
		JOIN users u ON u.memb_no = cm.memb_no
		LEFT JOIN flommast f ON f.mast_memb_no = cm.memb_no
		WHERE cm.is_active = 1
		  AND cm.deleted_at IS NULL
		  AND u.line_user_id IS NOT NULL
		  AND u.line_user_id != ''
	`).Scan(&recipients).Error
	return recipients, err
}

// committeeVisibilityRepository implements CommitteeVisibilityRepository.
type committeeVisibilityRepository struct {
	db *gorm.DB
}

// NewCommitteeVisibilityRepository creates a new committee visibility repository
func NewCommitteeVisibilityRepository(db *gorm.DB) CommitteeVisibilityRepository {
	return &committeeVisibilityRepository{db: db}
}

// Get returns the singleton visibility settings row, creating it (all fields
// visible by default) on first use.
func (r *committeeVisibilityRepository) Get(ctx context.Context) (*models.CommitteeVisibilitySetting, error) {
	var setting models.CommitteeVisibilitySetting
	err := r.db.WithContext(ctx).FirstOrCreate(&setting, models.CommitteeVisibilitySetting{ID: 1}).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// Update saves the singleton visibility settings row.
func (r *committeeVisibilityRepository) Update(ctx context.Context, setting *models.CommitteeVisibilitySetting) error {
	setting.ID = 1
	return r.db.WithContext(ctx).Save(setting).Error
}
