package provider

import "fmt"

// ============================================================================
// Registry — manages the set of known Provider implementations.
//
// Usage:
//   reg := provider.NewRegistry()
//   reg.Register(myProvider)
//   p, ok := reg.Get("caddy")  // look up by ID
//   all := reg.List()          // all registered providers
//   p, ok := reg.SelectForProtocol("http")  // auto-select
//
// Registration happens in two ways:
//   1. Explicit: cmd/aegis/main.go calls reg.Register(NewCaddyHTTPProvider(cfg))
//      for providers instantiated with runtime configuration.
//   2. Auto: built-in providers that don't need config (Aegis TCP/UDP Manager)
//      are registered via RegisterBuiltin() which uses zero-config constructors.
//
// The registry is used by:
//   - Provider discovery (discovery.go): iterate all registered types, detect each
//   - Config apply pipeline (apply/): iterate all, render + reload each
//   - API /api/admin/v1/providers: list known types with capabilities
//   - API /api/admin/v1/nodes/{id}/discovered-providers: detection results per node
// ============================================================================

// Registry manages a set of Provider implementations.
type Registry struct {
	providers map[string]Provider     // keyed by Provider.ID()
	order     []string                // registration order
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
	id := p.ID()
	if _, exists := r.providers[id]; !exists {
		r.order = append(r.order, id)
	}
	r.providers[id] = p
}

// RegisterBuiltin registers a zero-config constructor for a built-in provider.
// Built-in providers (Aegis TCP/UDP Manager, Transparent Proxy) don't need
// external config — they can be instantiated on demand during discovery.
func (r *Registry) RegisterBuiltin(id string, ctor func() Provider) {
	r.builtins[id] = ctor
}

// Get returns a registered provider by ID, or nil if not found.
func (r *Registry) Get(id string) Provider {
	p, ok := r.providers[id]
	if ok {
		return p
	}
	// Try built-in constructors
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

// List returns Info summaries for all registered providers.
func (r *Registry) List() []Info {
	var infos []Info
	for _, id := range r.order {
		p := r.providers[id]
		infos = append(infos, Info{
			ID:         p.ID(),
			Name:       p.Name(),
			Type:       p.Type(),
			ConfigPath: "", // filled by discovery
		})
	}
	// Add built-ins not yet explicitly registered
	for id, ctor := range r.builtins {
		if _, exists := r.providers[id]; !exists {
			p := ctor()
			infos = append(infos, Info{
				ID:   p.ID(),
				Name: p.Name(),
				Type: p.Type(),
			})
		}
	}
	return infos
}

// ListAll returns all registered Provider instances.
// Use for iteration during apply/discovery.
func (r *Registry) ListAll() []Provider {
	var all []Provider
	for _, id := range r.order {
		all = append(all, r.providers[id])
	}
	// Add built-ins not yet explicitly registered
	for id, ctor := range r.builtins {
		if _, exists := r.providers[id]; !exists {
			all = append(all, ctor())
		}
	}
	return all
}

// SelectForProtocol picks a default provider for the given protocol.
// Used by the config generation pipeline when a route doesn't specify a provider.
//
// Selection logic:
//   "http", "https"  → caddy (HTTP termination)
//   "tcp"            → haproxy_tcp if registered, otherwise aegis_tcp
//   "udp"            → aegis_udp (built-in)
func (r *Registry) SelectForProtocol(protocol string) (Provider, error) {
	switch protocol {
	case "http", "https":
		if p := r.Get("caddy"); p != nil {
			return p, nil
		}
		return nil, fmt.Errorf("no provider registered for protocol %s", protocol)
	case "tcp":
		if p := r.Get("haproxy_tcp"); p != nil {
			return p, nil
		}
		if p := r.Get("aegis_tcp"); p != nil {
			return p, nil
		}
		return nil, fmt.Errorf("no provider registered for protocol %s", protocol)
	case "udp":
		if p := r.Get("aegis_udp"); p != nil {
			return p, nil
		}
		return nil, fmt.Errorf("no provider registered for protocol %s", protocol)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}
