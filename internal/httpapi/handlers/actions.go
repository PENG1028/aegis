package handlers

import (
	"net/http"

	"aegis/internal/action"
	"aegis/internal/listener"
)

// BindHTTPDomain handles POST /api/v1/actions/bind-http-domain
func (h *Handlers) BindHTTPDomain(w http.ResponseWriter, r *http.Request) {
	var input action.BindHTTPDomainInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if input.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}
	if input.TargetHost == "" {
		writeError(w, http.StatusBadRequest, "target_host is required")
		return
	}
	if input.TargetPort <= 0 {
		// Default to the public HTTP port from listener config (not hardcoded).
		input.TargetPort = listenerPort("public_http", 80)
	}

	result, err := h.Action.BindHTTPDomain(r.Context(), input)
	if err != nil {
		writeActionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// BindTLSBackend handles POST /api/v1/actions/bind-tls-backend
func (h *Handlers) BindTLSBackend(w http.ResponseWriter, r *http.Request) {
	var input action.BindTLSBackendInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if input.SNIHost == "" {
		writeError(w, http.StatusBadRequest, "sni_host is required")
		return
	}
	if input.TargetHost == "" {
		writeError(w, http.StatusBadRequest, "target_host is required")
		return
	}
	if input.TargetPort <= 0 {
		// Default to the public TLS port from listener config (not hardcoded).
		input.TargetPort = listenerPort("public_tls_mux", 443)
	}

	result, err := h.Action.BindTLSBackend(r.Context(), input)
	if err != nil {
		writeActionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdateTarget handles PATCH /api/v1/actions/update-target
func (h *Handlers) UpdateTarget(w http.ResponseWriter, r *http.Request) {
	var input action.UpdateTargetInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if input.ResourceID == "" {
		writeError(w, http.StatusBadRequest, "resource_id is required")
		return
	}
	if input.ResourceType == "" {
		writeError(w, http.StatusBadRequest, "resource_type is required (service or edge_rule)")
		return
	}

	result, err := h.Action.UpdateTarget(r.Context(), input)
	if err != nil {
		writeActionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DisableDomain handles POST /api/v1/actions/disable-domain
func (h *Handlers) DisableDomain(w http.ResponseWriter, r *http.Request) {
	var input action.DisableDomainInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if input.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}

	result, err := h.Action.DisableDomain(r.Context(), input)
	if err != nil {
		writeActionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteDomain handles DELETE /api/v1/actions/domain
func (h *Handlers) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	var input action.DeleteDomainInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if input.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}

	result, err := h.Action.DeleteDomain(r.Context(), input)
	if err != nil {
		writeActionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListMyRoutes handles GET /api/v1/my/routes
func (h *Handlers) ListMyRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := h.Action.ListMyRoutes(r.Context())
	if err != nil {
		writeActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"routes": routes,
		"count":  len(routes),
	})
}

// ListMyServices handles GET /api/v1/my/services
func (h *Handlers) ListMyServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.Action.ListMyServices(r.Context())
	if err != nil {
		writeActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"services": services,
		"count":    len(services),
	})
}

// ListMyEdgeRules handles GET /api/v1/my/edge-rules
func (h *Handlers) ListMyEdgeRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.Action.ListMyEdgeRules(r.Context())
	if err != nil {
		writeActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"edge_rules": rules,
		"count":      len(rules),
	})
}

// ListMyOperations handles GET /api/v1/my/operations
func (h *Handlers) ListMyOperations(w http.ResponseWriter, r *http.Request) {
	ops, err := h.Action.ListMyOperations(r.Context(), 50)
	if err != nil {
		writeActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"operations": ops,
		"count":      len(ops),
	})
}

// writeActionError writes an ActionError or generic error as JSON.
func writeActionError(w http.ResponseWriter, err error) {
	ae, ok := err.(*action.ActionError)
	if ok {
		status := http.StatusBadRequest
		switch ae.Code {
		case action.ErrCodeScopeDenied:
			status = http.StatusForbidden
		case action.ErrCodeDomainAlreadyOwned:
			status = http.StatusConflict
		case action.ErrCodeApplyLocked:
			status = http.StatusLocked
		case action.ErrCodeResourceNotFound:
			status = http.StatusNotFound
		case action.ErrCodeResourceNotOwned:
			status = http.StatusForbidden
		case action.ErrCodeTargetNotAllowed:
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]interface{}{
			"error": ae,
		})
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

// listenerPort reads a port from the listener defaults by purpose, with a fallback.
// This avoids hardcoding port 80/443 throughout handler code.
func listenerPort(purpose string, fallback int) int {
	for _, l := range listener.EdgeMuxDefaults() {
		if l.Purpose == purpose {
			return l.Port
		}
	}
	for _, l := range listener.DefaultListeners() {
		if l.Purpose == purpose {
			return l.Port
		}
	}
	return fallback
}
