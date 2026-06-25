package models

import "time"

// FlommastSyncLog audits every push from the MSSQL sync agent.
// Read by admin Auto tab via /admin/flommast/sync-history.
type FlommastSyncLog struct {
	ID         uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	StartedAt  time.Time  `gorm:"not null;index;column:started_at" json:"started_at"`
	FinishedAt *time.Time `gorm:"column:finished_at"                json:"finished_at,omitempty"`

	Source       string `gorm:"size:50;not null;default:'mssql-agent';column:source" json:"source"`
	AgentVersion string `gorm:"size:20;column:agent_version"                          json:"agent_version,omitempty"`
	AgentHost    string `gorm:"size:100;column:agent_host"                            json:"agent_host,omitempty"`
	PublicIP     string `gorm:"size:50;column:public_ip"                              json:"public_ip,omitempty"`

	TotalRows *int `gorm:"column:total_rows" json:"total_rows,omitempty"`
	Inserted  *int `gorm:"column:inserted"   json:"inserted,omitempty"`
	Updated   *int `gorm:"column:updated"    json:"updated,omitempty"`
	Unchanged *int `gorm:"column:unchanged"  json:"unchanged,omitempty"`
	Missing   *int `gorm:"column:missing"    json:"missing,omitempty"`
	Deleted   *int `gorm:"column:deleted"    json:"deleted,omitempty"`

	// Phase 3A: list of memb_nos that were in DB but not in this sync.
	// Stored as JSON in MySQL; raw string here (handler json.Marshal/Unmarshal).
	MissingMembNos *string `gorm:"type:json;column:missing_memb_nos" json:"missing_memb_nos,omitempty"`

	Status       string `gorm:"size:20;not null;default:'running';column:status" json:"status"`
	ErrorMessage string `gorm:"type:text;column:error_message"                   json:"error_message,omitempty"`
	DurationMs   *int   `gorm:"column:duration_ms"                               json:"duration_ms,omitempty"`
}

// TableName overrides GORM default (would otherwise be "flommast_sync_logs")
func (FlommastSyncLog) TableName() string {
	return "flommast_sync_log"
}
