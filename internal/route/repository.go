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

const routeSelectCols = `id, domain, path_prefix, strip_prefix, service_id, tls_enabled, status, maintenance_enabled, maintenance_message, space_id, owner_type, owner_id, created_by_token_id, gateway_link_id, created_at, updated_at`

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
	stripVal := 0
	if rt.StripPrefix {
		stripVal = 1
	}

	_, err := r.DB.Exec(
		`INSERT INTO routes (id, domain, path_prefix, strip_prefix, service_id, tls_enabled, status, maintenance_enabled, maintenance_message, space_id, owner_type, owner_id, created_by_token_id, gateway_link_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rt.ID, rt.Domain, rt.PathPrefix, stripVal, rt.ServiceID, tlsVal, rt.Status, maintVal, rt.MaintenanceMessage,
		rt.SpaceID, rt.OwnerType, rt.OwnerID, rt.CreatedByTokenID,
		rt.CreatedAt.Format(time.RFC3339),
		rt.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert route: %w", err)
	}
	return nil
}

// FindAll returns all routes ordered by domain, then path_prefix depth desc.
func (r *Repository) FindAll() ([]Route, error) {
	rows, err := r.DB.Query(
		`SELECT ` + routeSelectCols + ` FROM routes ORDER BY domain, LENGTH(path_prefix) DESC`)
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
	var pathPrefix sql.NullString
	var tlsVal, maintVal, stripVal int
	var maintMsg sql.NullString
	err := r.DB.QueryRow(
		`SELECT `+routeSelectCols+` FROM routes WHERE id = ?`, id,
	).Scan(&rt.ID, &rt.Domain, &pathPrefix, &stripVal, &rt.ServiceID, &tlsVal, &rt.Status, &maintVal, &maintMsg, &rt.SpaceID, &rt.OwnerType, &rt.OwnerID, &rt.CreatedByTokenID, &rt.GatewayLinkID, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query route by id: %w", err)
	}
	rt.PathPrefix = pathPrefix.String
	rt.StripPrefix = stripVal == 1
	rt.TLSEnabled = tlsVal == 1
	rt.MaintenanceEnabled = maintVal == 1
	rt.MaintenanceMessage = maintMsg.String
	rt.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	rt.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &rt, nil
}

// FindByDomain returns all routes for a domain, longest path first.
func (r *Repository) FindByDomain(domain string) (*Route, error) {
	var rt Route
	var createdAt, updatedAt string
	var pathPrefix sql.NullString
	var tlsVal, maintVal, stripVal int
	var maintMsg sql.NullString
	err := r.DB.QueryRow(
		`SELECT `+routeSelectCols+` FROM routes WHERE domain = ? AND (path_prefix IS NULL OR path_prefix = '') LIMIT 1`, domain,
	).Scan(&rt.ID, &rt.Domain, &pathPrefix, &stripVal, &rt.ServiceID, &tlsVal, &rt.Status, &maintVal, &maintMsg, &rt.SpaceID, &rt.OwnerType, &rt.OwnerID, &rt.CreatedByTokenID, &rt.GatewayLinkID, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query route by domain: %w", err)
	}
	rt.PathPrefix = pathPrefix.String
	rt.StripPrefix = stripVal == 1
	rt.TLSEnabled = tlsVal == 1
	rt.MaintenanceEnabled = maintVal == 1
	rt.MaintenanceMessage = maintMsg.String
	rt.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	rt.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &rt, nil
}

// FindByDomainAndPath finds a route by domain and path_prefix.
func (r *Repository) FindByDomainAndPath(domain, pathPrefix string) (*Route, error) {
	var rt Route
	var createdAt, updatedAt string
	var dbPathPrefix sql.NullString
	var tlsVal, maintVal, stripVal int
	var maintMsg sql.NullString
	var row *sql.Row
	if pathPrefix == "" {
		row = r.DB.QueryRow(
			`SELECT `+routeSelectCols+` FROM routes WHERE domain = ? AND (path_prefix IS NULL OR path_prefix = '') LIMIT 1`, domain)
	} else {
		row = r.DB.QueryRow(
			`SELECT `+routeSelectCols+` FROM routes WHERE domain = ? AND path_prefix = ?`, domain, pathPrefix)
	}
	err := row.Scan(&rt.ID, &rt.Domain, &dbPathPrefix, &stripVal, &rt.ServiceID, &tlsVal, &rt.Status, &maintVal, &maintMsg, &rt.SpaceID, &rt.OwnerType, &rt.OwnerID, &rt.CreatedByTokenID, &rt.GatewayLinkID, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query route by domain+path: %w", err)
	}
	rt.PathPrefix = dbPathPrefix.String
	rt.StripPrefix = stripVal == 1
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
		`SELECT `+routeSelectCols+` FROM routes WHERE service_id = ? ORDER BY domain`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("query routes by service: %w", err)
	}
	defer rows.Close()
	return scanRoutes(rows)
}

// FindActive returns all active routes, longest path first per domain.
func (r *Repository) FindActive() ([]Route, error) {
	rows, err := r.DB.Query(
		`SELECT `+routeSelectCols+` FROM routes WHERE status = 'active' ORDER BY domain, LENGTH(COALESCE(path_prefix,'')) DESC`)
	if err != nil {
		return nil, fmt.Errorf("query active routes: %w", err)
	}
	defer rows.Close()
	return scanRoutes(rows)
}

// FindBySpaceID returns all routes for a space.
func (r *Repository) FindBySpaceID(spaceID string) ([]Route, error) {
	rows, err := r.DB.Query(
		`SELECT `+routeSelectCols+` FROM routes WHERE space_id = ? ORDER BY domain`, spaceID)
	if err != nil {
		return nil, fmt.Errorf("query routes by space_id: %w", err)
	}
	defer rows.Close()
	return scanRoutes(rows)
}

// CheckDuplicatePath checks for duplicate path_prefix on same domain.
func (r *Repository) CheckDuplicatePath(domain, pathPrefix, excludeID string) error {
	var count int
	var err error
	if pathPrefix == "" {
		err = r.DB.QueryRow(
			`SELECT COUNT(*) FROM routes WHERE domain = ? AND (path_prefix IS NULL OR path_prefix = '') AND id != ?`,
			domain, excludeID).Scan(&count)
	} else {
		err = r.DB.QueryRow(
			`SELECT COUNT(*) FROM routes WHERE domain = ? AND path_prefix = ? AND id != ?`,
			domain, pathPrefix, excludeID).Scan(&count)
	}
	if err != nil {
		return err
	}
	if count > 0 {
		if pathPrefix == "" {
			return fmt.Errorf("domain %s already has a domain-only route", domain)
		}
		return fmt.Errorf("domain %s already has a route with path_prefix %s (duplicate path not allowed)", domain, pathPrefix)
	}
	return nil
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
	stripVal := 0
	if rt.StripPrefix {
		stripVal = 1
	}

	_, err := r.DB.Exec(
		`UPDATE routes SET domain=?, path_prefix=?, strip_prefix=?, service_id=?, tls_enabled=?, status=?, maintenance_enabled=?, maintenance_message=?, space_id=?, owner_type=?, owner_id=?, created_by_token_id=?, updated_at=? WHERE id=?`,
		rt.Domain, rt.PathPrefix, stripVal, rt.ServiceID, tlsVal, rt.Status, maintVal, rt.MaintenanceMessage,
		rt.SpaceID, rt.OwnerType, rt.OwnerID, rt.CreatedByTokenID,
		rt.UpdatedAt.Format(time.RFC3339), rt.ID,
	)
	if err != nil {
		return fmt.Errorf("update route: %w", err)
	}
	return nil
}

// Delete removes a route by ID.
func (r *Repository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM routes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete route: %w", err)
	}
	return nil
}

func scanRoutes(rows *sql.Rows) ([]Route, error) {
	var routes []Route
	for rows.Next() {
		var rt Route
		var createdAt, updatedAt string
		var pathPrefix sql.NullString
		var tlsVal, maintVal, stripVal int
		var maintMsg sql.NullString
		if err := rows.Scan(&rt.ID, &rt.Domain, &pathPrefix, &stripVal, &rt.ServiceID, &tlsVal, &rt.Status, &maintVal, &maintMsg, &rt.SpaceID, &rt.OwnerType, &rt.OwnerID, &rt.CreatedByTokenID, &rt.GatewayLinkID, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		rt.PathPrefix = pathPrefix.String
		rt.StripPrefix = stripVal == 1
		rt.TLSEnabled = tlsVal == 1
		rt.MaintenanceEnabled = maintVal == 1
		rt.MaintenanceMessage = maintMsg.String
		rt.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		rt.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		routes = append(routes, rt)
	}
	return routes, rows.Err()
}
