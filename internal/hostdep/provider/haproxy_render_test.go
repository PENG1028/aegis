package provider

import (
	"strings"
	"testing"
)

// TestHAProxyRenderDeduplicatesSameSNI is a regression test: routes with
// different path_prefix on the SAME host each generated a `backend be_x`
// block, producing a config with duplicate backend names that `haproxy -c`
// rejects. SNI routing cannot distinguish paths, so same-host routes must
// collapse to one backend (highest priority wins).
func TestHAProxyRenderDeduplicatesSameSNI(t *testing.T) {
	p := &HAProxyProvider{inspectDelay: "3s"}
	routes := []RouteSpec{
		{Match: MatchSpec{SNI: "app.example.com"}, Upstream: UpstreamSpec{Type: "tcp", Target: "10.0.0.1:8001"}, Priority: 0},
		{Match: MatchSpec{SNI: "app.example.com"}, Upstream: UpstreamSpec{Type: "tcp", Target: "10.0.0.1:8002"}, Priority: 10},
		{Match: MatchSpec{SNI: "other.example.com"}, Upstream: UpstreamSpec{Type: "tcp", Target: "10.0.0.2:9000"}, Priority: 0},
	}

	out := string(p.renderMainConfig(nil, routes))

	// Exactly one backend block per SNI.
	if n := strings.Count(out, "backend be_app_example_com\n"); n != 1 {
		t.Fatalf("backend be_app_example_com appears %d times, want 1 (duplicate backend = invalid HAProxy config):\n%s", n, out)
	}
	if n := strings.Count(out, "backend be_other_example_com\n"); n != 1 {
		t.Fatalf("backend be_other_example_com appears %d times, want 1", n)
	}
	// Highest priority route (10 → port 8002) must win the dedup.
	if !strings.Contains(out, "10.0.0.1:8002") {
		t.Fatalf("highest-priority upstream missing from rendered config:\n%s", out)
	}
	if strings.Contains(out, "10.0.0.1:8001") {
		t.Fatalf("lower-priority upstream leaked into rendered config:\n%s", out)
	}
	// Same number of use_backend lines as backends.
	if n := strings.Count(out, "use_backend be_"); n != 2 {
		t.Fatalf("use_backend lines = %d, want 2", n)
	}
}
