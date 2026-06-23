package handlers

import (
	"aegis/internal/route"
	"net/http"
)

func (h *Handlers) ListRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := h.Route.ListRoutes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]map[string]interface{}, len(routes))
	for i, rt := range routes {
		result[i] = routeToMap(rt)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) CreateRoute(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Domain    string `json:"domain"`
		ServiceID string `json:"service_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rt, err := h.Route.CreateRoute(r.Context(), route.CreateRouteInput{
		Domain: input.Domain, ServiceID: input.ServiceID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, routeToMap(*rt))
}

func (h *Handlers) GetRoute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rt, err := h.Route.GetRoute(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, routeToMap(*rt))
}

func (h *Handlers) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (h *Handlers) EnableRoute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Route.EnableRoute(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

func (h *Handlers) DisableRoute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Route.DisableRoute(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (h *Handlers) SwitchRouteService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input struct {
		ServiceID string `json:"service_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.Route.SwitchRoute(r.Context(), id, input.ServiceID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "switched"})
}

func (h *Handlers) RouteMaintenanceOn(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input struct {
		Message string `json:"message"`
	}
	decodeJSON(r, &input)
	if err := h.Route.SetMaintenance(r.Context(), id, true, input.Message); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"maintenance": "on"})
}

func (h *Handlers) RouteMaintenanceOff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Route.SetMaintenance(r.Context(), id, false, ""); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"maintenance": "off"})
}

func routeToMap(rt route.Route) map[string]interface{} {
	return map[string]interface{}{
		"id":                  rt.ID,
		"domain":              rt.Domain,
		"service_id":          rt.ServiceID,
		"tls_enabled":         rt.TLSEnabled,
		"status":              rt.Status,
		"maintenance_enabled": rt.MaintenanceEnabled,
		"maintenance_message": rt.MaintenanceMessage,
		"created_at":          rt.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at":          rt.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
