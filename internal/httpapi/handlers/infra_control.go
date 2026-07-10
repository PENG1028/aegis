package handlers

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

// InfraInstall handles POST /api/admin/v1/infra/{name}/install
// Installs infrastructure dependencies via apt.
func (h *Handlers) InfraInstall(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(r.PathValue("name"))

	pkg := infraPackage(name)
	if pkg == "" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown infra: %s (supported: dnsmasq, certbot)", name))
		return
	}

	// Check if already installed
	if _, err := exec.LookPath(name); err == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"name":   name,
			"status": "already_installed",
		})
		return
	}

	cmd := exec.Command("apt-get", "install", "-y", "-qq", pkg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeError(w, http.StatusInternalServerError,
			fmt.Sprintf("install failed: %v — %s", err, string(out)))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":   name,
		"status": "installed",
	})
}

// InfraUninstall handles DELETE /api/admin/v1/infra/{name}
func (h *Handlers) InfraUninstall(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(r.PathValue("name"))

	pkg := infraPackage(name)
	if pkg == "" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown infra: %s", name))
		return
	}

	cmd := exec.Command("apt-get", "remove", "-y", "-qq", pkg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeError(w, http.StatusInternalServerError,
			fmt.Sprintf("uninstall failed: %v — %s", err, string(out)))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":   name,
		"status": "uninstalled",
	})
}

func infraPackage(name string) string {
	switch name {
	case "dnsmasq":
		return "dnsmasq"
	case "certbot":
		return "certbot"
	default:
		return ""
	}
}
