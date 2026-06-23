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
	writeError(w, http.StatusNotImplemented, "endpoint creation via API not yet implemented")
}

func (h *Handlers) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (h *Handlers) EnableEndpoint(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (h *Handlers) DisableEndpoint(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (h *Handlers) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func endpointToMap(ep endpoint.Endpoint) map[string]interface{} {
	return map[string]interface{}{
		"id":         ep.ID,
		"service_id": ep.ServiceID,
		"type":       ep.Type,
		"address":    ep.Address,
		"enabled":    ep.Enabled,
		"created_at": ep.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at": ep.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
