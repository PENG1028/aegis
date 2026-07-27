package health

import "time"

// Status constants.
const (
	StatusHealthy   = "healthy"
	StatusUnhealthy = "unhealthy"
	StatusUnknown   = "unknown"
)

// HealthCheck records the result of a health check.
type HealthCheck struct {
	ID         string    `json:"id"`
	ServiceID  string    `json:"service_id"`
	EndpointID string    `json:"endpoint_id"`
	Status     string    `json:"status"` // healthy | unhealthy | unknown
	LatencyMS  int64     `json:"latency_ms"`
	Message    string    `json:"message"`
	CheckedAt  time.Time `json:"checked_at"`
}
