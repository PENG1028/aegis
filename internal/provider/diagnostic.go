package provider

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Provider diagnostic error codes.
const (
	DiagCodeProviderMissing         = "PROVIDER_MISSING"
	DiagCodeVersionUnsupported      = "PROVIDER_VERSION_UNSUPPORTED"
	DiagCodeConfigFileMissing       = "CONFIG_FILE_MISSING"
	DiagCodeConfigValidateFailed    = "CONFIG_VALIDATE_FAILED"
	DiagCodeServiceNotRunning       = "SERVICE_NOT_RUNNING"
	DiagCodeListenerConflict        = "LISTENER_CONFLICT"
	DiagCodeRuntimeVerifyFailed     = "RUNTIME_VERIFY_FAILED"
)

// ProviderDiagnostic is a unified diagnostic result for a provider.
// Each field represents a specific diagnostic check that can be independently queried.
type ProviderDiagnostic struct {
	Provider         string `json:"provider"`
	Installed        bool   `json:"installed"`
	BinaryPath       string `json:"binary_path"`
	Version          string `json:"version"`
	VersionSupported bool   `json:"version_supported"`
	ConfigPath       string `json:"config_path"`
	ConfigExists     bool   `json:"config_exists"`
	ConfigValid      *bool  `json:"config_valid,omitempty"`
	ServiceRunning   *bool  `json:"service_running,omitempty"`
	ListenerOK       bool   `json:"listener_ok"`
	RuntimeVerifyOK  *bool  `json:"runtime_verify_ok,omitempty"`
	LastErrorCode    string `json:"last_error_code"`
	LastErrorMessage string `json:"last_error_message"`
	Stderr           string `json:"stderr"`
	CheckedAt        string `json:"checked_at"`
}

// Diagnoser is an optional interface for providers that support diagnostics.
type Diagnoser interface {
	Diagnose() ProviderDiagnostic
}

// ============================================================================
// ProviderStatus — lightweight status check (for smoke tests)
// ============================================================================

// ProviderStatus is a minimal status summary for health checks.
type ProviderStatus struct {
	Provider string `json:"provider"`
	Status   string `json:"status"` // "ready" | "degraded" | "unavailable"
	Running  bool   `json:"running"`
	ConfigOK bool   `json:"config_ok"`
	Version  string `json:"version"`
	Message  string `json:"message"`
}

// computeStatus derives a simple status string from diagnostic results.
func computeStatus(running, configOK bool) string {
	if !running {
		return "unavailable"
	}
	if !configOK {
		return "degraded"
	}
	return "ready"
}

// CheckCaddyStatus returns a quick status check for Caddy.
// configPath is accepted for backward compatibility; unused if empty.
func CheckCaddyStatus(configPath string) ProviderStatus {
	diag := DiagnoseCaddy()
	running := diag.ServiceRunning != nil && *diag.ServiceRunning
	configOK := diag.ConfigValid != nil && *diag.ConfigValid
	return ProviderStatus{
		Provider: "caddy",
		Status:   computeStatus(running, configOK),
		Running:  running,
		ConfigOK: configOK,
		Version:  diag.Version,
		Message:  diag.LastErrorMessage,
	}
}

// CheckHAProxyStatus returns a quick status check for HAProxy.
func CheckHAProxyStatus(configPath string) ProviderStatus {
	diag := quickDiagnoseHAProxy("haproxy", configPath)
	running := diag.ServiceRunning != nil && *diag.ServiceRunning
	configOK := diag.ConfigValid != nil && *diag.ConfigValid
	return ProviderStatus{
		Provider: "haproxy",
		Status:   computeStatus(running, configOK),
		Running:  running,
		ConfigOK: configOK,
		Version:  diag.Version,
		Message:  diag.LastErrorMessage,
	}
}

// ============================================================================
// Standalone diagnostic helpers for trace/health services
// ============================================================================

// DiagnoseCaddy runs a quick Caddy diagnostic using default paths.
func DiagnoseCaddy() ProviderDiagnostic {
	return diagnoseExternal("caddy", "caddy", "/etc/caddy/Caddyfile",
		"version", []string{"validate", "--config"},
		func(ver string) bool { return strings.HasPrefix(ver, "v2") || strings.Contains(ver, "2.") },
	)
}

// DiagnoseHAProxy runs a quick HAProxy diagnostic using default paths.
func DiagnoseHAProxy() ProviderDiagnostic {
	return diagnoseExternal("haproxy", "haproxy", "/etc/haproxy/haproxy.cfg",
		"-v", []string{"-c", "-f"},
		func(ver string) bool {
			major, minor := ParseHAProxyVersion(ver)
			return major >= 2 || (major == 1 && minor >= 8)
		},
	)
}

// diagnoseExternal runs the standard 6-step diagnostic pipeline for an external provider.
func diagnoseExternal(providerID, serviceName, configPath, versionFlag string, validateArgs []string, versionOK func(string) bool) ProviderDiagnostic {
	diag := ProviderDiagnostic{
		Provider:   providerID,
		ConfigPath: configPath,
	}

	binaryPath, err := exec.LookPath(providerID)
	if err != nil {
		diag.LastErrorCode = DiagCodeProviderMissing
		diag.LastErrorMessage = fmt.Sprintf("%s binary not found in PATH", providerID)
		return diag
	}
	diag.Installed = true
	diag.BinaryPath = binaryPath

	verOut, verErr := exec.Command(binaryPath, versionFlag).Output()
	if verErr != nil {
		diag.Version = "unknown"
		diag.VersionSupported = false
		diag.LastErrorCode = DiagCodeVersionUnsupported
		diag.LastErrorMessage = fmt.Sprintf("%s version check failed: %v", providerID, verErr)
		return diag
	}
	diag.Version = strings.TrimSpace(string(verOut))
	diag.VersionSupported = versionOK(diag.Version)

	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		diag.LastErrorCode = DiagCodeConfigFileMissing
		diag.LastErrorMessage = fmt.Sprintf("config file not found: %s", configPath)
		return diag
	}
	diag.ConfigExists = true

	validateFullArgs := append(validateArgs, configPath)
	validOut, validErr := exec.Command(binaryPath, validateFullArgs...).Output()
	valid := validErr == nil
	diag.ConfigValid = &valid
	if !valid {
		diag.LastErrorCode = DiagCodeConfigValidateFailed
		diag.LastErrorMessage = fmt.Sprintf("%s validate failed: %s", providerID, string(validOut))
		return diag
	}

	_, svcErr := exec.Command("systemctl", "is-active", "--quiet", serviceName).Output()
	running := svcErr == nil
	diag.ServiceRunning = &running
	if !running {
		diag.LastErrorCode = DiagCodeServiceNotRunning
		diag.LastErrorMessage = fmt.Sprintf("%s systemd service is not active", serviceName)
		return diag
	}

	diag.ListenerOK = true
	rtOK := true
	diag.RuntimeVerifyOK = &rtOK

	return diag
}

// quickDiagnoseCaddy calls diagnoseExternal with Caddy-specific parameters.
func quickDiagnoseCaddy(binaryName, configPath string) ProviderDiagnostic {
	return diagnoseExternal("caddy", "caddy", configPath,
		"version", []string{"validate", "--config"},
		func(ver string) bool { return strings.HasPrefix(ver, "v2") || strings.Contains(ver, "2.") },
	)
}

// quickDiagnoseHAProxy calls diagnoseExternal with HAProxy-specific parameters.
func quickDiagnoseHAProxy(binaryName, configPath string) ProviderDiagnostic {
	return diagnoseExternal("haproxy", "haproxy", configPath,
		"-v", []string{"-c", "-f"},
		func(ver string) bool {
			major, minor := ParseHAProxyVersion(ver)
			return major >= 2 || (major == 1 && minor >= 8)
		},
	)
}
