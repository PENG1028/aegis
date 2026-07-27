package endpoint

import (
	"context"
	"database/sql"
	"testing"

	"aegis/internal/logs"
	"aegis/internal/store"
)

func setupEndpointTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return db
}

type testMutationHook struct {
	calls []string
}

func (h *testMutationHook) OnEndpointChanged(ctx context.Context, endpointID string) error {
	h.calls = append(h.calls, endpointID)
	return nil
}

func TestCreateEndpoint(t *testing.T) {
	db := setupEndpointTestDB(t)
	repo := NewRepository(db)
	logRepo := logs.NewRepository(db)
	logSvc := logs.NewAppService(logRepo)
	svc := NewAppService(repo, logSvc)

	hook := &testMutationHook{}
	svc.SetMutationHook(hook)

	ep, err := svc.CreateEndpoint(context.Background(), CreateEndpointInput{
		ServiceID: "svc_test",
		Type:      "local",
		Address:   "127.0.0.1:3001",
		NodeID:    "node-a",
	})
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	if ep.ID == "" {
		t.Error("expected non-empty endpoint ID")
	}
	if ep.NodeID != "node-a" {
		t.Errorf("expected NodeID 'node-a', got %q", ep.NodeID)
	}
	if len(hook.calls) != 1 || hook.calls[0] != ep.ID {
		t.Errorf("expected hook call with endpoint ID %q, got %v", ep.ID, hook.calls)
	}

	// Verify it's in the DB
	found, err := svc.FindByID(ep.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil {
		t.Fatal("endpoint not found after create")
	}
	if found.Address != "127.0.0.1:3001" {
		t.Errorf("expected address '127.0.0.1:3001', got %q", found.Address)
	}
}

func TestEnableDisableEndpoint(t *testing.T) {
	db := setupEndpointTestDB(t)
	repo := NewRepository(db)
	logRepo := logs.NewRepository(db)
	logSvc := logs.NewAppService(logRepo)
	svc := NewAppService(repo, logSvc)

	hook := &testMutationHook{}
	svc.SetMutationHook(hook)

	ep, err := svc.CreateEndpoint(context.Background(), CreateEndpointInput{
		ServiceID: "svc_test",
		Type:      "local",
		Address:   "127.0.0.1:3002",
	})
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	hook.calls = nil

	// Disable
	if err := svc.DisableEndpoint(context.Background(), ep.ID); err != nil {
		t.Fatalf("DisableEndpoint: %v", err)
	}
	found, _ := svc.FindByID(ep.ID)
	if found.Enabled {
		t.Error("expected endpoint to be disabled")
	}
	if len(hook.calls) != 1 {
		t.Errorf("expected 1 hook call on disable, got %d", len(hook.calls))
	}
	hook.calls = nil

	// Enable
	if err := svc.EnableEndpoint(context.Background(), ep.ID); err != nil {
		t.Fatalf("EnableEndpoint: %v", err)
	}
	found, _ = svc.FindByID(ep.ID)
	if !found.Enabled {
		t.Error("expected endpoint to be enabled")
	}
	if len(hook.calls) != 1 {
		t.Errorf("expected 1 hook call on enable, got %d", len(hook.calls))
	}
}

func TestDeleteEndpoint(t *testing.T) {
	db := setupEndpointTestDB(t)
	repo := NewRepository(db)
	logRepo := logs.NewRepository(db)
	logSvc := logs.NewAppService(logRepo)
	svc := NewAppService(repo, logSvc)

	hook := &testMutationHook{}
	svc.SetMutationHook(hook)

	ep, err := svc.CreateEndpoint(context.Background(), CreateEndpointInput{
		ServiceID: "svc_test",
		Type:      "public",
		Address:   "10.0.0.1:443",
	})
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	hook.calls = nil

	if err := svc.DeleteEndpoint(context.Background(), ep.ID); err != nil {
		t.Fatalf("DeleteEndpoint: %v", err)
	}
	found, _ := svc.FindByID(ep.ID)
	if found != nil {
		t.Error("expected endpoint to be deleted")
	}
	if len(hook.calls) != 1 {
		t.Errorf("expected 1 hook call on delete, got %d", len(hook.calls))
	}
}

func TestCreateEndpointDefaultType(t *testing.T) {
	db := setupEndpointTestDB(t)
	repo := NewRepository(db)
	logRepo := logs.NewRepository(db)
	logSvc := logs.NewAppService(logRepo)
	svc := NewAppService(repo, logSvc)

	ep, err := svc.CreateEndpoint(context.Background(), CreateEndpointInput{
		ServiceID: "svc_test",
		Address:   "127.0.0.1:8080",
	})
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	if ep.Type != "local" {
		t.Errorf("expected default type 'local', got %q", ep.Type)
	}
}

func TestCreateEndpointValidation(t *testing.T) {
	db := setupEndpointTestDB(t)
	repo := NewRepository(db)
	logRepo := logs.NewRepository(db)
	logSvc := logs.NewAppService(logRepo)
	svc := NewAppService(repo, logSvc)

	_, err := svc.CreateEndpoint(context.Background(), CreateEndpointInput{
		ServiceID: "",
		Address:   "127.0.0.1:8080",
	})
	if err == nil {
		t.Error("expected error for empty ServiceID")
	}

	_, err = svc.CreateEndpoint(context.Background(), CreateEndpointInput{
		ServiceID: "svc_test",
		Address:   "",
	})
	if err == nil {
		t.Error("expected error for empty Address")
	}
}

func TestFindByNodeID(t *testing.T) {
	db := setupEndpointTestDB(t)
	repo := NewRepository(db)
	logRepo := logs.NewRepository(db)
	logSvc := logs.NewAppService(logRepo)
	svc := NewAppService(repo, logSvc)

	// Create endpoints on different nodes
	svc.CreateEndpoint(context.Background(), CreateEndpointInput{
		ServiceID: "svc_a", Type: "local", Address: "127.0.0.1:3001", NodeID: "node-a",
	})
	svc.CreateEndpoint(context.Background(), CreateEndpointInput{
		ServiceID: "svc_b", Type: "local", Address: "127.0.0.1:3002", NodeID: "node-b",
	})
	svc.CreateEndpoint(context.Background(), CreateEndpointInput{
		ServiceID: "svc_c", Type: "local", Address: "127.0.0.1:3003", NodeID: "node-a",
	})

	eps, err := svc.FindByNodeID("node-a")
	if err != nil {
		t.Fatalf("FindByNodeID: %v", err)
	}
	if len(eps) != 2 {
		t.Errorf("expected 2 endpoints for node-a, got %d", len(eps))
	}
	for _, ep := range eps {
		if ep.NodeID != "node-a" {
			t.Errorf("expected all endpoints on node-a, got node_id=%q", ep.NodeID)
		}
	}
}
