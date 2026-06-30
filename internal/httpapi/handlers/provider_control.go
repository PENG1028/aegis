package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// providerServiceMap maps a provider name to its systemd service name.
var providerServiceMap = map[string]string{
	"caddy":   "caddy",
	"haproxy": "haproxy",
}

// providerConfigPath returns the config file path for a provider.
func providerConfigPath(providerName string, caddyfilePath string) string {
	switch providerName {
	case "caddy":
		if caddyfilePath != "" {
			return caddyfilePath
		}
		return "/etc/caddy/Caddyfile"
	case "haproxy":
		return "/etc/haproxy/haproxy.cfg"
	default:
		return ""
	}
}

// ProviderReload handles POST /api/admin/v1/providers/{provider}/reload
func (h *Handlers) ProviderReload(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	svc, ok := providerServiceMap[providerName]
	if !ok {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported provider: %s (supported: caddy, haproxy)", providerName))
		return
	}

	out, err := exec.Command("systemctl", "reload", svc).CombinedOutput()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"provider": providerName,
			"action":   "reload",
			"status":   "failed",
			"error":    fmt.Sprintf("%v: %s", err, string(out)),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider": providerName,
		"action":   "reload",
		"status":   "success",
		"output":   strings.TrimSpace(string(out)),
	})
}

// ProviderServiceControl handles POST /api/admin/v1/providers/{provider}/service
// Body: {"action": "start" | "stop" | "restart"}
func (h *Handlers) ProviderServiceControl(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	svc, ok := providerServiceMap[providerName]
	if !ok {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported provider: %s (supported: caddy, haproxy)", providerName))
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
		// valid
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

	// Verify the new state
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

	info, ok := providerInstallMap[providerName]
	if !ok {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported provider: %s (supported: caddy, haproxy)", providerName))
		return
	}

	// Stop the service first
	exec.Command("systemctl", "stop", info.Service).Run()

	// Purge the package
	purgeCmd := exec.Command("sudo", "apt-get", "remove", "-y", "-qq", info.Package)
	purgeOut, purgeErr := purgeCmd.CombinedOutput()

	if purgeErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"provider":     providerName,
			"status":       "uninstall_failed",
			"error":        fmt.Sprintf("%v: %s", purgeErr, string(purgeOut)),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider": providerName,
		"status":   "uninstalled",
		"message":  fmt.Sprintf("%s removed. Config files preserved in /etc/%s/ (delete manually if needed).", providerName, providerName),
	})
}

// ProviderSaveConfig handles PUT /api/admin/v1/providers/{provider}/config
// Body: {"content": "<full config file content>"}
func (h *Handlers) ProviderSaveConfig(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	configPath := providerConfigPath(providerName, "")
	if h.Config != nil && providerName == "caddy" {
		configPath = providerConfigPath(providerName, h.Config.Proxy.CaddyfilePath)
	}

	if configPath == "" {
		writeError(w, http.StatusBadRequest, "unknown config path for provider: "+providerName)
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

	// Validate config before saving (if provider supports it)
	validationWarning := ""
	switch providerName {
	case "caddy":
		// Find caddy binary
		caddyBin := "caddy"
		if h.Config != nil && h.Config.Proxy.CaddyBinary != "" {
			caddyBin = h.Config.Proxy.CaddyBinary
		}
		// Write to temp file for validation
		tmpFile := configPath + ".validate.tmp"
		if err := os.WriteFile(tmpFile, []byte(req.Content), 0644); err == nil {
			defer os.Remove(tmpFile)
			if out, err := exec.Command(caddyBin, "validate", "--adapter", "caddyfile", "--config", tmpFile).CombinedOutput(); err != nil {
				validationWarning = fmt.Sprintf("config validation failed: %v — %s", err, string(out))
				// Still save, but warn
			}
		}
	case "haproxy":
		tmpFile := configPath + ".validate.tmp"
		if err := os.WriteFile(tmpFile, []byte(req.Content), 0644); err == nil {
			defer os.Remove(tmpFile)
			if out, err := exec.Command("haproxy", "-c", "-f", tmpFile).CombinedOutput(); err != nil {
				validationWarning = fmt.Sprintf("config validation failed: %v — %s", err, string(out))
			}
		}
	}

	// Backup existing config
	if existing, err := os.ReadFile(configPath); err == nil {
		backupPath := configPath + ".bak"
		os.WriteFile(backupPath, existing, 0644)
	}

	// Write new config
	if err := os.WriteFile(configPath, []byte(req.Content+"\n"), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "save config: "+err.Error())
		return
	}

	result := map[string]interface{}{
		"provider":    providerName,
		"config_path": configPath,
		"status":      "saved",
		"message":     "config written to " + configPath,
	}
	if validationWarning != "" {
		result["validation_warning"] = validationWarning
	}

	writeJSON(w, http.StatusOK, result)
}
