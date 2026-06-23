package handlers

import (
	"net/http"
)

func (h *Handlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"proxy": h.Config.Proxy,
		"store": h.Config.Store,
		"server": h.Config.Server,
		"managed_domain": h.Config.ManagedDomain,
		"runtime": h.Config.Runtime,
	})
}

func (h *Handlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "settings update not implemented yet")
}
