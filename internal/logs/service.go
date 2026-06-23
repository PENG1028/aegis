package logs

import (
	"context"
	"time"

	"aegis/internal/id"
)

// AppService defines the log application service interface.
type AppService struct {
	repo *Repository
}

// NewAppService creates a new log application service.
func NewAppService(repo *Repository) *AppService {
	return &AppService{repo: repo}
}

// Log records an operation log entry.
func (s *AppService) Log(ctx context.Context, action, targetType, targetID, result, message, actor string) {
	if actor == "" {
		actor = "system"
	}
	entry := &OperationLog{
		ID:         id.New("log"),
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Result:     result,
		Message:    message,
		Actor:      actor,
		CreatedAt:  time.Now(),
	}
	// Best-effort logging; don't fail the parent operation
	_ = s.repo.Create(entry)
}

// ListLogs returns recent operation logs.
func (s *AppService) ListLogs(ctx context.Context, action string, targetID string) ([]OperationLog, error) {
	if targetID != "" {
		return s.repo.FindByTarget(targetID, 100)
	}
	if action != "" {
		return s.repo.FindByAction(action, 100)
	}
	return s.repo.FindAll(100)
}
