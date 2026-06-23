package apply

import "time"

// ApplyVersion records a configuration apply operation.
type ApplyVersion struct {
	ID             string    `json:"id"`
	Version        string    `json:"version"`
	ConfigPath     string    `json:"config_path"`
	BackupPath     string    `json:"backup_path"`
	RenderedConfig string    `json:"rendered_config"`
	Status         string    `json:"status"` // success | failed | rolled_back
	Message        string    `json:"message"`
	CreatedAt      time.Time `json:"created_at"`
}
