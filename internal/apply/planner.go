package apply

import (
	"aegis/internal/endpoint"
	"aegis/internal/manageddomain"
	"aegis/internal/proxy"
	"aegis/internal/route"
	"aegis/internal/service"
	"fmt"
)

// Planner builds a GatewayConfig from the current state of routes and services.
type Planner struct {
	routeRepo          *route.Repository
	mdRepo             *manageddomain.Repository
	serviceRepo        *service.Repository
	endpointResolver   *endpoint.Resolver
}

// NewPlanner creates a new apply planner.
func NewPlanner(
	routeRepo *route.Repository,
	mdRepo *manageddomain.Repository,
	serviceRepo *service.Repository,
	endpointResolver *endpoint.Resolver,
) *Planner {
	return &Planner{
		routeRepo:        routeRepo,
		mdRepo:           mdRepo,
		serviceRepo:      serviceRepo,
		endpointResolver: endpointResolver,
	}
}

// Plan builds a GatewayConfig from all active routes and managed domains.
// It resolves endpoints for each service and returns warnings for issues.
func (p *Planner) Plan(email string) (*proxy.GatewayConfig, []string, error) {
	var routeConfigs []proxy.RouteConfig
	var warnings []string

	// Process active routes
	routes, err := p.routeRepo.FindActive()
	if err != nil {
		return nil, nil, fmt.Errorf("find active routes: %w", err)
	}

	for _, rt := range routes {
		rc, warn, err := p.resolveRouteConfig(rt.Domain, rt.ServiceID, rt.TLSEnabled, rt.MaintenanceEnabled, rt.MaintenanceMessage)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve route %s: %w", rt.Domain, err)
		}
		if rc == nil {
			warnings = append(warnings, warn...)
			continue
		}
		routeConfigs = append(routeConfigs, *rc)
		if len(warn) > 0 {
			warnings = append(warnings, warn...)
		}
	}

	// Process active managed domains
	mdDomains, err := p.mdRepo.FindActive()
	if err != nil {
		return nil, nil, fmt.Errorf("find active managed domains: %w", err)
	}

	for _, md := range mdDomains {
		rc, warn, err := p.resolveRouteConfig(md.Domain, md.ServiceID, true, false, "")
		if err != nil {
			return nil, nil, fmt.Errorf("resolve managed domain %s: %w", md.Domain, err)
		}
		if rc == nil {
			warnings = append(warnings, warn...)
			continue
		}
		routeConfigs = append(routeConfigs, *rc)
		if len(warn) > 0 {
			warnings = append(warnings, warn...)
		}
	}

	return &proxy.GatewayConfig{
		Routes: routeConfigs,
		Email:  email,
	}, warnings, nil
}

// resolveRouteConfig resolves a single route/domain to a RouteConfig.
// Returns nil if the route should be skipped (disabled service, no endpoint, etc.)
func (p *Planner) resolveRouteConfig(
	domain string,
	serviceID string,
	tlsEnabled bool,
	maintenanceEnabled bool,
	maintenanceMessage string,
) (*proxy.RouteConfig, []string, error) {
	var warnings []string

	svc, err := p.serviceRepo.FindByID(serviceID)
	if err != nil {
		return nil, nil, fmt.Errorf("find service %s: %w", serviceID, err)
	}
	if svc == nil {
		warnings = append(warnings,
			fmt.Sprintf("warning: %s points to non-existent service %s", domain, serviceID))
		return nil, warnings, nil
	}

	if svc.Status == "disabled" || svc.Status == "error" {
		warnings = append(warnings,
			fmt.Sprintf("warning: %s points to %s service %s (status: %s)",
				domain, svc.Status, svc.Name, svc.Status))
		return nil, warnings, nil
	}

	// Resolve endpoint
	ep, err := p.endpointResolver.Resolve(nil, svc.ID)
	if err != nil {
		warnings = append(warnings,
			fmt.Sprintf("warning: %s -> service %s: no available endpoint (%v)",
				domain, svc.Name, err))
		return nil, warnings, nil
	}

	return &proxy.RouteConfig{
		Domain:             domain,
		Kind:               "reverse_proxy",
		UpstreamURL:        ep.Address,
		TLSEnabled:          tlsEnabled,
		MaintenanceEnabled:  maintenanceEnabled,
		MaintenanceMessage:  maintenanceMessage,
		Options: proxy.ProxyOptions{
			EnableGzip: true,
		},
	}, warnings, nil
}
