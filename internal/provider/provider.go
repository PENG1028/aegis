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
// # Lifecycle layers (6 layers, each with dedicated interface methods):
//
//  1. DETECTION — Diagnose(), Info(), Capabilities()
//     "Is this provider present, healthy, and what can it do?"
//
//  2. LOCATION — ConfigPath(), BinaryPath(), ServiceName()
//     "Where are the config, binary, and systemd service for this provider?"
//
//  3. INSTALL — CanInstall(), Install()
//     "Can we install this provider, and if so, install it."
//
//  4. UNINSTALL — CanUninstall(), Uninstall()
//     "Can we remove this provider, and if so, remove it."
//     For shared-binary providers (e.g., HAProxy TCP shares haproxy with EdgeMux),
//     CanUninstall() returns false — the binary is managed by the sibling provider.
//
//  5. CONFIG — Render(), Validate(), Reload(), Backup(), Restore(),
//     GetCurrentConfig(), WriteConfig()
//     "Read, validate, write, backup, restore, and reload the provider's config."
//
//  6. UI — UIHints()
//     "How should the frontend render this provider's detail pages?"
//
// # To add a new provider:
//
//  1. Implement this interface (see existing implementations for reference)
//  2. Implement Capabilities() and UIHints() (defined in types.go)
//  3. Register in the Registry (see registry.go)
//  4. Add a frontend adapter in ui/src/adapters/<name>/
//
// # Key design principles:
//   - Render() takes []proxy.RouteConfig — every provider translates the unified
//     route model into its own config syntax (Caddyfile, haproxy.cfg, nginx.conf).
//   - HTTP handlers MUST delegate through this interface, not shell out directly.
//     If you find raw exec.Command("systemctl", ...) for a provider in a handler,
//     it is a bug — use provider.Reload() instead.
//   - Shared-binary providers (e.g., HAProxy EdgeMux + HAProxy TCP) share
//     Install/Uninstall responsibility. The "primary" provider (EdgeMux) owns
//     the binary lifecycle; secondary providers (TCP) set CanInstall=false,
//     CanUninstall=false.
// ============================================================================

// Provider is the unified interface for all proxy/gateway configuration backends.
type Provider interface {
	// ========================================================================
	// Layer 1: DETECTION — "Is this provider present and healthy?"
	// ========================================================================

	// Info returns a snapshot of the provider's current runtime state.
	// This is a lightweight summary for list views and status badges.
	// For detailed diagnostics, use Diagnose() instead.
	Info() Info

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

	// ========================================================================
	// Layer 2: LOCATION — "Where are the files for this provider?"
	// ========================================================================

	// ID returns the machine-readable provider identifier.
	// Must match the key used in the Registry. Examples: "caddy", "haproxy_edge_mux".
	ID() string

	// Name returns the human-readable provider name.
	// Examples: "Caddy HTTP", "HAProxy EdgeMux".
	Name() string

	// Type returns which of the 5 GatewayTypes this provider implements.
	Type() GatewayType

	// ConfigPath returns the absolute path to this provider's configuration file.
	// Examples: "/etc/caddy/Caddyfile", "/etc/haproxy/haproxy.cfg".
	ConfigPath() string

	// BinaryPath returns the absolute path to the provider's executable binary.
	// Returns "" if the binary is not installed or not found in PATH.
	// This is resolved at construction time or via exec.LookPath.
	BinaryPath() string

	// ServiceName returns the systemd service name for this provider.
	// Used by the HTTP API to issue start/stop/restart/reload commands.
	// Returns "" for providers that don't have a standalone service
	// (e.g., built-in providers like Aegis TCP/UDP manager).
	// Examples: "caddy", "haproxy".
	ServiceName() string

	// ========================================================================
	// Layer 3: INSTALL — "Can we install and remove this provider?"
	// ========================================================================

	// CanInstall returns true if this provider can be installed on the current OS.
	// For Debian/Ubuntu: returns true if apt-get is available AND this provider
	// is the "primary" owner of its binary.
	// Shared-binary providers (e.g., HAProxy TCP) return false — installation
	// is handled by the primary provider (HAProxy EdgeMux).
	CanInstall() bool

	// Install runs the OS package manager to install this provider.
	// For Debian/Ubuntu: sudo apt-get install -y <package> && sudo systemctl enable --now <service>.
	// Must only be called if CanInstall() returns true.
	Install() error

	// CanUninstall returns true if this provider can be removed from the current OS.
	// Shared-binary providers return false — the binary must stay for the
	// sibling provider. Built-in providers return false.
	CanUninstall() bool

	// Uninstall stops the service and removes the provider package.
	// Config files are preserved for manual cleanup.
	// Must only be called if CanUninstall() returns true.
	Uninstall() error

	// ========================================================================
	// Layer 4: CONFIG — "Read, write, validate, and apply configuration"
	// ========================================================================

	// Render generates the native configuration for this provider from the
	// unified route model. Returns the complete config file content.
	// Each provider translates []proxy.RouteConfig into its own syntax
	// (Caddyfile, haproxy.cfg, nginx.conf, iptables rules, etc.).
	// Does NOT write to disk — use WriteConfig() for that.
	Render(routes []proxy.RouteConfig) ([]byte, error)

	// Validate checks whether the configuration at the given path is syntactically
	// valid. Typically shells out to the provider's own validator
	// (caddy validate, haproxy -c, nginx -t).
	Validate(configPath string) error

	// Reload signals the running provider process to reload its configuration
	// without dropping connections. Typically shells out to systemctl reload
	// or the provider's own reload mechanism (e.g., haproxy -sf).
	Reload() error

	// Backup creates a timestamped backup of the current config file.
	// Returns the path to the backup file, or "" if no config exists yet.
	Backup() (string, error)

	// Restore replaces the current config with a previously-created backup.
	Restore(backupPath string) error

	// GetCurrentConfig returns the contents of the currently-active config file.
	// Returns "" if no config file exists yet.
	GetCurrentConfig() (string, error)

	// WriteConfig validates, backs up, writes, and reloads a config in one step.
	// This is the canonical write path for the HTTP Save Config API.
	// Implementations follow the same pattern: validate temp file → backup
	// current config → atomic write → reload on success.
	WriteConfig(content []byte) error

	// ========================================================================
	// Layer 5: UI — "How should the frontend render this provider?"
	// ========================================================================

	// UIHints returns hints for the frontend about how to render this provider's
	// detail pages. Includes custom panels, metrics, and operations.
	UIHints() ProviderUIHints
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
