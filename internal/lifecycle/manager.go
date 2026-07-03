// Package lifecycle — dimension 3: middleware lifecycle management.
//
// This package manages the full lifecycle of gateway middleware: install,
// uninstall, start, stop, restart, and status queries. It wraps apt-get
// (package management) and systemctl (service control) behind a unified
// Manager interface.
//
// The Manager operates on a provider registry — it knows how to install
// and control Caddy, HAProxy, and future providers through their LifecycleProvider
// optional interface.
//
// This package does NOT:
//   - Render or apply config (dimension 1: provider.Provider)
//   - Decide which middleware to use (dimension 2: topology.Planner)
//   - Handle transparent proxy forwarding (transparent.Manager)
package lifecycle

import (
	"fmt"
	"os/exec"
	"strings"

	"aegis/internal/provider"
)

// ============================================================================
// Manager — unified middleware lifecycle interface
// ============================================================================

// Manager controls the lifecycle of gateway middleware.
type Manager struct {
	provReg *provider.Registry
}

// NewManager creates a lifecycle Manager backed by the given provider registry.
func NewManager(provReg *provider.Registry) *Manager {
	return &Manager{provReg: provReg}
}

// ============================================================================
// Installation
// ============================================================================

// Install installs a middleware provider by its ID.
// e.g., Install("caddy") → sudo apt-get install -y caddy && sudo systemctl enable --now caddy
func (m *Manager) Install(providerID string) error {
	p := m.provReg.Get(providerID)
	if p == nil {
		return fmt.Errorf("unknown provider: %s", providerID)
	}

	lc, ok := p.(provider.LifecycleProvider)
	if !ok || !lc.CanInstall() {
		return fmt.Errorf("provider %s does not support installation", providerID)
	}

	// Check if already installed
	if _, err := exec.LookPath(providerID); err == nil {
		return fmt.Errorf("%s is already installed", providerID)
	}

	return lc.Install()
}

// Uninstall removes a middleware provider.
func (m *Manager) Uninstall(providerID string) error {
	p := m.provReg.Get(providerID)
	if p == nil {
		return fmt.Errorf("unknown provider: %s", providerID)
	}

	lc, ok := p.(provider.LifecycleProvider)
	if !ok || !lc.CanUninstall() {
		return fmt.Errorf("provider %s does not support uninstallation", providerID)
	}

	return lc.Uninstall()
}

// ============================================================================
// Service control (systemctl)
// ============================================================================

// Start starts the systemd service for a provider.
func (m *Manager) Start(providerID string) error {
	return systemctl(providerID, "start")
}

// Stop stops the systemd service for a provider.
func (m *Manager) Stop(providerID string) error {
	return systemctl(providerID, "stop")
}

// Restart restarts the systemd service for a provider.
func (m *Manager) Restart(providerID string) error {
	return systemctl(providerID, "restart")
}

// Reload reloads the configuration of a running provider.
func (m *Manager) Reload(providerID string) error {
	p := m.provReg.Get(providerID)
	if p == nil {
		return fmt.Errorf("unknown provider: %s", providerID)
	}

	reloadable, ok := p.(provider.ReloadableProvider)
	if ok {
		return reloadable.Reload()
	}
	// Fallback: systemctl reload
	return systemctl(providerID, "reload")
}

// ============================================================================
// Status
// ============================================================================

// Status returns the current state of a provider.
func (m *Manager) Status(providerID string) (provider.ProviderState, error) {
	p := m.provReg.Get(providerID)
	if p == nil {
		return provider.ProviderState{}, fmt.Errorf("unknown provider: %s", providerID)
	}
	return p.State(), nil
}

// IsRunning returns true if the provider's systemd service is active.
func (m *Manager) IsRunning(providerID string) bool {
	state, err := m.Status(providerID)
	if err != nil {
		return false
	}
	return state.Running
}

// ============================================================================
// systemctl helper
// ============================================================================

// systemctl runs a systemctl command for the given provider ID.
func systemctl(providerID, action string) error {
	cmd := exec.Command("systemctl", action, providerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s %s failed: %w\n%s", action, providerID, err, strings.TrimSpace(string(out)))
	}
	return nil
}
