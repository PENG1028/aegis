package hostdep

import "fmt"

// Status describes the detected state of a host dependency. It is the single
// detection-result type shared across provider and infra (infra.Status is an
// alias of this).
type Status struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Category  string `json:"category"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
	Message   string `json:"message"`
}

// HostDependency is an external dependency on the host that Aegis can detect
// and, when it is an installable package, install through the adaptation layer.
//
// Gateway middleware (Caddy/HAProxy) is a richer kind of host dependency — it
// also renders config and matches capabilities — so it stays on the Provider
// interface; but its install path shares the same InstallPackage flow used here,
// so there is one installation implementation across the codebase.
type HostDependency interface {
	// Name is the dependency's identifier ("dnsmasq", "iptables", ...).
	Name() string
	// Detect reports whether the dependency is present, its version, and health.
	Detect() Status
	// Installable reports whether Aegis can install this dependency. Embedded /
	// built-in dependencies (e.g. the lego ACME client) return false.
	Installable() bool
	// Install installs the dependency via the host package manager. Returns an
	// error if it is not Installable() or if installation fails.
	Install() error
}

// InstallPackage is the single package-install flow: detect the host package
// manager, refresh the index, install pkg, and (when service is non-empty)
// enable+start the systemd service. Both provider install and infra tools route
// through here, so there is exactly one install implementation.
func InstallPackage(pkg, service string) error {
	pm := Detect()
	if pm == nil {
		return fmt.Errorf("no supported package manager found on host (need apt-get)")
	}
	if err := pm.Update(); err != nil {
		return err
	}
	if err := pm.Install(pkg); err != nil {
		return err
	}
	if service != "" {
		if err := enableService(service); err != nil {
			return err
		}
	}
	return nil
}

// RemovePackage stops the dependency's service (best-effort, when service is
// non-empty) and removes the package via the host package manager.
func RemovePackage(pkg, service string) error {
	if service != "" {
		stopService(service) // best-effort
	}
	pm := Detect()
	if pm == nil {
		return fmt.Errorf("no supported package manager found on host")
	}
	return pm.Remove(pkg)
}
