package handlers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// providerInstallMap maps a provider name to the package name and post-install
// service name used by apt-get.
var providerInstallMap = map[string]struct {
	Package string
	Service string
}{
	"caddy":   {Package: "caddy", Service: "caddy"},
	"haproxy": {Package: "haproxy", Service: "haproxy"},
}

// ProviderInstall installs a middleware provider (caddy / haproxy) via apt-get.
// POST /api/admin/v1/providers/{provider}/install
func (h *Handlers) ProviderInstall(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	info, ok := providerInstallMap[providerName]
	if !ok {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported provider: %s (supported: caddy, haproxy)", providerName))
		return
	}

	// Check if already installed
	if _, err := exec.LookPath(providerName); err == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"provider": providerName,
			"status":   "already_installed",
			"message":  fmt.Sprintf("%s is already installed", providerName),
		})
		return
	}

	// Run apt-get install
	updateCmd := exec.Command("sudo", "apt-get", "update", "-qq")
	updateOut, _ := updateCmd.CombinedOutput()

	installCmd := exec.Command("sudo", "apt-get", "install", "-y", "-qq", info.Package)
	installOut, installErr := installCmd.CombinedOutput()

	if installErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"provider":        providerName,
			"status":          "install_failed",
			"message":         installErr.Error(),
			"apt_update_out":  string(updateOut),
			"apt_install_out": string(installOut),
		})
		return
	}

	// Enable and start the service
	enableCmd := exec.Command("sudo", "systemctl", "enable", "--now", info.Service)
	enableOut, enableErr := enableCmd.CombinedOutput()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider":       providerName,
		"status":         "installed",
		"message":        fmt.Sprintf("%s installed and %s service started", providerName, info.Service),
		"apt_update_out":  string(updateOut),
		"apt_install_out": string(installOut),
		"systemctl_out":  string(enableOut),
		"systemctl_error": func() string {
			if enableErr != nil {
				return enableErr.Error()
			}
			return ""
		}(),
	})
}

// ProviderConfigPreview returns the current config for a provider.
// GET /api/admin/v1/providers/{provider}/config
func (h *Handlers) ProviderConfigPreview(w http.ResponseWriter, r *http.Request) {
	providerName := strings.ToLower(r.PathValue("provider"))

	var configPath string
	switch providerName {
	case "caddy":
		if h.Config != nil {
			configPath = h.Config.Proxy.CaddyfilePath
		}
	case "haproxy":
		configPath = "/etc/haproxy/haproxy.cfg"
	default:
		writeError(w, http.StatusBadRequest, "unsupported provider: "+providerName)
		return
	}

	if configPath == "" {
		writeError(w, http.StatusNotFound, "config path not configured")
		return
	}

	data, err := os.ReadFile(configPath)
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider":    providerName,
		"config_path": configPath,
		"exists":      true,
		"content":     string(data),
	})
}
