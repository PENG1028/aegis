package noderuntime

import (
	"testing"
)

func TestRoutingTableToRouteSpecs(t *testing.T) {
	entries := []RoutingTableEntry{
		{
			Domain:          "app.example.com",
			RouteID:         "rt-1",
			ServiceID:       "svc-1",
			TargetLocalHost: "127.0.0.1",
			TargetLocalPort: 3001,
			Status:          "available",
			Protocol:        "http",
		},
		{
			Domain:          "api.example.com",
			RouteID:         "rt-2",
			ServiceID:       "svc-2",
			TargetLocalHost: "127.0.0.1",
			TargetLocalPort: 3002,
			Status:          "available",
			Protocol:        "http",
		},
		{
			Domain:          "disabled.example.com",
			RouteID:         "rt-3",
			ServiceID:       "svc-3",
			TargetLocalHost: "10.0.0.1",
			TargetLocalPort: 8080,
			Status:          "disabled",
		},
		{
			Domain:          "missing.example.com",
			RouteID:         "rt-4",
			ServiceID:       "svc-4",
			TargetLocalHost: "",
			TargetLocalPort: 0,
			Status:          "available",
		},
	}

	routes := routingTableToRouteSpecs(entries)

	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}

	if routes[0].Match.Host != "app.example.com" {
		t.Errorf("expected first route domain 'app.example.com', got %q", routes[0].Match.Host)
	}
	if routes[0].Upstream.Target != "http://127.0.0.1:3001" {
		t.Errorf("expected upstream 'http://127.0.0.1:3001', got %q", routes[0].Upstream.Target)
	}

	if routes[1].Upstream.Target != "http://127.0.0.1:3002" {
		t.Errorf("expected upstream 'http://127.0.0.1:3002', got %q", routes[1].Upstream.Target)
	}
}

func TestRoutingTableToRouteSpecsEmpty(t *testing.T) {
	routes := routingTableToRouteSpecs(nil)
	if len(routes) != 0 {
		t.Errorf("expected 0 routes for nil input, got %d", len(routes))
	}

	routes = routingTableToRouteSpecs([]RoutingTableEntry{})
	if len(routes) != 0 {
		t.Errorf("expected 0 routes for empty input, got %d", len(routes))
	}
}

func TestRoutingTableToRouteSpecsAllDisabled(t *testing.T) {
	entries := []RoutingTableEntry{
		{Domain: "a.example.com", Status: "disabled", TargetLocalHost: "127.0.0.1", TargetLocalPort: 3001},
		{Domain: "b.example.com", Status: "missing_endpoint", TargetLocalHost: "127.0.0.1", TargetLocalPort: 3002},
		{Domain: "c.example.com", Status: "unavailable", TargetLocalHost: "127.0.0.1", TargetLocalPort: 3003},
	}

	routes := routingTableToRouteSpecs(entries)
	if len(routes) != 0 {
		t.Errorf("expected 0 routes when all disabled, got %d", len(routes))
	}
}
