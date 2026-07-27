package handlers

import (
	"net/http"

	"aegis/internal/routingpolicy"
)

// AdminGetServicePolicy handles GET /api/admin/v1/services/{id}/gateway-policy
func (h *Handlers) AdminGetServicePolicy(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("id")
	if serviceID == "" {
		writeError(w, http.StatusBadRequest, "service id is required")
		return
	}

	policy, err := h.PolicySvc.GetServicePolicy(serviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if policy == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"service_id": serviceID,
			"policy":     "default",
			"mode":       routingpolicy.ModeAuto,
		})
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

// AdminSetServicePolicy handles PUT /api/admin/v1/services/{id}/gateway-policy
func (h *Handlers) AdminSetServicePolicy(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("id")
	if serviceID == "" {
		writeError(w, http.StatusBadRequest, "service id is required")
		return
	}

	var input routingpolicy.PolicyInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	input.ServiceID = serviceID

	policy, err := h.PolicySvc.SetServicePolicy(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

// AdminGetRoutePolicy handles GET /api/admin/v1/routes/{id}/gateway-policy
func (h *Handlers) AdminGetRoutePolicy(w http.ResponseWriter, r *http.Request) {
	routeID := r.PathValue("id")
	if routeID == "" {
		writeError(w, http.StatusBadRequest, "route id is required")
		return
	}

	policy, err := h.PolicySvc.GetRoutePolicy(routeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if policy == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"route_id": routeID,
			"policy":   "default",
			"mode":     routingpolicy.ModeAuto,
		})
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

// AdminSetRoutePolicy handles PUT /api/admin/v1/routes/{id}/gateway-policy
func (h *Handlers) AdminSetRoutePolicy(w http.ResponseWriter, r *http.Request) {
	routeID := r.PathValue("id")
	if routeID == "" {
		writeError(w, http.StatusBadRequest, "route id is required")
		return
	}

	var input routingpolicy.PolicyInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	input.RouteID = routeID

	policy, err := h.PolicySvc.SetRoutePolicy(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, policy)
}
