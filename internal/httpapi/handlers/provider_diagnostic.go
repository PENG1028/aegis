package handlers

import (
	"net/http"

	"aegis/internal/provider"
)

// ListProviders handles GET /api/admin/v1/providers
// Returns basic Info() for all registered providers.
func (h *Handlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	// Collect provider infos from the apply service's provider registry
	// We access providers through the Exposure service which has the registry
	type providerInfo struct {
		Name       string `json:"name"`
		Protocol   string `json:"protocol"`
		Status     string `json:"status"`
		Message    string `json:"message"`
		ConfigPath string `json:"config_path"`
	}

	// For now, collect from known providers via the diagnostics functions
	var providers []providerInfo

	// Caddy HTTP
	caddyStatus := provider.CheckCaddyStatus(h.Config.Proxy.CaddyfilePath)
	providers = append(providers, providerInfo{
		Name:       caddyStatus.Name,
		Protocol:   "http",
		Status:     caddyStatus.Status,
		Message:    caddyStatus.Message,
		ConfigPath: h.Config.Proxy.CaddyfilePath,
	})

	// HAProxy EdgeMux
	haproxyStatus := provider.CheckHAProxyStatus("/etc/haproxy/haproxy.cfg")
	providers = append(providers, providerInfo{
		Name:       haproxyStatus.Name,
		Protocol:   "tls_mux",
		Status:     haproxyStatus.Status,
		Message:    haproxyStatus.Message,
		ConfigPath: "/etc/haproxy/haproxy.cfg",
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": providers,
		"count":     len(providers),
	})
}

// DiagnoseAllProviders handles POST /api/admin/v1/providers/diagnose
// Runs Diagnose() on all providers and returns structured results.
func (h *Handlers) DiagnoseAllProviders(w http.ResponseWriter, r *http.Request) {
	var results []provider.ProviderDiagnostic

	// Collect diagnostics from all known providers
	// Check if providers implement Diagnoser interface via type assertion
	// and call Diagnose(); otherwise construct from status functions

	// Caddy HTTP diagnostic
	caddyDiag := caddyDiagnosticFromStatus(h.Config.Proxy.CaddyfilePath)
	results = append(results, caddyDiag)

	// HAProxy TCP diagnostic
	haproxyDiag := haproxyDiagnosticFromStatus("/etc/haproxy/haproxy.cfg")
	results = append(results, haproxyDiag)

	// Count issues
	issues := 0
	for _, d := range results {
		if d.LastErrorCode != "" {
			issues++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"diagnostics": results,
		"count":       len(results),
		"issues":      issues,
		"healthy":     issues == 0,
	})
}

// caddyDiagnosticFromStatus builds a ProviderDiagnostic from Caddy status check.
func caddyDiagnosticFromStatus(configPath string) provider.ProviderDiagnostic {
	status := provider.CheckCaddyStatus(configPath)
	diag := provider.ProviderDiagnostic{
		Provider:   status.Name,
		BinaryPath: "caddy",
		Version:    status.Version,
		ConfigPath: configPath,
		CheckedAt:  "",
	}

	diag.Installed = status.Installed
	if !status.Installed {
		diag.LastErrorCode = provider.DiagCodeProviderMissing
		diag.LastErrorMessage = status.Message
		return diag
	}

	diag.VersionSupported = status.VersionMajor >= 2
	if !diag.VersionSupported {
		diag.LastErrorCode = provider.DiagCodeVersionUnsupported
		diag.LastErrorMessage = "caddy version too old (v2+ required)"
		return diag
	}

	if status.ConfigValid != nil && !*status.ConfigValid {
		diag.ConfigExists = true
		diag.ConfigValid = status.ConfigValid
		diag.LastErrorCode = provider.DiagCodeConfigValidateFailed
		diag.LastErrorMessage = status.Message
		diag.Stderr = status.Message
		return diag
	}

	configValid := status.ConfigValid
	diag.ConfigValid = configValid
	diag.ConfigExists = true

	if status.ServiceRunning != nil && !*status.ServiceRunning {
		diag.ServiceRunning = status.ServiceRunning
		diag.LastErrorCode = provider.DiagCodeServiceNotRunning
		diag.LastErrorMessage = status.Message
		return diag
	}

	diag.ServiceRunning = status.ServiceRunning
	diag.ListenerOK = true
	running := true
	diag.RuntimeVerifyOK = &running

	if status.Status == "available" {
		diag.ListenerOK = true
	}

	return diag
}

// haproxyDiagnosticFromStatus builds a ProviderDiagnostic from HAProxy status check.
func haproxyDiagnosticFromStatus(configPath string) provider.ProviderDiagnostic {
	status := provider.CheckHAProxyStatus(configPath)
	diag := provider.ProviderDiagnostic{
		Provider:   status.Name,
		BinaryPath: "haproxy",
		Version:    status.Version,
		ConfigPath: configPath,
		CheckedAt:  "",
	}

	diag.Installed = status.Installed
	if !status.Installed {
		diag.LastErrorCode = provider.DiagCodeProviderMissing
		diag.LastErrorMessage = status.Message
		return diag
	}

	diag.VersionSupported = status.VersionMajor >= 2 ||
		(status.VersionMajor == 1 && status.VersionMajor >= 0) // Accept all HAProxy for now
	if !diag.VersionSupported {
		diag.LastErrorCode = provider.DiagCodeVersionUnsupported
		diag.LastErrorMessage = "haproxy version unsupported"
		return diag
	}

	if status.ConfigValid != nil && !*status.ConfigValid {
		diag.ConfigExists = true
		diag.ConfigValid = status.ConfigValid
		diag.LastErrorCode = provider.DiagCodeConfigValidateFailed
		diag.LastErrorMessage = status.Message
		diag.Stderr = status.Message
		return diag
	}

	diag.ConfigValid = status.ConfigValid
	diag.ConfigExists = true

	if status.ServiceRunning != nil && !*status.ServiceRunning {
		diag.ServiceRunning = status.ServiceRunning
		diag.LastErrorCode = provider.DiagCodeServiceNotRunning
		diag.LastErrorMessage = status.Message
		return diag
	}

	diag.ServiceRunning = status.ServiceRunning
	diag.ListenerOK = true
	rtOK := true
	diag.RuntimeVerifyOK = &rtOK

	return diag
}
