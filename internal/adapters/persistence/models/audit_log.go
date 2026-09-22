package models

import "time"

// AuditLog records who accessed or changed sensitive data (PDPA / incident tracing).
// Written by middleware.Audit on selected routes; read by Admin via /admin/audit-logs.
type AuditLog struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"not null;index" json:"created_at"`

	// Actor (from JWT; empty for unauthenticated calls such as LIFF login)
	UserID   *uint  `gorm:"index" json:"user_id,omitempty"`
	Username string `gorm:"size:100" json:"username,omitempty"`
	Role     string `gorm:"size:20" json:"role,omitempty"`
	MembNo   string `gorm:"size:20;index" json:"memb_no,omitempty"`

	// What happened
	Action     string `gorm:"size:50;not null;index" json:"action"`     // e.g. member.view, mortgage.approve
	TargetID   string `gorm:"size:50;index" json:"target_id,omitempty"` // memb_no / mortgage id / user id from route params
	Method     string `gorm:"size:10;not null" json:"method"`
	Path       string `gorm:"size:255;not null" json:"path"`
	StatusCode int    `gorm:"not null" json:"status_code"`
	Detail     string `gorm:"type:text" json:"detail,omitempty"` // JSON: route params + selected query keys

	IP        string `gorm:"size:50;index" json:"ip"`
	UserAgent string `gorm:"size:255" json:"user_agent,omitempty"`
}

// TableName overrides GORM default
func (AuditLog) TableName() string {
	return "audit_logs"
}
