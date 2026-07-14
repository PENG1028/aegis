package handlers

import (
	"context"
	"net/http"

	"aegis/internal/hostdep/provider"
)

// driftRoutes converts DB routes to RouteSpecs for drift comparison.
func (h *Handlers) driftRoutes(ctx context.Context) []provider.RouteSpec {
	routes, err := h.Route.ListRoutes(ctx)
	if err != nil || routes == nil {
		return nil
	}
	specs := make([]provider.RouteSpec, 0, len(routes))
	for _, r := range routes {
		specs = append(specs, provider.RouteSpec{
			Transport: "tcp",
			Match: provider.MatchSpec{
				Host: r.Domain,
				Path: r.PathPrefix,
			},
			AppProtocol: "http",
			Upstream: provider.UpstreamSpec{
				Type:   "http",
				Target: r.ServiceID, // 精确目标需要查 endpoint
			},
		})
	}
	return specs
}

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

// ProviderDrift reads the actual config and compares with DB state.
// GET /api/admin/v1/providers/{provider}/drift
func (h *Handlers) ProviderDrift(w http.ResponseWriter, r *http.Request) {
	providerID := r.PathValue("provider")
	if h.ProvReg == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"provider_id": providerID,
			"error":       "provider registry not available",
		})
		return
	}

	prov := h.ProvReg.Get(providerID)
	if prov == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"provider_id": providerID,
			"error":       "provider not found",
		})
		return
	}
	reader, ok := prov.(provider.Reader)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"provider_id": providerID,
			"error":       "provider does not support config read-back",
		})
		return
	}

	snapshot, err := reader.ReadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"provider_id": providerID,
			"error":       err.Error(),
		})
		return
	}

	// Count DB routes for comparison
	dbRouteCount := 0
	if routes, err := h.Route.ListRoutes(r.Context()); err == nil {
		dbRouteCount = len(routes)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider_id":      snapshot.ProviderID,
		"db_routes":        dbRouteCount,
		"config_routes":    len(snapshot.Routes),
		"routes":           snapshot.Routes,
		"unmanaged_blocks": snapshot.Unmanaged,
		"consistent":       dbRouteCount == len(snapshot.Routes) && len(snapshot.Unmanaged) == 0,
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
