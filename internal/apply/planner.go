package apply

import (
	"aegis/internal/endpoint"
	"aegis/internal/manageddomain"
	"aegis/internal/proxy"
	"aegis/internal/route"
	"aegis/internal/service"
	"fmt"
)

// Planner builds a GatewayConfig and ApplyPlan from routes and managed domains.
type Planner struct {
	routeRepo        *route.Repository
	mdRepo           *manageddomain.Repository
	serviceRepo      *service.Repository
	endpointResolver *endpoint.Resolver
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

// Plan builds a full ApplyPlan from routes and managed domains.
func (p *Planner) Plan(email string) (*ApplyPlan, error) {
	plan := &ApplyPlan{
		Warnings: []ApplyWarning{},
	}

	var routeConfigs []proxy.RouteConfig

	// Process active routes
	routes, err := p.routeRepo.FindActive()
	if err != nil {
		return nil, fmt.Errorf("find active routes: %w", err)
	}

	for _, rt := range routes {
		rc, warns := p.resolveRouteConfig(rt.Domain, rt.ServiceID, rt.TLSEnabled, rt.MaintenanceEnabled, rt.MaintenanceMessage)
		if rc == nil {
			plan.SkippedCount++
		} else {
			routeConfigs = append(routeConfigs, *rc)
			plan.RouteCount++
		}
		plan.Warnings = append(plan.Warnings, warns...)
	}

	// Process active managed domains
	mdDomains, err := p.mdRepo.FindActive()
	if err != nil {
		return nil, fmt.Errorf("find active managed domains: %w", err)
	}

	for _, md := range mdDomains {
		rc, warns := p.resolveRouteConfig(md.Domain, md.ServiceID, true, false, "")
		if rc == nil {
			plan.SkippedCount++
		} else {
			routeConfigs = append(routeConfigs, *rc)
			plan.ManagedDomainCount++
		}
		plan.Warnings = append(plan.Warnings, warns...)
	}

	plan.Routes = routeConfigs
	return plan, nil
}

// resolveRouteConfig resolves a single domain to a RouteConfig with warnings.
func (p *Planner) resolveRouteConfig(
	domain string, serviceID string, tlsEnabled bool,
	maintenanceEnabled bool, maintenanceMessage string,
) (*proxy.RouteConfig, []ApplyWarning) {
	var warnings []ApplyWarning

	svc, err := p.serviceRepo.FindByID(serviceID)
	if err != nil {
		warnings = append(warnings, ApplyWarning{
			Code: WarningRouteSkipped, Severity: "critical",
			Message: fmt.Sprintf("service lookup failed for %s: %v", domain, err),
			Target:  serviceID,
		})
		return nil, warnings
	}
	if svc == nil {
		warnings = append(warnings, ApplyWarning{
			Code: WarningRouteSkipped, Severity: "critical",
			Message: fmt.Sprintf("%s points to non-existent service %s", domain, serviceID),
			Target:  serviceID,
		})
		return nil, warnings
	}

	if svc.Status == "disabled" || svc.Status == "error" {
		warnings = append(warnings, ApplyWarning{
			Code: WarningServiceDisabled, Severity: "warning",
			Message: fmt.Sprintf("%s points to %s service %s", domain, svc.Status, svc.Name),
			Target:  svc.ID,
		})
		return nil, warnings
	}

	// Resolve endpoint
	result := p.endpointResolver.ResolveWithResult(nil, svc.ID)
	if result.Endpoint == nil {
		warnings = append(warnings, ApplyWarning{
			Code: WarningNoAvailableEndpoint, Severity: "critical",
			Message: fmt.Sprintf("%s -> service %s: no available endpoint", domain, svc.Name),
			Target:  svc.ID,
		})
		return nil, warnings
	}

	// Check if the resolved endpoint had failed attempts
	for _, att := range result.Attempts {
		if !att.Success {
			warnings = append(warnings, ApplyWarning{
				Code: WarningEndpointUnreachable, Severity: "warning",
				Message: fmt.Sprintf("%s: %s %s unreachable: %s", domain, att.Type, att.Address, att.Message),
				Target:  att.EndpointID,
			})
		}
	}

	return &proxy.RouteConfig{
		Domain:             domain,
		Kind:               "reverse_proxy",
		UpstreamURL:        result.Endpoint.Address,
		TLSEnabled:          tlsEnabled,
		MaintenanceEnabled:  maintenanceEnabled,
		MaintenanceMessage:  maintenanceMessage,
		Options: proxy.ProxyOptions{
			EnableGzip: true,
		},
	}, warnings
}
