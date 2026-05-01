package repositories

import (
	"context"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// memberRepository implements MemberRepository interface.
// flommast is now managed via /admin/flommast/apply (admin import).
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
		Where("mast_memb_no = ?", membNo).
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
		Where("mast_memb_no = ?", membNo).
		Count(&count).Error
	return count > 0, err
}

// Search searches for members by name or member number (no eligibility filter — backward compat)
func (r *memberRepository) Search(ctx context.Context, query string, limit int) ([]*models.Flommast, error) {
	var members []*models.Flommast
	searchQuery := "%" + query + "%"
	err := r.db.WithContext(ctx).
		Where("mast_memb_no LIKE ? OR full_name LIKE ?", searchQuery, searchQuery).
		Limit(limit).
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

// excludedTypes — สมาชิกประเภทเหล่านี้กู้สามัญไม่ได้
// (สมทบทุกชนิด + บุตร/คู่สมรส/บิดามารดา)
var excludedStsTypes = []string{
	"บุตรสมาชิก",
	"คู่สมรส",
	"บิดา มารดาสมาชิก",
}

// applyEligibilityFilter adds WHERE clauses to exclude non-eligible members.
//   - exclude any "สมทบ" substring (covers พนักงานสหกรณ์ (สมทบ), สมทบ อสม., สมทบเกษียน)
//   - exclude บุตร/คู่สมรส/บิดามารดา (exact match)
func applyEligibilityFilter(tx *gorm.DB) *gorm.DB {
	return tx.
		Where("sts_type_desc NOT LIKE ?", "%สมทบ%").
		Where("sts_type_desc NOT IN ?", excludedStsTypes)
}

// SearchActive searches for ELIGIBLE members only (filters out สมทบ/บุตร/คู่สมรส/บิดามารดา).
// Used by the loan-print module — only members who can actually apply for a loan.
func (r *memberRepository) SearchActive(ctx context.Context, query string, limit int) ([]*models.Flommast, error) {
	var members []*models.Flommast
	tx := r.db.WithContext(ctx).Model(&models.Flommast{})
	tx = applyEligibilityFilter(tx)

	if query != "" {
		searchQuery := "%" + query + "%"
		tx = tx.Where("mast_memb_no LIKE ? OR full_name LIKE ?", searchQuery, searchQuery)
	}

	err := tx.
		Order("mast_memb_no DESC").
		Limit(limit).
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

// GetFullByMembNo gets a member with full data — only if eligible.
// Returns gorm.ErrRecordNotFound if member exists but is ineligible (สมทบ etc.).
func (r *memberRepository) GetFullByMembNo(ctx context.Context, membNo string) (*models.Flommast, error) {
	var member models.Flommast
	tx := r.db.WithContext(ctx).Model(&models.Flommast{})
	tx = applyEligibilityFilter(tx)
	err := tx.Where("mast_memb_no = ?", membNo).First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// CountActive counts eligible members (for dashboard / status pages)
func (r *memberRepository) CountActive(ctx context.Context) (int64, error) {
	var count int64
	tx := r.db.WithContext(ctx).Model(&models.Flommast{})
	tx = applyEligibilityFilter(tx)
	err := tx.Count(&count).Error
	return count, err
}
