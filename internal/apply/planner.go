package apply

import (
	"fmt"

	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	gatewaylink "aegis/internal/gateway_link"
	"aegis/internal/manageddomain"
	"aegis/internal/proxy"
	"aegis/internal/route"
	"aegis/internal/safety"
	"aegis/internal/secrets"
	"aegis/internal/service"
)

// Planner builds a GatewayConfig and ApplyPlan from routes, managed domains, and HTTP exposures.
type Planner struct {
	routeRepo        *route.Repository
	mdRepo           *manageddomain.Repository
	exposureRepo     *exposure.Repository
	serviceRepo      *service.Repository
	endpointResolver *endpoint.Resolver
	gwLinkRepo       *gatewaylink.Repository // v1.7AB
	safetySvc        *safety.Service         // v1.8A
	masterKey        *secrets.MasterKey      // v1.8B-5: for decrypting GatewayLink secrets
}

// NewPlanner creates a new apply planner.
func NewPlanner(
	routeRepo *route.Repository,
	mdRepo *manageddomain.Repository,
	exposureRepo *exposure.Repository,
	serviceRepo *service.Repository,
	endpointResolver *endpoint.Resolver,
	gwLinkRepo *gatewaylink.Repository, // v1.7AB
	safetySvc *safety.Service,          // v1.8A
	masterKey *secrets.MasterKey,       // v1.8B-5
) *Planner {
	return &Planner{
		routeRepo:        routeRepo,
		mdRepo:           mdRepo,
		exposureRepo:     exposureRepo,
		serviceRepo:      serviceRepo,
		endpointResolver: endpointResolver,
		gwLinkRepo:       gwLinkRepo,
		safetySvc:        safetySvc,
		masterKey:        masterKey,
	}
}

// Plan builds a full ApplyPlan from routes, managed domains, and HTTP exposures.
func (p *Planner) Plan(email string) (*ApplyPlan, error) {
	plan := &ApplyPlan{
		Warnings: []ApplyWarning{},
	}

	var routeConfigs []proxy.RouteConfig

	// Phase 0: Collect all service IDs upfront to avoid N+1 queries
	serviceIDSet := make(map[string]struct{})

	routes, err := p.routeRepo.FindActive()
	if err != nil {
		return nil, fmt.Errorf("find active routes: %w", err)
	}
	for _, rt := range routes {
		serviceIDSet[rt.ServiceID] = struct{}{}
	}

	mdDomains, err := p.mdRepo.FindActive()
	if err != nil {
		return nil, fmt.Errorf("find active managed domains: %w", err)
	}
	for _, md := range mdDomains {
		serviceIDSet[md.ServiceID] = struct{}{}
	}

	httpExposures, err := p.exposureRepo.FindActiveHTTP()
	if err != nil {
		return nil, fmt.Errorf("find active http exposures: %w", err)
	}
	for _, exp := range httpExposures {
		serviceIDSet[exp.ServiceID] = struct{}{}
	}

	// Batch-load all services in a single query
	allIDs := make([]string, 0, len(serviceIDSet))
	for id := range serviceIDSet {
		allIDs = append(allIDs, id)
	}
	serviceMap, err := p.serviceRepo.FindByIDs(allIDs)
	if err != nil {
		return nil, fmt.Errorf("batch load services: %w", err)
	}

	// Phase 1: Process active routes (internal, admin-managed)
	for _, rt := range routes {
		rc, warns := p.resolveRouteConfigWithService(rt.Domain, rt.PathPrefix, rt.StripPrefix, rt.TLSEnabled, rt.MaintenanceEnabled, rt.MaintenanceMessage, rt.GatewayLinkID, rt.ServiceID, serviceMap)
		if rc == nil {
			plan.SkippedCount++
		} else {
			routeConfigs = append(routeConfigs, *rc)
			plan.RouteCount++
		}
		plan.Warnings = append(plan.Warnings, warns...)
	}

	// Phase 2: Process active managed domains (verified external domains)
	for _, md := range mdDomains {
		rc, warns := p.resolveRouteConfigWithService(md.Domain, "", false, true, false, "", "", md.ServiceID, serviceMap)
		if rc == nil {
			plan.SkippedCount++
		} else {
			routeConfigs = append(routeConfigs, *rc)
			plan.ManagedDomainCount++
		}
		plan.Warnings = append(plan.Warnings, warns...)
	}

	// Phase 3: Process active HTTP exposures (generate config from exposure host/path)
	for _, exp := range httpExposures {
		domain := exp.Host
		if exp.Port > 0 && exp.Port != 80 && exp.Port != 443 {
			domain = fmt.Sprintf("%s:%d", exp.Host, exp.Port)
		}
		rc, warns := p.resolveRouteConfigWithService(domain, "", false, true, false, "", "", exp.ServiceID, serviceMap)
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

// resolveRouteConfigWithService is like resolveRouteConfig but takes a pre-loaded service map.
func (p *Planner) resolveRouteConfigWithService(
	domain string, pathPrefix string, stripPrefix bool, tlsEnabled bool,
	maintenanceEnabled bool, maintenanceMessage string, gatewayLinkID string,
	serviceID string, serviceMap map[string]*service.Service,
) (*proxy.RouteConfig, []ApplyWarning) {
	var warnings []ApplyWarning

	svc := serviceMap[serviceID]
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

	for _, att := range result.Attempts {
		if !att.Success {
			warnings = append(warnings, ApplyWarning{
				Code: WarningEndpointUnreachable, Severity: "warning",
				Message: fmt.Sprintf("%s: %s %s unreachable: %s", domain, att.Type, att.Address, att.Message),
				Target:  att.EndpointID,
			})
		}
	}

	rc := &proxy.RouteConfig{
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
	}

		// v1.7AB: Inject Gateway Link headers if route is linked to a downstream gateway
		if gatewayLinkID != "" && p.gwLinkRepo != nil {
			gw, err := p.gwLinkRepo.FindByID(gatewayLinkID)
			if err == nil && gw != nil && gw.Status == gatewaylink.StatusActive {
				rc.Options.ExtraHeaders = map[string]string{
					"X-Aegis-Gateway-Link": gw.ID,
				}
				// v1.8B-5: Get raw secret (decrypts encrypted secret, falls back to HMAC hash)
				if gw.HasSecret() {
					secret, err := gw.GetRawSecret(p.masterKey)
					if err == nil && secret != "" {
						rc.Options.ExtraHeaders["X-Aegis-Gateway-Token"] = secret
					}
				}
			}
		}
	// v1.8A: Safety warnings from Planner (detection only, does not block apply)
	if p.safetySvc != nil {
		targetHost := result.Endpoint.Address
		safetyRisks := p.safetySvc.GetPlannerWarnings(domain, targetHost, gatewayLinkID)
		for _, risk := range safetyRisks {
			warnings = append(warnings, ApplyWarning{
				Code:     "SAFETY_" + risk.Code,
				Severity: risk.Severity,
				Message:  risk.Message,
				Target:   domain,
			})
		}
	}

	return rc, warnings
}
