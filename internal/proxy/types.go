// Package proxy — bridge types for the v1.8L transition.
//
// These types (RouteConfig, GatewayConfig, ProxyOptions, ProxyAdapter) are
// temporary bridge types during the 3-dimension architecture migration.
// They will be replaced by provider.RouteSpec, provider.Plan, and the new
// provider.Provider interface in Phase 5 (integration).
//
// New code should use provider.RouteSpec and provider.Plan from state.go.
// These bridge types exist only to keep existing renderers compiling.
package proxy

// RouteConfig describes a single HTTP reverse-proxy route in Caddy-centric terms.
// DEPRECATED: use provider.RouteSpec from state.go instead.
type RouteConfig struct {
	Domain             string            `json:"domain"`
	PathPrefix         string            `json:"path_prefix,omitempty"`
	Kind               string            `json:"kind"`
	UpstreamURL        string            `json:"upstream_url"`
	TLSEnabled         bool              `json:"tls_enabled"`
	MaintenanceEnabled bool              `json:"maintenance_enabled,omitempty"`
	MaintenanceMessage string            `json:"maintenance_message,omitempty"`
	Options            ProxyOptions      `json:"options"`
}

// ProxyOptions holds optional proxy behavior flags.
// DEPRECATED: use provider.RouteSpec fields instead.
type ProxyOptions struct {
	EnableGzip   bool              `json:"enable_gzip,omitempty"`
	StripPrefix  bool              `json:"strip_prefix,omitempty"`
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
}

// GatewayConfig holds global gateway configuration for rendering.
// DEPRECATED: use provider.Plan from state.go instead.
type GatewayConfig struct {
	Routes         []RouteConfig `json:"routes"`
	Email          string        `json:"email,omitempty"`
	PortPolicyMode string        `json:"port_policy_mode,omitempty"`
}

// ProxyAdapter is the old adapter interface for rendering and validating config.
// DEPRECATED: use provider.Provider from provider.go instead.
type ProxyAdapter interface {
	Name() string
	Render(gwCfg GatewayConfig) ([]byte, error)
	Validate(configPath string) error
	Reload(command string) error
}
