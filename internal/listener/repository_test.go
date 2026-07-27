package listener

import (
	"database/sql"
	"testing"
	"time"

	"aegis/internal/store"
)

func setupListenerTestDB(t *testing.T) *sql.DB {
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

func TestListenerNodeIDRoundTrip(t *testing.T) {
	db := setupListenerTestDB(t)
	repo := NewRepository(db)

	now := time.Now()
	l := &Listener{
		ID:        "listener_test_1",
		NodeID:    "node-a",
		Provider:  "caddy_http",
		Protocol:  "http",
		BindIP:    "0.0.0.0",
		Port:      8080,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repo.Create(l); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// FindAll
	all, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 listener, got %d", len(all))
	}
	if all[0].NodeID != "node-a" {
		t.Errorf("expected NodeID 'node-a', got %q", all[0].NodeID)
	}

	// FindByNodeID
	byNode, err := repo.FindByNodeID("node-a")
	if err != nil {
		t.Fatalf("FindByNodeID: %v", err)
	}
	if len(byNode) != 1 {
		t.Errorf("expected 1 listener for node-a, got %d", len(byNode))
	}

	// FindByNodeID for non-existent node
	empty, err := repo.FindByNodeID("node-b")
	if err != nil {
		t.Fatalf("FindByNodeID(node-b): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected 0 listeners for node-b, got %d", len(empty))
	}
}

func TestListenerUpdate(t *testing.T) {
	db := setupListenerTestDB(t)
	repo := NewRepository(db)

	now := time.Now()
	l := &Listener{
		ID:        "listener_test_2",
		NodeID:    "node-a",
		Provider:  "caddy_http",
		Protocol:  "http",
		BindIP:    "0.0.0.0",
		Port:      9090,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.Create(l)

	// Update node assignment
	l.NodeID = "node-b"
	l.UpdatedAt = time.Now()
	if err := repo.Update(l); err != nil {
		t.Fatalf("Update: %v", err)
	}

	all, _ := repo.FindAll()
	if all[0].NodeID != "node-b" {
		t.Errorf("expected NodeID 'node-b' after update, got %q", all[0].NodeID)
	}
}

func TestListenerFindByBind(t *testing.T) {
	db := setupListenerTestDB(t)
	repo := NewRepository(db)

	now := time.Now()
	l := &Listener{
		ID:        "listener_bind_test",
		NodeID:    "node-a",
		Provider:  "caddy_http",
		Protocol:  "http",
		BindIP:    "0.0.0.0",
		Port:      443,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.Create(l)

	// Exact match
	found, err := repo.FindByBind("0.0.0.0", 443)
	if err != nil {
		t.Fatalf("FindByBind: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find listener")
	}
	if found.NodeID != "node-a" {
		t.Errorf("expected NodeID 'node-a', got %q", found.NodeID)
	}

	// Wildcard match
	found, err = repo.FindByBind("192.168.1.1", 443)
	if err != nil {
		t.Fatalf("FindByBind wildcard: %v", err)
	}
	if found == nil {
		t.Error("expected wildcard bind to match 0.0.0.0")
	}

	// No match
	found, err = repo.FindByBind("127.0.0.1", 9999)
	if err != nil {
		t.Fatalf("FindByBind no match: %v", err)
	}
	if found != nil {
		t.Error("expected no match for unused port")
	}
}
