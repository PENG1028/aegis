package handlers

import (
	"net/http"

	"aegis/internal/provider"
)

// enrichedProvider extends ProviderState with theoretical capabilities for the UI.
type enrichedProvider struct {
	provider.ProviderState
	TheoreticalCapabilities []provider.Capability `json:"theoretical_capabilities"`
}

// ListProviders returns all registered providers with their current state,
// plus the capability universe and theoretical capabilities for the matrix UI.
// GET /api/admin/v1/providers
func (h *Handlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	if h.ProvReg == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"providers":           []interface{}{},
			"capability_universe": provider.AllCapabilities(),
		})
		return
	}

	states := h.ProvReg.List()
	enriched := make([]enrichedProvider, 0, len(states))
	for _, s := range states {
		enriched = append(enriched, enrichedProvider{
			ProviderState:           s,
			TheoreticalCapabilities: provider.TheoreticalMaxCapabilities(s.GatewayType),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers":           enriched,
		"capability_universe": provider.AllCapabilities(),
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
