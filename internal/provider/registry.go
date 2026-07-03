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

// SelectForProtocol returns the best provider for a given protocol type.
// This is a simple lookup — the topology planner (dimension 2) is the full
// capability-based selector. This method is a convenience bridge for code
// that hasn't been migrated to the topology planner yet.
func (r *Registry) SelectForProtocol(protoType string) (Provider, error) {
	// Map protocol types to preferred provider IDs
	preferred := map[string]string{
		"http":     "caddy",
		"tcp":      "haproxy",
		"udp":      "haproxy",
		"tunnel":   "haproxy",
		"internal": "caddy",
	}

	id, ok := preferred[protoType]
	if !ok {
		id = "caddy"
	}

	p := r.Get(id)
	if p == nil {
		return nil, fmt.Errorf("no provider available for protocol %s", protoType)
	}
	return p, nil
}
