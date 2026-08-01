package topology

import (
	"fmt"
	"os"
	"time"

	"aegis/internal/certstore"
	"aegis/internal/endpoint"
	"aegis/internal/flowbridge"
	gatewaylink "aegis/internal/gateway"
	"aegis/internal/hostdep/provider"
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
	// FlowBridgeRepo resolves flowbridge-bound routes to instance addresses.
	// nil disables flowbridge routing (e.g. tests that do not exercise it).
	FlowBridgeRepo *flowbridge.Repository
	// ControlPort is the aegis API/control port (parsed from cfg.Server.Addr).
	// When > 0, the Planner exposes this node's control plane through the ingress
	// HTTP provider so cross-node distnode traffic traverses the 80/443 edge.
	// Zero disables the injection (e.g. in tests). See docs/distnode-onboarding-fix.md.
	ControlPort int
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
			Domain:             rt.Domain,
			Port:               compDef.Port,
			Transport:          compDef.Transport,
			TLSMode:            compDef.TLSMode,
			Path:               rt.PathPrefix,
			AppProtocol:        compDef.AppProtocol,
			Composition:        rt.Composition,
			StripPathPrefix:    rt.StripPrefix,
			MaintenanceEnabled: rt.MaintenanceEnabled,
			MaintenanceMessage: rt.MaintenanceMessage,
			TLSBindingMode:     rt.TLSBindingMode,
			TLSExecutor:        rt.TLSProvider,
			gatewayLinkID:      rt.GatewayLinkID,
			serviceID:          rt.ServiceID,
			flowbridgeID:       flowbridgeIDStr(rt.FlowBridgeID),
			CertID:             certIDStr(rt.CertID),
		}
		// WHY: a certificate binding is an explicit security choice. Missing local
		// material must block Apply instead of silently changing it to provider ACME.
		if rt.CertID != nil && *rt.CertID != "" && p.deps.CertStore != nil {
			cert, err := p.deps.CertStore.Get(*rt.CertID)
			if err != nil {
				return nil, warnings, fmt.Errorf("route %s: certificate %s unresolved: %w", rt.Domain, *rt.CertID, err)
			}
			if cert == nil {
				return nil, warnings, fmt.Errorf("route %s: certificate %s not found", rt.Domain, *rt.CertID)
			}
			if cert.Source == certstore.SourceGatewayAuto {
				return nil, warnings, fmt.Errorf("route %s: certificate %s is provider-managed and cannot be loaded as a PEM asset", rt.Domain, *rt.CertID)
			}
			if !certstore.ValidAt(cert, time.Now()) {
				warnings = append(warnings, fmt.Sprintf("route %s: certificate %s is not currently valid; renewal or replacement is required", rt.Domain, *rt.CertID))
			}
			if !certstore.CoversDomain(cert, rt.Domain) {
				return nil, warnings, fmt.Errorf("route %s: certificate %s does not cover the route domain", rt.Domain, *rt.CertID)
			}
			if !fileExists(cert.CertPath) || !fileExists(cert.KeyPath) {
				return nil, warnings, fmt.Errorf("route %s: certificate %s files are missing", rt.Domain, *rt.CertID)
			}
			ri.CertPath = cert.CertPath
			ri.KeyPath = cert.KeyPath
		}
		intents = append(intents, ri)
	}

	return intents, warnings, nil
}

// fileExists reports whether path names an existing regular file.
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ============================================================================
// Phase 2: Resolve intents — endpoint addresses, gateway links, safety
// ============================================================================

func (p *Planner) resolveIntents(intents []RouteIntent) ([]RouteIntent, []string) {
	var resolved []RouteIntent
	var warnings []string

	for _, ri := range intents {
		// FlowBridge-bound routes resolve the instance directly: traffic follows
		// the instance's machine_ip:data_plane_port, never the service endpoint.
		// A missing or disabled instance skips the route with a warning — the
		// same degradation policy as a disabled service. No fallback.
		if ri.flowbridgeID != "" {
			if p.deps.FlowBridgeRepo == nil {
				warnings = append(warnings, fmt.Sprintf("%s: flowbridge resolution unavailable", ri.Domain))
				continue
			}
			inst, err := p.deps.FlowBridgeRepo.FindByID(ri.flowbridgeID)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: flowbridge instance lookup failed: %v", ri.Domain, err))
				continue
			}
			if inst == nil {
				warnings = append(warnings, fmt.Sprintf("%s: flowbridge instance %s not found", ri.Domain, ri.flowbridgeID))
				continue
			}
			if !inst.Enabled {
				warnings = append(warnings, fmt.Sprintf("%s: flowbridge instance %s is disabled", ri.Domain, inst.Name))
				continue
			}
			ri.Upstream = fmt.Sprintf("http://%s:%d", inst.MachineIP, inst.DataPlanePort)
			ri.ExtraHeaders = ensureHeader(ri.ExtraHeaders, "Host", ri.Domain)
			resolved = append(resolved, ri)
			continue
		}

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
	healthy := healthyProviders(available)
	return p.planWithMode(email, healthy, provider.DetectRuntimeMode(healthy), false)
}

// PlanForMode plans against an explicit target mode. Stopped but installed
// target providers are valid here because Workflow stages config before start.
func (p *Planner) PlanForMode(email string, available []provider.ProviderState, mode provider.RuntimeMode) (*TopologyPlan, error) {
	wanted := make(map[string]bool, len(mode.Providers))
	for _, id := range mode.ProviderIDs() {
		wanted[id] = true
	}
	var candidates []provider.ProviderState
	for _, state := range available {
		if wanted[state.ID] && state.Installed {
			candidates = append(candidates, state)
		}
	}
	if len(candidates) != len(wanted) {
		return nil, fmt.Errorf("target mode %s requires all providers to be installed", mode.ID)
	}
	return p.planWithMode(email, candidates, mode, true)
}

func (p *Planner) planWithMode(_ string, available []provider.ProviderState, mode provider.RuntimeMode, allowMigration bool) (*TopologyPlan, error) {
	// Phase 1-2: Collect + resolve intents
	intents, warnings, err := p.collectIntents()
	if err != nil {
		return nil, err
	}
	resolved, resolveWarns := p.resolveIntents(intents)
	warnings = append(warnings, resolveWarns...)
	if err := validateExecutionAffinity(resolved, available, mode, allowMigration); err != nil {
		return nil, err
	}

	// Phase 3: Match templates
	var best *TopologyPlan
	var alternatives []Solution

	for _, tmpl := range p.templates {
		plan, err := tmpl.BuildPlan(resolved, available, mode)
		if err != nil {
			level, explanation := EvaluateFallback(tmpl.RequiredCapabilities(), available)
			alternatives = append(alternatives, Solution{
				TemplateName: tmpl.Name(), Level: level,
				Description: tmpl.Description(), Warnings: []string{explanation},
			})
			continue
		}
		if !planMatchesMode(plan, mode) {
			alternatives = append(alternatives, Solution{
				TemplateName: tmpl.Name(), Level: 1,
				Description: tmpl.Description(), Warnings: []string{"template does not use the providers assigned by the runtime mode"},
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
		fallback := FallbackSolution(resolved, available)
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

	// v1.9B: expose this node's control plane (/api/*) through the ingress HTTP
	// provider so cross-node distnode health checks + RPC traverse the 80/443
	// edge instead of the localhost-only API port. Capability-selected — never
	// keyed on a provider name. See docs/distnode-onboarding-fix.md.
	p.injectControlPlaneRoute(best, available)

	// ForwardTarget for transparent proxy
	for _, ri := range resolved {
		if ri.Transport == "tcp" && ri.AppProtocol == "raw" {
			// Cross-node transparent forwarding needed
			best.ForwardTarget = findForwardTarget(available, mode)
			break
		}
	}

	return best, nil
}

func validateExecutionAffinity(intents []RouteIntent, available []provider.ProviderState, mode provider.RuntimeMode, allowMigration bool) error {
	for _, intent := range intents {
		if intent.TLSBindingMode != route.TLSBindingProviderAuto {
			continue
		}
		if !allowMigration && intent.TLSExecutor != "" {
			if providerAvailableForCapability(intent.TLSExecutor, provider.CapAutoCert, available, mode) {
				continue
			}
			return fmt.Errorf("route %s: automatic TLS execution binding %s is unavailable; explicit migration is required", intent.Domain, intent.TLSExecutor)
		}
		found := false
		for _, providerID := range mode.ProviderIDs() {
			if providerAvailableForCapability(providerID, provider.CapAutoCert, available, mode) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("route %s: target mode cannot recreate automatic TLS state", intent.Domain)
		}
	}
	return nil
}

func providerAvailableForCapability(providerID string, capability provider.Capability, available []provider.ProviderState, mode provider.RuntimeMode) bool {
	if providerID == "" {
		return false
	}
	inMode := false
	for _, id := range mode.ProviderIDs() {
		if id == providerID {
			inMode = true
			break
		}
	}
	if !inMode {
		return false
	}
	for _, state := range available {
		if state.ID == providerID && state.HasCapability(capability) {
			return true
		}
	}
	return false
}

func planMatchesMode(plan *TopologyPlan, mode provider.RuntimeMode) bool {
	if plan == nil || len(plan.Plans) != len(mode.Providers) {
		return false
	}
	for _, id := range mode.ProviderIDs() {
		if _, ok := plan.Plans[id]; !ok {
			return false
		}
	}
	return true
}

// ============================================================================
// Helpers
// ============================================================================

// injectControlPlaneRoute appends a reserved HTTP route that exposes this node's
// own API (/api/*) on the ingress edge, forwarded to the local control port.
//
// The target provider is selected by CAPABILITY (HTTP host routing + TCP
// upstream), never by name, so it follows whichever middleware serves ingress.
// It is a no-op when no control port is configured or no capable provider is in
// the plan (e.g. a pure SNI-passthrough topology).
//
// Rendering notes (see docs/distnode-onboarding-fix.md): Host "http://" renders
// as an HTTP catch-all site (matches the raw-IP Host that distnode dials, and
// coexists with auto-HTTPS domains); Path "/api" is wildcarded to "/api/*" by
// the renderer, covering /api/healthz and /api/distnode/*.
func (p *Planner) injectControlPlaneRoute(plan *TopologyPlan, healthy []provider.ProviderState) {
	if plan == nil || p.deps.ControlPort <= 0 {
		return
	}
	// Deterministic capability-based selection: first healthy provider that is in
	// the plan and can route HTTP by host to a TCP upstream.
	var targetID string
	for _, ps := range healthy {
		if _, ok := plan.Plans[ps.ID]; !ok {
			continue
		}
		if ps.HasCapability(provider.CapRouteHost) && ps.HasCapability(provider.CapUpstreamTCP) {
			targetID = ps.ID
			break
		}
	}
	if targetID == "" {
		return
	}
	pl := plan.Plans[targetID]
	// WHY: lego must not bind :80 beside the gateway. The catch-all HTTP route
	// lets the existing Aegis listener serve only active ACME challenge tokens.
	pl.Routes = append(pl.Routes, provider.RouteSpec{
		Transport:   "tcp",
		AppProtocol: "http",
		Match: provider.MatchSpec{
			Host: "http://",
			Path: "/.well-known/acme-challenge",
		},
		Upstream: provider.UpstreamSpec{
			Type:   "http",
			Target: fmt.Sprintf("http://127.0.0.1:%d", p.deps.ControlPort),
		},
		Priority: provider.RoutePriorityControlPlane + 100,
	})
	pl.Routes = append(pl.Routes, provider.RouteSpec{
		Transport:   "tcp",
		TLSMode:     "terminate",
		AppProtocol: "http",
		Match: provider.MatchSpec{
			Host: "http://", // HTTP catch-all — matches raw-IP Host from distnode
			Path: "/api",    // renderer wildcards to /api/* (healthz + distnode RPC)
		},
		Upstream: provider.UpstreamSpec{
			Type:   "http",
			Target: fmt.Sprintf("http://127.0.0.1:%d", p.deps.ControlPort),
		},
		Priority: provider.RoutePriorityControlPlane,
	})
	// Also inject a catch-all /* → Aegis SPA for the control plane UI
	pl.Routes = append(pl.Routes, provider.RouteSpec{
		Transport:   "tcp",
		AppProtocol: "http",
		Match: provider.MatchSpec{
			Host: "http://", // same HTTP catch-all site
			Path: "",        // empty = catch-all fallback
		},
		Upstream: provider.UpstreamSpec{
			Type:   "http",
			Target: fmt.Sprintf("http://127.0.0.1:%d", p.deps.ControlPort),
		},
		Priority: provider.RoutePriorityControlPlane + 1,
	})
	plan.Plans[targetID] = pl
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

func flowbridgeIDStr(id *string) string {
	if id == nil {
		return ""
	}
	return *id
}

// ensureHeader returns the header map with key set to value (if absent).
func ensureHeader(h map[string]string, key, value string) map[string]string {
	if h == nil {
		h = make(map[string]string)
	}
	if _, ok := h[key]; !ok {
		h[key] = value
	}
	return h
}
