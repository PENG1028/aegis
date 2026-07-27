package logs

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for operation logs.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new log repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new operation log.
func (r *Repository) Create(log *OperationLog) error {
	_, err := r.DB.Exec(
		`INSERT INTO operation_logs (id, action, target_type, target_id, result, message, actor, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.Action, log.TargetType, log.TargetID, log.Result, log.Message, log.Actor,
		log.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert operation_log: %w", err)
	}
	return nil
}

// FindAll returns recent operation logs, newest first.
func (r *Repository) FindAll(limit int) ([]OperationLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.DB.Query(
		`SELECT id, action, target_type, target_id, result, message, actor, created_at
		 FROM operation_logs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query operation_logs: %w", err)
	}
	defer rows.Close()
	return scanLogs(rows)
}

// FindByAction returns logs filtered by action.
func (r *Repository) FindByAction(action string, limit int) ([]OperationLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.DB.Query(
		`SELECT id, action, target_type, target_id, result, message, actor, created_at
		 FROM operation_logs WHERE action = ? ORDER BY created_at DESC LIMIT ?`, action, limit)
	if err != nil {
		return nil, fmt.Errorf("query operation_logs by action: %w", err)
	}
	defer rows.Close()
	return scanLogs(rows)
}

// FindByTarget returns logs for a specific target.
func (r *Repository) FindByTarget(targetID string, limit int) ([]OperationLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.DB.Query(
		`SELECT id, action, target_type, target_id, result, message, actor, created_at
		 FROM operation_logs WHERE target_id = ? ORDER BY created_at DESC LIMIT ?`, targetID, limit)
	if err != nil {
		return nil, fmt.Errorf("query operation_logs by target: %w", err)
	}
	defer rows.Close()
	return scanLogs(rows)
}

func scanLogs(rows *sql.Rows) ([]OperationLog, error) {
	var logs []OperationLog
	for rows.Next() {
		var l OperationLog
		var createdAt string
		var targetType, targetID, message, actor sql.NullString
		if err := rows.Scan(&l.ID, &l.Action, &targetType, &targetID, &l.Result, &message, &actor, &createdAt); err != nil {
			return nil, fmt.Errorf("scan operation_log: %w", err)
		}
		l.TargetType = targetType.String
		l.TargetID = targetID.String
		l.Message = message.String
		l.Actor = actor.String
		l.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// --- ApplyLog Repository ---

// ApplyLogRepository provides database access for apply logs.
type ApplyLogRepository struct {
	DB *sql.DB
}

// NewApplyLogRepository creates a new apply log repository.
func NewApplyLogRepository(db *sql.DB) *ApplyLogRepository {
	return &ApplyLogRepository{DB: db}
}

// Create inserts a new apply log.
func (r *ApplyLogRepository) Create(l *ApplyLog) error {
	_, err := r.DB.Exec(
		`INSERT INTO apply_logs (id, operation_id, state_version, config_hash_before, config_hash_after, provider, validate_status, reload_status, runtime_verify_status, stderr, step_log, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.OperationID, l.StateVersion, l.ConfigHashBefore, l.ConfigHashAfter,
		l.Provider, l.ValidateStatus, l.ReloadStatus, l.RuntimeVerifyStatus,
		l.Stderr, l.StepLog, l.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert apply_log: %w", err)
	}
	return nil
}

// FindAll returns recent apply logs.
func (r *ApplyLogRepository) FindAll(limit int) ([]ApplyLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.DB.Query(
		`SELECT id, operation_id, state_version, config_hash_before, config_hash_after, provider, validate_status, reload_status, runtime_verify_status, stderr, step_log, created_at
		 FROM apply_logs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query apply_logs: %w", err)
	}
	defer rows.Close()
	return scanApplyLogs(rows)
}

// FindByOperationID returns apply logs for a specific operation.
func (r *ApplyLogRepository) FindByOperationID(opID string) ([]ApplyLog, error) {
	rows, err := r.DB.Query(
		`SELECT id, operation_id, state_version, config_hash_before, config_hash_after, provider, validate_status, reload_status, runtime_verify_status, stderr, step_log, created_at
		 FROM apply_logs WHERE operation_id = ? ORDER BY created_at`, opID)
	if err != nil {
		return nil, fmt.Errorf("query apply_logs by operation: %w", err)
	}
	defer rows.Close()
	return scanApplyLogs(rows)
}

func scanApplyLogs(rows *sql.Rows) ([]ApplyLog, error) {
	var logs []ApplyLog
	for rows.Next() {
		var l ApplyLog
		var createdAt, stderr, stepLog string
		if err := rows.Scan(&l.ID, &l.OperationID, &l.StateVersion, &l.ConfigHashBefore, &l.ConfigHashAfter, &l.Provider, &l.ValidateStatus, &l.ReloadStatus, &l.RuntimeVerifyStatus, &stderr, &stepLog, &createdAt); err != nil {
			return nil, fmt.Errorf("scan apply_log: %w", err)
		}
		l.Stderr = stderr
		l.StepLog = stepLog
		l.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// --- AuditLog Repository ---

// AuditLogRepository provides database access for audit logs.
type AuditLogRepository struct {
	DB *sql.DB
}

// NewAuditLogRepository creates a new audit log repository.
func NewAuditLogRepository(db *sql.DB) *AuditLogRepository {
	return &AuditLogRepository{DB: db}
}

// Create inserts a new audit log.
func (r *AuditLogRepository) Create(l *AuditLog) error {
	_, err := r.DB.Exec(
		`INSERT INTO audit_logs (id, actor_type, actor_id, event_type, ip, user_agent, target_type, target_id, result, error_code, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.ActorType, l.ActorID, l.EventType, l.IP, l.UserAgent,
		l.TargetType, l.TargetID, l.Result, l.ErrorCode, l.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert audit_log: %w", err)
	}
	return nil
}

// FindAll returns recent audit logs.
func (r *AuditLogRepository) FindAll(limit int) ([]AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.DB.Query(
		`SELECT id, actor_type, actor_id, event_type, ip, user_agent, target_type, target_id, result, error_code, created_at
		 FROM audit_logs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query audit_logs: %w", err)
	}
	defer rows.Close()
	return scanAuditLogs(rows)
}

// FindByEventType returns audit logs filtered by event type.
func (r *AuditLogRepository) FindByEventType(eventType string, limit int) ([]AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.DB.Query(
		`SELECT id, actor_type, actor_id, event_type, ip, user_agent, target_type, target_id, result, error_code, created_at
		 FROM audit_logs WHERE event_type = ? ORDER BY created_at DESC LIMIT ?`, eventType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuditLogs(rows)
}

func scanAuditLogs(rows *sql.Rows) ([]AuditLog, error) {
	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		var createdAt string
		if err := rows.Scan(&l.ID, &l.ActorType, &l.ActorID, &l.EventType, &l.IP, &l.UserAgent, &l.TargetType, &l.TargetID, &l.Result, &l.ErrorCode, &createdAt); err != nil {
			return nil, fmt.Errorf("scan audit_log: %w", err)
		}
		l.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// --- NodeEvent Repository ---

// NodeEventRepository provides database access for node events.
type NodeEventRepository struct {
	DB *sql.DB
}

// NewNodeEventRepository creates a new node event repository.
func NewNodeEventRepository(db *sql.DB) *NodeEventRepository {
	return &NodeEventRepository{DB: db}
}

// Create inserts a new node event.
func (r *NodeEventRepository) Create(e *NodeEvent) error {
	_, err := r.DB.Exec(
		`INSERT INTO node_events (id, node_id, event_type, state_version, severity, message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.NodeID, e.EventType, e.StateVersion, e.Severity, e.Message, e.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert node_event: %w", err)
	}
	return nil
}

// FindAll returns recent node events.
func (r *NodeEventRepository) FindAll(limit int) ([]NodeEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.DB.Query(
		`SELECT id, node_id, event_type, state_version, severity, message, created_at
		 FROM node_events ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query node_events: %w", err)
	}
	defer rows.Close()
	return scanNodeEvents(rows)
}

// FindByNodeID returns events for a specific node.
func (r *NodeEventRepository) FindByNodeID(nodeID string, limit int) ([]NodeEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.DB.Query(
		`SELECT id, node_id, event_type, state_version, severity, message, created_at
		 FROM node_events WHERE node_id = ? ORDER BY created_at DESC LIMIT ?`, nodeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodeEvents(rows)
}

func scanNodeEvents(rows *sql.Rows) ([]NodeEvent, error) {
	var events []NodeEvent
	for rows.Next() {
		var e NodeEvent
		var createdAt string
		if err := rows.Scan(&e.ID, &e.NodeID, &e.EventType, &e.StateVersion, &e.Severity, &e.Message, &createdAt); err != nil {
			return nil, fmt.Errorf("scan node_event: %w", err)
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		events = append(events, e)
	}
	return events, rows.Err()
}
