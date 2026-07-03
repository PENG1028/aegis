package topology

import (
	"fmt"

	"aegis/internal/endpoint"
	gatewaylink "aegis/internal/gateway_link"
	"aegis/internal/provider"
	"aegis/internal/route"
	"aegis/internal/safety"
	"aegis/internal/secrets"
	"aegis/internal/service"
)

// ============================================================================
// Template — a named topology pattern
// ============================================================================

// Template describes a known topology pattern (e.g., "single Caddy", "HAProxy + Caddy").
type Template interface {
	Name() string
	Description() string
	RequiredCapabilities() []provider.Capability
	BuildPlan(intents []RouteIntent, available []provider.ProviderState) (*TopologyPlan, error)
}

// ============================================================================
// Dependencies — data access for the Planner
// ============================================================================

// Dependencies provides the Planner with all the data it needs to resolve
// RouteIntents into fully-specified provider.Plan objects.
type Dependencies struct {
	RouteRepo        *route.Repository
	ServiceRepo      *service.Repository
	EndpointResolver *endpoint.Resolver
	GwLinkRepo       *gatewaylink.Repository
	SafetySvc        *safety.Service
	MasterKey        *secrets.MasterKey
}

// ============================================================================
// Planner — dimension 2: the single source of truth for traffic routing decisions
// ============================================================================

// Planner converts user route data + available middleware into per-provider
// configuration Plans. It replaces apply.Planner.
//
// Responsibilities:
//   - Collect active routes → RouteIntents
//   - Resolve endpoints → concrete upstream addresses
//   - Resolve gateway links → cross-machine targets + auth headers
//   - Check safety → warnings (non-blocking)
//   - Match topology templates → per-provider Plans
//   - Set ForwardTarget for transparent proxy
type Planner struct {
	templates []Template
	deps      Dependencies
}

// NewPlanner creates a Planner with standard templates and data dependencies.
func NewPlanner(templates []Template, deps Dependencies) *Planner {
	return &Planner{templates: templates, deps: deps}
}

// Plan collects all active routes, resolves endpoints, matches a topology
// template, and returns a complete TopologyPlan with per-provider Plans.
func (p *Planner) Plan(email string) (*TopologyPlan, error) {
	// Phase 1: Collect all active routes → RouteIntents
	intents, warnings, err := p.collectIntents()
	if err != nil {
		return nil, fmt.Errorf("collect intents: %w", err)
	}

	// Phase 2: Resolve each intent — endpoint → upstream, gateway link → auth
	resolved, resolveWarns := p.resolveIntents(intents)
	warnings = append(warnings, resolveWarns...)

	// Phase 3: Get available providers
	provReg := p.deps.RouteRepo // FIXME: need provider registry access
	_ = provReg

	available := p.gatherProviderStates()

	// Phase 4: Match topology templates
	var best *TopologyPlan
	var alternatives []Solution

	for _, tmpl := range p.templates {
		plan, err := tmpl.BuildPlan(resolved, available)
		if err != nil {
			level, explanation := EvaluateFallback(tmpl.RequiredCapabilities(), available)
			alternatives = append(alternatives, Solution{
				TemplateName: tmpl.Name(),
				Level:        level,
				Description:  tmpl.Description(),
				Warnings:     []string{explanation},
			})
			continue
		}

		if best == nil {
			best = plan
		}

		alternatives = append(alternatives, Solution{
			TemplateName: tmpl.Name(),
			Level:        0,
			Description:  tmpl.Description(),
			Providers:    plan.Primary.Providers,
		})
	}

	if best == nil {
		fallback := FallbackSolution(resolved, available)
		if len(alternatives) > 0 {
			fallback = alternatives[0]
		}
		return &TopologyPlan{
			Primary:      fallback,
			Alternatives: alternatives,
			Warnings:     append(warnings, "no template fully satisfies requirements"),
		}, fmt.Errorf("no template fully satisfies requirements: %s", fallback.Description)
	}

	best.Alternatives = alternatives
	best.Warnings = append(best.Warnings, warnings...)
	return best, nil
}

// ============================================================================
// Phase 1: Collect RouteIntents from active routes
// ============================================================================

func (p *Planner) collectIntents() ([]RouteIntent, []string, error) {
	routes, err := p.deps.RouteRepo.FindActive()
	if err != nil {
		return nil, nil, fmt.Errorf("find active routes: %w", err)
	}

	// Collect service IDs for batch loading
	svcIDSet := make(map[string]struct{})
	for _, rt := range routes {
		svcIDSet[rt.ServiceID] = struct{}{}
	}

	svcIDs := make([]string, 0, len(svcIDSet))
	for id := range svcIDSet {
		svcIDs = append(svcIDs, id)
	}

	svcMap, err := p.deps.ServiceRepo.FindByIDs(svcIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("batch load services: %w", err)
	}

	var intents []RouteIntent
	var warnings []string

	for _, rt := range routes {
		svc := svcMap[rt.ServiceID]
		if svc == nil {
			warnings = append(warnings, fmt.Sprintf("route %s points to non-existent service %s", rt.Domain, rt.ServiceID))
			continue
		}
		if svc.Status == "disabled" || svc.Status == "error" {
			warnings = append(warnings, fmt.Sprintf("route %s: service %s is %s", rt.Domain, svc.Name, svc.Status))
			continue
		}

		intents = append(intents, RouteIntent{
			Domain:             rt.Domain,
			Port:               443,
			Transport:          "tcp",
			TLSMode:            tlsModeFromRoute(rt),
			Path:               rt.PathPrefix,
			AppProtocol:        "http",
			StripPathPrefix:    rt.StripPrefix,
			MaintenanceEnabled: rt.MaintenanceEnabled,
			MaintenanceMessage: rt.MaintenanceMessage,
			gatewayLinkID:      rt.GatewayLinkID,
			serviceID:          rt.ServiceID,
		})
	}

	return intents, warnings, nil
}

// ============================================================================
// Phase 2: Resolve intents — endpoint addresses, gateway links, safety
// ============================================================================

func (p *Planner) resolveIntents(intents []RouteIntent) ([]RouteIntent, []string) {
	var resolved []RouteIntent
	var warnings []string

	for _, ri := range intents {
		// Resolve endpoint → find best upstream address
		result := p.deps.EndpointResolver.ResolveWithResult(nil, ri.serviceID)
		if result.Endpoint == nil {
			warnings = append(warnings, fmt.Sprintf("%s: no available endpoint", ri.Domain))
			continue
		}

		upstream := result.Endpoint.Address
		// Convert Unix sockets to Caddy-compatible format
		if epAddr := result.Endpoint.Addr(); epAddr.IsUnix() {
			upstream = epAddr.CaddyTarget()
		}

		ri.Upstream = upstream

		// Gateway Link resolution
		if ri.gatewayLinkID != "" && p.deps.GwLinkRepo != nil {
			gw, err := p.deps.GwLinkRepo.FindByID(ri.gatewayLinkID)
			if err == nil && gw != nil && gw.Status == gatewaylink.StatusActive {
				targetHost := gw.ResolveHost()
				ri.Upstream = fmt.Sprintf("http://%s:%d", targetHost, gw.Port)

				if ri.ExtraHeaders == nil {
					ri.ExtraHeaders = make(map[string]string)
				}
				ri.ExtraHeaders["X-Aegis-Gateway-Link"] = gw.ID
				ri.ExtraHeaders["Host"] = ri.Domain

				if gw.HasSecret() && p.deps.MasterKey != nil {
					secret, err := gw.GetRawSecret(p.deps.MasterKey)
					if err == nil && secret != "" {
						ri.ExtraHeaders["X-Aegis-Gateway-Token"] = secret
					}
				}
			}
		}

		// Safety checks
		if p.deps.SafetySvc != nil {
			risks := p.deps.SafetySvc.GetPlannerWarnings(ri.Domain, result.Endpoint.Address, ri.gatewayLinkID)
			for _, risk := range risks {
				warnings = append(warnings, fmt.Sprintf("SAFETY_%s: %s — %s", risk.Code, ri.Domain, risk.Message))
			}
		}

		// Endpoint resolution attempt warnings
		for _, att := range result.Attempts {
			if !att.Success {
				warnings = append(warnings, fmt.Sprintf("%s: %s %s unreachable: %s", ri.Domain, att.Type, att.Address, att.Message))
			}
		}

		resolved = append(resolved, ri)
	}

	return resolved, warnings
}

// ============================================================================
// Provider state gathering (for capability matching)
// ============================================================================

// gatherProviderStates collects ProviderState from all registered providers.
// This is a placeholder — in production, the caller passes available providers
// through the Plan method's second argument, or we accept them separately.
func (p *Planner) gatherProviderStates() []provider.ProviderState {
	// Provider states are passed separately via PlanWithProviders
	return nil
}

// PlanWithProviders is the full version that accepts pre-discovered provider states.
func (p *Planner) PlanWithProviders(email string, available []provider.ProviderState) (*TopologyPlan, error) {
	// Phase 1-2: Collect + resolve intents
	intents, warnings, err := p.collectIntents()
	if err != nil {
		return nil, err
	}
	resolved, resolveWarns := p.resolveIntents(intents)
	warnings = append(warnings, resolveWarns...)

	healthy := healthyProviders(available)

	// Phase 3: Match templates
	var best *TopologyPlan
	var alternatives []Solution

	for _, tmpl := range p.templates {
		plan, err := tmpl.BuildPlan(resolved, healthy)
		if err != nil {
			level, explanation := EvaluateFallback(tmpl.RequiredCapabilities(), healthy)
			alternatives = append(alternatives, Solution{
				TemplateName: tmpl.Name(), Level: level,
				Description: tmpl.Description(), Warnings: []string{explanation},
			})
			continue
		}
		if best == nil {
			best = plan
		}
		alternatives = append(alternatives, Solution{
			TemplateName: tmpl.Name(), Level: 0,
			Description: tmpl.Description(), Providers: plan.Primary.Providers,
		})
	}

	if best == nil {
		fallback := FallbackSolution(resolved, healthy)
		if len(alternatives) > 0 {
			fallback = alternatives[0]
		}
		return &TopologyPlan{
			Primary: fallback, Alternatives: alternatives,
			Warnings: append(warnings, "no template fully satisfies requirements"),
		}, fmt.Errorf("no template fully satisfies: %s", fallback.Description)
	}

	best.Alternatives = alternatives
	best.Warnings = append(best.Warnings, warnings...)

	// ForwardTarget for transparent proxy
	for _, ri := range resolved {
		if ri.Transport == "tcp" && ri.AppProtocol == "raw" {
			// Cross-node transparent forwarding needed
			best.ForwardTarget = findForwardTarget(healthy)
			break
		}
	}

	return best, nil
}

// ============================================================================
// Helpers
// ============================================================================

func tlsModeFromRoute(rt route.Route) string {
	if rt.TLSEnabled {
		return "terminate"
	}
	return "none"
}

// healthyProviders filters to only running providers.
func healthyProviders(all []provider.ProviderState) []provider.ProviderState {
	var out []provider.ProviderState
	for _, s := range all {
		if s.Healthy() {
			out = append(out, s)
		}
	}
	return out
}

// missingCapabilities returns capabilities no provider satisfies.
func missingCapabilities(required []provider.Capability, available []provider.ProviderState) []provider.Capability {
	var missing []provider.Capability
	for _, cap := range required {
		found := false
		for _, p := range available {
			if p.HasCapability(cap) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, cap)
		}
	}
	return missing
}

// findForwardTarget finds the HTTP router to forward transparent proxy traffic to.
func findForwardTarget(available []provider.ProviderState) *provider.ForwardTarget {
	for _, p := range available {
		if p.HasCapability(provider.CapRouteHost) && p.HasCapability(provider.CapUpstreamTCP) {
			for _, port := range p.Ports {
				if port.Purpose == "http" || port.Purpose == "internal_https" {
					return &provider.ForwardTarget{Host: "127.0.0.1", Port: port.Port}
				}
			}
			return &provider.ForwardTarget{Host: "127.0.0.1", Port: 80}
		}
	}
	return nil
}

// providerIDs extracts IDs from ProviderStates.
func providerIDs(states []provider.ProviderState) []string {
	ids := make([]string, len(states))
	for i, s := range states {
		ids[i] = s.ID
	}
	return ids
}
