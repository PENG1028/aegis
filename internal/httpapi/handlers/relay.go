package handlers

import (
	"net/http"

	"aegis/internal/relay"
)

// RelayResolver exposes the relay resolver for the admin API.
type RelayResolver struct {
	Resolver *relay.Resolver
}

// ResolveRelay handles GET /api/admin/v1/relay/resolve?domain=<domain>&from_node=<node_id>
func (h *Handlers) ResolveRelay(w http.ResponseWriter, r *http.Request) {
	if h.RelayResolver == nil {
		writeError(w, http.StatusNotImplemented, "relay resolver not available")
		return
	}

	domain := r.URL.Query().Get("domain")
	if domain == "" {
		writeError(w, http.StatusBadRequest, "domain query parameter is required")
		return
	}

	fromNode := r.URL.Query().Get("from_node")
	if fromNode == "" {
		fromNode = "self"
	}

	result := h.RelayResolver.Resolver.ResolveManagedRelay(domain, fromNode)
	writeJSON(w, http.StatusOK, result)
}
