package route

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for routes.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new route repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new route.
func (r *Repository) Create(rt *Route) error {
	tlsVal := 0
	if rt.TLSEnabled {
		tlsVal = 1
	}
	maintVal := 0
	if rt.MaintenanceEnabled {
		maintVal = 1
	}

	_, err := r.DB.Exec(
		`INSERT INTO routes (id, domain, service_id, tls_enabled, status, maintenance_enabled, maintenance_message, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rt.ID, rt.Domain, rt.ServiceID, tlsVal, rt.Status, maintVal, rt.MaintenanceMessage,
		rt.CreatedAt.Format(time.RFC3339),
		rt.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert route: %w", err)
	}
	return nil
}

// FindAll returns all routes ordered by domain.
func (r *Repository) FindAll() ([]Route, error) {
	rows, err := r.DB.Query(
		`SELECT id, domain, service_id, tls_enabled, status, maintenance_enabled, maintenance_message, created_at, updated_at
		 FROM routes ORDER BY domain`)
	if err != nil {
		return nil, fmt.Errorf("query routes: %w", err)
	}
	defer rows.Close()
	return scanRoutes(rows)
}

// FindByID returns a route by ID.
func (r *Repository) FindByID(id string) (*Route, error) {
	var rt Route
	var createdAt, updatedAt string
	var tlsVal, maintVal int
	var maintMsg sql.NullString
	err := r.DB.QueryRow(
		`SELECT id, domain, service_id, tls_enabled, status, maintenance_enabled, maintenance_message, created_at, updated_at
		 FROM routes WHERE id = ?`, id,
	).Scan(&rt.ID, &rt.Domain, &rt.ServiceID, &tlsVal, &rt.Status, &maintVal, &maintMsg, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query route by id: %w", err)
	}
	rt.TLSEnabled = tlsVal == 1
	rt.MaintenanceEnabled = maintVal == 1
	rt.MaintenanceMessage = maintMsg.String
	rt.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	rt.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &rt, nil
}

// FindByDomain returns a route by domain.
func (r *Repository) FindByDomain(domain string) (*Route, error) {
	var rt Route
	var createdAt, updatedAt string
	var tlsVal, maintVal int
	var maintMsg sql.NullString
	err := r.DB.QueryRow(
		`SELECT id, domain, service_id, tls_enabled, status, maintenance_enabled, maintenance_message, created_at, updated_at
		 FROM routes WHERE domain = ?`, domain,
	).Scan(&rt.ID, &rt.Domain, &rt.ServiceID, &tlsVal, &rt.Status, &maintVal, &maintMsg, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query route by domain: %w", err)
	}
	rt.TLSEnabled = tlsVal == 1
	rt.MaintenanceEnabled = maintVal == 1
	rt.MaintenanceMessage = maintMsg.String
	rt.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	rt.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &rt, nil
}

// FindByServiceID returns all routes for a service.
func (r *Repository) FindByServiceID(serviceID string) ([]Route, error) {
	rows, err := r.DB.Query(
		`SELECT id, domain, service_id, tls_enabled, status, maintenance_enabled, maintenance_message, created_at, updated_at
		 FROM routes WHERE service_id = ? ORDER BY domain`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("query routes by service: %w", err)
	}
	defer rows.Close()
	return scanRoutes(rows)
}

// FindActive returns all active routes.
func (r *Repository) FindActive() ([]Route, error) {
	rows, err := r.DB.Query(
		`SELECT id, domain, service_id, tls_enabled, status, maintenance_enabled, maintenance_message, created_at, updated_at
		 FROM routes WHERE status = 'active' ORDER BY domain`)
	if err != nil {
		return nil, fmt.Errorf("query active routes: %w", err)
	}
	defer rows.Close()
	return scanRoutes(rows)
}

// Update updates a route.
func (r *Repository) Update(rt *Route) error {
	tlsVal := 0
	if rt.TLSEnabled {
		tlsVal = 1
	}
	maintVal := 0
	if rt.MaintenanceEnabled {
		maintVal = 1
	}

	_, err := r.DB.Exec(
		`UPDATE routes SET domain=?, service_id=?, tls_enabled=?, status=?, maintenance_enabled=?, maintenance_message=?, updated_at=? WHERE id=?`,
		rt.Domain, rt.ServiceID, tlsVal, rt.Status, maintVal, rt.MaintenanceMessage,
		rt.UpdatedAt.Format(time.RFC3339), rt.ID,
	)
	if err != nil {
		return fmt.Errorf("update route: %w", err)
	}
	return nil
}

func scanRoutes(rows *sql.Rows) ([]Route, error) {
	var routes []Route
	for rows.Next() {
		var rt Route
		var createdAt, updatedAt string
		var tlsVal, maintVal int
		var maintMsg sql.NullString
		if err := rows.Scan(&rt.ID, &rt.Domain, &rt.ServiceID, &tlsVal, &rt.Status, &maintVal, &maintMsg, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		rt.TLSEnabled = tlsVal == 1
		rt.MaintenanceEnabled = maintVal == 1
		rt.MaintenanceMessage = maintMsg.String
		rt.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		rt.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		routes = append(routes, rt)
	}
	return routes, rows.Err()
}
