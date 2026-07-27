// Package hostdep abstracts operations against host dependencies that live
// outside the Aegis process — package managers, and (in later steps) service
// managers and dependency detection.
//
// Its purpose is to remove hardcoded, single-OS assumptions (e.g. "install =
// apt-get") from the provider and infra packages, replacing them with an
// adaptation layer that picks the right tool for the host.
//
// This package imports only the standard library so it stays a reusable leaf.
package hostdep

import (
	"fmt"
	"os/exec"
)

// PackageManager abstracts host package installation so provider/infra install
// logic is not hardcoded to one OS package manager. Use Detect() to obtain the
// implementation available on the host.
//
// Implementations:
//   - apt   (Debian/Ubuntu) — implemented, Aegis's primary target
//   - yum   (RHEL/CentOS)    — extension point, not yet implemented
//   - apk   (Alpine)         — extension point, not yet implemented
type PackageManager interface {
	// Name returns the manager's identifier ("apt", "yum", "apk").
	Name() string
	// Update refreshes the package index.
	Update() error
	// Install installs the named package.
	Install(pkg string) error
	// Remove uninstalls the named package (config files preserved where possible).
	Remove(pkg string) error
}

// Detect returns the PackageManager available on the host, or nil if none is
// found. apt is checked first (Debian/Ubuntu). yum and apk are recognized
// extension points — add their implementations when a non-Debian target is
// required; the callers already route through this interface, so no caller
// changes are needed then.
func Detect() PackageManager {
	if _, err := exec.LookPath("apt-get"); err == nil {
		return aptManager{}
	}
	// Extension points (implement as needed):
	//   if _, err := exec.LookPath("yum"); err == nil { return yumManager{} }
	//   if _, err := exec.LookPath("apk"); err == nil { return apkManager{} }
	return nil
}

// ─── apt (Debian/Ubuntu) ────────────────────────────────────────────────────

// aptManager implements PackageManager via apt-get.
type aptManager struct{}

func (aptManager) Name() string { return "apt" }

func (aptManager) Update() error {
	if out, err := exec.Command("sudo", "apt-get", "update", "-qq").CombinedOutput(); err != nil {
		return fmt.Errorf("apt-get update failed: %w\n%s", err, string(out))
	}
	return nil
}

func (aptManager) Install(pkg string) error {
	if out, err := exec.Command("sudo", "apt-get", "install", "-y", "-qq", pkg).CombinedOutput(); err != nil {
		return fmt.Errorf("apt-get install %s failed: %w\n%s", pkg, err, string(out))
	}
	return nil
}

func (aptManager) Remove(pkg string) error {
	if out, err := exec.Command("sudo", "apt-get", "remove", "-y", "-qq", pkg).CombinedOutput(); err != nil {
		return fmt.Errorf("apt-get remove %s failed: %w\n%s", pkg, err, string(out))
	}
	return nil
}
