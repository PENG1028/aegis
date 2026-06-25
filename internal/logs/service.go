package logs

import (
	"context"
	"time"

	"aegis/internal/id"
)

// AppService defines the log application service interface.
type AppService struct {
	repo          *Repository
	applyRepo     *ApplyLogRepository
	auditRepo     *AuditLogRepository
	nodeEventRepo *NodeEventRepository
}

// NewAppService creates a new log application service.
func NewAppService(repo *Repository) *AppService {
	return &AppService{repo: repo}
}

// SetApplyRepo sets the apply log repository.
func (s *AppService) SetApplyRepo(r *ApplyLogRepository) { s.applyRepo = r }

// SetAuditRepo sets the audit log repository.
func (s *AppService) SetAuditRepo(r *AuditLogRepository) { s.auditRepo = r }

// SetNodeEventRepo sets the node event repository.
func (s *AppService) SetNodeEventRepo(r *NodeEventRepository) { s.nodeEventRepo = r }

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

// LogAudit writes an audit log entry.
func (s *AppService) LogAudit(actorType, actorID, eventType, ip, userAgent, targetType, targetID, result, errorCode string) {
	if s.auditRepo == nil {
		return
	}
	entry := &AuditLog{
		ID:         id.New("audit"),
		ActorType:  actorType,
		ActorID:    actorID,
		EventType:  eventType,
		IP:         ip,
		UserAgent:  userAgent,
		TargetType: targetType,
		TargetID:   targetID,
		Result:     result,
		ErrorCode:  errorCode,
		CreatedAt:  time.Now(),
	}
	_ = s.auditRepo.Create(entry)
}

// LogApply writes an apply log entry.
func (s *AppService) LogApply(l *ApplyLog) {
	if s.applyRepo == nil {
		return
	}
	_ = s.applyRepo.Create(l)
}

// LogNodeEvent writes a node event entry.
func (s *AppService) LogNodeEvent(e *NodeEvent) {
	if s.nodeEventRepo == nil {
		return
	}
	_ = s.nodeEventRepo.Create(e)
}

// ListApplyLogs returns recent apply logs.
func (s *AppService) ListApplyLogs(limit int) ([]ApplyLog, error) {
	if s.applyRepo == nil {
		return nil, nil
	}
	return s.applyRepo.FindAll(limit)
}

// ListAuditLogs returns recent audit logs.
func (s *AppService) ListAuditLogs(limit int) ([]AuditLog, error) {
	if s.auditRepo == nil {
		return nil, nil
	}
	return s.auditRepo.FindAll(limit)
}

// ListNodeEvents returns recent node events.
func (s *AppService) ListNodeEvents(limit int) ([]NodeEvent, error) {
	if s.nodeEventRepo == nil {
		return nil, nil
	}
	return s.nodeEventRepo.FindAll(limit)
}
