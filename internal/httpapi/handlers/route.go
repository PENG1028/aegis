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
	// Mark pending — admin CRUD modifies desired state but doesn't auto-apply
	if h.PendingState != nil {
		h.PendingState.MarkPending("route created: " + rt.ID)
	}
	if h.Logs != nil {
		h.Logs.Log(r.Context(), "route.create", "route", rt.ID, "success", "route created via admin CRUD", "admin")
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

// AdminGetRoute handles GET /api/admin/v1/routes/{id} — same as GetRoute but with admin cookie auth.
func (h *Handlers) AdminGetRoute(w http.ResponseWriter, r *http.Request) {
	h.GetRoute(w, r)
}

// AdminDeleteRoute handles DELETE /api/admin/v1/routes/{id}
func (h *Handlers) AdminDeleteRoute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Route.DeleteRoute(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "route_id": id})
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
	if h.PendingState != nil {
		h.PendingState.MarkPending("route enabled: " + id)
	}
	if h.Logs != nil {
		h.Logs.Log(r.Context(), "route.enable", "route", id, "success", "route enabled via admin CRUD", "admin")
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

func (h *Handlers) DisableRoute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Route.DisableRoute(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.PendingState != nil {
		h.PendingState.MarkPending("route disabled: " + id)
	}
	if h.Logs != nil {
		h.Logs.Log(r.Context(), "route.disable", "route", id, "success", "route disabled via admin CRUD", "admin")
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
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
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
