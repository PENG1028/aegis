package gateway

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LocalForwarder forwards requests to local targets.
type LocalForwarder struct {
	client *http.Client
}

// NewLocalForwarder creates a new local forwarder.
func NewLocalForwarder(timeoutSec int) *LocalForwarder {
	return &LocalForwarder{
		client: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
			// Don't follow redirects — return them to caller
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Forward forwards a request to a local target.
func (f *LocalForwarder) Forward(w http.ResponseWriter, r *http.Request, targetHost string, targetPort int, routeID string) {
	targetURL := fmt.Sprintf("http://%s:%d%s", targetHost, targetPort, r.URL.RequestURI())

	// Create outgoing request
	outReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("create forward request: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy headers (excluding any X-Aegis-* that might bypass stripAegisHeaders)
	for key, values := range r.Header {
		// HARDENING: Never forward X-Aegis-* headers to target.
		// All Aegis internal headers are stripped at ServeHTTP entry.
		// This is defense-in-depth for any that might slip through.
		if isAegisHeader(key) {
			continue
		}
		for _, v := range values {
			outReq.Header.Add(key, v)
		}
	}

	// Add Aegis trace headers (allowed for local dispatch tracking)
	outReq.Header.Set("X-Aegis-From-Node", "127.0.0.1")
	outReq.Header.Set("X-Aegis-Route-ID", routeID)
	outReq.Header.Set("X-Aegis-Hop", "1")

	// Execute request
	resp, err := f.client.Do(outReq)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no such host") ||
			strings.Contains(errMsg, "timeout") {
			http.Error(w, "target unavailable", http.StatusBadGateway)
		} else {
			http.Error(w, fmt.Sprintf("forward error: %v", err), http.StatusInternalServerError)
		}
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}

	// Write status code and body
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// isAegisHeader checks if a header key is an internal Aegis header (X-Aegis-*).
func isAegisHeader(key string) bool {
	return len(key) >= 8 && strings.HasPrefix(strings.ToUpper(key), "X-AEGIS-")
}
