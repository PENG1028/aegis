package flowbridge

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

// HTTPProbe checks an instance by GET http://<control_address>/health.
// FlowBridge's /health is deliberately unauthenticated (liveness probe), so no
// token is required. We never probe business targets behind FlowBridge.
type HTTPProbe struct {
	client *http.Client
}

// NewHTTPProbe creates a probe with the given timeout (default 3s).
func NewHTTPProbe(timeout time.Duration) *HTTPProbe {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &HTTPProbe{client: &http.Client{Timeout: timeout}}
}

// Check implements Probe.
func (p *HTTPProbe) Check(ctx context.Context, inst *Instance) (string, int64, string) {
	addr := strings.TrimSpace(inst.ControlAddress)
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return HealthUnhealthy, 0, "invalid control_address: " + err.Error()
	}
	url := "http://" + addr + "/health"
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return HealthUnhealthy, 0, "build request: " + err.Error()
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return HealthUnhealthy, time.Since(start).Milliseconds(), "GET " + url + " failed: " + err.Error()
	}
	defer resp.Body.Close()
	latency := time.Since(start).Milliseconds()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return HealthHealthy, latency, "control plane /health OK (" + resp.Status + ")"
	}
	return HealthUnhealthy, latency, "control plane /health returned " + resp.Status
}
