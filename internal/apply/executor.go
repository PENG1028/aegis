package apply

import (
	"aegis/internal/config"
	"aegis/internal/proxy"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Executor handles writing and backing up configuration files.
type Executor struct {
	cfg *config.Config
}

// NewExecutor creates a new apply executor.
func NewExecutor(cfg *config.Config) *Executor {
	return &Executor{cfg: cfg}
}

// WriteTemp writes rendered config to a temporary file and returns the path.
func (e *Executor) WriteTemp(rendered []byte) (string, error) {
	dir := filepath.Dir(e.cfg.Proxy.CaddyfilePath)
	tmpFile := filepath.Join(dir, ".Caddyfile.tmp")

	if err := os.WriteFile(tmpFile, rendered, 0644); err != nil {
		return "", fmt.Errorf("write temp config: %w", err)
	}
	return tmpFile, nil
}

// Backup copies the current config file to the backup directory.
// Returns the backup path.
func (e *Executor) Backup() (string, error) {
	configPath := e.cfg.Proxy.CaddyfilePath

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// No existing config to back up; that's fine for first apply
		return "", nil
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(e.cfg.Proxy.BackupDir, 0755); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("Caddyfile.%s.bak", timestamp)
	backupPath := filepath.Join(e.cfg.Proxy.BackupDir, backupName)

	src, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("read config for backup: %w", err)
	}

	if err := os.WriteFile(backupPath, src, 0644); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}

	return backupPath, nil
}

// Replace atomically moves the temp file to the actual config path.
// Uses os.Rename which is atomic on the same filesystem (Linux).
func (e *Executor) Replace(tempPath string) error {
	configPath := e.cfg.Proxy.CaddyfilePath

	// Atomic rename — if the process crashes, either the old or new file is intact.
	// os.Rename is atomic on Linux when source and destination are on the same filesystem.
	if err := os.Rename(tempPath, configPath); err != nil {
		// Fallback: read+write if rename fails (e.g., cross-filesystem)
		data, readErr := os.ReadFile(tempPath)
		if readErr != nil {
			return fmt.Errorf("rename config (and read fallback failed): %w", err)
		}
		if writeErr := os.WriteFile(configPath, data, 0644); writeErr != nil {
			return fmt.Errorf("write config (rename fallback): %w", writeErr)
		}
		os.Remove(tempPath)
	}

	return nil
}

// RestoreBackup copies a backup file back to the config path.
func (e *Executor) RestoreBackup(backupPath string) error {
	if backupPath == "" {
		return fmt.Errorf("no backup path provided")
	}

	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read backup file %s: %w", backupPath, err)
	}

	configPath := e.cfg.Proxy.CaddyfilePath
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}

	return nil
}

// ValidateAdapter wraps the adapter's Validate method.
func (e *Executor) ValidateAdapter(adapter proxy.ProxyAdapter, configPath string) error {
	return adapter.Validate(configPath)
}

// ReloadAdapter wraps the adapter's Reload method.
func (e *Executor) ReloadAdapter(adapter proxy.ProxyAdapter) error {
	return adapter.Reload("")
}

// VerifyProxyHealth performs a quick HTTP check to confirm the proxy is serving after reload.
// Returns nil if the proxy responds (any status code), error if connection refused or times out.
func (e *Executor) VerifyProxyHealth() error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:80/")
	if err != nil {
		return fmt.Errorf("proxy health check failed (port 80 unreachable): %w", err)
	}
	resp.Body.Close()
	// Any response (2xx, 4xx, even 5xx) means the proxy is running and accepting connections
	return nil
}
