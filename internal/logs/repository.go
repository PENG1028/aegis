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
