package endpoint

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for endpoints.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new endpoint repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// endpointSelectCols is the column list for scanning.
const endpointSelectCols = `id, service_id, type, address, enabled, node_id, created_at, updated_at`

// Create inserts a new endpoint.
func (r *Repository) Create(ep *Endpoint) error {
	enabledVal := 0
	if ep.Enabled {
		enabledVal = 1
	}
	_, err := r.DB.Exec(
		`INSERT INTO endpoints (id, service_id, type, address, enabled, node_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ep.ID, ep.ServiceID, ep.Type, ep.Address, enabledVal, ep.NodeID,
		ep.CreatedAt.Format(time.RFC3339),
		ep.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert endpoint: %w", err)
	}
	return nil
}

// FindByID returns an endpoint by ID.
func (r *Repository) FindByID(id string) (*Endpoint, error) {
	return scanEndpointRow(r.DB.QueryRow(
		`SELECT `+endpointSelectCols+` FROM endpoints WHERE id = ?`, id))
}

// FindByServiceID returns all endpoints for a service.
func (r *Repository) FindByServiceID(serviceID string) ([]Endpoint, error) {
	rows, err := r.DB.Query(
		`SELECT `+endpointSelectCols+` FROM endpoints WHERE service_id = ? ORDER BY type`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("query endpoints by service: %w", err)
	}
	defer rows.Close()
	return scanEndpoints(rows)
}

// FindEnabledByServiceID returns enabled endpoints ordered by type priority.
func (r *Repository) FindEnabledByServiceID(serviceID string) ([]Endpoint, error) {
	rows, err := r.DB.Query(
		`SELECT `+endpointSelectCols+` FROM endpoints WHERE service_id = ? AND enabled = 1
		 ORDER BY CASE type WHEN 'local' THEN 0 WHEN 'private' THEN 1 WHEN 'public' THEN 2 ELSE 99 END`,
		serviceID)
	if err != nil {
		return nil, fmt.Errorf("query enabled endpoints: %w", err)
	}
	defer rows.Close()
	return scanEndpoints(rows)
}

// FindByNodeID returns all endpoints for a given node.
func (r *Repository) FindByNodeID(nodeID string) ([]Endpoint, error) {
	rows, err := r.DB.Query(
		`SELECT `+endpointSelectCols+` FROM endpoints WHERE node_id = ? ORDER BY service_id`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query endpoints by node_id: %w", err)
	}
	defer rows.Close()
	return scanEndpoints(rows)
}

// Update updates an endpoint.
func (r *Repository) Update(ep *Endpoint) error {
	enabledVal := 0
	if ep.Enabled {
		enabledVal = 1
	}
	_, err := r.DB.Exec(
		`UPDATE endpoints SET type=?, address=?, enabled=?, node_id=?, updated_at=? WHERE id=?`,
		ep.Type, ep.Address, enabledVal, ep.NodeID,
		ep.UpdatedAt.Format(time.RFC3339), ep.ID)
	if err != nil {
		return fmt.Errorf("update endpoint: %w", err)
	}
	return nil
}

// Delete removes an endpoint by ID.
func (r *Repository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM endpoints WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete endpoint: %w", err)
	}
	return nil
}

func scanEndpoints(rows *sql.Rows) ([]Endpoint, error) {
	var endpoints []Endpoint
	for rows.Next() {
		ep, err := scanEndpoint(rows)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, *ep)
	}
	return endpoints, rows.Err()
}

func scanEndpoint(s scanner) (*Endpoint, error) {
	var ep Endpoint
	var createdAt, updatedAt string
	var enabledVal int
	var nodeID sql.NullString
	err := s.Scan(&ep.ID, &ep.ServiceID, &ep.Type, &ep.Address, &enabledVal, &nodeID, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan endpoint: %w", err)
	}
	ep.Enabled = enabledVal == 1
	ep.NodeID = nodeID.String
	ep.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	ep.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &ep, nil
}

func scanEndpointRow(row *sql.Row) (*Endpoint, error) {
	ep, err := scanEndpoint(row)
	if err != nil {
		return nil, err
	}
	return ep, nil
}

// scanner is an interface shared by *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...interface{}) error
}
