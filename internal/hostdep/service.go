package hostdep

import (
	"fmt"
	"os/exec"
)

// Minimal systemd service control used by the install/remove flow. A full host
// service manager (start/stop/restart/reload/is-active) is a later step of the
// host-dependency refactor; for now only enable/stop are needed here.

// enableService enables and starts a systemd service.
func enableService(service string) error {
	if out, err := exec.Command("sudo", "systemctl", "enable", "--now", service).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable %s failed: %w\n%s", service, err, string(out))
	}
	return nil
}

// stopService stops a systemd service (best-effort; errors are ignored by the
// caller since removal proceeds regardless).
func stopService(service string) {
	exec.Command("sudo", "systemctl", "stop", service).Run()
}
