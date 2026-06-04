package config

// SyncConfig holds MSSQL sync agent service-to-service config.
// Currently only an API key for /admin/flommast/sync endpoint.
type SyncConfig struct {
	APIKey string
}

// loadSyncConfig loads sync config (env-only, no dev/prod prefix —
// the same key is used in both modes; it's an operational secret).
func loadSyncConfig() SyncConfig {
	return SyncConfig{
		APIKey: getEnv("SYNC_API_KEY", ""),
	}
}
