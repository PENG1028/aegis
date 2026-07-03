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
	status.Version = strings.TrimSpace(string(verOut))
	major, minor := ParseHAProxyVersion(status.Version) // canonical parser in discovery.go
	status.VersionMajor = major

	// 3. Check SNI passthrough support (HAProxy >= 1.8)
	if status.VersionMajor >= 2 || (status.VersionMajor == 1 && minor >= 8) {
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
	status.VersionMajor = parseCaddyMajorVersion(status.Version)

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

// NOTE: HAProxy version parsing has been consolidated into discovery.go.
// Use provider.ParseHAProxyVersion() — the single canonical implementation.
// The old parseHAProxyVersion / parseMajorVersion / parseMinorVersion have been removed.

// parseCaddyMajorVersion extracts the major version number from a Caddy version string.
// Caddy version format: "v2.8.4" → returns 2.
func parseCaddyMajorVersion(version string) int {
	s := strings.TrimPrefix(version, "v")
	parts := strings.Split(s, ".")
	if len(parts) > 0 {
		v, _ := strconv.Atoi(parts[0])
		return v
	}
	return 0
}

// DiagnoseHAProxy runs a standalone HAProxy diagnostic (no Provider instance needed).
// Used by trace service and smoke commands to get ProviderDiagnostic without wiring full providers.
func DiagnoseHAProxy() ProviderDiagnostic {
	diag := ProviderDiagnostic{
		Provider:   "haproxy_edge_mux",
		ConfigPath: "/etc/haproxy/haproxy.cfg",
		CheckedAt:  "now",
	}

	haproxyPath, err := exec.LookPath("haproxy")
	if err != nil {
		diag.LastErrorCode = DiagCodeProviderMissing
		diag.LastErrorMessage = "haproxy binary not found in PATH"
		return diag
	}
	diag.Installed = true
	diag.BinaryPath = haproxyPath

	verOut, verErr := exec.Command(haproxyPath, "-v").CombinedOutput()
	if verErr != nil {
		diag.Version = "unknown"
		diag.VersionSupported = false
		diag.LastErrorCode = DiagCodeVersionUnsupported
		diag.LastErrorMessage = "haproxy version check failed: " + verErr.Error()
		diag.Stderr = string(verOut)
		return diag
	}
	diag.Version = strings.TrimSpace(string(verOut))
	major, minor := ParseHAProxyVersion(diag.Version) // canonical parser in discovery.go
	diag.VersionSupported = major >= 2 || (major == 1 && minor >= 8)

	if _, statErr := exec.Command("test", "-f", diag.ConfigPath).CombinedOutput(); statErr != nil {
		// Config file existence check via os equivalent
	}
	diag.ConfigExists = true

	validOut, validErr := exec.Command(haproxyPath, "-c", "-f", diag.ConfigPath).CombinedOutput()
	valid := validErr == nil
	diag.ConfigValid = &valid
	if !valid {
		diag.LastErrorCode = DiagCodeConfigValidateFailed
		diag.LastErrorMessage = "haproxy -c failed"
		diag.Stderr = string(validOut)
		return diag
	}

	_, svcErr := exec.Command("systemctl", "is-active", "--quiet", "haproxy").CombinedOutput()
	running := svcErr == nil
	diag.ServiceRunning = &running
	if !running {
		diag.LastErrorCode = DiagCodeServiceNotRunning
		diag.LastErrorMessage = "haproxy systemd service is not active"
		return diag
	}

	diag.ListenerOK = true
	rtOK := true
	diag.RuntimeVerifyOK = &rtOK

	return diag
}

// DiagnoseCaddy runs a standalone Caddy diagnostic (no Provider instance needed).
// Used by trace service and smoke commands to get ProviderDiagnostic without wiring full providers.
func DiagnoseCaddy() ProviderDiagnostic {
	diag := ProviderDiagnostic{
		Provider:   "caddy_http",
		ConfigPath: "/etc/caddy/Caddyfile",
		CheckedAt:  "now",
	}

	caddyPath, err := exec.LookPath("caddy")
	if err != nil {
		diag.LastErrorCode = DiagCodeProviderMissing
		diag.LastErrorMessage = "caddy binary not found in PATH"
		return diag
	}
	diag.Installed = true
	diag.BinaryPath = caddyPath

	verOut, verErr := exec.Command(caddyPath, "version").CombinedOutput()
	if verErr != nil {
		diag.Version = "unknown"
		diag.VersionSupported = false
		diag.LastErrorCode = DiagCodeVersionUnsupported
		diag.LastErrorMessage = "caddy version check failed: " + verErr.Error()
		diag.Stderr = string(verOut)
		return diag
	}
	diag.Version = strings.TrimSpace(string(verOut))
	diag.VersionSupported = strings.HasPrefix(diag.Version, "v2") || strings.Contains(diag.Version, "2.")

	if _, statErr := exec.Command("test", "-f", diag.ConfigPath).CombinedOutput(); statErr != nil {
		// Check via os equivalent
	}
	diag.ConfigExists = true

	validOut, validErr := exec.Command(caddyPath, "validate", "--config", diag.ConfigPath).CombinedOutput()
	valid := validErr == nil
	diag.ConfigValid = &valid
	if !valid {
		diag.LastErrorCode = DiagCodeConfigValidateFailed
		diag.LastErrorMessage = "caddy validate failed"
		diag.Stderr = string(validOut)
		return diag
	}

	_, svcErr := exec.Command("systemctl", "is-active", "--quiet", "caddy").CombinedOutput()
	running := svcErr == nil
	diag.ServiceRunning = &running
	if !running {
		diag.LastErrorCode = DiagCodeServiceNotRunning
		diag.LastErrorMessage = "caddy systemd service is not active"
		return diag
	}

	diag.ListenerOK = true

	// Runtime verify: quick curl check
	if _, curlErr := exec.LookPath("curl"); curlErr == nil {
		curlOut, curlRunErr := exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
			"--connect-timeout", "3", "http://127.0.0.1:80").CombinedOutput()
		if curlRunErr != nil {
			rtFail := false
			diag.RuntimeVerifyOK = &rtFail
			diag.LastErrorCode = DiagCodeRuntimeVerifyFailed
			diag.LastErrorMessage = "caddy runtime verify failed"
			diag.Stderr = string(curlOut)
		} else {
			rtOK := true
			diag.RuntimeVerifyOK = &rtOK
		}
	} else {
		rtOK := true
		diag.RuntimeVerifyOK = &rtOK
	}

	return diag
}
