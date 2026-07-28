package handlers

import (
	"aegis/internal/route"
	"aegis/internal/tlslifecycle"
	"fmt"
	"net/http"
	"strconv"
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

// AdminDeleteRoute deletes the route and its TLS binding. Certificate assets
// are retained because their lifecycle is independent from route usage.
func (h *Handlers) AdminDeleteRoute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	deleteUnusedCertificate, _ := strconv.ParseBool(r.URL.Query().Get("delete_unused_certificate"))
	var preview *tlslifecycle.RouteDeletePreview
	if deleteUnusedCertificate && h.TLSLifecycle == nil {
		writeError(w, http.StatusNotImplemented, "TLS lifecycle service not available")
		return
	}
	if h.TLSLifecycle != nil {
		var err error
		preview, err = h.TLSLifecycle.PreviewDeleteRoute(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if deleteUnusedCertificate && !preview.DeleteUnusedCertificateAllowed {
			writeErrorCode(w, http.StatusConflict, "CERTIFICATE_NOT_UNUSED", "the bound certificate is still referenced or is not an independent asset")
			return
		}
	}

	if err := h.Route.DeleteRoute(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	certificateDeleted := false
	cleanupWarning := ""
	if deleteUnusedCertificate && preview != nil && preview.CertificateID != "" {
		impact, err := h.TLSLifecycle.DeleteCertificate(r.Context(), preview.CertificateID)
		if err != nil || impact == nil || !impact.Allowed {
			cleanupWarning = "route was deleted but the certificate asset could not be removed"
		} else {
			certificateDeleted = true
		}
	}
	if h.PendingState != nil {
		_ = h.PendingState.MarkPending("route deleted: " + id)
	}

	// Trigger Apply to regenerate configs (HAProxy SNI, Caddyfile) without deleted route
	if h.Apply != nil {
		if _, err := h.Apply.TryApply(r.Context()); err != nil {
			response := map[string]interface{}{
				"status": "pending_apply", "route_id": id, "warning": err.Error(),
				"certificate_deleted": certificateDeleted, "impact": preview,
			}
			if cleanupWarning != "" {
				response["cleanup_warning"] = cleanupWarning
			}
			writeJSON(w, http.StatusAccepted, response)
			return
		}
	}

	response := map[string]interface{}{
		"status": "deleted", "route_id": id, "certificate_deleted": certificateDeleted, "impact": preview,
	}
	if cleanupWarning != "" {
		response["cleanup_warning"] = cleanupWarning
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handlers) AdminPreviewDeleteRoute(w http.ResponseWriter, r *http.Request) {
	if h.TLSLifecycle == nil {
		writeError(w, http.StatusNotImplemented, "TLS lifecycle service not available")
		return
	}
	preview, err := h.TLSLifecycle.PreviewDeleteRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *Handlers) AdminSetRouteTLSBinding(w http.ResponseWriter, r *http.Request) {
	if h.TLSLifecycle == nil {
		writeError(w, http.StatusNotImplemented, "TLS lifecycle service not available")
		return
	}
	var input struct {
		Mode       string `json:"mode"`
		ProviderID string `json:"provider_id"`
		CertID     string `json:"cert_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var (
		rt  *route.Route
		err error
	)
	switch input.Mode {
	case route.TLSBindingCertificate:
		rt, err = h.TLSLifecycle.BindCertificate(r.Context(), r.PathValue("id"), input.CertID)
	case route.TLSBindingProviderAuto:
		rt, err = h.TLSLifecycle.UseProviderAuto(r.Context(), r.PathValue("id"), input.ProviderID)
	default:
		err = fmt.Errorf("mode must be %q or %q", route.TLSBindingCertificate, route.TLSBindingProviderAuto)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.PendingState != nil {
		_ = h.PendingState.MarkPending("route TLS binding changed: " + rt.ID)
	}
	if h.Apply != nil {
		if _, err := h.Apply.TryApply(r.Context()); err != nil {
			writeJSON(w, http.StatusAccepted, map[string]interface{}{
				"status": "pending_apply", "route": routeToMap(*rt), "warning": err.Error(),
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "updated", "route": routeToMap(*rt)})
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
	m := map[string]interface{}{
		"id":                  rt.ID,
		"domain":              rt.Domain,
		"path_prefix":         rt.PathPrefix,
		"service_id":          rt.ServiceID,
		"composition":         rt.Composition,
		"tls_enabled":         rt.TLSEnabled,
		"tls_binding_mode":    rt.TLSBindingMode,
		"tls_provider":        rt.TLSProvider,
		"source_provider":     rt.SourceProvider,
		"source_capabilities": parseCapJSON(rt.SourceCapabilities),
		"status":              rt.Status,
		"maintenance_enabled": rt.MaintenanceEnabled,
		"maintenance_message": rt.MaintenanceMessage,
		"owner_type":          rt.OwnerType,
		"gateway_link_id":     rt.GatewayLinkID,
		"created_at":          rt.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at":          rt.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if rt.CertID != nil && *rt.CertID != "" {
		m["cert_id"] = *rt.CertID
	}
	return m
}
