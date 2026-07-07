package main

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aegis/internal/serviceauth"
)

func registerRoutes(mux *http.ServeMux, svc *serviceauth.Service) {
	// === Service-side API (called by SDK, no admin auth) ===
	mux.HandleFunc("POST /api/service-auth/v1/register", handleRegister(svc))
	mux.HandleFunc("GET /api/service-auth/v1/sync", handleSync(svc))
	mux.HandleFunc("POST /api/service-auth/v1/report", handleReport(svc))

	// === Admin API ===
	mux.HandleFunc("GET /api/admin/v1/service-auth/services", handleListServices(svc))
	mux.HandleFunc("GET /api/admin/v1/service-auth/services/{id}", handleGetService(svc))
	mux.HandleFunc("POST /api/admin/v1/service-auth/services/{id}/block", handleBlockService(svc))
	mux.HandleFunc("POST /api/admin/v1/service-auth/apis/{id}/block", handleBlockAPI(svc))
	mux.HandleFunc("POST /api/admin/v1/service-auth/blocklist/{id}/unblock", handleUnblock(svc))
	mux.HandleFunc("GET /api/admin/v1/service-auth/topology", handleTopology(svc))
	mux.HandleFunc("GET /api/admin/v1/service-auth/call-logs", handleCallLogs(svc))
}

// ============================================================================
// Helpers
// ============================================================================

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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// ============================================================================
// Service-side handlers
// ============================================================================

func handleRegister(svc *serviceauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req serviceauth.RegisterRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, 400, "invalid request: "+err.Error())
			return
		}
		if req.ServiceName == "" || req.PublicKey == "" {
			writeError(w, 400, "service_name and public_key are required")
			return
		}

		resp, err := svc.Register(r.Context(), req, clientIP(r))
		if err != nil {
			if errors.Is(err, serviceauth.ErrNotInCluster) {
				writeError(w, 403, "not in cluster")
				return
			}
			if errors.Is(err, serviceauth.ErrInvalidInput) {
				writeError(w, 400, err.Error())
				return
			}
			writeError(w, 500, err.Error())
			return
		}

		writeJSON(w, 200, resp)
	}
}

func handleSync(svc *serviceauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		blVer, _ := strconv.ParseInt(r.URL.Query().Get("bl_version"), 10, 64)
		catVer, _ := strconv.ParseInt(r.URL.Query().Get("cat_version"), 10, 64)

		resp, err := svc.Sync(r.Context(), blVer, catVer)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}

		if resp.NotModified {
			w.WriteHeader(304)
			return
		}
		writeJSON(w, 200, resp)
	}
}

func handleReport(svc *serviceauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req serviceauth.ReportRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, 400, "invalid request: "+err.Error())
			return
		}

		if err := svc.Report(r.Context(), req); err != nil {
			writeError(w, 500, err.Error())
			return
		}

		writeJSON(w, 200, map[string]string{"status": "ok"})
	}
}

// ============================================================================
// Admin handlers
// ============================================================================

func handleListServices(svc *serviceauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		services, err := svc.ListServices(r.Context())
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, services)
	}
}

func handleGetService(svc *serviceauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		s, err := svc.GetService(r.Context(), id)
		if err != nil {
			if errors.Is(err, serviceauth.ErrServiceNotFound) {
				writeError(w, 404, "service not found")
				return
			}
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, s)
	}
}

func handleBlockService(svc *serviceauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Reason string `json:"reason"`
		}
		decodeJSON(r, &body)

		if err := svc.BlockService(r.Context(), id, body.Reason); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "blocked"})
	}
}

func handleBlockAPI(svc *serviceauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// api id is the path value, but we need service_id and api_name from body
		_ = r.PathValue("id") // not used directly; body has service_id + api_name
		var body struct {
			ServiceID string `json:"service_id"`
			APIName   string `json:"api_name"`
			Reason    string `json:"reason"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, 400, "invalid request: "+err.Error())
			return
		}

		if err := svc.BlockAPI(r.Context(), body.ServiceID, body.APIName, body.Reason); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "blocked"})
	}
}

func handleUnblock(svc *serviceauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := svc.Unblock(r.Context(), id); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "unblocked"})
	}
}

func handleTopology(svc *serviceauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		window := 1 * time.Hour
		if w := r.URL.Query().Get("window"); w != "" {
			if d, err := time.ParseDuration(w); err == nil {
				window = d
			}
		}

		data, err := svc.GetTopology(r.Context(), window)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, data)
	}
}

func handleCallLogs(svc *serviceauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		logs, err := svc.GetCallLogs(r.Context(), since, limit)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, logs)
	}
}
