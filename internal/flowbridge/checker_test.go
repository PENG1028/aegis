package flowbridge

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPProbeHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	inst := &Instance{ControlAddress: strings.TrimPrefix(srv.URL, "http://")}
	probe := NewHTTPProbe(time.Second)
	status, latency, msg := probe.Check(context.Background(), inst)
	if status != HealthHealthy {
		t.Fatalf("expected healthy, got %s (%s)", status, msg)
	}
	if latency < 0 {
		t.Fatalf("negative latency: %d", latency)
	}
}

func TestHTTPProbeUnhealthyStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	inst := &Instance{ControlAddress: strings.TrimPrefix(srv.URL, "http://")}
	probe := NewHTTPProbe(time.Second)
	status, _, msg := probe.Check(context.Background(), inst)
	if status != HealthUnhealthy || !strings.Contains(msg, "503") {
		t.Fatalf("expected unhealthy with 503, got %s (%s)", status, msg)
	}
}

func TestHTTPProbeConnectionRefused(t *testing.T) {
	// Reserve a port, then close the listener so the dial is refused.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	inst := &Instance{ControlAddress: addr}
	probe := NewHTTPProbe(500 * time.Millisecond)
	status, _, msg := probe.Check(context.Background(), inst)
	if status != HealthUnhealthy || !strings.Contains(msg, "failed") {
		t.Fatalf("expected unhealthy on refused connection, got %s (%s)", status, msg)
	}
}

func TestHTTPProbeInvalidControlAddress(t *testing.T) {
	inst := &Instance{ControlAddress: "not-an-address"}
	probe := NewHTTPProbe(time.Second)
	status, _, msg := probe.Check(context.Background(), inst)
	if status != HealthUnhealthy || !strings.Contains(msg, "invalid control_address") {
		t.Fatalf("expected invalid address failure, got %s (%s)", status, msg)
	}
}
