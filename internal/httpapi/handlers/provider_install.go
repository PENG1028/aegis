package handlers

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"aegis/internal/provider"
)

// ProviderInstall installs a middleware provider.
// POST /api/admin/v1/providers/{provider}/install
func (h *Handlers) ProviderInstall(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	p := h.providerForName(providerName)
	if p == nil {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported provider: %s (supported: caddy, haproxy)", providerName))
		return
	}

	// Check for optional LifecycleProvider interface
	lc, ok := p.(provider.LifecycleProvider)
	if !ok || !lc.CanInstall() {
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

	if err := lc.Install(); err != nil {
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
func (h *Handlers) ProviderConfigPreview(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	p := h.providerForName(providerName)
	if p == nil {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported provider: %s (supported: caddy, haproxy)", providerName))
		return
	}

	configPath := p.State().ConfigPath
	if configPath == "" {
		writeError(w, http.StatusNotFound, "config path not configured")
		return
	}

	// Try ConfigReader optional interface
	var data string
	var err error
	if reader, ok := p.(provider.ConfigReader); ok {
		data, err = reader.GetCurrentConfig()
	} else {
		writeError(w, http.StatusNotFound, "provider does not support config reading")
		return
	}

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
