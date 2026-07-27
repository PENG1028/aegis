package handlers

import (
	"net/http"

	"aegis/internal/trace"
)

// TraceDomain handles GET /api/admin/v1/trace/domain/{domain}
func (h *Handlers) TraceDomain(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if domain == "" {
		writeError(w, http.StatusBadRequest, "domain path parameter is required")
		return
	}

	if h.TraceSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "trace service not configured")
		return
	}

	result := h.TraceSvc.TraceDomain(r.Context(), domain)
	status := http.StatusOK
	if result.TraceStatus == trace.StatusNotFound {
		status = http.StatusNotFound
	}
	writeJSON(w, status, result)
}

// TraceSNI handles GET /api/admin/v1/trace/sni/{sni_host}
func (h *Handlers) TraceSNI(w http.ResponseWriter, r *http.Request) {
	sniHost := r.PathValue("sni_host")
	if sniHost == "" {
		writeError(w, http.StatusBadRequest, "sni_host path parameter is required")
		return
	}

	if h.TraceSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "trace service not configured")
		return
	}

	result := h.TraceSvc.TraceSNI(r.Context(), sniHost)
	status := http.StatusOK
	if result.TraceStatus == trace.StatusNotFound {
		status = http.StatusNotFound
	}
	writeJSON(w, status, result)
}

// TraceRoute handles GET /api/admin/v1/trace/route/{route_id}
func (h *Handlers) TraceRoute(w http.ResponseWriter, r *http.Request) {
	routeID := r.PathValue("route_id")
	if routeID == "" {
		writeError(w, http.StatusBadRequest, "route_id path parameter is required")
		return
	}

	if h.TraceSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "trace service not configured")
		return
	}

	result := h.TraceSvc.TraceRoute(r.Context(), routeID)
	status := http.StatusOK
	if result.TraceStatus == trace.StatusNotFound {
		status = http.StatusNotFound
	}
	writeJSON(w, status, result)
}
