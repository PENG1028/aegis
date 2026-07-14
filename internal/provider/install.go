package provider

import (
	"aegis/internal/hostdep"
)

// ============================================================================
// installPackage / uninstallPackage — the single install path for providers.
//
// Both delegate to hostdep.InstallPackage/RemovePackage, the one install flow
// shared with the infra host-dependency tools. Provider install is no longer
// hardcoded to apt-get, and there is exactly one installation implementation.
//
// If you find another apt-get invocation in the codebase, it should use this.
// ============================================================================

// installPackage installs a package and enables its systemd service.
func installPackage(pkg, service string) error {
	return hostdep.InstallPackage(pkg, service)
}

// uninstallPackage stops the service and removes the package (config preserved).
func uninstallPackage(pkg, service string) error {
	return hostdep.RemovePackage(pkg, service)
}
