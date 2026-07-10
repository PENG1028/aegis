package handlers

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

func (h *Handlers) InfraInstall(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(r.PathValue("name"))
	pkg := infraPackage(name)
	if pkg == "" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown infra: %s", name))
		return
	}
	if _, err := exec.LookPath(name); err == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"name": name, "status": "installed", "message": "already present",
		})
		return
	}
	out, err := exec.Command("apt-get", "install", "-y", "-qq", pkg).CombinedOutput()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"name": name, "status": "failed", "error": fmt.Sprintf("apt-get: %v — %s", err, string(out)),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name": name, "status": "installed",
	})
}

func (h *Handlers) InfraUninstall(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(r.PathValue("name"))
	pkg := infraPackage(name)
	if pkg == "" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown infra: %s", name))
		return
	}
	out, err := exec.Command("apt-get", "remove", "-y", "-qq", pkg).CombinedOutput()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"name": name, "status": "failed", "error": fmt.Sprintf("apt-get: %v — %s", err, string(out)),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name": name, "status": "uninstalled",
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
