package handlers

import (
	"aegis/internal/safety"
	"net/http"
)

// CheckRouteSafety handles GET /api/admin/v1/routes/{id}/safety
func (h *Handlers) CheckRouteSafety(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.SafetySvc == nil {
		writeError(w, http.StatusNotImplemented, "safety service not available")
		return
	}
	result, err := h.SafetySvc.CheckRouteSafety(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// CheckAllRoutesSafety handles GET /api/admin/v1/routes/safety
func (h *Handlers) CheckAllRoutesSafety(w http.ResponseWriter, r *http.Request) {
	if h.SafetySvc == nil {
		writeError(w, http.StatusNotImplemented, "safety service not available")
		return
	}
	results, err := h.SafetySvc.CheckAllRoutesSafety()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []safety.RouteSafetyResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

// TraceEgress handles GET /api/admin/v1/trace/egress
func (h *Handlers) TraceEgress(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	fromNode := r.URL.Query().Get("from_node")
	if domain == "" {
		writeError(w, http.StatusBadRequest, "domain query parameter is required")
		return
	}
	if h.SafetySvc == nil {
		writeError(w, http.StatusNotImplemented, "safety service not available")
		return
	}
	result, err := h.SafetySvc.TraceEgress(domain, fromNode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
