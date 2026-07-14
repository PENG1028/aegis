package provider

import (
	"fmt"
	"os/exec"

	"aegis/internal/hostdep"
)

// ============================================================================
// installPackage / uninstallPackage — the single install path for providers.
//
// Package installation is delegated to hostdep.PackageManager (the adaptation
// layer), so provider install is no longer hardcoded to apt-get. Service
// enable/start/stop still uses systemctl inline here — that is unified into a
// host service manager in a later step.
//
// This is the SINGLE installation path for all external providers.
// If you find another apt-get invocation in the codebase, it should use this.
// ============================================================================

// installPackage installs a package via the host's package manager and enables
// the corresponding systemd service. Returns nil on success.
func installPackage(pkg, service string) error {
	pm := hostdep.Detect()
	if pm == nil {
		return fmt.Errorf("no supported package manager found on host (need apt-get)")
	}
	if err := pm.Update(); err != nil {
		return err
	}
	if err := pm.Install(pkg); err != nil {
		return err
	}
	// Enable and start the service (systemd). Unified into a host service
	// manager in a later step of the host-dependency refactor.
	if out, err := exec.Command("sudo", "systemctl", "enable", "--now", service).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable %s failed: %w\n%s", service, err, string(out))
	}
	return nil
}

// uninstallPackage removes a package via the host's package manager after
// stopping its service. Config files are preserved.
func uninstallPackage(pkg, service string) error {
	// Stop service first (best-effort).
	exec.Command("sudo", "systemctl", "stop", service).Run()

	pm := hostdep.Detect()
	if pm == nil {
		return fmt.Errorf("no supported package manager found on host")
	}
	return pm.Remove(pkg)
}
