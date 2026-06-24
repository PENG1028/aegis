package provider

import (
	"os/exec"
	"strconv"
	"strings"
)

// ProviderStatus represents the current state of a provider.
type ProviderStatus struct {
	Name                string `json:"name"`
	Status              string `json:"status"` // available | missing_binary | config_invalid | service_not_running | reload_failed | unknown
	Installed           bool   `json:"installed"`
	Version             string `json:"version"`
	VersionMajor        int    `json:"version_major"`
	ConfigValid         *bool  `json:"config_valid,omitempty"`
	ServiceRunning      *bool  `json:"service_running,omitempty"`
	SNIPassthroughReady bool   `json:"sni_passthrough_ready"`
	EdgeMuxReady        bool   `json:"edge_mux_ready"`
	Message             string `json:"message"`
}

// CheckHAProxyStatus runs diagnostics for the HAProxy EdgeMux provider.
func CheckHAProxyStatus(configPath string) ProviderStatus {
	status := ProviderStatus{
		Name:     "haproxy_edge_mux",
		Status:   "unknown",
		Installed: false,
	}

	// 1. Look for binary
	haproxyPath, err := exec.LookPath("haproxy")
	if err != nil {
		status.Status = "missing_binary"
		status.Message = "haproxy binary not found in PATH"
		return status
	}
	status.Installed = true

	// 2. Get version
	verOut, err := exec.Command(haproxyPath, "-vv").CombinedOutput()
	if err != nil {
		status.Status = "unknown"
		status.Message = "haproxy -vv failed: " + err.Error()
		return status
	}
	status.Version = parseHAProxyVersion(string(verOut))
	status.VersionMajor = parseMajorVersion(status.Version)

	// 3. Check SNI passthrough support (HAProxy >= 1.8)
	if status.VersionMajor >= 2 || (status.VersionMajor == 1 && parseMinorVersion(status.Version) >= 8) {
		status.SNIPassthroughReady = true
	}

	// 4. Validate config if exists
	if configPath != "" {
		validOut, validErr := exec.Command(haproxyPath, "-c", "-f", configPath).CombinedOutput()
		valid := validErr == nil
		status.ConfigValid = &valid
		if !valid {
			status.Status = "config_invalid"
			status.Message = "haproxy -c failed: " + string(validOut)
			return status
		}
	}

	// 5. Check if haproxy service is running
	_, svcErr := exec.Command("systemctl", "is-active", "--quiet", "haproxy").CombinedOutput()
	running := svcErr == nil
	status.ServiceRunning = &running
	if !running {
		status.Status = "service_not_running"
		status.Message = "haproxy systemd service is not active"
	} else {
		status.Status = "available"
		status.Message = "haproxy is running and configured"
		status.EdgeMuxReady = status.SNIPassthroughReady
	}

	return status
}

// CheckCaddyStatus runs diagnostics for the Caddy HTTP provider.
func CheckCaddyStatus(configPath string) ProviderStatus {
	status := ProviderStatus{
		Name:     "caddy_http",
		Status:   "unknown",
		Installed: false,
	}

	caddyPath, err := exec.LookPath("caddy")
	if err != nil {
		status.Status = "missing_binary"
		status.Message = "caddy binary not found in PATH"
		return status
	}
	status.Installed = true

	verOut, err := exec.Command(caddyPath, "version").CombinedOutput()
	if err != nil {
		status.Status = "unknown"
		status.Message = "caddy version failed"
		return status
	}
	status.Version = strings.TrimSpace(string(verOut))
	status.VersionMajor = parseMajorVersion(status.Version)

	if configPath != "" {
		validOut, validErr := exec.Command(caddyPath, "validate", "--config", configPath).CombinedOutput()
		valid := validErr == nil
		status.ConfigValid = &valid
		if !valid {
			status.Status = "config_invalid"
			status.Message = "caddy validate failed: " + string(validOut)
			return status
		}
	}

	_, svcErr := exec.Command("systemctl", "is-active", "--quiet", "caddy").CombinedOutput()
	running := svcErr == nil
	status.ServiceRunning = &running
	if running {
		status.Status = "available"
		status.Message = "caddy is running"
	} else {
		status.Status = "service_not_running"
		status.Message = "caddy systemd service is not active"
	}

	return status
}

func parseHAProxyVersion(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "HAProxy") && strings.Contains(line, "version") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "version" && i+1 < len(fields) {
					return strings.TrimRight(fields[i+1], ",")
				}
			}
		}
	}
	return "unknown"
}

func parseMajorVersion(version string) int {
	parts := strings.Split(version, ".")
	if len(parts) > 0 {
		v, _ := strconv.Atoi(parts[0])
		return v
	}
	return 0
}

func parseMinorVersion(version string) int {
	parts := strings.Split(version, ".")
	if len(parts) > 1 {
		v, _ := strconv.Atoi(strings.TrimRight(parts[1], "-0123456789"))
		return v
	}
	return 0
}
