package routingpolicy

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// Repository provides DB access for gateway policies.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new policy repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// ============================================================================
// Service Gateway Policy
// ============================================================================

// CreateServicePolicy creates a service gateway policy.
func (r *Repository) CreateServicePolicy(p *ServiceGatewayPolicy) error {
	fbJSON, err := json.Marshal(p.FallbackGatewayIDs)
	if err != nil {
		return fmt.Errorf("marshal fallback ids: %w", err)
	}

	_, err = r.DB.Exec(
		`INSERT INTO service_gateway_policies
		(policy_id, service_id, mode, primary_gateway_id, fallback_gateway_ids_json,
		 allow_local, allow_private, allow_public, require_gateway_link, require_relay,
		 preserve_host, tls_mode, priority, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.PolicyID, p.ServiceID, p.Mode, p.PrimaryGatewayID, string(fbJSON),
		boolToInt(p.AllowLocal), boolToInt(p.AllowPrivate), boolToInt(p.AllowPublic),
		boolToInt(p.RequireGatewayLink), boolToInt(p.RequireRelay),
		boolToInt(p.PreserveHost), p.TLSMode, p.Priority, boolToInt(p.Enabled),
		p.CreatedAt, p.UpdatedAt,
	)
	return err
}

// GetServicePolicy returns the service gateway policy for a service.
func (r *Repository) GetServicePolicy(serviceID string) (*ServiceGatewayPolicy, error) {
	var p ServiceGatewayPolicy
	var fbJSON string
	var allowLocal, allowPriv, allowPub, reqGL, reqRelay, presHost, en int

	err := r.DB.QueryRow(
		`SELECT policy_id, service_id, mode, primary_gateway_id, fallback_gateway_ids_json,
		 allow_local, allow_private, allow_public, require_gateway_link, require_relay,
		 preserve_host, tls_mode, priority, enabled, created_at, updated_at
		 FROM service_gateway_policies WHERE service_id = ?`, serviceID,
	).Scan(&p.PolicyID, &p.ServiceID, &p.Mode, &p.PrimaryGatewayID, &fbJSON,
		&allowLocal, &allowPriv, &allowPub, &reqGL, &reqRelay,
		&presHost, &p.TLSMode, &p.Priority, &en, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query service policy: %w", err)
	}

	p.AllowLocal = allowLocal == 1
	p.AllowPrivate = allowPriv == 1
	p.AllowPublic = allowPub == 1
	p.RequireGatewayLink = reqGL == 1
	p.RequireRelay = reqRelay == 1
	p.PreserveHost = presHost == 1
	p.Enabled = en == 1

	if fbJSON != "" {
		json.Unmarshal([]byte(fbJSON), &p.FallbackGatewayIDs)
	}
	if p.FallbackGatewayIDs == nil {
		p.FallbackGatewayIDs = []string{}
	}

	return &p, nil
}

// UpsertServicePolicy creates or updates a service gateway policy.
func (r *Repository) UpsertServicePolicy(p *ServiceGatewayPolicy) error {
	existing, err := r.GetServicePolicy(p.ServiceID)
	if err != nil {
		return err
	}
	if existing != nil {
		p.PolicyID = existing.PolicyID
		p.CreatedAt = existing.CreatedAt
		return r.updateServicePolicy(p)
	}
	return r.CreateServicePolicy(p)
}

func (r *Repository) updateServicePolicy(p *ServiceGatewayPolicy) error {
	fbJSON, err := json.Marshal(p.FallbackGatewayIDs)
	if err != nil {
		return fmt.Errorf("marshal fallback ids: %w", err)
	}

	_, err = r.DB.Exec(
		`UPDATE service_gateway_policies SET mode=?, primary_gateway_id=?, fallback_gateway_ids_json=?,
		 allow_local=?, allow_private=?, allow_public=?, require_gateway_link=?, require_relay=?,
		 preserve_host=?, tls_mode=?, priority=?, enabled=?, updated_at=?
		 WHERE service_id=?`,
		p.Mode, p.PrimaryGatewayID, string(fbJSON),
		boolToInt(p.AllowLocal), boolToInt(p.AllowPrivate), boolToInt(p.AllowPublic),
		boolToInt(p.RequireGatewayLink), boolToInt(p.RequireRelay),
		boolToInt(p.PreserveHost), p.TLSMode, p.Priority, boolToInt(p.Enabled),
		p.UpdatedAt, p.ServiceID,
	)
	return err
}

// DeleteServicePolicy deletes the service gateway policy.
func (r *Repository) DeleteServicePolicy(serviceID string) error {
	_, err := r.DB.Exec(`DELETE FROM service_gateway_policies WHERE service_id=?`, serviceID)
	return err
}


// ============================================================================
// Route Gateway Policy
// ============================================================================

// CreateRoutePolicy creates a route gateway policy.
func (r *Repository) CreateRoutePolicy(p *RouteGatewayPolicy) error {
	fbJSON, err := json.Marshal(p.FallbackGatewayIDs)
	if err != nil {
		return fmt.Errorf("marshal fallback ids: %w", err)
	}

	_, err = r.DB.Exec(
		`INSERT INTO route_gateway_policies
		(policy_id, route_id, mode, primary_gateway_id, fallback_gateway_ids_json,
		 allow_local, allow_private, allow_public, require_gateway_link, require_relay,
		 preserve_host, tls_mode, priority, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.PolicyID, p.RouteID, p.Mode, p.PrimaryGatewayID, string(fbJSON),
		boolToInt(p.AllowLocal), boolToInt(p.AllowPrivate), boolToInt(p.AllowPublic),
		boolToInt(p.RequireGatewayLink), boolToInt(p.RequireRelay),
		boolToInt(p.PreserveHost), p.TLSMode, p.Priority, boolToInt(p.Enabled),
		p.CreatedAt, p.UpdatedAt,
	)
	return err
}

// GetRoutePolicy returns the route gateway policy for a route.
func (r *Repository) GetRoutePolicy(routeID string) (*RouteGatewayPolicy, error) {
	var p RouteGatewayPolicy
	var fbJSON string
	var allowLocal, allowPriv, allowPub, reqGL, reqRelay, presHost, en int

	err := r.DB.QueryRow(
		`SELECT policy_id, route_id, mode, primary_gateway_id, fallback_gateway_ids_json,
		 allow_local, allow_private, allow_public, require_gateway_link, require_relay,
		 preserve_host, tls_mode, priority, enabled, created_at, updated_at
		 FROM route_gateway_policies WHERE route_id = ?`, routeID,
	).Scan(&p.PolicyID, &p.RouteID, &p.Mode, &p.PrimaryGatewayID, &fbJSON,
		&allowLocal, &allowPriv, &allowPub, &reqGL, &reqRelay,
		&presHost, &p.TLSMode, &p.Priority, &en, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query route policy: %w", err)
	}

	p.AllowLocal = allowLocal == 1
	p.AllowPrivate = allowPriv == 1
	p.AllowPublic = allowPub == 1
	p.RequireGatewayLink = reqGL == 1
	p.RequireRelay = reqRelay == 1
	p.PreserveHost = presHost == 1
	p.Enabled = en == 1

	if fbJSON != "" {
		json.Unmarshal([]byte(fbJSON), &p.FallbackGatewayIDs)
	}
	if p.FallbackGatewayIDs == nil {
		p.FallbackGatewayIDs = []string{}
	}

	return &p, nil
}

// UpsertRoutePolicy creates or updates a route gateway policy.
func (r *Repository) UpsertRoutePolicy(p *RouteGatewayPolicy) error {
	existing, err := r.GetRoutePolicy(p.RouteID)
	if err != nil {
		return err
	}
	if existing != nil {
		p.PolicyID = existing.PolicyID
		p.CreatedAt = existing.CreatedAt
		return r.updateRoutePolicy(p)
	}
	return r.CreateRoutePolicy(p)
}

func (r *Repository) updateRoutePolicy(p *RouteGatewayPolicy) error {
	fbJSON, err := json.Marshal(p.FallbackGatewayIDs)
	if err != nil {
		return fmt.Errorf("marshal fallback ids: %w", err)
	}

	_, err = r.DB.Exec(
		`UPDATE route_gateway_policies SET mode=?, primary_gateway_id=?, fallback_gateway_ids_json=?,
		 allow_local=?, allow_private=?, allow_public=?, require_gateway_link=?, require_relay=?,
		 preserve_host=?, tls_mode=?, priority=?, enabled=?, updated_at=?
		 WHERE route_id=?`,
		p.Mode, p.PrimaryGatewayID, string(fbJSON),
		boolToInt(p.AllowLocal), boolToInt(p.AllowPrivate), boolToInt(p.AllowPublic),
		boolToInt(p.RequireGatewayLink), boolToInt(p.RequireRelay),
		boolToInt(p.PreserveHost), p.TLSMode, p.Priority, boolToInt(p.Enabled),
		p.UpdatedAt, p.RouteID,
	)
	return err
}

// DeleteRoutePolicy deletes the route gateway policy.
func (r *Repository) DeleteRoutePolicy(routeID string) error {
	_, err := r.DB.Exec(`DELETE FROM route_gateway_policies WHERE route_id=?`, routeID)
	return err
}


// ============================================================================
// Resolution
// ============================================================================

// ResolvePolicy resolves the effective policy for a route+service combination.
// Precedence: route policy > service policy > default.
func (r *Repository) ResolvePolicy(routeID, serviceID string) (*ResolvedPolicy, error) {
	// 1. Check route policy
	rp, err := r.GetRoutePolicy(routeID)
	if err != nil {
		return nil, err
	}
	if rp != nil && rp.Enabled {
		return &ResolvedPolicy{
			Source:             "route",
			Mode:               rp.Mode,
			PrimaryGatewayID:   rp.PrimaryGatewayID,
			FallbackGatewayIDs: rp.FallbackGatewayIDs,
			AllowLocal:         rp.AllowLocal,
			AllowPrivate:       rp.AllowPrivate,
			AllowPublic:        rp.AllowPublic,
			RequireGatewayLink: rp.RequireGatewayLink,
			RequireRelay:       rp.RequireRelay,
			PreserveHost:       rp.PreserveHost,
			TLSMode:            rp.TLSMode,
		}, nil
	}

	// 2. Check service policy
	sp, err := r.GetServicePolicy(serviceID)
	if err != nil {
		return nil, err
	}
	if sp != nil && sp.Enabled {
		return &ResolvedPolicy{
			Source:             "service",
			Mode:               sp.Mode,
			PrimaryGatewayID:   sp.PrimaryGatewayID,
			FallbackGatewayIDs: sp.FallbackGatewayIDs,
			AllowLocal:         sp.AllowLocal,
			AllowPrivate:       sp.AllowPrivate,
			AllowPublic:        sp.AllowPublic,
			RequireGatewayLink: sp.RequireGatewayLink,
			RequireRelay:       sp.RequireRelay,
			PreserveHost:       sp.PreserveHost,
			TLSMode:            sp.TLSMode,
		}, nil
	}

	// 3. Default
	def := DefaultPolicy()
	return &def, nil
}

// ============================================================================
// Helpers
// ============================================================================

func scanServicePolicies(rows *sql.Rows) ([]ServiceGatewayPolicy, error) {
	var policies []ServiceGatewayPolicy
	for rows.Next() {
		var p ServiceGatewayPolicy
		var fbJSON string
		var allowLocal, allowPriv, allowPub, reqGL, reqRelay, presHost, en int

		if err := rows.Scan(&p.PolicyID, &p.ServiceID, &p.Mode, &p.PrimaryGatewayID, &fbJSON,
			&allowLocal, &allowPriv, &allowPub, &reqGL, &reqRelay,
			&presHost, &p.TLSMode, &p.Priority, &en, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan service policy: %w", err)
		}

		p.AllowLocal = allowLocal == 1
		p.AllowPrivate = allowPriv == 1
		p.AllowPublic = allowPub == 1
		p.RequireGatewayLink = reqGL == 1
		p.RequireRelay = reqRelay == 1
		p.PreserveHost = presHost == 1
		p.Enabled = en == 1

		if fbJSON != "" {
			json.Unmarshal([]byte(fbJSON), &p.FallbackGatewayIDs)
		}
		if p.FallbackGatewayIDs == nil {
			p.FallbackGatewayIDs = []string{}
		}

		policies = append(policies, p)
	}
	if policies == nil {
		policies = []ServiceGatewayPolicy{}
	}
	return policies, rows.Err()
}

func scanRoutePolicies(rows *sql.Rows) ([]RouteGatewayPolicy, error) {
	var policies []RouteGatewayPolicy
	for rows.Next() {
		var p RouteGatewayPolicy
		var fbJSON string
		var allowLocal, allowPriv, allowPub, reqGL, reqRelay, presHost, en int

		if err := rows.Scan(&p.PolicyID, &p.RouteID, &p.Mode, &p.PrimaryGatewayID, &fbJSON,
			&allowLocal, &allowPriv, &allowPub, &reqGL, &reqRelay,
			&presHost, &p.TLSMode, &p.Priority, &en, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan route policy: %w", err)
		}

		p.AllowLocal = allowLocal == 1
		p.AllowPrivate = allowPriv == 1
		p.AllowPublic = allowPub == 1
		p.RequireGatewayLink = reqGL == 1
		p.RequireRelay = reqRelay == 1
		p.PreserveHost = presHost == 1
		p.Enabled = en == 1

		if fbJSON != "" {
			json.Unmarshal([]byte(fbJSON), &p.FallbackGatewayIDs)
		}
		if p.FallbackGatewayIDs == nil {
			p.FallbackGatewayIDs = []string{}
		}

		policies = append(policies, p)
	}
	if policies == nil {
		policies = []RouteGatewayPolicy{}
	}
	return policies, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
