package handlers

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aegis/internal/action"
	"aegis/internal/serviceauth"
)

// clientIP extracts the real client IP from a request.
// Only trusts X-Forwarded-For when the direct TCP peer is localhost (Caddy reverse proxy).
// Otherwise uses the unspoofable RemoteAddr to prevent isInCluster bypass.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	// Only trust X-Forwarded-For from the local reverse proxy.
	if host == "127.0.0.1" || host == "::1" {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			parts := strings.Split(fwd, ",")
			return strings.TrimSpace(parts[0])
		}
	}
	return host
}

// ============================================================================
// Service-side API (called by SDK, no admin auth required)
// ============================================================================

// ServiceAuthRegister handles POST /api/service-auth/v1/register
func (h *Handlers) ServiceAuthRegister(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	var req serviceauth.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if req.ServiceName == "" || req.PublicKey == "" {
		writeError(w, http.StatusBadRequest, "service_name and public_key are required")
		return
	}

	resp, err := h.ServiceAuthSvc.Register(r.Context(), req, clientIP(r))
	if err != nil {
		if errors.Is(err, serviceauth.ErrNotInCluster) {
			writeError(w, http.StatusForbidden, "not in cluster")
			return
		}
		if errors.Is(err, serviceauth.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// ServiceAuthSync handles GET /api/service-auth/v1/sync
func (h *Handlers) ServiceAuthSync(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	blVer, _ := strconv.ParseInt(r.URL.Query().Get("bl_version"), 10, 64)
	catVer, _ := strconv.ParseInt(r.URL.Query().Get("cat_version"), 10, 64)

	resp, err := h.ServiceAuthSvc.Sync(r.Context(), blVer, catVer)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if resp.NotModified {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ServiceAuthReport handles POST /api/service-auth/v1/report
func (h *Handlers) ServiceAuthReport(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	var req serviceauth.ReportRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.ServiceAuthSvc.Report(r.Context(), req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
// ServiceAuthHeartbeat handles POST /api/service-auth/v1/heartbeat
func (h *Handlers) ServiceAuthHeartbeat(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}
	var body struct {
		Name       string `json:"name"`
		InstanceID string `json:"instance_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.ServiceAuthSvc.Heartbeat(r.Context(), body.Name, body.InstanceID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status":"ok"})
}

// ServiceAuthScopedServices handles GET /api/service-auth/v1/services
// Returns per-service scoped view (callers + deps), identified via X-Service-Ticket.
func (h *Handlers) ServiceAuthScopedServices(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}
	ac := action.GetActionContext(r.Context())
	if ac == nil || !ac.IsService() {
		writeError(w, http.StatusUnauthorized, "service auth required")
		return
	}
	serviceName := ac.SpaceID
	if serviceName == "" {
		writeError(w, http.StatusBadRequest, "unknown caller")
		return
	}
	window := 1 * time.Hour
	if w := r.URL.Query().Get("window"); w != "" {
		if d, err := time.ParseDuration(w); err == nil {
			window = d
		}
	}
	callers, _ := h.ServiceAuthSvc.CallersOf(r.Context(), serviceName, window)
	deps, _ := h.ServiceAuthSvc.DepsOf(r.Context(), serviceName, window)
	type relEntry struct {
		Service  string `json:"service"`
		API      string `json:"api"`
		Count    int64  `json:"count"`
		LastSeen string `json:"last_seen"`
	}
	callerList := make([]relEntry, 0, len(callers))
	for _, e := range callers {
		callerList = append(callerList, relEntry{Service: e.Caller, API: e.API, Count: e.Count, LastSeen: e.LastSeen})
	}
	depList := make([]relEntry, 0, len(deps))
	for _, e := range deps {
		depList = append(depList, relEntry{Service: e.Target, API: e.API, Count: e.Count, LastSeen: e.LastSeen})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service": serviceName,
		"callers": callerList,
		"deps":    depList,
	})
}
// ServiceAuthScopedServices handles GET /api/service-auth/v1/services
// Returns per-service scoped view (callers + deps), identified via X-Service-Ticket.
func (h *Handlers) AdminListServiceAuthServices(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	services, err := h.ServiceAuthSvc.ListServices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if services == nil { services = []serviceauth.ServiceRecord{} }
	writeJSON(w, http.StatusOK, map[string]interface{}{"services": services, "count": len(services)})
}

// AdminGetServiceAuthService handles GET /api/admin/v1/service-auth/services/{id}
func (h *Handlers) AdminGetServiceAuthService(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	id := r.PathValue("id")
	s, err := h.ServiceAuthSvc.GetService(r.Context(), id)
	if err != nil {
		if errors.Is(err, serviceauth.ErrServiceNotFound) {
			writeError(w, http.StatusNotFound, "service not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// AdminBlockServiceAuthService handles POST /api/admin/v1/service-auth/services/{id}/block
func (h *Handlers) AdminBlockServiceAuthService(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	id := r.PathValue("id")
	var body struct {
		Reason string `json:"reason"`
	}
	decodeJSON(r, &body)

	if err := h.ServiceAuthSvc.BlockService(r.Context(), id, body.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.PendingState != nil {
		h.PendingState.MarkPending("service auth block: " + id)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "blocked"})
}

// AdminUnblockServiceAuth handles POST /api/admin/v1/service-auth/blocklist/{id}/unblock
func (h *Handlers) AdminUnblockServiceAuth(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	id := r.PathValue("id")
	if err := h.ServiceAuthSvc.Unblock(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unblocked"})
}

// AdminServiceAuthTopology handles GET /api/admin/v1/service-auth/topology
func (h *Handlers) AdminServiceAuthTopology(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	window := 1 * time.Hour
	if w := r.URL.Query().Get("window"); w != "" {
		if d, err := time.ParseDuration(w); err == nil {
			window = d
		}
	}

	data, err := h.ServiceAuthSvc.GetTopology(r.Context(), window)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// AdminServiceAuthCallLogs handles GET /api/admin/v1/service-auth/call-logs
func (h *Handlers) AdminServiceAuthCallLogs(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	since := time.Now().Add(-1 * time.Hour)
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	logs, err := h.ServiceAuthSvc.GetCallLogs(r.Context(), since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// ─── Groups ───




// ─── Policies ───



