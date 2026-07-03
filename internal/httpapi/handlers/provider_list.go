package handlers

import (
	"net/http"

	"aegis/internal/provider"
)

// ListProviders returns all registered providers with their current state.
// GET /api/admin/v1/providers
func (h *Handlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	if h.ProvReg == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"providers": []interface{}{},
		})
		return
	}

	states := h.ProvReg.List()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": states,
	})
}

// DiagnoseAllProviders runs diagnostics on all registered providers.
// POST /api/admin/v1/providers/diagnose
func (h *Handlers) DiagnoseAllProviders(w http.ResponseWriter, r *http.Request) {
	if h.ProvReg == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"results": []interface{}{},
		})
		return
	}

	results := provider.DiscoverProviders(h.ProvReg)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
	})
}
