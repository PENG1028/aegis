package handlers

import (
	"net/http"
)

// ListGatewayDomains handles GET /api/admin/v1/gateway/domains
// v1.7R: Returns domains from the real source-of-truth tables (managed_domains + routes),
// NOT from the shadow gateway_* tables. This is a read-only consolidated view.
func (h *Handlers) ListGatewayDomains(w http.ResponseWriter, r *http.Request) {
	// Read from managed_domains (real source of truth for domain state)
	mdDomains, _ := h.ManagedDomain.ListManagedDomains(r.Context())
	routes, _ := h.Route.ListRoutes(r.Context())

	// Build a consolidated domain list from routes + managed domains
	type domainView struct {
		Domain       string `json:"domain"`
		RouteCount   int    `json:"route_count"`
		TLSEnabled   bool   `json:"tls_enabled"`
		Verification string `json:"verification_status"`
		Status       string `json:"status"`
	}

	domainMap := make(map[string]*domainView)

	// From managed domains
	for _, md := range mdDomains {
		dv := &domainView{
			Domain:       md.Domain,
			Verification: md.Status,
			Status:       md.Status,
		}
		domainMap[md.Domain] = dv
	}

	// From routes
	for _, rt := range routes {
		if dv, ok := domainMap[rt.Domain]; ok {
			dv.RouteCount++
			if rt.TLSEnabled {
				dv.TLSEnabled = true
			}
		} else {
			domainMap[rt.Domain] = &domainView{
				Domain:     rt.Domain,
				RouteCount: 1,
				TLSEnabled: rt.TLSEnabled,
				Status:     rt.Status,
			}
		}
	}

	domains := make([]domainView, 0, len(domainMap))
	for _, dv := range domainMap {
		domains = append(domains, *dv)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"domains":  domains,
		"count":    len(domains),
		"_source":  "routes + managed_domains (v1.7R consolidated view)",
	})
}

// ListGatewayListeners handles GET /api/admin/v1/gateway/listeners
// v1.7R: Returns listeners from the real listeners table (source of truth),
// NOT from the shadow gateway_listeners table.
func (h *Handlers) ListGatewayListeners(w http.ResponseWriter, r *http.Request) {
	listeners, err := h.ListenerSvc.ListAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"listeners": listeners,
		"count":     len(listeners),
		"_source":   "listeners table (v1.7R)",
	})
}
