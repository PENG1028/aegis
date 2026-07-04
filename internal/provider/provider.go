// Package provider — dimension 1 of the 3-dimension gateway architecture.
//
// This file defines the unified Provider interface. Every gateway middleware
// (Caddy, HAProxy, Nginx) implements this interface. The interface is lean by
// design — only 4 methods:
//
//	State()    — who am I, what can I do, am I healthy?        (→ ProviderState)
//	Diagnose() — deep diagnostic check                           (→ ProviderDiagnostic)
//	Render()   — translate a Plan into native config files       (Plan → []ConfigFile)
//	Apply()    — validate, backup, write, reload config files    ([]ConfigFile → nil)
//
// # What moved where:
//
//	Install / Uninstall      → dimension 3 (lifecycle.Manager)
//	ID / Name / Type / Path  → dimension 1 (ProviderState fields)
//	Capabilities             → dimension 1 (ProviderState.Capabilities)
//	Validate / Reload / etc. → encapsulated inside Apply()
//
// # To implement a new Provider:
//
//  1. Create a struct that holds config + binary paths
//  2. Implement State() — return identity + capabilities
//  3. Implement Diagnose() — check binary, config, service
//  4. Implement Render() — translate Plan → native config syntax
//  5. Implement Apply() — the canonical 6-step write flow
//  6. Register in registry.go
package provider

// ============================================================================
// Provider — the unified interface for all gateway configuration backends
// ============================================================================

// Provider is the single interface every gateway middleware must implement.
//
// Design principles:
//   - One middleware = one Provider. HAProxy is ONE provider, not split by config file.
//   - Render() takes a Plan (from dimension 2), returns ConfigFiles. The Provider
//     does NOT decide what to render — it only translates Plan → native syntax.
//   - Apply() encapsulates the full write pipeline. Callers do not need to
//     manually validate→backup→write→reload.
//   - State() is the single source of truth for identity + capabilities + health.
//     Dimensions 2 and 3 read it; dimension 3 writes to it via lifecycle operations.
type Provider interface {
	// State returns a point-in-time snapshot of this provider's identity,
	// capabilities, and health. Lightweight — for list views and status badges.
	// For detailed diagnostics, use Diagnose().
	State() ProviderState

	// Diagnose performs a full diagnostic check: binary presence → version →
	// config existence → config validity → service running → listener health.
	// Returns a structured ProviderDiagnostic with detailed failure information.
	Diagnose() ProviderDiagnostic

	// Render translates a Plan (listeners + routes) into native configuration
	// files. One Plan may produce multiple ConfigFiles (e.g., HAProxy generates
	// haproxy.cfg for SNI routing + haproxy_tcp.cfg for TCP forwarding).
	// Does NOT write to disk — use Apply() for that.
	Render(plan Plan) ([]ConfigFile, error)

	// Apply validates, backs up, writes, and reloads the given configuration
	// files. This is the canonical write path — all config mutations flow
	// through here.
	//
	// The standard implementation follows a 6-step pipeline:
	//   1. Validate each config file (provider's own validator)
	//   2. Backup current config files (timestamped)
	//   3. Write new config files to disk (atomic where possible)
	//   4. Reload the provider (graceful, no connection drop)
	//   5. Verify the reload succeeded (smoke test)
	//   6. Rollback to backup on failure
	Apply(configs []ConfigFile) error
}

// ============================================================================
// ApplyResult — outcome of a single provider's config apply
// ============================================================================

// ApplyResult holds the result of applying a provider's configuration.
// Used by the HTTP API to report per-provider status.
type ApplyResult struct {
	Provider string `json:"provider"`
	Status   string `json:"status"` // "success" | "failed" | "skipped"
	Message  string `json:"message"`
	Rendered string `json:"rendered,omitempty"`
}

// ============================================================================
// Optional extension interfaces (temporary bridges to dimension 3)
// ============================================================================

// LifecycleProvider is an optional interface for providers that support
// install/uninstall operations. This will move to dimension 3 (lifecycle.Manager)
// in Phase 4.
type LifecycleProvider interface {
	Provider
	CanInstall() bool
	Install() error
	CanUninstall() bool
	Uninstall() error
}

// ReloadableProvider is an optional interface for providers that support
// standalone reload (without a full Apply cycle). Used by the reload HTTP handler.
type ReloadableProvider interface {
	Provider
	Reload() error
}

// ConfigReader is an optional interface for providers whose current config
// can be read back. Used by the config preview HTTP handler.
type ConfigReader interface {
	Provider
	GetCurrentConfig() (string, error)
}

// ServiceController is an optional interface for providers backed by a system
// service (systemd). Used during mode switching to stop a provider that is no
// longer in the active plan (e.g. stopping HAProxy when switching to Legacy).
type ServiceController interface {
	Provider
	Start() error
	Stop() error
	Restart() error
}

// ConfigCleaner is an optional interface for providers that can remove their
// configuration files. Used during mode switching to clean up stale configs.
type ConfigCleaner interface {
	Provider
	CleanConfig() error
}
