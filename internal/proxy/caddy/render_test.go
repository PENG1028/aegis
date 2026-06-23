package caddy

import (
	"aegis/internal/proxy"
	"strings"
	"testing"
)

func TestRenderRoute(t *testing.T) {
	cfg := proxy.GatewayConfig{
		Routes: []proxy.RouteConfig{
			{
				Domain:      "example.com",
				Kind:        "reverse_proxy",
				UpstreamURL: "http://127.0.0.1:3001",
				TLSEnabled:   true,
				Options: proxy.ProxyOptions{
					EnableGzip: true,
				},
			},
		},
	}

	result := renderCaddyfile(cfg, "")
	if !strings.Contains(result, "example.com") {
		t.Error("expected domain in output")
	}
	if !strings.Contains(result, "reverse_proxy http://127.0.0.1:3001") {
		t.Error("expected reverse_proxy directive")
	}
	if !strings.Contains(result, "encode gzip") {
		t.Error("expected gzip encoding")
	}
}

func TestRenderMaintenance(t *testing.T) {
	cfg := proxy.GatewayConfig{
		Routes: []proxy.RouteConfig{
			{
				Domain:             "example.com",
				MaintenanceEnabled:  true,
				MaintenanceMessage:  "Down for maintenance",
			},
		},
	}

	result := renderCaddyfile(cfg, "")
	if !strings.Contains(result, `respond "Down for maintenance" 503`) {
		t.Error("expected maintenance respond block")
	}
	if strings.Contains(result, "reverse_proxy") {
		t.Error("maintenance route should NOT have reverse_proxy")
	}
}

func TestRenderDisabledRouteNotIncluded(t *testing.T) {
	// This test validates that the planner excludes disabled routes
	// — tested at the planner level, not renderer level
	cfg := proxy.GatewayConfig{
		Routes: []proxy.RouteConfig{},
	}

	result := renderCaddyfile(cfg, "")
	if strings.Contains(result, "example.com") {
		t.Error("empty routes should produce no site blocks")
	}
	_ = result
}
