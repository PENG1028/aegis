// Package e2e contains end-to-end integration tests for the Aegis control plane.
//
// Scenario A: Single Node Full Flow
// Tests the complete lifecycle: bootstrap -> create project, service, endpoint, route
// -> enable -> dry run -> verify Caddyfile -> check pending state -> apply.
package e2e

import (
	"context"
	"os"
	"strings"
	"testing"

	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/edgemux"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/project"
	"aegis/internal/proxy"
	"aegis/internal/route"
	"aegis/internal/safety"
	"aegis/internal/secrets"
	"aegis/internal/service"
	"aegis/internal/store"

	gatewaylink "aegis/internal/gateway_link"
)

// setupTempDB creates a temporary SQLite database file, initializes the schema,
// and returns the *sql.DB handle along with a cleanup function.
func setupTempDB(t *testing.T) (db *store.Store, cleanup func()) {
	t.Helper()

	f, err := os.CreateTemp("", "aegis-e2e-a-*.db")
	if err != nil {
		t.Fatalf("create temp db file: %v", err)
	}
	dbPath := f.Name()
	f.Close()
	os.Remove(dbPath) // OpenSQLite will recreate it

	sqlDB, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := store.Initialize(sqlDB); err != nil {
		sqlDB.Close()
		t.Fatalf("initialize schema: %v", err)
	}

	st := store.New(sqlDB)
	cleanup = func() {
		st.Close()
		os.Remove(dbPath)
	}
	return st, cleanup
}

// TestFullFlow_SingleNode verifies the end-to-end full flow on a single node.
func TestFullFlow_SingleNode(t *testing.T) {
	// Step 1: Bootstrap — create temp DB, load config
	st, cleanup := setupTempDB(t)
	defer cleanup()
	ctx := context.Background()

	cfg := config.DefaultConfig()

	// Override config paths for testing
	tmpDir := t.TempDir()
	cfg.Proxy.CaddyfilePath = tmpDir + "/Caddyfile"
	cfg.Proxy.BackupDir = tmpDir + "/backups"
	cfg.Proxy.Email = "test@aegis.local"

	// Shared log service (all service layers use the same logs repo)
	logRepo := logs.NewRepository(st.DB)
	logSvc := logs.NewAppService(logRepo)

	// Step 2: Create a Project
	projectRepo := project.NewRepository(st.DB)
	projectSvc := project.NewAppService(projectRepo, logSvc)
	proj, err := projectSvc.CreateProject(ctx, project.CreateProjectInput{
		Name:        "e2e-test-project",
		Description: "Project for e2e full flow test",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if proj == nil || proj.ID == "" {
		t.Fatal("project was not created")
	}
	t.Logf("created project: id=%s name=%s", proj.ID, proj.Name)

	// Step 3: Create a Service
	serviceRepo := service.NewRepository(st.DB)
	serviceSvc := service.NewAppService(serviceRepo, logSvc)
	svc, err := serviceSvc.CreateService(ctx, service.CreateServiceInput{
		ProjectID:   proj.ID,
		ProjectName: proj.Name,
		Name:        "e2e-test-service",
		Kind:        "http",
		Env:         "dev",
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	if svc == nil || svc.ID == "" {
		t.Fatal("service was not created")
	}
	t.Logf("created service: id=%s name=%s", svc.ID, svc.Name)

	// Step 4: Create an Endpoint
	endpointRepo := endpoint.NewRepository(st.DB)
	endpointSvc := endpoint.NewAppService(endpointRepo, logSvc)
	ep, err := endpointSvc.CreateEndpoint(ctx, endpoint.CreateEndpointInput{
		ServiceID: svc.ID,
		Type:      "local",
		Address:   "127.0.0.1:8080",
		NodeID:    "", // local endpoint, no node binding needed
	})
	if err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	if ep == nil || ep.ID == "" {
		t.Fatal("endpoint was not created")
	}
	if !ep.Enabled {
		t.Fatal("newly created endpoint should be enabled by default")
	}
	t.Logf("created endpoint: id=%s address=%s type=%s enabled=%v", ep.ID, ep.Address, ep.Type, ep.Enabled)

	// Step 5: Create a Route
	edgemuxRepo := edgemux.NewRepository(st.DB)
	edgemuxSvc := edgemux.NewAppService(edgemuxRepo, logSvc)
	routeRepo := route.NewRepository(st.DB)
	routeSvc := route.NewAppService(routeRepo, logSvc, edgemuxSvc)
	rt, err := routeSvc.CreateRoute(ctx, route.CreateRouteInput{
		Domain:      "e2e-test.aegis.local",
		PathPrefix:  "/",
		StripPrefix: false,
		ServiceID:   svc.ID,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if rt == nil || rt.ID == "" {
		t.Fatal("route was not created")
	}
	if rt.Status != "active" {
		t.Fatalf("newly created route should be active, got status=%s", rt.Status)
	}
	t.Logf("created route: id=%s domain=%s status=%s", rt.ID, rt.Domain, rt.Status)

	// Step 6: Route is already active on creation — verify status
	if rt.Status != "active" {
		// EnableRoute only works if currently disabled
		if err := routeSvc.EnableRoute(ctx, rt.ID); err != nil {
			t.Fatalf("enable route: %v", err)
		}
	}
	t.Logf("route active: id=%s status=%s", rt.ID, rt.Status)

	// Step 7: Set up Apply service with FakeProxyAdapter and PendingState
	fakeAdapter := proxy.NewFakeAdapter()

	mdRepo := manageddomain.NewRepository(st.DB)
	exposureRepo := exposure.NewRepository(st.DB)
	endpointResolver := endpoint.NewResolver(endpointRepo)
	applyRepo := apply.NewRepository(st.DB)
	gwLinkRepo := gatewaylink.NewRepository(st.DB)

	safetyDeps := safety.Dependencies{
		RouteRepo:    routeRepo,
		MDRRepo:      mdRepo,
		EndpointRepo: endpointRepo,
		NodeRepo:     nil, // not needed for this test
		GWLinkRepo:   gwLinkRepo,
		ListenerRepo: nil, // not needed for this test
	}
	safetySvc := safety.NewService(safetyDeps)

	masterKey := secrets.MustDevKey(t)

	applySvc := apply.NewAppService(
		cfg, fakeAdapter,
		routeRepo, mdRepo, exposureRepo, serviceRepo,
		endpointResolver, applyRepo, logSvc,
		gwLinkRepo, safetySvc, masterKey,
	)

	// Wire up pending state (used by mutation hooks to MarkPending)
	pendingState := cluster.NewPendingState(st.DB)
	applySvc.SetPendingState(pendingState)

	// Mark pending to simulate mutations having occurred
	if err := pendingState.MarkPending("route created"); err != nil {
		t.Fatalf("mark pending: %v", err)
	}

	// Verify pending state is active
	status := pendingState.Status()
	if !status.Pending {
		t.Fatal("expected pending state to be true after MarkPending")
	}
	t.Logf("pending state verified: pending=%v reason=%s", status.Pending, status.Reason)

	// Step 8: Call DryRun to preview the Caddyfile
	plan, err := applySvc.DryRun(ctx)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if plan == nil {
		t.Fatal("dry run plan should not be nil")
	}
	t.Logf("dry run: route_count=%d managed_domain_count=%d warnings=%d",
		plan.RouteCount, plan.ManagedDomainCount, len(plan.Warnings))

	// Step 9: Verify the rendered Caddyfile contains expected directives
	rendered := plan.RenderedConfig
	if rendered == "" {
		t.Fatal("rendered config should not be empty")
	}
	t.Logf("rendered Caddyfile:\n%s", rendered)

	// Check for expected directives
	expectedDirectives := []string{
		"reverse_proxy",
		"127.0.0.1:8080",
		"e2e-test.aegis.local",
	}
	for _, directive := range expectedDirectives {
		if !strings.Contains(rendered, directive) {
			t.Errorf("rendered Caddyfile should contain %q", directive)
		} else {
			t.Logf("verified directive present: %s", directive)
		}
	}

	// Step 10: Call Apply
	// Write an empty Caddyfile first so Backup doesn't fail (file must exist)
	if err := os.WriteFile(cfg.Proxy.CaddyfilePath, []byte("# initial empty config\n"), 0644); err != nil {
		t.Fatalf("write initial caddyfile: %v", err)
	}

	applyPlan, err := applySvc.Apply(ctx)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applyPlan == nil {
		t.Fatal("apply plan should not be nil")
	}
	t.Logf("apply succeeded: route_count=%d backup_path=%s",
		applyPlan.RouteCount, applyPlan.BackupPath)

	// Verify pending state was cleared after successful apply
	statusAfter := pendingState.Status()
	if statusAfter.Pending {
		t.Error("expected pending state to be cleared after successful apply")
	} else {
		t.Log("pending state cleared after apply — verified")
	}

	// Verify the Caddyfile was written to the actual path
	caddyContent, err := os.ReadFile(cfg.Proxy.CaddyfilePath)
	if err != nil {
		t.Fatalf("read applied caddyfile: %v", err)
	}
	if len(caddyContent) == 0 {
		t.Fatal("applied Caddyfile should not be empty")
	}
	t.Logf("applied Caddyfile content length: %d bytes", len(caddyContent))

	// Verify history recorded the apply
	history, err := applySvc.History(ctx)
	if err != nil {
		t.Fatalf("get apply history: %v", err)
	}
	if len(history) == 0 {
		t.Error("expected at least one apply history entry")
	} else {
		t.Logf("apply history entries: %d, latest status=%s", len(history), history[0].Status)
	}
}
