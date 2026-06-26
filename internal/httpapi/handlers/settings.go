package handlers

import (
	"net/http"
)

func (h *Handlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	// Sanitize server config — never expose the admin token
	serverSafe := h.Config.Server
	if len(serverSafe.AdminToken) > 8 {
		serverSafe.AdminToken = serverSafe.AdminToken[:4] + "..." + serverSafe.AdminToken[len(serverSafe.AdminToken)-4:]
	} else {
		serverSafe.AdminToken = "***REDACTED***"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"proxy":          h.Config.Proxy,
		"store":          h.Config.Store,
		"server":         serverSafe,
		"managed_domain": h.Config.ManagedDomain,
		"runtime":        h.Config.Runtime,
	})
}

func (h *Handlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "settings update not implemented yet")
}
