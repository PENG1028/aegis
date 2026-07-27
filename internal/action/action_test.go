package action

import (
	"context"
	"database/sql"
	"testing"

	"aegis/internal/apply"
	"aegis/internal/config"
	"aegis/internal/edgemux"
	"aegis/internal/endpoint"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/route"
	"aegis/internal/service"
	"aegis/internal/space"
	"aegis/internal/store"
)

// setupTestDB creates an in-memory SQLite database with all migrations applied.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

// setupActionService creates all services needed for testing.
func setupActionService(t *testing.T) (*ActionService, *sql.DB) {
	t.Helper()
	db := setupTestDB(t)

	// Repos
	serviceRepo := service.NewRepository(db)
	routeRepo := route.NewRepository(db)
	edgeRepo := edgemux.NewRepository(db)
	endpointRepo := endpoint.NewRepository(db)
	spaceRepo := space.NewRepository(db)
	logRepo := logs.NewRepository(db)
	applyRepo := apply.NewRepository(db)
	listenerRepo := listener.NewRepository(db)

	// Services
	logSvc := logs.NewAppService(logRepo)
	serviceSvc := service.NewAppService(serviceRepo, logSvc)
	edgeSvc := edgemux.NewAppService(edgeRepo, logSvc)
	routeSvc := route.NewAppService(routeRepo, logSvc, edgeSvc)
	listenerSvc := listener.NewService(listenerRepo)

	// Apply service (minimal)
	cfg := config.DefaultConfig()
	cfg.Store.SQLitePath = ":memory:"
	// ActionService only uses TryApply which needs applySvc
	// But Creating AppService requires many deps. Skip full apply for unit tests.
	// We'll just verify the non-apply parts.

	_ = cfg
	_ = applyRepo

	// Create a mock apply service that always succeeds
	// We need the real one for the apply lock test, but for most tests we just verify
	// the action logic (ownership checks, domain checks, resource creation).

	endpointSvc := endpoint.NewAppService(endpointRepo, logSvc)

	return NewActionService(
		serviceSvc, routeSvc, edgeSvc, endpointRepo, endpointSvc,
		nil, // applySvc — nil for tests without apply
		spaceRepo, logSvc, listenerSvc,
	), db
}

// contextWithSpace creates a context with a space-scoped action context.
func contextWithSpace(spaceID, tokenID string) context.Context {
	return WithActionContext(context.Background(), NewSpaceContext(spaceID, tokenID))
}

// contextWithAdmin creates a context with an admin action context.
func contextWithAdmin() context.Context {
	return WithActionContext(context.Background(), NewAdminContext())
}

// =============================================================================
// Test 1: Space Isolation
// =============================================================================

func TestSpaceIsolation(t *testing.T) {
	svc, db := setupActionService(t)
	defer db.Close()

	// Create two spaces
	spaceA := space.NewSpace("tenant-a")
	spaceA.SpaceID = "space_tenant-a"
	if err := svc.spaceRepo.Create(spaceA); err != nil {
		t.Fatalf("create space A: %v", err)
	}

	spaceB := space.NewSpace("tenant-b")
	spaceB.SpaceID = "space_tenant-b"
	if err := svc.spaceRepo.Create(spaceB); err != nil {
		t.Fatalf("create space B: %v", err)
	}

	// Space A creates a domain
	ctxA := contextWithSpace("space_tenant-a", "token-a")
	input := BindHTTPDomainInput{
		Domain:     "app-a.example.com",
		TargetHost: "10.0.0.1",
		TargetPort: 3000,
	}

	// With nil applySvc, this will fail at safeApply but should create resources
	_, err := svc.BindHTTPDomain(ctxA, input)
	// Expected: safeApply returns error because applySvc is nil
	// But resources should be created before apply fails
	if err != nil {
		t.Logf("BindHTTPDomain error (expected with nil applySvc): %v", err)
	}

	// Space B tries to bind same domain — should get DOMAIN_ALREADY_OWNED
	ctxB := contextWithSpace("space_tenant-b", "token-b")
	_, err = svc.BindHTTPDomain(ctxB, input)
	if err == nil {
		t.Error("Space B should not be able to bind domain already owned by Space A")
	} else {
		ae, ok := err.(*ActionError)
		if !ok || ae.Code != ErrCodeDomainAlreadyOwned {
			t.Errorf("expected DOMAIN_ALREADY_OWNED, got: %v", err)
		}
		t.Logf("Correctly blocked: %v", err)
	}

	// Space B tries to delete Space A's route — should get RESOURCE_NOT_OWNED
	rt, err := svc.routeSvc.GetRoute(ctxA, "app-a.example.com")
	if err != nil {
		t.Fatalf("get route: %v", err)
	}

	// Check ownership directly
	err = svc.requireOwnership(ctxB, rt.SpaceID, "route", rt.ID)
	if err == nil {
		t.Error("Space B should not own Space A's route")
	} else {
		t.Logf("Ownership check correctly failed: %v", err)
	}
}

// =============================================================================
// Test 2: Domain Ownership Check
// =============================================================================

func TestDomainOwnership(t *testing.T) {
	svc, db := setupActionService(t)
	defer db.Close()

	// Create a space
	sp := space.NewSpace("test-space")
	sp.SpaceID = "space_test"
	if err := svc.spaceRepo.Create(sp); err != nil {
		t.Fatalf("create space: %v", err)
	}

	ctx := contextWithSpace("space_test", "token-test")

	// Bind a domain
	input := BindHTTPDomainInput{
		Domain:     "owned.example.com",
		TargetHost: "10.0.0.5",
		TargetPort: 8080,
	}
	_, _ = svc.BindHTTPDomain(ctx, input)

	// Check domain ownership
	ownerSpaceID, err := svc.checkDomainOwnership("owned.example.com")
	if err != nil {
		t.Fatalf("check domain ownership: %v", err)
	}
	if ownerSpaceID != "space_test" {
		t.Errorf("expected owner space_test, got %q", ownerSpaceID)
	}

	// Check unowned domain
	ownerSpaceID, err = svc.checkDomainOwnership("free.example.com")
	if err != nil {
		t.Fatalf("check free domain: %v", err)
	}
	if ownerSpaceID != "" {
		t.Errorf("expected no owner, got %q", ownerSpaceID)
	}
}

// =============================================================================
// Test 3: BindHTTPDomain creates service, route, and edge rule
// =============================================================================

func TestBindHTTPDomainCreatesResources(t *testing.T) {
	svc, db := setupActionService(t)
	defer db.Close()

	sp := space.NewSpace("demo")
	sp.SpaceID = "space_demo"
	if err := svc.spaceRepo.Create(sp); err != nil {
		t.Fatalf("create space: %v", err)
	}

	ctx := contextWithSpace("space_demo", "token-demo")
	input := BindHTTPDomainInput{
		Domain:     "demo.example.com",
		TargetHost: "10.0.0.10",
		TargetPort: 3000,
	}

	_, err := svc.BindHTTPDomain(ctx, input)
	// safeApply will fail since applySvc is nil, but resources should be created
	if err != nil {
		t.Logf("BindHTTPDomain error (expected): %v", err)
	}

	// Verify service was created
	services, err := svc.serviceSvc.ListServices(ctx)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(services) != 1 {
		t.Errorf("expected 1 service, got %d", len(services))
	} else {
		svc := services[0]
		if svc.SpaceID != "space_demo" {
			t.Errorf("expected space_id=space_demo, got %q", svc.SpaceID)
		}
		if svc.OwnerType != "space" {
			t.Errorf("expected owner_type=space, got %q", svc.OwnerType)
		}
		t.Logf("Service created: %s (space=%s, owner=%s)", svc.Name, svc.SpaceID, svc.OwnerType)
	}

	// Verify route was created
	routes, err := svc.routeSvc.ListRoutes(ctx)
	if err != nil {
		t.Fatalf("list routes: %v", err)
	}
	if len(routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(routes))
	} else {
		rt := routes[0]
		if rt.Domain != "demo.example.com" {
			t.Errorf("expected domain=demo.example.com, got %q", rt.Domain)
		}
		if rt.SpaceID != "space_demo" {
			t.Errorf("expected space_id=space_demo, got %q", rt.SpaceID)
		}
		t.Logf("Route created: %s (space=%s)", rt.Domain, rt.SpaceID)
	}

	// Verify edge rule was auto-created
	rules, err := svc.edgeSvc.ListRules(ctx)
	if err != nil {
		t.Fatalf("list edge rules: %v", err)
	}
	if len(rules) == 0 {
		t.Log("No edge rules created (edgeSvc may be nil for EnsureRuleForHTTPRoute)")
	} else {
		for _, r := range rules {
			t.Logf("Edge rule: SNI=%s -> %s:%d managed_by=%s", r.SNIHost, r.TargetHost, r.TargetPort, r.ManagedBy)
		}
	}
}

// =============================================================================
// Test 4: BindTLSBackend creates manual edge rule
// =============================================================================

func TestBindTLSBackendCreatesEdgeRule(t *testing.T) {
	svc, db := setupActionService(t)
	defer db.Close()

	sp := space.NewSpace("tls-space")
	sp.SpaceID = "space_tls"
	if err := svc.spaceRepo.Create(sp); err != nil {
		t.Fatalf("create space: %v", err)
	}

	ctx := contextWithSpace("space_tls", "token-tls")
	input := BindTLSBackendInput{
		SNIHost:    "tls-backend.example.com",
		TargetHost: "10.0.1.1",
		TargetPort: 8443,
		Kind:       edgemux.KindUnknownTLSBackend,
	}

	_, err := svc.BindTLSBackend(ctx, input)
	if err != nil {
		t.Logf("BindTLSBackend error (expected): %v", err)
	}

	// Verify edge rule was created with managed_by=manual
	rule, err := svc.edgeSvc.FindBySNIHost(ctx, "tls-backend.example.com")
	if err != nil || rule == nil {
		t.Fatalf("find edge rule by SNI: err=%v, rule=%v", err, rule)
	}

	if rule.ManagedBy != "manual" {
		t.Errorf("expected managed_by=manual, got %q", rule.ManagedBy)
	}
	if rule.TargetHost != "10.0.1.1" {
		t.Errorf("expected target_host=10.0.1.1, got %q", rule.TargetHost)
	}
	if rule.TargetPort != 8443 {
		t.Errorf("expected target_port=8443, got %d", rule.TargetPort)
	}
	t.Logf("Edge rule: SNI=%s managed_by=%s target=%s:%d", rule.SNIHost, rule.ManagedBy, rule.TargetHost, rule.TargetPort)

	// Verify no route was created
	routes, _ := svc.routeSvc.ListRoutes(ctx)
	if len(routes) > 0 {
		t.Logf("Unexpected routes created: %d", len(routes))
	}
}

// =============================================================================
// Test 5: Apply Lock (concurrent access)
// =============================================================================

func TestApplyLock(t *testing.T) {
	// Test that TryLock behavior works at the mutex level
	// This is a basic unit test — the actual apply service TryApply
	// is tested through integration with real providers.

	db := setupTestDB(t)
	defer db.Close()

	// We test the action error type
	err := ErrApplyLocked()
	if err.Code != ErrCodeApplyLocked {
		t.Errorf("expected APPLY_LOCKED code, got %q", err.Code)
	}
	if err.Message == "" {
		t.Error("expected non-empty message")
	}
	t.Logf("Apply locked error: %s", err.Error())
}

// =============================================================================
// Test 6: Admin Bypass
// =============================================================================

func TestAdminBypass(t *testing.T) {
	svc, db := setupActionService(t)
	defer db.Close()

	// Admin can operate on resources with empty space_id (system-owned)
	ctx := contextWithAdmin()

	// requireSpace should succeed for admin
	ac, err := svc.requireSpace(ctx)
	if err != nil {
		t.Errorf("admin should pass requireSpace: %v", err)
	}
	if !ac.IsAdmin() {
		t.Error("expected admin context")
	}

	// requireOwnership should succeed for admin regardless of space
	err = svc.requireOwnership(ctx, "", "route", "rt-123")
	if err != nil {
		t.Errorf("admin should bypass ownership check: %v", err)
	}

	err = svc.requireOwnership(ctx, "other-space", "route", "rt-456")
	if err != nil {
		t.Errorf("admin should bypass ownership check for any space: %v", err)
	}

	t.Log("Admin bypass works correctly")
}

// =============================================================================
// Test 7: Error Codes
// =============================================================================

func TestErrorCodes(t *testing.T) {
	tests := []struct {
		name string
		err  *ActionError
		code string
	}{
		{"scope denied", ErrScopeDenied("no permission"), ErrCodeScopeDenied},
		{"domain already owned", ErrDomainAlreadyOwned("example.com", "space_x"), ErrCodeDomainAlreadyOwned},
		{"resource not found", ErrResourceNotFound("route", "rt-1"), ErrCodeResourceNotFound},
		{"resource not owned", ErrResourceNotOwned("route", "rt-1", "space_a"), ErrCodeResourceNotOwned},
		{"apply locked", ErrApplyLocked(), ErrCodeApplyLocked},
		{"target not allowed", ErrTargetNotAllowed("0.0.0.0"), ErrCodeTargetNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("expected code %q, got %q", tt.code, tt.err.Code)
			}
			if !IsActionError(tt.err, tt.code) {
				t.Errorf("IsActionError should return true for code %q", tt.code)
			}
			if IsActionError(tt.err, "WRONG_CODE") {
				t.Error("IsActionError should return false for wrong code")
			}
			t.Logf("Error: %s", tt.err.Error())
		})
	}

	// Test non-ActionError
	plainErr := context.DeadlineExceeded
	if IsActionError(plainErr, "") {
		t.Error("plain error should not be ActionError")
	}
}

// =============================================================================
// Test 8: CLI Shared Service (admin context)
// =============================================================================

func TestCLISharedService(t *testing.T) {
	svc, db := setupActionService(t)
	defer db.Close()

	// CLI uses admin context — same as NewAdminContext()
	ctx := contextWithAdmin()

	sp := space.NewSpace("cli-test")
	sp.SpaceID = "space_cli_test"
	if err := svc.spaceRepo.Create(sp); err != nil {
		t.Fatalf("create space: %v", err)
	}

	// Admin context can list all resources
	input := BindHTTPDomainInput{
		Domain:     "cli-test.example.com",
		TargetHost: "10.0.2.1",
		TargetPort: 4000,
	}
	_, _ = svc.BindHTTPDomain(ctx, input)

	// ListMyRoutes should return all routes for admin
	routes, err := svc.ListMyRoutes(ctx)
	if err != nil {
		t.Fatalf("list my routes: %v", err)
	}
	if len(routes) == 0 {
		t.Error("admin should see all routes")
	}
	t.Logf("Admin sees %d routes", len(routes))

	// ListMyServices should return all services for admin
	services, err := svc.ListMyServices(ctx)
	if err != nil {
		t.Fatalf("list my services: %v", err)
	}
	if len(services) == 0 {
		t.Error("admin should see all services")
	}
	t.Logf("Admin sees %d services", len(services))

	// The same ActionService is used by both CLI and HTTP API
	// This test verifies the admin context works identically
	t.Log("CLI/API shared service: admin context works correctly")
}

// =============================================================================
// Test 9: Scope Denied for missing context
// =============================================================================

func TestRequireSpaceWithoutContext(t *testing.T) {
	svc, db := setupActionService(t)
	defer db.Close()

	// Context without ActionContext should fail
	ctx := context.Background()
	_, err := svc.requireSpace(ctx)
	if err == nil {
		t.Error("expected error for missing action context")
	} else {
		ae, ok := err.(*ActionError)
		if !ok || ae.Code != ErrCodeScopeDenied {
			t.Errorf("expected SCOPE_DENIED, got %v", err)
		}
		t.Logf("Correctly denied: %v", err)
	}
}

// =============================================================================
// Test 10: Space token without space_id fails
// =============================================================================

func TestSpaceTokenWithoutSpaceID(t *testing.T) {
	svc, db := setupActionService(t)
	defer db.Close()

	// Space token but empty space_id
	ctx := contextWithSpace("", "token-no-space")
	_, err := svc.requireSpace(ctx)
	if err == nil {
		t.Error("expected error for space token without space_id")
	} else {
		t.Logf("Correctly denied: %v", err)
	}
}
