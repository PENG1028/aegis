package handlers

import (
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	if req.ServiceName == "" || req.Host == "" || req.Port <= 0 {
		writeError(w, http.StatusBadRequest, "service_name, host, and port are required")
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

// ============================================================================
// Admin API
// ============================================================================

// AdminListServiceAuthServices handles GET /api/admin/v1/service-auth/services
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

// AdminBlockServiceAuthAPI handles POST /api/admin/v1/service-auth/apis/{id}/block
func (h *Handlers) AdminBlockServiceAuthAPI(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	var body struct {
		ServiceID string `json:"service_id"`
		APIName   string `json:"api_name"`
		Reason    string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.ServiceAuthSvc.BlockAPI(r.Context(), body.ServiceID, body.APIName, body.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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

// AdminRebindService handles POST /api/admin/v1/service-auth/services/{name}/rebind
func (h *Handlers) AdminRebindService(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	oldName := r.PathValue("name")
	var body struct {
		NewName string `json:"new_name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	kp, err := h.ServiceAuthSvc.Rebind(r.Context(), oldName, body.NewName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("[serviceauth] rebind: %s → %s (admin action)", oldName, body.NewName)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "rebound",
		"old_name":    oldName,
		"new_name":    body.NewName,
		"public_key":  kp.PublicKey,
		"private_key": kp.PrivateKey,
	})
}

// ─── Groups ───

func (h *Handlers) AdminListServiceAuthGroups(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil { writeError(w, http.StatusNotImplemented, "N/A"); return }
	groups, err := h.ServiceAuthSvc.ListGroups(r.Context())
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]interface{}{"groups": groups, "count": len(groups)})
}

func (h *Handlers) AdminUpsertServiceAuthGroup(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil { writeError(w, http.StatusNotImplemented, "N/A"); return }
	var g serviceauth.ServiceGroup
	if err := decodeJSON(r, &g); err != nil { writeError(w, http.StatusBadRequest, err.Error()); return }
	if err := h.ServiceAuthSvc.UpsertGroup(r.Context(), &g); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusCreated, g)
}

func (h *Handlers) AdminDeleteServiceAuthGroup(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil { writeError(w, http.StatusNotImplemented, "N/A"); return }
	if err := h.ServiceAuthSvc.DeleteGroup(r.Context(), r.PathValue("id")); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ─── Policies ───

func (h *Handlers) AdminListServiceAuthPolicies(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil { writeError(w, http.StatusNotImplemented, "N/A"); return }
	policies, err := h.ServiceAuthSvc.ListPolicies(r.Context())
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]interface{}{"policies": policies, "count": len(policies)})
}

func (h *Handlers) AdminUpsertServiceAuthPolicy(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil { writeError(w, http.StatusNotImplemented, "N/A"); return }
	var p serviceauth.Policy
	if err := decodeJSON(r, &p); err != nil { writeError(w, http.StatusBadRequest, err.Error()); return }
	if err := h.ServiceAuthSvc.UpsertPolicy(r.Context(), &p); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handlers) AdminDeleteServiceAuthPolicy(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil { writeError(w, http.StatusNotImplemented, "N/A"); return }
	if err := h.ServiceAuthSvc.DeletePolicy(r.Context(), r.PathValue("id")); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
