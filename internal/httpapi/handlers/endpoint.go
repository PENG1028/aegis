package handlers

import (
	"aegis/internal/endpoint"
	"net/http"
)

func (h *Handlers) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	svcID := r.PathValue("id")
	endpoints, err := h.EndpointRepo.FindByServiceID(svcID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]map[string]interface{}, len(endpoints))
	for i, ep := range endpoints {
		result[i] = endpointToMap(ep)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	svcID := r.PathValue("id")

	var input struct {
		Type    string `json:"type"`
		Address string `json:"address"`
		NodeID  string `json:"node_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ep, err := h.EndpointSvc.CreateEndpoint(r.Context(), endpoint.CreateEndpointInput{
		ServiceID: svcID,
		Type:      input.Type,
		Address:   input.Address,
		NodeID:    input.NodeID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, endpointToMap(*ep))
}

func (h *Handlers) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	epID := r.PathValue("id")

	ep, err := h.EndpointRepo.FindByID(epID)
	if err != nil || ep == nil {
		writeError(w, http.StatusNotFound, "endpoint not found")
		return
	}

	var input struct {
		Type    *string `json:"type"`
		Address *string `json:"address"`
		NodeID  *string `json:"node_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if input.Type != nil {
		ep.Type = *input.Type
	}
	if input.Address != nil {
		ep.Address = *input.Address
	}
	if input.NodeID != nil {
		ep.NodeID = *input.NodeID
	}

	if err := h.EndpointSvc.UpdateEndpoint(r.Context(), ep); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, endpointToMap(*ep))
}

func (h *Handlers) EnableEndpoint(w http.ResponseWriter, r *http.Request) {
	epID := r.PathValue("id")

	if err := h.EndpointSvc.EnableEndpoint(r.Context(), epID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

func (h *Handlers) DisableEndpoint(w http.ResponseWriter, r *http.Request) {
	epID := r.PathValue("id")

	if err := h.EndpointSvc.DisableEndpoint(r.Context(), epID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (h *Handlers) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	epID := r.PathValue("id")

	if err := h.EndpointSvc.DeleteEndpoint(r.Context(), epID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func endpointToMap(ep endpoint.Endpoint) map[string]interface{} {
	return map[string]interface{}{
		"id":         ep.ID,
		"service_id": ep.ServiceID,
		"type":       ep.Type,
		"address":    ep.Address,
		"enabled":    ep.Enabled,
		"node_id":    ep.NodeID,
		"created_at": ep.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at": ep.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
