package provider

import (
	"aegis/internal/proxy"
)

// ============================================================================
// Provider — the unified interface for all gateway configuration backends.
//
// This is the SINGLE interface that every gateway tool (Caddy, HAProxy, Nginx,
// Aegis TCP/UDP Manager, etc.) must implement. It merges what were previously
// three separate interfaces (provider.Provider, proxy.ProxyAdapter, Diagnoser).
//
// To add a new provider:
//   1. Implement this interface (see existing implementations for reference)
//   2. Implement Capabilities() and UIHints() (defined in types.go)
//   3. Register in the Registry (see registry.go)
//   4. Add a frontend adapter in ui/src/adapters/<name>/
//
// Key design decision: Render() takes []proxy.RouteConfig, NOT []EdgeMuxEntry
// or any provider-specific type. Every provider translates the unified route
// model into its own config syntax. See config_transform.go for the translation.
// ============================================================================

// Provider is the unified interface for all proxy/gateway configuration backends.
type Provider interface {
	// ─── Identity ───

	// ID returns the machine-readable provider identifier.
	// Must match the key used in the Registry. Examples: "caddy", "haproxy_edge_mux".
	ID() string

	// Name returns the human-readable provider name.
	// Examples: "Caddy HTTP", "HAProxy EdgeMux".
	Name() string

	// Type returns which of the 5 GatewayTypes this provider implements.
	// Must be one of the constants defined in types.go.
	Type() GatewayType

	// Info returns a snapshot of the provider's current runtime state.
	// This is a lightweight summary for list views and status badges.
	// For detailed diagnostics, use Diagnose() instead.
	Info() Info

	// ─── Configuration ───

	// Render generates the native configuration for this provider from the
	// unified route model. Returns the complete config file content.
	// Each provider translates []proxy.RouteConfig into its own syntax
	// (Caddyfile, haproxy.cfg, nginx.conf, iptables rules, etc.).
	Render(routes []proxy.RouteConfig) ([]byte, error)

	// Validate checks whether the configuration at the given path is syntactically
	// valid. Typically shells out to the provider's own validator
	// (caddy validate, haproxy -c, nginx -t).
	Validate(configPath string) error

	// Reload signals the running provider process to reload its configuration
	// without dropping connections. Typically shells out to systemctl reload
	// or the provider's own reload mechanism.
	Reload() error

	// ─── Lifecycle ───

	// Backup creates a timestamped backup of the current config file.
	// Returns the path to the backup file, or "" if no config exists yet.
	Backup() (string, error)

	// Restore replaces the current config with a previously-created backup.
	Restore(backupPath string) error

	// GetCurrentConfig returns the contents of the currently-active config file.
	// Returns "" if no config file exists yet.
	GetCurrentConfig() (string, error)

	// ─── Discovery & Diagnostics ───

	// Diagnose performs a full diagnostic check of this provider on the local node.
	// Checks: binary presence → version → config existence → config validity →
	// service running → listener conflicts → runtime smoke test.
	// Returns a structured ProviderDiagnostic with detailed failure information.
	Diagnose() ProviderDiagnostic

	// Capabilities returns the static capability declaration for this provider type.
	// Used by the capability matrix to answer "what can this provider do?"
	// This is a static declaration — it describes what the provider CAN do,
	// not what it IS doing right now. For current status, use Diagnose().
	Capabilities() ProviderCapabilities

	// UIHints returns hints for the frontend about how to render this provider's
	// detail pages. Includes custom panels, metrics, and operations.
	UIHints() ProviderUIHints

	// ─── Installation ───

	// CanInstall returns true if this provider can be installed on the current OS.
	// For Debian/Ubuntu: returns true if apt-get is available.
	CanInstall() bool

	// Install runs the OS package manager to install this provider.
	// For Debian/Ubuntu: sudo apt-get install -y <package> && sudo systemctl enable --now <service>.
	Install() error
}

// ============================================================================
// Shared types used by the Provider interface
// ============================================================================

// Info describes a provider's current runtime state.
// Unlike Capabilities (static) and Diagnose (detailed check), Info is a quick
// summary suitable for list views and status badges.
type Info struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Type       GatewayType `json:"type"`
	Status     string      `json:"status"` // "ready" | "degraded" | "unavailable"
	Message    string      `json:"message"`
	ConfigPath string      `json:"config_path"`
}

// Plan holds the rendered config and metadata for a single provider's apply operation.
type Plan struct {
	Provider Info
	Routes   []proxy.RouteConfig
	Rendered string
	Warnings []string
}

// ApplyResult holds the result of applying a provider's config.
type ApplyResult struct {
	Provider string `json:"provider"`
	Status   string `json:"status"` // "success" | "failed" | "skipped"
	Message  string `json:"message"`
	Rendered string `json:"rendered,omitempty"`
}
