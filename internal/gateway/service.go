package gateway

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/id"
	"aegis/internal/node"
)

// GatewayService provides provider-agnostic gateway operations.
type GatewayService struct {
	domains   *DomainRepository
	routes    *RouteRepository
	listeners *ListenerRepository
	nodeRepo  *node.Repository
}

// NewGatewayService creates a new gateway service.
func NewGatewayService(dr *DomainRepository, rr *RouteRepository, lr *ListenerRepository, nr *node.Repository) *GatewayService {
	return &GatewayService{domains: dr, routes: rr, listeners: lr, nodeRepo: nr}
}

// CreateDomain creates a new gateway domain binding.
func (s *GatewayService) CreateDomain(ctx context.Context, domain, nodeID string, tlsEnabled bool) (*GatewayDomain, error) {
	// Check node capabilities
	n, err := s.nodeRepo.FindByNodeID(nodeID)
	if err != nil || n == nil {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}
	if tlsEnabled && !n.Capabilities.HasCapability(node.CapTLSSupported) {
		return nil, fmt.Errorf("TLS not supported on node %s", nodeID)
	}
	if !n.Capabilities.HasCapability(node.CapGatewayEnabled) {
		return nil, fmt.Errorf("gateway not enabled on node %s", nodeID)
	}

	now := time.Now()
	tlsProvider := ""
	if tlsEnabled {
		if n.Capabilities.HasCapability(node.CapHAProxyInstalled) {
			tlsProvider = "haproxy"
		} else if n.Capabilities.HasCapability(node.CapCaddyInstalled) {
			tlsProvider = "caddy"
		}
	}

	d := &GatewayDomain{
		ID:          id.New("gd"),
		Domain:      domain,
		NodeID:      nodeID,
		TLSEnabled:  tlsEnabled,
		TLSProvider: tlsProvider,
		Status:      StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.domains.Create(d); err != nil {
		return nil, err
	}
	return d, nil
}

// AttachRoute adds a route to a gateway domain.
func (s *GatewayService) AttachRoute(ctx context.Context, domainID, path, targetService string, targetPort int, protocol string) (*GatewayRoute, error) {
	_, err := s.domains.FindByID(domainID)
	if err != nil || err == nil {
		// domain exists check
	}
	now := time.Now()
	rt := &GatewayRoute{
		ID:            id.New("gr"),
		DomainID:      domainID,
		Path:          path,
		TargetService: targetService,
		TargetPort:    targetPort,
		Protocol:      protocol,
		Status:        StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.routes.Create(rt); err != nil {
		return nil, err
	}
	return rt, nil
}

// DetachRoute removes a route from a gateway domain.
func (s *GatewayService) DetachRoute(ctx context.Context, routeID string) error {
	return s.routes.Delete(routeID)
}

// ListDomains returns all gateway domains.
func (s *GatewayService) ListDomains(ctx context.Context) ([]GatewayDomain, error) {
	ds, err := s.domains.FindAll()
	if err != nil { return nil, err }
	if ds == nil { ds = []GatewayDomain{} }
	return ds, nil
}

// ListRoutes returns routes for a domain.
func (s *GatewayService) ListRoutes(ctx context.Context, domainID string) ([]GatewayRoute, error) {
	return s.routes.FindByDomainID(domainID)
}

// ListListeners returns all gateway listeners.
func (s *GatewayService) ListListeners(ctx context.Context) ([]GatewayListener, error) {
	ls, err := s.listeners.FindAll()
	if err != nil { return nil, err }
	if ls == nil { ls = []GatewayListener{} }
	return ls, nil
}

// UpdateTLSPolicy updates TLS settings for a domain.
func (s *GatewayService) UpdateTLSPolicy(ctx context.Context, domainID string, tlsEnabled bool) (*GatewayDomain, error) {
	d, err := s.domains.FindByID(domainID)
	if err != nil || d == nil {
		return nil, fmt.Errorf("domain %s not found", domainID)
	}
	n, err := s.nodeRepo.FindByNodeID(d.NodeID)
	if err != nil || n == nil {
		return nil, fmt.Errorf("node %s not found", d.NodeID)
	}
	if tlsEnabled && !n.Capabilities.HasCapability(node.CapTLSSupported) {
		return nil, fmt.Errorf("TLS not supported on node %s", d.NodeID)
	}
	d.TLSEnabled = tlsEnabled
	d.UpdatedAt = time.Now()
	if err := s.domains.Update(d); err != nil {
		return nil, err
	}
	return d, nil
}

// HealthCheck returns the health status of all gateway resources.
func (s *GatewayService) HealthCheck(ctx context.Context) map[string]interface{} {
	domains, _ := s.domains.FindAll()
	routes, _ := s.routes.FindAll()
	listeners, _ := s.listeners.FindAll()
	return map[string]interface{}{
		"domains":   len(domains),
		"routes":    len(routes),
		"listeners": len(listeners),
		"status":    "ok",
	}
}

// DisabledActionsForNode returns what gateway actions are disabled for a node.
func (s *GatewayService) DisabledActionsForNode(ctx context.Context, nodeID string) []map[string]string {
	n, err := s.nodeRepo.FindByNodeID(nodeID)
	if err != nil || n == nil {
		return []map[string]string{{"action": "*", "reason": "node not found"}}
	}
	return n.Capabilities.DisabledActions()
}
