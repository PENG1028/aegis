package gateway

import (
	"database/sql"
	"fmt"
	"time"
)

// DomainRepository provides database access for gateway domains.
type DomainRepository struct{ DB *sql.DB }

func NewDomainRepository(db *sql.DB) *DomainRepository { return &DomainRepository{DB: db} }

func (r *DomainRepository) Create(d *GatewayDomain) error {
	tls := 0
	if d.TLSEnabled { tls = 1 }
	_, err := r.DB.Exec(
		`INSERT INTO gateway_domains (id, domain, node_id, tls_enabled, tls_provider, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Domain, d.NodeID, tls, d.TLSProvider, d.Status,
		d.CreatedAt.Format(time.RFC3339), d.UpdatedAt.Format(time.RFC3339))
	if err != nil { return fmt.Errorf("insert gateway_domain: %w", err) }
	return nil
}

func (r *DomainRepository) FindAll() ([]GatewayDomain, error) {
	rows, err := r.DB.Query(`SELECT id, domain, node_id, tls_enabled, tls_provider, status, created_at, updated_at FROM gateway_domains ORDER BY domain`)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanDomains(rows)
}

func (r *DomainRepository) FindByID(id string) (*GatewayDomain, error) {
	var d GatewayDomain
	var ca, ua string
	var tls int
	err := r.DB.QueryRow(`SELECT id, domain, node_id, tls_enabled, tls_provider, status, created_at, updated_at FROM gateway_domains WHERE id=?`, id).
		Scan(&d.ID, &d.Domain, &d.NodeID, &tls, &d.TLSProvider, &d.Status, &ca, &ua)
	if err != nil {
		if err == sql.ErrNoRows { return nil, nil }
		return nil, err
	}
	d.TLSEnabled = tls == 1
	d.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	d.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &d, nil
}

func (r *DomainRepository) FindByNodeID(nodeID string) ([]GatewayDomain, error) {
	rows, err := r.DB.Query(`SELECT id, domain, node_id, tls_enabled, tls_provider, status, created_at, updated_at FROM gateway_domains WHERE node_id=? ORDER BY domain`, nodeID)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanDomains(rows)
}

func (r *DomainRepository) Update(d *GatewayDomain) error {
	tls := 0
	if d.TLSEnabled { tls = 1 }
	_, err := r.DB.Exec(`UPDATE gateway_domains SET domain=?, node_id=?, tls_enabled=?, tls_provider=?, status=?, updated_at=? WHERE id=?`,
		d.Domain, d.NodeID, tls, d.TLSProvider, d.Status, d.UpdatedAt.Format(time.RFC3339), d.ID)
	return err
}

func (r *DomainRepository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM gateway_domains WHERE id=?`, id)
	return err
}

func scanDomains(rows *sql.Rows) ([]GatewayDomain, error) {
	var ds []GatewayDomain
	for rows.Next() {
		var d GatewayDomain
		var ca, ua string
		var tls int
		if err := rows.Scan(&d.ID, &d.Domain, &d.NodeID, &tls, &d.TLSProvider, &d.Status, &ca, &ua); err != nil {
			return nil, err
		}
		d.TLSEnabled = tls == 1
		d.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		d.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		ds = append(ds, d)
	}
	return ds, rows.Err()
}

// RouteRepository provides database access for gateway routes.
type RouteRepository struct{ DB *sql.DB }

func NewRouteRepository(db *sql.DB) *RouteRepository { return &RouteRepository{DB: db} }

func (r *RouteRepository) Create(rt *GatewayRoute) error {
	_, err := r.DB.Exec(
		`INSERT INTO gateway_routes (id, domain_id, path, target_service, target_port, protocol, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rt.ID, rt.DomainID, rt.Path, rt.TargetService, rt.TargetPort, rt.Protocol, rt.Status,
		rt.CreatedAt.Format(time.RFC3339), rt.UpdatedAt.Format(time.RFC3339))
	if err != nil { return fmt.Errorf("insert gateway_route: %w", err) }
	return nil
}

func (r *RouteRepository) FindAll() ([]GatewayRoute, error) {
	rows, err := r.DB.Query(`SELECT id, domain_id, path, target_service, target_port, protocol, status, created_at, updated_at FROM gateway_routes ORDER BY path`)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanRoutes(rows)
}

func (r *RouteRepository) FindByDomainID(domainID string) ([]GatewayRoute, error) {
	rows, err := r.DB.Query(`SELECT id, domain_id, path, target_service, target_port, protocol, status, created_at, updated_at FROM gateway_routes WHERE domain_id=? ORDER BY path`, domainID)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanRoutes(rows)
}

func (r *RouteRepository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM gateway_routes WHERE id=?`, id)
	return err
}

func scanRoutes(rows *sql.Rows) ([]GatewayRoute, error) {
	var rts []GatewayRoute
	for rows.Next() {
		var rt GatewayRoute
		var ca, ua string
		if err := rows.Scan(&rt.ID, &rt.DomainID, &rt.Path, &rt.TargetService, &rt.TargetPort, &rt.Protocol, &rt.Status, &ca, &ua); err != nil {
			return nil, err
		}
		rt.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		rt.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		rts = append(rts, rt)
	}
	return rts, rows.Err()
}

// ListenerRepository provides database access for gateway listeners.
type ListenerRepository struct{ DB *sql.DB }

func NewListenerRepository(db *sql.DB) *ListenerRepository { return &ListenerRepository{DB: db} }

func (r *ListenerRepository) Create(l *GatewayListener) error {
	tls := 0
	if l.TLSEnabled { tls = 1 }
	_, err := r.DB.Exec(
		`INSERT INTO gateway_listeners (id, node_id, port, tls_enabled, protocol, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.NodeID, l.Port, tls, l.Protocol, l.Status,
		l.CreatedAt.Format(time.RFC3339), l.UpdatedAt.Format(time.RFC3339))
	if err != nil { return fmt.Errorf("insert gateway_listener: %w", err) }
	return nil
}

func (r *ListenerRepository) FindAll() ([]GatewayListener, error) {
	rows, err := r.DB.Query(`SELECT id, node_id, port, tls_enabled, protocol, status, created_at, updated_at FROM gateway_listeners ORDER BY port`)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanListeners(rows)
}

func (r *ListenerRepository) FindByNodeID(nodeID string) ([]GatewayListener, error) {
	rows, err := r.DB.Query(`SELECT id, node_id, port, tls_enabled, protocol, status, created_at, updated_at FROM gateway_listeners WHERE node_id=? ORDER BY port`, nodeID)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanListeners(rows)
}

func scanListeners(rows *sql.Rows) ([]GatewayListener, error) {
	var ls []GatewayListener
	for rows.Next() {
		var l GatewayListener
		var ca, ua string
		var tls int
		if err := rows.Scan(&l.ID, &l.NodeID, &l.Port, &tls, &l.Protocol, &l.Status, &ca, &ua); err != nil {
			return nil, err
		}
		l.TLSEnabled = tls == 1
		l.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		l.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		ls = append(ls, l)
	}
	return ls, rows.Err()
}
