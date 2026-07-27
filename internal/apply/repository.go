package apply

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for apply versions.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new apply version repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new apply version record.
func (r *Repository) Create(v *ApplyVersion) error {
	_, err := r.DB.Exec(
		`INSERT INTO apply_versions (id, version, config_path, backup_path, rendered_config, status, message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.Version, v.ConfigPath, v.BackupPath, v.RenderedConfig, v.Status, v.Message,
		v.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert apply_version: %w", err)
	}
	return nil
}

// FindAll returns apply versions, newest first.
func (r *Repository) FindAll(limit int) ([]ApplyVersion, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.DB.Query(
		`SELECT id, version, config_path, backup_path, rendered_config, status, message, created_at
		 FROM apply_versions ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query apply_versions: %w", err)
	}
	defer rows.Close()

	var versions []ApplyVersion
	for rows.Next() {
		var v ApplyVersion
		var createdAt string
		var backupPath, renderedConfig, message sql.NullString
		if err := rows.Scan(&v.ID, &v.Version, &v.ConfigPath, &backupPath, &renderedConfig, &v.Status, &message, &createdAt); err != nil {
			return nil, fmt.Errorf("scan apply_version: %w", err)
		}
		v.BackupPath = backupPath.String
		v.RenderedConfig = renderedConfig.String
		v.Message = message.String
		v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// FindLastSuccess returns the most recent successful apply.
func (r *Repository) FindLastSuccess() (*ApplyVersion, error) {
	var v ApplyVersion
	var createdAt string
	var backupPath, renderedConfig, message sql.NullString
	err := r.DB.QueryRow(
		`SELECT id, version, config_path, backup_path, rendered_config, status, message, created_at
		 FROM apply_versions WHERE status = 'success' ORDER BY created_at DESC LIMIT 1`,
	).Scan(&v.ID, &v.Version, &v.ConfigPath, &backupPath, &renderedConfig, &v.Status, &message, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query last success: %w", err)
	}
	v.BackupPath = backupPath.String
	v.RenderedConfig = renderedConfig.String
	v.Message = message.String
	v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &v, nil
}

// Update updates an apply version record.
func (r *Repository) Update(v *ApplyVersion) error {
	_, err := r.DB.Exec(
		`UPDATE apply_versions SET status=?, message=? WHERE id=?`,
		v.Status, v.Message, v.ID,
	)
	if err != nil {
		return fmt.Errorf("update apply_version: %w", err)
	}
	return nil
}
