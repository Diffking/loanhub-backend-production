package repositories

import (
	"context"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// AuditLogRepository stores and queries audit log entries.
type AuditLogRepository struct {
	db *gorm.DB
}

// NewAuditLogRepository creates a new audit log repository
func NewAuditLogRepository(db *gorm.DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

// AuditLogFilter narrows an audit log listing. Zero values are ignored.
type AuditLogFilter struct {
	UserID   uint
	Action   string // exact, or prefix when ending with "." (e.g. "member.")
	TargetID string
	IP       string
	From     time.Time
	To       time.Time
	Offset   int
	Limit    int
}

// Create inserts one audit log entry
func (r *AuditLogRepository) Create(ctx context.Context, entry *models.AuditLog) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

// List returns entries (newest first) matching the filter, plus the total count
func (r *AuditLogRepository) List(ctx context.Context, f AuditLogFilter) ([]models.AuditLog, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.AuditLog{})
	if f.UserID > 0 {
		q = q.Where("user_id = ?", f.UserID)
	}
	if f.Action != "" {
		if f.Action[len(f.Action)-1] == '.' {
			q = q.Where("action LIKE ?", f.Action+"%")
		} else {
			q = q.Where("action = ?", f.Action)
		}
	}
	if f.TargetID != "" {
		q = q.Where("target_id = ?", f.TargetID)
	}
	if f.IP != "" {
		q = q.Where("ip = ?", f.IP)
	}
	if !f.From.IsZero() {
		q = q.Where("created_at >= ?", f.From)
	}
	if !f.To.IsZero() {
		q = q.Where("created_at < ?", f.To)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []models.AuditLog
	err := q.Order("id DESC").Offset(f.Offset).Limit(f.Limit).Find(&rows).Error
	return rows, total, err
}

// DeleteOlderThan removes entries older than the cutoff (retention policy)
func (r *AuditLogRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&models.AuditLog{})
	return res.RowsAffected, res.Error
}
