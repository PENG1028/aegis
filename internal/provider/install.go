package provider

import (
	"fmt"
	"os/exec"
)

// ============================================================================
// installPackage — shared apt-get installation helper for Debian/Ubuntu.
//
// Used by Provider.Install() implementations. Each provider calls this with
// its package name and systemd service name.
//
// This is the SINGLE installation path for all external providers.
// If you find another apt-get invocation in the codebase, it should use this.
// ============================================================================

// installPackage installs a Debian/Ubuntu package via apt-get and enables the
// corresponding systemd service. Returns nil on success.
func installPackage(pkg, service string) error {
	// Update package lists
	updateCmd := exec.Command("sudo", "apt-get", "update", "-qq")
	if out, err := updateCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("apt-get update failed: %w\n%s", err, string(out))
	}

	// Install
	installCmd := exec.Command("sudo", "apt-get", "install", "-y", "-qq", pkg)
	if out, err := installCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("apt-get install %s failed: %w\n%s", pkg, err, string(out))
	}

	// Enable and start
	enableCmd := exec.Command("sudo", "systemctl", "enable", "--now", service)
	if out, err := enableCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable %s failed: %w\n%s", service, err, string(out))
	}

	return nil
}
