package provider

import (
	"strings"
	"testing"
)

func TestSortRoutesForMatchControlPlaneBeforeCatchAll(t *testing.T) {
	routes := []RouteSpec{
		{
			Match:    MatchSpec{Host: "http://"},
			Upstream: UpstreamSpec{Target: "http://43.160.211.232:80"},
		},
		{
			Match:    MatchSpec{Host: "http://", Path: "/api"},
			Upstream: UpstreamSpec{Target: "http://127.0.0.1:7380"},
			Priority: RoutePriorityControlPlane,
		},
	}

	got := SortRoutesForMatch(routes)
	if got[0].Upstream.Target != "http://127.0.0.1:7380" {
		t.Fatalf("control-plane route sorted after business fallback: %#v", got)
	}
}

func TestCaddyRenderKeepsControlPlaneBeforeGatewayFallback(t *testing.T) {
	p := &CaddyProvider{}
	content := string(p.renderCaddyfile(Plan{
		Routes: []RouteSpec{
			{
				Transport:   "tcp",
				TLSMode:     "terminate",
				AppProtocol: "http",
				Match:       MatchSpec{Host: "http://"},
				Upstream:    UpstreamSpec{Type: "http", Target: "http://43.160.211.232:80"},
			},
			{
				Transport:   "tcp",
				TLSMode:     "terminate",
				AppProtocol: "http",
				Match:       MatchSpec{Host: "http://", Path: "/api"},
				Upstream:    UpstreamSpec{Type: "http", Target: "http://127.0.0.1:7380"},
				Priority:    RoutePriorityControlPlane,
			},
		},
	}))

	apiIdx := strings.Index(content, "handle /api/*")
	fallbackIdx := strings.Index(content, "reverse_proxy http://43.160.211.232:80")
	if apiIdx < 0 || fallbackIdx < 0 {
		t.Fatalf("rendered config missing expected routes:\n%s", content)
	}
	if apiIdx > fallbackIdx {
		t.Fatalf("control-plane route rendered after gateway fallback:\n%s", content)
	}
}
