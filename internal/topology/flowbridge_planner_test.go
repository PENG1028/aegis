package topology

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"aegis/internal/endpoint"
	"aegis/internal/flowbridge"
	"aegis/internal/route"
	"aegis/internal/service"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

// newFlowBridgeTestPlanner seeds one flowbridge-bound route (service with NO
// endpoint — resolution must come from the instance alone) plus one regular
// endpoint route, and returns a Planner whose deps can resolve both.
// useRepo=false builds the planner with a nil FlowBridgeRepo to exercise the
// degradation path.
func newFlowBridgeTestPlanner(t *testing.T, enabled, useRepo bool) *Planner {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Format(time.RFC3339)

	if _, err := db.Exec(
		`INSERT INTO services (id, project_id, name, kind, env, status, note, created_at, updated_at)
		 VALUES ('svc_ep', '', 'ep-svc', 'http', 'prod', 'active', '', ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO endpoints (id, service_id, type, address, enabled, created_at, updated_at)
		 VALUES ('ep_1', 'svc_ep', 'private', '127.0.0.1:3000', 1, ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO services (id, project_id, name, kind, env, status, note, created_at, updated_at)
		 VALUES ('svc_fb', '', 'fb-svc', 'http', 'prod', 'active', '', ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}

	fbRepo := flowbridge.NewRepository(db)
	if useRepo {
		if err := fbRepo.Create(&flowbridge.Instance{
			ID: "fb_1", Name: "edge", MachineIP: "10.0.0.9", DataPlanePort: 8080,
			ControlAddress: "10.0.0.9:9090", Enabled: enabled,
			OwnerType: "admin", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	insertRoute := func(id, domain, svcID, fbID string) {
		if _, err := db.Exec(
			`INSERT INTO routes (id, domain, path_prefix, strip_prefix, service_id, tls_enabled, composition,
				source_provider, source_capabilities, tls_binding_mode, tls_provider, status,
				maintenance_enabled, maintenance_message, space_id, owner_type, owner_id,
				created_by_token_id, gateway_link_id, cert_id, flowbridge_id, created_at, updated_at)
			 VALUES (?, ?, '', 0, ?, 1, 'https_route', 'caddy', '', 'provider_auto', 'caddy', 'active',
				0, '', '', 'admin', '', '', '', '', ?, ?, ?)`,
			id, domain, svcID, fbID, now, now); err != nil {
			t.Fatal(err)
		}
	}
	insertRoute("rt_ep", "ep.example.com", "svc_ep", "")
	rtFbID := ""
	if useRepo {
		rtFbID = "fb_1"
	}
	insertRoute("rt_fb", "fb.example.com", "svc_fb", rtFbID)

	var repo *flowbridge.Repository
	if useRepo {
		repo = fbRepo
	}
	return NewPlanner(nil, Dependencies{
		RouteRepo:        route.NewRepository(db),
		ServiceRepo:      service.NewRepository(db),
		EndpointResolver: endpoint.NewResolver(endpoint.NewRepository(db)),
		FlowBridgeRepo:   repo,
	})
}

func TestResolveIntentsFlowBridgeResolvesInstance(t *testing.T) {
	planner := newFlowBridgeTestPlanner(t, true, true)
	resolved, warnings := planner.resolveIntents([]RouteIntent{
		{Domain: "fb.example.com", Composition: "https_route", flowbridgeID: "fb_1", serviceID: "svc_fb"},
	})
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved flowbridge route, got %d (warnings: %v)", len(resolved), warnings)
	}
	if resolved[0].Upstream != "http://10.0.0.9:8080" {
		t.Fatalf("unexpected upstream %q", resolved[0].Upstream)
	}
	if resolved[0].ExtraHeaders["Host"] != "fb.example.com" {
		t.Fatalf("Host header preservation not set: %v", resolved[0].ExtraHeaders)
	}
}

func TestResolveIntentsFlowBridgeMissingInstanceSkips(t *testing.T) {
	planner := newFlowBridgeTestPlanner(t, true, true)
	resolved, warnings := planner.resolveIntents([]RouteIntent{
		{Domain: "fb.example.com", Composition: "https_route", flowbridgeID: "fb_missing", serviceID: "svc_fb"},
	})
	if len(resolved) != 0 {
		t.Fatalf("missing instance must skip route, got %d", len(resolved))
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "not found") {
		t.Fatalf("missing-instance warning absent: %v", warnings)
	}
}

func TestResolveIntentsFlowBridgeDisabledSkips(t *testing.T) {
	planner := newFlowBridgeTestPlanner(t, false, true)
	resolved, warnings := planner.resolveIntents([]RouteIntent{
		{Domain: "fb.example.com", Composition: "https_route", flowbridgeID: "fb_1", serviceID: "svc_fb"},
	})
	if len(resolved) != 0 {
		t.Fatalf("disabled instance must skip route, got %d", len(resolved))
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "disabled") {
		t.Fatalf("disabled-instance warning absent: %v", warnings)
	}
}

func TestResolveIntentsFlowBridgeNeverFallsBackToEndpoint(t *testing.T) {
	// The flowbridge-bound service has NO endpoint in the DB. If the planner
	// tried endpoint resolution as a fallback it would produce a "no available
	// endpoint" warning — proving the instance branch is exclusive.
	planner := newFlowBridgeTestPlanner(t, true, true)
	resolved, warnings := planner.resolveIntents([]RouteIntent{
		{Domain: "fb.example.com", Composition: "https_route", flowbridgeID: "fb_1", serviceID: "svc_fb"},
	})
	if len(resolved) != 1 {
		t.Fatalf("expected resolution despite missing endpoints, got %d (warnings: %v)", len(resolved), warnings)
	}
	for _, w := range warnings {
		if strings.Contains(w, "no available endpoint") {
			t.Fatalf("endpoint resolution leaked into flowbridge branch: %v", warnings)
		}
	}
}

func TestResolveIntentsFlowBridgeNilRepoSkips(t *testing.T) {
	planner := newFlowBridgeTestPlanner(t, true, false)
	resolved, warnings := planner.resolveIntents([]RouteIntent{
		{Domain: "fb.example.com", Composition: "https_route", flowbridgeID: "fb_1", serviceID: "svc_fb"},
	})
	if len(resolved) != 0 {
		t.Fatalf("nil repo must skip route, got %d", len(resolved))
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "unavailable") {
		t.Fatalf("nil-repo warning absent: %v", warnings)
	}
}

func TestFlowBridgeRequiresNoNewCapabilities(t *testing.T) {
	ri := RouteIntent{Composition: "https_route", Transport: "tcp", TLSMode: "terminate", AppProtocol: "http"}
	caps := ri.RequirementsOf()
	if len(caps) == 0 {
		t.Fatal("https_route must still require capabilities")
	}
	for _, c := range caps {
		if strings.Contains(string(c), "flowbridge") {
			t.Fatalf("flowbridge leaked into capability requirements: %v", c)
		}
	}
}
