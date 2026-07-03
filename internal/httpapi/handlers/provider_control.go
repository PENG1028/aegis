package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"aegis/internal/provider"
)

// providerForName looks up a Provider from the registry by its URL-friendly name.
// Maps: "caddy" → "caddy", "haproxy" → "haproxy".
func (h *Handlers) providerForName(name string) provider.Provider {
	if h.ProvReg == nil {
		return nil
	}
	return h.ProvReg.Get(name)
}

// ProviderReload handles POST /api/admin/v1/providers/{provider}/reload
func (h *Handlers) ProviderReload(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	p := h.providerForName(providerName)
	if p == nil {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported provider: %s (supported: caddy, haproxy)", providerName))
		return
	}

	// Check for optional ReloadableProvider interface
	reloadable, ok := p.(provider.ReloadableProvider)
	if !ok {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("provider %s does not support standalone reload", providerName))
		return
	}

	if err := reloadable.Reload(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"provider": providerName,
			"action":   "reload",
			"status":   "failed",
			"error":    err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider": providerName,
		"action":   "reload",
		"status":   "success",
	})
}

// ProviderServiceControl handles POST /api/admin/v1/providers/{provider}/service
// Body: {"action": "start" | "stop" | "restart"}
func (h *Handlers) ProviderServiceControl(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	p := h.providerForName(providerName)
	if p == nil {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported provider: %s (supported: caddy, haproxy)", providerName))
		return
	}

	// Derive systemd service name from provider ID
	svc := p.State().ID
	if svc == "" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("provider %s has no service", providerName))
		return
	}

	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	action := strings.ToLower(req.Action)
	switch action {
	case "start", "stop", "restart":
	default:
		writeError(w, http.StatusBadRequest, "action must be one of: start, stop, restart")
		return
	}

	cmd := exec.Command("systemctl", action, svc)
	out, err := cmd.CombinedOutput()

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"provider": providerName,
			"action":   action,
			"status":   "failed",
			"error":    fmt.Sprintf("%v: %s", err, string(out)),
		})
		return
	}

	verifyCmd := exec.Command("systemctl", "is-active", svc)
	verifyOut, _ := verifyCmd.CombinedOutput()
	running := strings.TrimSpace(string(verifyOut)) == "active"

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider": providerName,
		"action":   action,
		"status":   "success",
		"running":  running,
		"output":   strings.TrimSpace(string(out)),
	})
}

// ProviderUninstall handles DELETE /api/admin/v1/providers/{provider}
func (h *Handlers) ProviderUninstall(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	p := h.providerForName(providerName)
	if p == nil {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported provider: %s (supported: caddy, haproxy)", providerName))
		return
	}

	// Check for optional LifecycleProvider interface
	lc, ok := p.(provider.LifecycleProvider)
	if !ok || !lc.CanUninstall() {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("provider %s cannot be uninstalled (shared binary or built-in)", providerName))
		return
	}

	if err := lc.Uninstall(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"provider": providerName,
			"status":   "uninstall_failed",
			"error":    err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider": providerName,
		"status":   "uninstalled",
		"message":  fmt.Sprintf("%s removed. Config files preserved.", providerName),
	})
}

// ProviderSaveConfig handles PUT /api/admin/v1/providers/{provider}/config
// Body: {"content": "<full config file content>"}
func (h *Handlers) ProviderSaveConfig(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	p := h.providerForName(providerName)
	if p == nil {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported provider: %s (supported: caddy, haproxy)", providerName))
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	configPath := p.State().ConfigPath

	// Delegate to provider.Apply() — handles validate → backup → write → reload
	if err := p.Apply([]provider.ConfigFile{
		{Path: configPath, Content: []byte(req.Content + "\n")},
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"provider":    providerName,
			"config_path": configPath,
			"status":      "save_failed",
			"error":       err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider":    providerName,
		"config_path": configPath,
		"status":      "saved",
		"message":     "config validated, backed up, written, and reloaded",
	})
}
