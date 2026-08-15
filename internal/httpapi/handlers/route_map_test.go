package handlers

import (
	"testing"
	"time"

	"aegis/internal/route"
)

// TestRouteToMapIncludesFlowBridgeID is a regression test: routeToMap never
// emitted flowbridge_id, so the UI could not distinguish flowbridge-bound
// routes from plain upstream routes (EntryList read `r.flowbridge_id || ''`
// and always got undefined).
func TestRouteToMapIncludesFlowBridgeID(t *testing.T) {
	now := time.Now()
	fb := "fb_edge_1"
	rt := route.Route{
		ID:            "rt_1",
		Domain:        "app.example.com",
		ServiceID:     "svc_1",
		Composition:   "https_route",
		Status:        "active",
		FlowBridgeID:  &fb,
		GatewayLinkID: "gwlink_x",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	m := routeToMap(rt)
	if got, ok := m["flowbridge_id"]; !ok || got != "fb_edge_1" {
		t.Fatalf("flowbridge_id = %v (present=%v), want fb_edge_1", got, ok)
	}
}

// TestRouteToMapOmitsEmptyFlowBridgeID keeps the plain-upstream case clean.
func TestRouteToMapOmitsEmptyFlowBridgeID(t *testing.T) {
	now := time.Now()
	rt := route.Route{
		ID:        "rt_2",
		Domain:    "plain.example.com",
		ServiceID: "svc_2",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	m := routeToMap(rt)
	if _, ok := m["flowbridge_id"]; ok {
		t.Fatal("flowbridge_id should be omitted when unset")
	}
}
