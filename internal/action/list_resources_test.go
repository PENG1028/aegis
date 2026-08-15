package action

import (
	"context"
	"testing"

	"aegis/internal/logs"
)

// TestListMyOperationsSpaceIsolation is a regression test: space tokens used
// to receive EVERY space's "action.*" operation logs (the filter was a
// prefix match, not a space match) — a cross-tenant information leak. Now
// logs carry space_id (migration 049) and space tokens only see their own.
func TestListMyOperationsSpaceIsolation(t *testing.T) {
	svc, db := setupActionService(t)
	defer db.Close()

	// Two spaces each create a project (which writes an action-scoped log).
	ctxA := contextWithSpace("space_a", "tok_a")

	// Write logs attributed to two different spaces (as action handlers do
	// via LogWithSpace) plus one system-wide log with no space.
	logSvc := logs.NewAppService(logs.NewRepository(db))
	logSvc.LogWithSpace(context.Background(), "space_a", "action.create-service", "service", "svc-a", "success", "created", "service")
	logSvc.LogWithSpace(context.Background(), "space_b", "action.create-service", "service", "svc-b", "success", "created", "service")
	logSvc.Log(context.Background(), "system.apply", "apply", "", "success", "applied", "system")

	// Space A must only see its own operations.
	opsA, err := svc.ListMyOperations(ctxA, 50)
	if err != nil {
		t.Fatalf("ListMyOperations(A): %v", err)
	}
	if len(opsA) == 0 {
		t.Fatal("space A should see its own operation")
	}
	for _, op := range opsA {
		if op.SpaceID != "space_a" {
			t.Fatalf("space A received operation of space %q: %+v", op.SpaceID, op)
		}
	}

	// Admin sees everything.
	opsAdmin, err := svc.ListMyOperations(contextWithAdmin(), 50)
	if err != nil {
		t.Fatalf("ListMyOperations(admin): %v", err)
	}
	if len(opsAdmin) < 3 {
		t.Fatalf("admin should see all operations, got %d", len(opsAdmin))
	}
}
