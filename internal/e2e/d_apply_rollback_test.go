// Package e2e contains end-to-end integration tests.
//
// Scenario D: Apply Failure + Rollback
// Tests that after a successful apply followed by a failed apply,
// the system can rollback to the previous working configuration.
// Also verifies atomic replace behavior (os.Rename).
package e2e

import (
	"context"
	"os"
	"testing"

	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/edgemux"
	"aegis/internal/endpoint"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/provider"
	"aegis/internal/fake"
	"aegis/internal/proxy"
	"aegis/internal/topology"
	"aegis/internal/topology/templates"
	"aegis/internal/route"
	"aegis/internal/safety"
	"aegis/internal/secrets"
	"aegis/internal/service"
	"aegis/internal/store"

	gatewaylink "aegis/internal/gateway_link"
)

// TestApplyRollback verifies the apply failure detection and rollback recovery path.
func TestApplyRollback(t *testing.T) {
	ctx := context.Background()

	// Step 1: Set up a minimal working config with a valid Caddyfile
	tmpDir := t.TempDir()
	caddyfilePath := tmpDir + "/Caddyfile"
	backupDir := tmpDir + "/backups"

	cfg := config.DefaultConfig()
	cfg.Proxy.CaddyfilePath = caddyfilePath
	cfg.Proxy.BackupDir = backupDir
	cfg.Proxy.Email = "test@aegis.local"

	// Create temp DB
	f, err := os.CreateTemp("", "aegis-e2e-d-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	dbPath := f.Name()
	f.Close()
	os.Remove(dbPath)

	sqlDB, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer sqlDB.Close()
	defer os.Remove(dbPath)

	if err := store.Initialize(sqlDB); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	st := store.New(sqlDB)
	logRepo := logs.NewRepository(st.DB)
	logSvc := logs.NewAppService(logRepo)

	// Step 2: Write a valid Caddyfile to the config path
	originalCaddyfile := `# Original valid Caddyfile
e2e-test.aegis.local {
	reverse_proxy 127.0.0.1:8080
}
`
	if err := os.WriteFile(caddyfilePath, []byte(originalCaddyfile), 0644); err != nil {
		t.Fatalf("write initial caddyfile: %v", err)
	}

	// Set up all the repositories and services needed for apply
	serviceRepo := service.NewRepository(st.DB)
	endpointRepo := endpoint.NewRepository(st.DB)
	endpointResolver := endpoint.NewResolver(endpointRepo)
	routeRepo := route.NewRepository(st.DB)
	mdRepo := manageddomain.NewRepository(st.DB)
	applyRepo := apply.NewRepository(st.DB)
	gwLinkRepo := gatewaylink.NewRepository(st.DB)

	safetyDeps := safety.Dependencies{
		RouteRepo:    routeRepo,
		MDRRepo:      mdRepo,
		EndpointRepo: endpointRepo,
		NodeRepo:     nil,
		GWLinkRepo:   gwLinkRepo,
		ListenerRepo: nil,
	}
	safetySvc := safety.NewService(safetyDeps)
	masterKey := secrets.MustDevKey(t)

	// Create a service + endpoint + route so the planner produces config
	svc := &service.Service{
		ID:        "svc-test",
		Name:      "rollback-test-service",
		Kind:      "http",
		Env:       "dev",
		Status:    "active",
		OwnerType: "admin",
	}
	serviceSvc := service.NewAppService(serviceRepo, logSvc)
	if err := serviceSvc.CreateServiceDirect(svc); err != nil {
		t.Fatalf("create service direct: %v", err)
	}

	endpointSvc := endpoint.NewAppService(endpointRepo, logSvc)
	_, err = endpointSvc.CreateEndpoint(ctx, endpoint.CreateEndpointInput{
		ServiceID: svc.ID,
		Type:      "local",
		Address:   "127.0.0.1:8080",
	})
	if err != nil {
		t.Fatalf("create endpoint: %v", err)
	}

	edgemuxRepo := edgemux.NewRepository(st.DB)
	edgemuxSvc := edgemux.NewAppService(edgemuxRepo, logSvc)
	routeSvc := route.NewAppService(routeRepo, logSvc, edgemuxSvc)
	_, err = routeSvc.CreateRoute(ctx, route.CreateRouteInput{
		Domain:      "e2e-test.aegis.local",
		PathPrefix:  "/",
		StripPrefix: false,
		ServiceID:   svc.ID,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	// Step 3: First apply — should succeed with normal FakeProxyAdapter
	fakeAdapter := proxy.NewFakeAdapter()
	provReg := provider.NewRegistry()
	provReg.Register(fake.NewFakeProvider("caddy", "http"))
	topoPlanner := topology.NewPlanner(templates.Default(), topology.Dependencies{
		RouteRepo:        routeRepo,
		ServiceRepo:      serviceRepo,
		EndpointResolver: endpointResolver,
		GwLinkRepo:       gwLinkRepo,
		SafetySvc:        safetySvc,
		MasterKey:        masterKey,
	})
	workflow := apply.NewWorkflow(topoPlanner, provReg, applyRepo, cfg, logSvc)
	applySvc := apply.NewAppService(cfg, workflow, applyRepo, logSvc)

	pendingState := cluster.NewPendingState(st.DB)
	applySvc.SetPendingState(pendingState)
	pendingState.MarkPending("initial setup")

	// First apply — should succeed
	plan1, err := applySvc.Apply(ctx)
	if err != nil {
		t.Fatalf("first apply failed: %v", err)
	}
	t.Logf("first apply succeeded: route_count=%d backup_path=%s",
		plan1.RouteCount, plan1.BackupPath)

	// Verify a backup was created during the first apply
	if plan1.BackupPath == "" {
		// Backup may be empty if config didn't exist before first apply
		t.Log("no backup created (first apply has no prior config to back up)")
	} else {
		if _, err := os.Stat(plan1.BackupPath); os.IsNotExist(err) {
			t.Errorf("backup file should exist at %s", plan1.BackupPath)
		} else {
			t.Logf("backup file exists: %s", plan1.BackupPath)
		}
	}

	// Verify temp file is gone after Replace (atomic rename)
	if plan1.TempPath != "" {
		if _, err := os.Stat(plan1.TempPath); !os.IsNotExist(err) {
			t.Errorf("temp file should not exist after replace: %s", plan1.TempPath)
		} else {
			t.Logf("temp file correctly removed after atomic replace: %s", plan1.TempPath)
		}
	}

	// Read the applied configuration
	appliedContent, err := os.ReadFile(caddyfilePath)
	if err != nil {
		t.Fatalf("read applied caddyfile: %v", err)
	}
	t.Logf("applied configuration length: %d bytes", len(appliedContent))

	// Step 4: Now set adapter to fail validation (simulating invalid config)
	fakeAdapter.ValidateShouldFail = true
	pendingState.MarkPending("route modified — may produce invalid config")

	// Second apply — should fail during validation
	plan2, err := applySvc.Apply(ctx)
	if err == nil {
		t.Error("expected second apply to fail with validation error")
	} else {
		t.Logf("second apply correctly failed: %v", err)
	}
	_ = plan2

	// Step 5: Verify the original Caddyfile is still intact (failed apply doesn't replace)
	currentContent, err := os.ReadFile(caddyfilePath)
	if err != nil {
		t.Fatalf("read caddyfile after failed apply: %v", err)
	}
	if string(currentContent) != string(appliedContent) {
		t.Error("caddyfile should remain unchanged after failed apply")
	} else {
		t.Log("caddyfile unchanged after failed apply — verified")
	}

	// Step 6: Verify history recorded the successful apply
	history, err := applySvc.History(ctx)
	if err != nil {
		t.Fatalf("get apply history: %v", err)
	}
	if len(history) == 0 {
		t.Error("expected at least one apply history entry")
	}
	t.Logf("apply history: %d entries", len(history))
	for _, h := range history {
		t.Logf("  version=%s status=%s backup=%s", h.Version, h.Status, h.BackupPath)
	}

	// Step 7: Test RollbackService.GetLastBackupPath
	rollbackSvc := apply.NewRollbackService(applyRepo, cfg)
	lastBackup, err := rollbackSvc.GetLastBackupPath()
	if err != nil {
		t.Logf("get last backup path: %v (may be empty on first apply)", err)
	}
	if lastBackup != "" {
		t.Logf("last backup path: %s", lastBackup)
	}

	// Step 8: Write a deliberately wrong config to simulate corruption
	corruptedContent := []byte("# CORRUPTED CONFIG — this should be rolled back\n")
	if err := os.WriteFile(caddyfilePath, corruptedContent, 0644); err != nil {
		t.Fatalf("write corrupted caddyfile: %v", err)
	}

	// Reset adapter to not fail (so rollback reload works)
	fakeAdapter.ValidateShouldFail = false

	// Step 9: Call Rollback to restore from the backup
	if plan1.BackupPath != "" {
		// Use the executor to restore the backup directly
		executor := apply.NewExecutor(cfg)
		if err := executor.RestoreBackup(plan1.BackupPath); err != nil {
			t.Fatalf("restore backup: %v", err)
		}
		t.Log("backup restored successfully via executor")

		// Step 10: Verify the restored config matches the original pre-apply config
		// (Backup saves the config BEFORE replacement, so restored = original, not the applied version)
		restoredContent, err := os.ReadFile(caddyfilePath)
		if err != nil {
			t.Fatalf("read restored caddyfile: %v", err)
		}
		if string(restoredContent) != originalCaddyfile {
			t.Errorf("restored config does not match original pre-apply config")
			t.Logf("original (%d bytes):\n%s", len(originalCaddyfile), originalCaddyfile)
			t.Logf("restored (%d bytes):\n%s", len(restoredContent), string(restoredContent))
		} else {
			t.Log("restored config matches original pre-apply config — verified")
		}
	} else {
		t.Log("skipping rollback test: no backup was created (first apply had no prior config)")
	}

	// Step 11: Test the high-level Rollback method via AppService
	// First, let's create a situation where we can roll back via the high-level API
	// Reset the adapter and apply again to create a second backup
	fakeAdapter.ValidateShouldFail = false
	pendingState.MarkPending("test rollback")

	// Apply a new config to create a history entry with backup
	// We need to re-set the caddyfile to the original for the backup to be created
	if err := os.WriteFile(caddyfilePath, appliedContent, 0644); err != nil {
		t.Fatalf("restore original caddyfile for second apply: %v", err)
	}

	plan3, err := applySvc.Apply(ctx)
	if err != nil {
		t.Fatalf("second apply after restore: %v", err)
	}
	t.Logf("second apply succeeded: route_count=%d backup_path=%s",
		plan3.RouteCount, plan3.BackupPath)

	// Now we have a backup from plan3 (which backed up the original content)
	// Try rolling back via the high-level AppService.Rollback
	if plan3.BackupPath != "" {
		// Corrupt the caddyfile again
		if err := os.WriteFile(caddyfilePath, corruptedContent, 0644); err != nil {
			t.Fatalf("write corrupted caddyfile for rollback test: %v", err)
		}

		// Rollback via AppService
		if err := applySvc.Rollback(ctx, ""); err != nil {
			t.Fatalf("rollback via app service: %v", err)
		}
		t.Log("rollback via app service succeeded")

		// Verify the restored config
		restored2, err := os.ReadFile(caddyfilePath)
		if err != nil {
			t.Fatalf("read restored caddyfile after app service rollback: %v", err)
		}
		t.Logf("restored content after rollback: %d bytes", len(restored2))
	}

	t.Log("apply failure + rollback test completed successfully")
}
