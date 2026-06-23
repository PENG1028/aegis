package apply

import (
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/manageddomain"
	"aegis/internal/proxy"
	"aegis/internal/route"
	"aegis/internal/service"
	"fmt"
)

// Planner builds a GatewayConfig and ApplyPlan from routes, managed domains, and HTTP exposures.
type Planner struct {
	routeRepo        *route.Repository
	mdRepo           *manageddomain.Repository
	exposureRepo     *exposure.Repository
	serviceRepo      *service.Repository
	endpointResolver *endpoint.Resolver
}

// NewPlanner creates a new apply planner.
func NewPlanner(
	routeRepo *route.Repository,
	mdRepo *manageddomain.Repository,
	exposureRepo *exposure.Repository,
	serviceRepo *service.Repository,
	endpointResolver *endpoint.Resolver,
) *Planner {
	return &Planner{
		routeRepo:        routeRepo,
		mdRepo:           mdRepo,
		exposureRepo:     exposureRepo,
		serviceRepo:      serviceRepo,
		endpointResolver: endpointResolver,
	}
}

// Plan builds a full ApplyPlan from routes, managed domains, and HTTP exposures.
func (p *Planner) Plan(email string) (*ApplyPlan, error) {
	plan := &ApplyPlan{
		Warnings: []ApplyWarning{},
	}

	var routeConfigs []proxy.RouteConfig

	// Phase 1: Process active routes (internal, admin-managed)
	routes, err := p.routeRepo.FindActive()
	if err != nil {
		return nil, fmt.Errorf("find active routes: %w", err)
	}

	for _, rt := range routes {
		rc, warns := p.resolveRouteConfig(rt.Domain, rt.PathPrefix, rt.StripPrefix, rt.ServiceID, rt.TLSEnabled, rt.MaintenanceEnabled, rt.MaintenanceMessage)
		if rc == nil {
			plan.SkippedCount++
		} else {
			routeConfigs = append(routeConfigs, *rc)
			plan.RouteCount++
		}
		plan.Warnings = append(plan.Warnings, warns...)
	}

	// Phase 2: Process active managed domains (verified external domains)
	mdDomains, err := p.mdRepo.FindActive()
	if err != nil {
		return nil, fmt.Errorf("find active managed domains: %w", err)
	}

	for _, md := range mdDomains {
		rc, warns := p.resolveRouteConfig(md.Domain, "", false, md.ServiceID, true, false, "")
		if rc == nil {
			plan.SkippedCount++
		} else {
			routeConfigs = append(routeConfigs, *rc)
			plan.ManagedDomainCount++
		}
		plan.Warnings = append(plan.Warnings, warns...)
	}

	// Phase 3: Process active HTTP exposures (generate config from exposure host/path)
	httpExposures, err := p.exposureRepo.FindActiveHTTP()
	if err != nil {
		return nil, fmt.Errorf("find active http exposures: %w", err)
	}

	for _, exp := range httpExposures {
		domain := exp.Host
		if exp.Port > 0 && exp.Port != 80 && exp.Port != 443 {
			domain = fmt.Sprintf("%s:%d", exp.Host, exp.Port)
		}
		rc, warns := p.resolveRouteConfig(domain, "", false, exp.ServiceID, true, false, "")
		if rc == nil {
			plan.SkippedCount++
		} else {
			routeConfigs = append(routeConfigs, *rc)
		}
		plan.Warnings = append(plan.Warnings, warns...)
	}

	plan.Routes = routeConfigs
	return plan, nil
}

// resolveRouteConfig resolves a single domain to a RouteConfig with warnings.
func (p *Planner) resolveRouteConfig(
	domain string, pathPrefix string, stripPrefix bool, serviceID string, tlsEnabled bool,
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
		PathPrefix:         pathPrefix,
		Kind:               "reverse_proxy",
		UpstreamURL:        result.Endpoint.Address,
		TLSEnabled:          tlsEnabled,
		MaintenanceEnabled:  maintenanceEnabled,
		MaintenanceMessage:  maintenanceMessage,
		Options: proxy.ProxyOptions{
			EnableGzip:  true,
			StripPrefix: stripPrefix,
		},
	}, warnings
}
