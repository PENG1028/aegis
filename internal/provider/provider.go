package provider

import (
	"aegis/internal/proxy"
)

// Info describes a provider's identity and capabilities.
type Info struct {
	Name     string   `json:"name"`
	Protocol string   `json:"protocol"` // http | tcp | udp | tls_mux
	Status   string   `json:"status"`   // ready | degraded | unavailable
	Message  string   `json:"message"`
	ConfigPath string `json:"config_path"`
}

// Provider is the unified interface for all proxy configuration backends.
type Provider interface {
	Info() Info
	Render(routes []proxy.RouteConfig) ([]byte, error)
	Validate(configPath string) error
	Reload() error
	Backup() (string, error)
	Restore(backupPath string) error
	GetCurrentConfig() (string, error)
}

// Plan holds the rendered config and metadata for a single provider's apply.
type Plan struct {
	Provider Info
	Routes   []proxy.RouteConfig
	Rendered string
	Warnings []string
}

// ApplyResult holds the result of applying a provider's config.
type ApplyResult struct {
	Provider string `json:"provider"`
	Status   string `json:"status"` // success | failed | skipped
	Message  string `json:"message"`
	Rendered string `json:"rendered,omitempty"`
}

// Registry manages registered providers.
type Registry struct {
	providers map[string]Provider
	order     []string
}

// NewRegistry creates a provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.providers[p.Info().Name] = p
	r.order = append(r.order, p.Info().Name)
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// List returns all registered providers.
func (r *Registry) List() []Info {
	var infos []Info
	for _, name := range r.order {
		infos = append(infos, r.providers[name].Info())
	}
	return infos
}

// SelectForProtocol picks the default provider for a protocol.
func (r *Registry) SelectForProtocol(protocol string) (Provider, bool) {
	switch protocol {
	case "http", "https":
		return r.Get("caddy_http")
	case "tcp":
		if p, ok := r.Get("haproxy_tcp"); ok {
			return p, ok
		}
	case "udp", "tunnel":
		// Not yet implemented — record only
		return nil, false
	}
	return nil, false
}
