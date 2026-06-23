package apply

import (
	"aegis/internal/config"
	"aegis/internal/proxy"
	"fmt"
	"os"
)

// RollbackService handles rolling back to a previous configuration.
type RollbackService struct {
	repo *Repository
	cfg  *config.Config
}

// NewRollbackService creates a new rollback service.
func NewRollbackService(repo *Repository, cfg *config.Config) *RollbackService {
	return &RollbackService{repo: repo, cfg: cfg}
}

// Rollback restores the most recent successful backup.
func (s *RollbackService) Rollback(adapter proxy.ProxyAdapter) error {
	// Find the last successful apply
	lastSuccess, err := s.repo.FindLastSuccess()
	if err != nil {
		return fmt.Errorf("find last success: %w", err)
	}
	if lastSuccess == nil {
		return fmt.Errorf("no successful apply to rollback to")
	}
	if lastSuccess.BackupPath == "" {
		return fmt.Errorf("no backup available for the last successful apply")
	}

	// Check backup file exists
	if _, err := os.Stat(lastSuccess.BackupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", lastSuccess.BackupPath)
	}

	// Restore backup
	executor := NewExecutor(s.cfg)
	if err := executor.RestoreBackup(lastSuccess.BackupPath); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}

	// Validate restored config
	configPath := s.cfg.Proxy.CaddyfilePath
	if err := adapter.Validate(configPath); err != nil {
		return fmt.Errorf("validate restored config: %w", err)
	}

	// Reload
	if err := adapter.Reload(""); err != nil {
		return fmt.Errorf("reload after rollback: %w", err)
	}

	return nil
}

// GetLastBackupPath returns the path of the last successful backup.
func (s *RollbackService) GetLastBackupPath() (string, error) {
	lastSuccess, err := s.repo.FindLastSuccess()
	if err != nil {
		return "", fmt.Errorf("find last success: %w", err)
	}
	if lastSuccess == nil {
		return "", fmt.Errorf("no successful apply to rollback to")
	}
	return lastSuccess.BackupPath, nil
}
