package handlers

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

// ProviderInstall installs a middleware provider via its Provider interface.
// POST /api/admin/v1/providers/{provider}/install
// Delegates to provider.CanInstall() and provider.Install() — no raw apt-get calls.
func (h *Handlers) ProviderInstall(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	p := h.providerForName(providerName)
	if p == nil {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported provider: %s (supported: caddy, haproxy)", providerName))
		return
	}

	if !p.CanInstall() {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("provider %s cannot be installed directly (shared binary or built-in)", providerName))
		return
	}

	// Check if already installed (binary exists in PATH)
	if _, err := exec.LookPath(providerName); err == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"provider": providerName,
			"status":   "already_installed",
			"message":  fmt.Sprintf("%s is already installed", providerName),
		})
		return
	}

	// Delegate to provider.Install()
	if err := p.Install(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"provider": providerName,
			"status":   "install_failed",
			"error":    err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider": providerName,
		"status":   "installed",
		"message":  fmt.Sprintf("%s installed and service started", providerName),
	})
}

// ProviderConfigPreview returns the current config for a provider.
// GET /api/admin/v1/providers/{provider}/config
// Delegates to provider.GetCurrentConfig() and provider.ConfigPath().
func (h *Handlers) ProviderConfigPreview(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	p := h.providerForName(providerName)
	if p == nil {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported provider: %s (supported: caddy, haproxy)", providerName))
		return
	}

	configPath := p.ConfigPath()
	if configPath == "" {
		writeError(w, http.StatusNotFound, "config path not configured")
		return
	}

	data, err := p.GetCurrentConfig()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"provider":    providerName,
			"config_path": configPath,
			"exists":      false,
			"content":     "",
			"error":       err.Error(),
		})
		return
	}

	exists := data != ""
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider":    providerName,
		"config_path": configPath,
		"exists":      exists,
		"content":     data,
	})
}
