package apply

import "time"

// ApplyVersion records a configuration apply operation.
type ApplyVersion struct {
	ID             string    `json:"id"`
	Version        string    `json:"version"`
	ConfigPath     string    `json:"config_path"`
	BackupPath     string    `json:"backup_path"`               // legacy: caddy backup path
	BackupPaths    map[string]string `json:"backup_paths"`      // v1.8L-20: per-provider backup paths
	RenderedConfig string    `json:"rendered_config"`
	Status         string    `json:"status"` // success | failed | rolled_back
	Message        string    `json:"message"`
	CreatedAt      time.Time `json:"created_at"`
}
