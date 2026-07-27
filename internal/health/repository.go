package health

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for health checks.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new health check repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new health check record.
func (r *Repository) Create(h *HealthCheck) error {
	_, err := r.DB.Exec(
		`INSERT INTO health_checks (id, service_id, endpoint_id, status, latency_ms, message, checked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		h.ID, h.ServiceID, h.EndpointID, h.Status, h.LatencyMS, h.Message,
		h.CheckedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert health_check: %w", err)
	}
	return nil
}

// FindLatestByServiceID returns the most recent health check for a service.
func (r *Repository) FindLatestByServiceID(serviceID string) (*HealthCheck, error) {
	var h HealthCheck
	var checkedAt string
	var message, endpointID sql.NullString
	var latency sql.NullInt64
	err := r.DB.QueryRow(
		`SELECT id, service_id, endpoint_id, status, latency_ms, message, checked_at
		 FROM health_checks WHERE service_id = ?
		 ORDER BY checked_at DESC LIMIT 1`, serviceID,
	).Scan(&h.ID, &h.ServiceID, &endpointID, &h.Status, &latency, &message, &checkedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query latest health_check: %w", err)
	}
	h.EndpointID = endpointID.String
	h.LatencyMS = latency.Int64
	h.Message = message.String
	h.CheckedAt, _ = time.Parse(time.RFC3339, checkedAt)
	return &h, nil
}

// FindLatestForAll returns the latest health check for each service.
func (r *Repository) FindLatestForAll() ([]HealthCheck, error) {
	rows, err := r.DB.Query(
		`SELECT h.id, h.service_id, h.endpoint_id, h.status, h.latency_ms, h.message, h.checked_at
		 FROM health_checks h
		 INNER JOIN (
			 SELECT service_id, MAX(checked_at) as max_checked
			 FROM health_checks
			 GROUP BY service_id
		 ) latest ON h.service_id = latest.service_id AND h.checked_at = latest.max_checked
		 ORDER BY h.service_id`)
	if err != nil {
		return nil, fmt.Errorf("query latest health_checks: %w", err)
	}
	defer rows.Close()

	var checks []HealthCheck
	for rows.Next() {
		var h HealthCheck
		var checkedAt string
		var message, endpointID sql.NullString
		var latency sql.NullInt64
		if err := rows.Scan(&h.ID, &h.ServiceID, &endpointID, &h.Status, &latency, &message, &checkedAt); err != nil {
			return nil, fmt.Errorf("scan health_check: %w", err)
		}
		h.EndpointID = endpointID.String
		h.LatencyMS = latency.Int64
		h.Message = message.String
		h.CheckedAt, _ = time.Parse(time.RFC3339, checkedAt)
		checks = append(checks, h)
	}
	return checks, rows.Err()
}
