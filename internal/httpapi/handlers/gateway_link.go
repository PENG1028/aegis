package handlers

import (
	"net/http"

	gatewaylink "aegis/internal/gateway"
)

// CreateGatewayLink handles POST /api/admin/v1/gateway-links
func (h *Handlers) CreateGatewayLink(w http.ResponseWriter, r *http.Request) {
	if h.GatewayLinkSvc == nil {
		writeError(w, http.StatusNotImplemented, "gateway link service not available")
		return
	}
	var input struct {
		Name       string `json:"name"`
		Host       string `json:"host"`
		PrivateIP  string `json:"private_ip,omitempty"`
		Port       int    `json:"port"`
		GatewayType string `json:"gateway_type"`
		AutoRoute  bool   `json:"auto_route"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if input.Name == "" || input.Host == "" || input.Port <= 0 {
		writeError(w, http.StatusBadRequest, "name, host, and port are required")
		return
	}
	if input.GatewayType == "" {
		input.GatewayType = gatewaylink.TypeUpstream
	}

	gw, secret, err := h.GatewayLinkSvc.Register(input.Name, input.Host, input.PrivateIP, input.Port, input.GatewayType, input.AutoRoute)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.PendingState != nil {
		h.PendingState.MarkPending("gateway link created: " + gw.ID)
	}
	if h.Logs != nil {
		h.Logs.Log(r.Context(), "gateway-link.create", "gateway_link", gw.ID, "success", "gateway link created", "admin")
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":            gw.ID,
		"name":          gw.Name,
		"host":          gw.Host,
		"private_ip":    gw.PrivateIP,
		"port":          gw.Port,
		"gateway_type":  gw.GatewayType,
		"auto_route":    gw.AutoRoute,
		"status":        gw.Status,
		"auth_type":     gw.AuthType,
		"secret":        secret, // raw secret returned once
		"warning":       "store this secret securely — it will not be shown again",
	})
}

// ListGatewayLinks handles GET /api/admin/v1/gateway-links
func (h *Handlers) ListGatewayLinks(w http.ResponseWriter, r *http.Request) {
	if h.GatewayLinkSvc == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	links, err := h.GatewayLinkSvc.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if links == nil {
		links = []gatewaylink.TrustedGateway{}
	}
	writeJSON(w, http.StatusOK, links)
}

// GetGatewayLink handles GET /api/admin/v1/gateway-links/{id}
func (h *Handlers) GetGatewayLink(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.GatewayLinkSvc == nil {
		writeError(w, http.StatusNotFound, "gateway link service not available")
		return
	}
	gw, err := h.GatewayLinkSvc.Get(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if gw == nil {
		writeError(w, http.StatusNotFound, "gateway link not found")
		return
	}
	writeJSON(w, http.StatusOK, gw)
}

// DeleteGatewayLink handles DELETE /api/admin/v1/gateway-links/{id}
func (h *Handlers) DeleteGatewayLink(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.GatewayLinkSvc == nil {
		writeError(w, http.StatusNotFound, "gateway link service not available")
		return
	}
	if err := h.GatewayLinkSvc.Remove(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if h.PendingState != nil {
		h.PendingState.MarkPending("gateway link deleted: " + id)
	}
	if h.Logs != nil {
		h.Logs.Log(r.Context(), "gateway-link.delete", "gateway_link", id, "success", "gateway link deleted", "admin")
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// RotateGatewayLinkSecret handles POST /api/admin/v1/gateway-links/{id}/rotate
func (h *Handlers) RotateGatewayLinkSecret(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.GatewayLinkSvc == nil {
		writeError(w, http.StatusNotFound, "gateway link service not available")
		return
	}
	secret, err := h.GatewayLinkSvc.RotateSecret(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if h.PendingState != nil {
		h.PendingState.MarkPending("gateway link secret rotated: " + id)
	}
	if h.Logs != nil {
		h.Logs.Log(r.Context(), "gateway-link.rotate", "gateway_link", id, "success", "gateway link secret rotated", "admin")
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      id,
		"secret":  secret,
		"warning": "store this secret securely — old secret is invalidated",
	})
}
