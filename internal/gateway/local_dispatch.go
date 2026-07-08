package gateway

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// KeepAliveTransport wraps http.Transport with keep-alive disabled to prevent
// connection reuse between heartbeat cycles.
var KeepAliveTransport *http.Transport

func init() {
	KeepAliveTransport = &http.Transport{
		DisableKeepAlives: true,
	}
}

// HealthCheckForwarder holds dependencies for forwarding health checks.
type HealthCheckForwarder struct {
	client *http.Client
}

// NewHealthCheckForwarder creates a forwarder for TCP health checks.
func NewHealthCheckForwarder() *HealthCheckForwarder {
	return &HealthCheckForwarder{
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: KeepAliveTransport,
		},
	}
}

// Forward forwards a health check request to the given target.
func (f *HealthCheckForwarder) Forward(w http.ResponseWriter, r *http.Request, targetHost string, targetPort int) {
	url := fmt.Sprintf("http://%s/healthz", net.JoinHostPort(targetHost, strconv.Itoa(targetPort)))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	resp, err := f.client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// LocalForwarder forwards requests to local services.
type LocalForwarder struct {
	client *http.Client
}

// NewLocalForwarder creates a new local forwarder.
func NewLocalForwarder(timeoutSec int) *LocalForwarder {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &LocalForwarder{
		client: &http.Client{
			Timeout:   time.Duration(timeoutSec) * time.Second,
			Transport: KeepAliveTransport,
		},
	}
}

// Forward forwards a request to a local target.
func (f *LocalForwarder) Forward(w http.ResponseWriter, r *http.Request, targetHost string, targetPort int, routeID string) {
	targetURL := fmt.Sprintf("http://%s%s", net.JoinHostPort(targetHost, strconv.Itoa(targetPort)), r.URL.RequestURI())

	// Create outgoing request
	outReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	// Copy headers
	for k, v := range r.Header {
		outReq.Header[k] = v
	}

	// Remove hop-by-hop headers
	outReq.Header.Del("X-Service-Ticket") // don't forward the ticket upstream

	resp, err := f.client.Do(outReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
