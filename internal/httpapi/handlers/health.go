package handlers

import (
	"aegis/internal/health"
	"net/http"
)

func (h *Handlers) GetHealth(w http.ResponseWriter, r *http.Request) {
	checks, err := h.Health.GetLatestForAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]map[string]interface{}, len(checks))
	for i, c := range checks {
		result[i] = healthCheckToMap(c)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) CheckAllHealth(w http.ResponseWriter, r *http.Request) {
	checks, err := h.Health.CheckAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]map[string]interface{}, len(checks))
	for i, c := range checks {
		result[i] = healthCheckToMap(c)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) GetServiceHealth(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	check, err := h.Health.GetLatestForService(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if check == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "no data"})
		return
	}
	writeJSON(w, http.StatusOK, healthCheckToMap(*check))
}

func healthCheckToMap(hc health.HealthCheck) map[string]interface{} {
	return map[string]interface{}{
		"id":          hc.ID,
		"service_id":  hc.ServiceID,
		"endpoint_id": hc.EndpointID,
		"status":      hc.Status,
		"latency_ms":  hc.LatencyMS,
		"message":     hc.Message,
		"checked_at":  hc.CheckedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// Liveness handles GET /api/healthz — minimal check: is the process alive?
// Returns 200 immediately. No DB queries, no dependency checks.
// For Kubernetes liveness probes / load balancer health checks.
func (h *Handlers) Liveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

// Readiness handles GET /api/readyz — is the server ready to serve traffic?
// Checks database connectivity. Returns 200 if ready, 503 if not.
// For Kubernetes readiness probes / traffic draining.
func (h *Handlers) Readiness(w http.ResponseWriter, r *http.Request) {
	if h.DB != nil {
		if err := h.DB.Ping(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "not ready",
				"reason": "database unavailable",
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
