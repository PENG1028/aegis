package provider

import "fmt"

// ============================================================================
// Registry — manages the set of known Provider implementations.
//
// Usage:
//   reg := provider.NewRegistry()
//   reg.Register(myProvider)
//   p, ok := reg.Get("caddy")  // look up by ID
//   all := reg.List()          // all registered providers (ProviderState)
//   allInstances := reg.ListAll() // all Provider instances (for iteration)
//
// Registration happens in cmd/aegis/main.go. Each Provider is instantiated
// with its runtime configuration and registered here.
//
// The registry is used by:
//   - Provider discovery (discovery.go): iterate all registered types, detect each
//   - Config apply pipeline (apply/): iterate all, render + apply each
//   - API /api/admin/v1/providers: list known types with state
// ============================================================================

// Registry manages a set of Provider implementations.
type Registry struct {
	providers map[string]Provider       // keyed by Provider.State().ID
	order     []string                  // registration order
	builtins  map[string]func() Provider // zero-config constructors for built-in providers
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
		builtins:  make(map[string]func() Provider),
	}
}

// Register adds a fully-initialized Provider to the registry.
// Use this for providers that need runtime configuration (Caddy, HAProxy).
func (r *Registry) Register(p Provider) {
	id := p.State().ID
	if _, exists := r.providers[id]; !exists {
		r.order = append(r.order, id)
	}
	r.providers[id] = p
}

// RegisterBuiltin registers a zero-config constructor for a built-in provider.
// Built-in providers don't need external config — they can be instantiated
// on demand during discovery.
func (r *Registry) RegisterBuiltin(id string, ctor func() Provider) {
	r.builtins[id] = ctor
}

// Get returns a registered provider by ID, or nil if not found.
func (r *Registry) Get(id string) Provider {
	p, ok := r.providers[id]
	if ok {
		return p
	}
	ctor, ok := r.builtins[id]
	if ok {
		return ctor()
	}
	return nil
}

// Has returns true if a provider with the given ID is registered.
func (r *Registry) Has(id string) bool {
	_, ok := r.providers[id]
	if ok {
		return true
	}
	_, ok = r.builtins[id]
	return ok
}

// List returns ProviderState snapshots for all registered providers.
// Suitable for list views and status badges.
func (r *Registry) List() []ProviderState {
	var states []ProviderState
	for _, id := range r.order {
		p := r.providers[id]
		states = append(states, p.State())
	}
	for id, ctor := range r.builtins {
		if _, exists := r.providers[id]; !exists {
			p := ctor()
			states = append(states, p.State())
		}
	}
	return states
}

// ListAll returns all registered Provider instances for iteration.
// Use during discovery and apply pipelines.
func (r *Registry) ListAll() []Provider {
	var all []Provider
	for _, id := range r.order {
		all = append(all, r.providers[id])
	}
	for id, ctor := range r.builtins {
		if _, exists := r.providers[id]; !exists {
			all = append(all, ctor())
		}
	}
	return all
}

// protocolCaps maps protocol types to required capabilities.
// Replaces the old hardcoded provider-ID map with capability-based matching —
// when new providers are registered, they auto-qualify for protocols they support.
var protocolCaps = map[string][]Capability{
	"http":     {CapListenTCP, CapUpstreamTCP, CapHTTP1, CapRouteHost},
	"tcp":      {CapListenTCP, CapUpstreamTCP, CapRawTCP},
	"udp":      {CapListenUDP, CapUpstreamUDP},
	"tunnel":   {CapListenTCP, CapUpstreamTCP, CapTLSPassthrough, CapSNIPreread},
	"internal": {CapListenTCP, CapUpstreamTCP, CapHTTP1, CapRouteHost},
}

// SelectForProtocol returns the best provider for a given protocol type.
// Searches registered providers by capability — no hardcoded provider names.
func (r *Registry) SelectForProtocol(protoType string) (Provider, error) {
	required, ok := protocolCaps[protoType]
	if !ok {
		required = protocolCaps["http"] // fallback
	}

	for _, p := range r.ListAll() {
		state := p.State()
		hasAll := true
		for _, cap := range required {
			if !state.HasCapability(cap) {
				hasAll = false
				break
			}
		}
		if hasAll {
			return p, nil
		}
	}

	return nil, fmt.Errorf("no provider available for protocol %s", protoType)
}
