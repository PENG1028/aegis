package topology

import (
	"fmt"

	"aegis/internal/certstore"
	"aegis/internal/endpoint"
	gatewaylink "aegis/internal/gateway"
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
	BuildPlan(intents []RouteIntent, available []provider.ProviderState, mode provider.RuntimeMode) (*TopologyPlan, error)
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
	GwLinkRepo       *gatewaylink.LinkRepository
	SafetySvc        *safety.Service
	MasterKey        *secrets.MasterKey
	CertStore        *certstore.Service // v1.9C: resolve CertID → file paths
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

// PlanWithProviders is the full version that accepts pre-discovered provider states.

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

		// v1.8L-22: derive fields from composition registry
		compDef := rt.CompDef()
		if compDef == nil {
			warnings = append(warnings, fmt.Sprintf("route %s: unknown composition %q, skipping", rt.Domain, rt.Composition))
			continue
		}

		ri := RouteIntent{
			Domain:      rt.Domain,
			Port:        compDef.Port,
			Transport:   compDef.Transport,
			TLSMode:     compDef.TLSMode,
			Path:        rt.PathPrefix,
			AppProtocol: compDef.AppProtocol,
			Composition: rt.Composition,
			StripPathPrefix:    rt.StripPrefix,
			MaintenanceEnabled: rt.MaintenanceEnabled,
			MaintenanceMessage: rt.MaintenanceMessage,
			gatewayLinkID:      rt.GatewayLinkID,
			serviceID:          rt.ServiceID,
			CertID:             certIDStr(rt.CertID),
		}
		// Resolve cert paths from certstore for custom certs
		if rt.CertID != nil && *rt.CertID != "" && p.deps.CertStore != nil {
			if cp, kp, err := p.deps.CertStore.GetPaths(*rt.CertID); err == nil {
				ri.CertPath = cp
				ri.KeyPath = kp
			}
		}
		intents = append(intents, ri)
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
			if err == nil && gw != nil && gw.Status == gatewaylink.LinkStatusActive {
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

func (p *Planner) PlanWithProviders(email string, available []provider.ProviderState) (*TopologyPlan, error) {
	// Phase 1-2: Collect + resolve intents
	intents, warnings, err := p.collectIntents()
	if err != nil {
		return nil, err
	}
	resolved, resolveWarns := p.resolveIntents(intents)
	warnings = append(warnings, resolveWarns...)

	healthy := healthyProviders(available)

	// Detect the active runtime mode from available providers.
	// This replaces the old shell-based CurrentPortPolicyMode() — now the Planner
	// and API use the same detection function, eliminating divergence.
	mode := provider.DetectRuntimeMode(healthy)

	// Phase 3: Match templates
	var best *TopologyPlan
	var alternatives []Solution

	for _, tmpl := range p.templates {
		plan, err := tmpl.BuildPlan(resolved, healthy, mode)
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
			best.ForwardTarget = findForwardTarget(healthy, mode)
			break
		}
	}

	return best, nil
}

// ============================================================================
// Helpers
// ============================================================================

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

// findForwardTarget finds forward targets for transparent proxy iptables interception.
// v1.8L-22: auto-discovers from composition registry — same logic as transparent status handler.
// When new compositions are added with IsTransparentForwardTarget()=true, this auto-picks them up.
func findForwardTarget(available []provider.ProviderState, mode provider.RuntimeMode) *provider.ForwardTarget {
	for _, comp := range provider.AllCompositions() {
		if !comp.IsTransparentForwardTarget() {
			continue
		}
		for _, p := range available {
			hasAll := true
			for _, cap := range comp.Requirements() {
				if !p.HasCapability(cap) {
					hasAll = false
					break
				}
			}
			if !hasAll {
				continue
			}
			listeners := mode.ListenerSpecsFor(p.ID)
			for _, l := range listeners {
				if l.Purpose == "http" || l.Purpose == "https" || l.Purpose == "internal_https" {
					return &provider.ForwardTarget{Host: "127.0.0.1", Port: l.Port}
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

func certIDStr(certID *string) string {
	if certID == nil {
		return ""
	}
	return *certID
}
