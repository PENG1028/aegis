package exposure

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for exposures.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new exposure repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new exposure.
func (r *Repository) Create(e *Exposure) error {
	_, err := r.DB.Exec(
		`INSERT INTO exposures
		 (id, type, mode, host, port, path, service_id, node_id, owner_ref, target_ref, status, message, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Type, e.Mode, e.Host, e.Port, e.Path, e.ServiceID, e.NodeID,
		e.OwnerRef, e.TargetRef, e.Status, e.Message,
		e.CreatedAt.Format(time.RFC3339),
		e.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert exposure: %w", err)
	}
	return nil
}

// FindByID returns an exposure by ID.
func (r *Repository) FindByID(id string) (*Exposure, error) {
	var e Exposure
	var createdAt, updatedAt string
	var path, nodeID, targetRef, message sql.NullString
	var port sql.NullInt64
	err := r.DB.QueryRow(
		`SELECT id, type, mode, host, port, path, service_id, node_id, owner_ref, target_ref, status, message, created_at, updated_at
		 FROM exposures WHERE id = ?`, id,
	).Scan(&e.ID, &e.Type, &e.Mode, &e.Host, &port, &path, &e.ServiceID,
		&nodeID, &e.OwnerRef, &targetRef, &e.Status, &message, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query exposure by id: %w", err)
	}
	e.Port = int(port.Int64)
	e.Path = path.String
	e.NodeID = nodeID.String
	e.TargetRef = targetRef.String
	e.Message = message.String
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	e.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &e, nil
}

// FindAll returns all exposures ordered by host.
func (r *Repository) FindAll() ([]Exposure, error) {
	rows, err := r.DB.Query(
		`SELECT id, type, mode, host, port, path, service_id, node_id, owner_ref, target_ref, status, message, created_at, updated_at
		 FROM exposures ORDER BY host`)
	if err != nil {
		return nil, fmt.Errorf("query exposures: %w", err)
	}
	defer rows.Close()
	return scanExposures(rows)
}

// FindByOwnerRef returns exposures for a specific owner.
func (r *Repository) FindByOwnerRef(ownerRef string) ([]Exposure, error) {
	rows, err := r.DB.Query(
		`SELECT id, type, mode, host, port, path, service_id, node_id, owner_ref, target_ref, status, message, created_at, updated_at
		 FROM exposures WHERE owner_ref = ? ORDER BY host`, ownerRef)
	if err != nil {
		return nil, fmt.Errorf("query exposures by owner: %w", err)
	}
	defer rows.Close()
	return scanExposures(rows)
}

// FindActiveHTTP returns all active HTTP exposures (for apply planner).
func (r *Repository) FindActiveHTTP() ([]Exposure, error) {
	rows, err := r.DB.Query(
		`SELECT id, type, mode, host, port, path, service_id, node_id, owner_ref, target_ref, status, message, created_at, updated_at
		 FROM exposures WHERE type = 'http' AND status = 'active' ORDER BY host`)
	if err != nil {
		return nil, fmt.Errorf("query active http exposures: %w", err)
	}
	defer rows.Close()
	return scanExposures(rows)
}

// Update updates an exposure.
func (r *Repository) Update(e *Exposure) error {
	_, err := r.DB.Exec(
		`UPDATE exposures SET type=?, mode=?, host=?, port=?, path=?, service_id=?, node_id=?, owner_ref=?, target_ref=?, status=?, message=?, updated_at=? WHERE id=?`,
		e.Type, e.Mode, e.Host, e.Port, e.Path, e.ServiceID, e.NodeID,
		e.OwnerRef, e.TargetRef, e.Status, e.Message,
		e.UpdatedAt.Format(time.RFC3339), e.ID,
	)
	if err != nil {
		return fmt.Errorf("update exposure: %w", err)
	}
	return nil
}

// Delete removes an exposure by ID.
func (r *Repository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM exposures WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete exposure: %w", err)
	}
	return nil
}

// GetStats returns exposure statistics.
func (r *Repository) GetStats() (*Stats, error) {
	stats := &Stats{
		ByType:   make(map[string]int),
		ByStatus: make(map[string]int),
	}

	rows, err := r.DB.Query(`SELECT type, status, COUNT(*) FROM exposures GROUP BY type, status`)
	if err != nil {
		return nil, fmt.Errorf("query exposure stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var typ, status string
		var count int
		if err := rows.Scan(&typ, &status, &count); err != nil {
			return nil, err
		}
		stats.Total += count
		stats.ByType[typ] += count
		stats.ByStatus[status] += count
		if typ == TypeHTTP && status == StatusActive {
			stats.HTTPActive += count
		}
		if typ != TypeHTTP && (status == StatusActive || status == StatusActiveRecorded) {
			stats.NonHTTPRecorded += count
		}
	}

	return stats, rows.Err()
}

func scanExposures(rows *sql.Rows) ([]Exposure, error) {
	var exposures []Exposure
	for rows.Next() {
		var e Exposure
		var createdAt, updatedAt string
		var path, nodeID, targetRef, message sql.NullString
		var port sql.NullInt64
		if err := rows.Scan(&e.ID, &e.Type, &e.Mode, &e.Host, &port, &path, &e.ServiceID,
			&nodeID, &e.OwnerRef, &targetRef, &e.Status, &message, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan exposure: %w", err)
		}
		e.Port = int(port.Int64)
		e.Path = path.String
		e.NodeID = nodeID.String
		e.TargetRef = targetRef.String
		e.Message = message.String
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		e.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		exposures = append(exposures, e)
	}
	return exposures, rows.Err()
}
