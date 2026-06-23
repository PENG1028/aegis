package proxy

import "errors"

// ProxyAdapter is the unified interface for proxy configuration backends.
type ProxyAdapter interface {
	Name() string
	Render(cfg GatewayConfig) ([]byte, error)
	Validate(configPath string) error
	Reload(command string) error
}

// GatewayConfig is Aegis's internal configuration model.
type GatewayConfig struct {
	Routes []RouteConfig
	Email  string
}

// RouteConfig represents a single route to be proxied.
type RouteConfig struct {
	Domain             string
	Kind               string // reverse_proxy | static_site | tcp_proxy | file_hosting
	UpstreamURL        string
	RootDir            string
	TLSEnabled          bool
	MaintenanceEnabled  bool
	MaintenanceMessage  string
	Options            ProxyOptions
}

// ProxyOptions holds optional proxy settings.
type ProxyOptions struct {
	EnableGzip     bool
	WebSocket      bool
	SPAFallback    bool
	MaxBodySizeMB  int
	ReadTimeoutSec int
	PreserveHost   bool
	StripPrefix    bool
}

// ErrNotImplemented is returned by adapters that are not yet implemented.
var ErrNotImplemented = errors.New("adapter not implemented yet")
