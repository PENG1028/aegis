package routingpolicy

import (
	"fmt"
	"time"

	"aegis/internal/core"
)

// Service provides gateway policy business logic.
type Service struct {
	repo *Repository
}

// NewService creates a new policy service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SetServicePolicy creates or updates a service gateway policy.
func (s *Service) SetServicePolicy(input PolicyInput) (*ServiceGatewayPolicy, error) {
	if err := validatePolicyInput(input); err != nil {
		return nil, err
	}

	now := time.Now().Format(time.RFC3339)
	mode := input.Mode
	if mode == "" {
		mode = ModeAuto
	}

	policy := &ServiceGatewayPolicy{
		PolicyID:           core.NewID("pol"),
		ServiceID:          input.ServiceID,
		Mode:               mode,
		PrimaryGatewayID:   input.PrimaryGatewayID,
		FallbackGatewayIDs: input.FallbackGatewayIDs,
		AllowLocal:         true,
		AllowPrivate:       true,
		AllowPublic:        false,
		RequireGatewayLink: true,
		RequireRelay:       true,
		PreserveHost:       true,
		TLSMode:            TLSModeHTTPOnly,
		Priority:           input.Priority,
		Enabled:            true,
		UpdatedAt:          now,
	}

	if input.AllowLocal != nil {
		policy.AllowLocal = *input.AllowLocal
	}
	if input.AllowPrivate != nil {
		policy.AllowPrivate = *input.AllowPrivate
	}
	if input.AllowPublic != nil {
		policy.AllowPublic = *input.AllowPublic
	}
	if input.RequireGatewayLink != nil {
		policy.RequireGatewayLink = *input.RequireGatewayLink
	}
	if input.RequireRelay != nil {
		policy.RequireRelay = *input.RequireRelay
	}
	if input.PreserveHost != nil {
		policy.PreserveHost = *input.PreserveHost
	}
	if input.TLSMode != "" {
		policy.TLSMode = input.TLSMode
	}
	if input.Enabled != nil {
		policy.Enabled = *input.Enabled
	}
	if policy.FallbackGatewayIDs == nil {
		policy.FallbackGatewayIDs = []string{}
	}

	if err := s.repo.UpsertServicePolicy(policy); err != nil {
		return nil, fmt.Errorf("upsert service policy: %w", err)
	}
	return policy, nil
}

// SetRoutePolicy creates or updates a route gateway policy.
func (s *Service) SetRoutePolicy(input PolicyInput) (*RouteGatewayPolicy, error) {
	if err := validatePolicyInput(input); err != nil {
		return nil, err
	}

	now := time.Now().Format(time.RFC3339)
	mode := input.Mode
	if mode == "" {
		mode = ModeAuto
	}

	policy := &RouteGatewayPolicy{
		PolicyID:           core.NewID("pol"),
		RouteID:            input.RouteID,
		Mode:               mode,
		PrimaryGatewayID:   input.PrimaryGatewayID,
		FallbackGatewayIDs: input.FallbackGatewayIDs,
		AllowLocal:         true,
		AllowPrivate:       true,
		AllowPublic:        false,
		RequireGatewayLink: true,
		RequireRelay:       true,
		PreserveHost:       true,
		TLSMode:            TLSModeHTTPOnly,
		Priority:           input.Priority,
		Enabled:            true,
		UpdatedAt:          now,
	}

	if input.AllowLocal != nil {
		policy.AllowLocal = *input.AllowLocal
	}
	if input.AllowPrivate != nil {
		policy.AllowPrivate = *input.AllowPrivate
	}
	if input.AllowPublic != nil {
		policy.AllowPublic = *input.AllowPublic
	}
	if input.RequireGatewayLink != nil {
		policy.RequireGatewayLink = *input.RequireGatewayLink
	}
	if input.RequireRelay != nil {
		policy.RequireRelay = *input.RequireRelay
	}
	if input.PreserveHost != nil {
		policy.PreserveHost = *input.PreserveHost
	}
	if input.TLSMode != "" {
		policy.TLSMode = input.TLSMode
	}
	if input.Enabled != nil {
		policy.Enabled = *input.Enabled
	}
	if policy.FallbackGatewayIDs == nil {
		policy.FallbackGatewayIDs = []string{}
	}

	if err := s.repo.UpsertRoutePolicy(policy); err != nil {
		return nil, fmt.Errorf("upsert route policy: %w", err)
	}
	return policy, nil
}

// GetServicePolicy retrieves the service gateway policy.
func (s *Service) GetServicePolicy(serviceID string) (*ServiceGatewayPolicy, error) {
	return s.repo.GetServicePolicy(serviceID)
}

// GetRoutePolicy retrieves the route gateway policy.
func (s *Service) GetRoutePolicy(routeID string) (*RouteGatewayPolicy, error) {
	return s.repo.GetRoutePolicy(routeID)
}

// ResolvePolicy resolves the effective policy for a route+service combination.
func (s *Service) ResolvePolicy(routeID, serviceID string) (*ResolvedPolicy, error) {
	return s.repo.ResolvePolicy(routeID, serviceID)
}

// ListServicePolicies returns all service policies.
func (s *Service) ListServicePolicies() ([]ServiceGatewayPolicy, error) {
	return s.repo.ListServicePolicies()
}

// ListRoutePolicies returns all route policies.
func (s *Service) ListRoutePolicies() ([]RouteGatewayPolicy, error) {
	return s.repo.ListRoutePolicies()
}

// validatePolicyInput validates policy input fields.
func validatePolicyInput(input PolicyInput) error {
	if input.Mode != "" && !IsValidMode(input.Mode) {
		return fmt.Errorf("invalid mode: %s (valid: auto, fixed, multi, disabled)", input.Mode)
	}
	if input.TLSMode != "" && !IsValidTLSMode(input.TLSMode) {
		return fmt.Errorf("invalid tls_mode: %s", input.TLSMode)
	}
	if input.Mode == ModeFixed && input.PrimaryGatewayID == "" {
		return fmt.Errorf("fixed mode requires primary_gateway_id")
	}
	if input.Mode == ModeMulti && len(input.FallbackGatewayIDs) == 0 && input.PrimaryGatewayID == "" {
		return fmt.Errorf("multi mode requires at least primary_gateway_id or fallback_gateway_ids")
	}
	return nil
}
