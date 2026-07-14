package route

import (
	"fmt"
	"strings"
	"time"

	"aegis/internal/hostdep/provider"
)

// Route represents a routing rule that maps a domain to a backend service.
type Route struct {
	ID                 string    `json:"id"`
	Domain             string    `json:"domain"`
	PathPrefix         string    `json:"path_prefix"`
	StripPrefix        bool      `json:"strip_prefix"`
	ServiceID          string    `json:"service_id"`
	TLSEnabled          bool      `json:"tls_enabled"`           // deprecated — derived from Composition
	Composition        string    `json:"composition,omitempty"`  // v1.8L-22 → CompKey string, e.g. "https_route"
	Status             string    `json:"status"` // active | disabled
	MaintenanceEnabled bool      `json:"maintenance_enabled"`
	MaintenanceMessage string    `json:"maintenance_message"`
	SpaceID            string    `json:"space_id"`
	OwnerType          string    `json:"owner_type"`         // space | admin
	OwnerID            string    `json:"owner_id"`           // space_id when owner_type=space
	CreatedByTokenID   string    `json:"created_by_token_id"`
	GatewayLinkID      string    `json:"gateway_link_id,omitempty"` // v1.7AB
	CertID             *string   `json:"cert_id,omitempty"`          // v1.9C — custom TLS certificate reference
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// CompDef returns the composition definition for this route, or nil.
func (r *Route) CompDef() *provider.CompDef {
	if r.Composition != "" {
		return provider.LookupComp(provider.CompKey(r.Composition))
	}
	// Backward-compat: derive from TLSEnabled for routes created before v1.8L-22
	if r.TLSEnabled {
		return provider.LookupComp(provider.CompHTTPSRoute)
	}
	return provider.LookupComp(provider.CompHTTPRoute)
}

// CreateRouteInput is the input for creating a route.
type CreateRouteInput struct {
	Domain      string
	PathPrefix  string
	StripPrefix bool
	ServiceID   string
	Composition string `json:"composition,omitempty"` // v1.8L-22 — CompKey
}

// SwitchRouteInput is the input for switching a route's service.
type SwitchRouteInput struct {
	ServiceID string
}

// ValidatePathPrefix checks path_prefix validity.
func ValidatePathPrefix(path string) error {
	if path == "" {
		return nil // empty is valid (domain-only route)
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("path_prefix must start with /")
	}
	if path == "*" {
		return fmt.Errorf("path_prefix cannot be '*'")
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("path_prefix cannot contain '..'")
	}
	if strings.Contains(path, " ") {
		return fmt.Errorf("path_prefix cannot contain spaces")
	}
	return nil
}

// PathDepth returns the depth of a path (used for sorting).
func PathDepth(path string) int {
	if path == "" {
		return 0
	}
	return len(strings.Split(strings.Trim(path, "/"), "/"))
}
