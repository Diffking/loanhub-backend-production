package repositories

import (
	"context"

	"gorm.io/gorm"

	"spsc-loaneasy/internal/adapters/persistence/models"
)

// SavingsRepositoryImpl implements SavingsRepository.
// Phase 3b: ดึงข้อมูลบัญชีเงินฝากของสมาชิกเพื่อใช้ค้ำประกันเงินกู้ (cap 95%).
type SavingsRepositoryImpl struct {
	db *gorm.DB
}

func NewSavingsRepository(db *gorm.DB) SavingsRepository {
	return &SavingsRepositoryImpl{db: db}
}

func (r *SavingsRepositoryImpl) GetByMembNo(ctx context.Context, membNo string) ([]*models.SavingsAccount, error) {
	var accounts []*models.SavingsAccount
	err := r.db.WithContext(ctx).
		Where("mast_memb_no = ?", membNo).
		Order("balance DESC").
		Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *SavingsRepositoryImpl) CountByMembNo(ctx context.Context, membNo string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.SavingsAccount{}).
		Where("mast_memb_no = ?", membNo).
		Count(&count).Error
	return count, err
}

func (r *SavingsRepositoryImpl) TotalBalance(ctx context.Context, membNo string) (float64, error) {
	var total float64
	err := r.db.WithContext(ctx).
		Model(&models.SavingsAccount{}).
		Where("mast_memb_no = ?", membNo).
		Select("COALESCE(SUM(balance), 0)").
		Scan(&total).Error
	return total, err
}
