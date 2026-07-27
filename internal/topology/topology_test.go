package topology

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTopoTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS topology_edges (
			id TEXT PRIMARY KEY, from_node_id TEXT NOT NULL, to_node_id TEXT NOT NULL,
			private_reachable INTEGER NOT NULL DEFAULT 0, public_reachable INTEGER NOT NULL DEFAULT 0,
			preferred_gateway_id TEXT DEFAULT '', gateway_link_id TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'unknown', last_verified_at TEXT DEFAULT '',
			last_error TEXT DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			UNIQUE(from_node_id, to_node_id)
		)
	`)
	if err != nil {
		t.Fatalf("create topology_edges: %v", err)
	}
	return db
}

func TestCreateTopologyEdge(t *testing.T) {
	db := setupTopoTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	edge, err := svc.CreateOrUpdateEdge(CreateEdgeInput{
		FromNodeID: "nd_a", ToNodeID: "nd_b",
		PrivateReachable: boolPtr(true), PublicReachable: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("create edge: %v", err)
	}
	if edge.FromNodeID != "nd_a" || edge.ToNodeID != "nd_b" {
		t.Errorf("unexpected node ids: %s → %s", edge.FromNodeID, edge.ToNodeID)
	}
	if !edge.PrivateReachable {
		t.Error("expected private_reachable = true")
	}
	if edge.Status != StatusUnknown {
		t.Errorf("expected status unknown, got %s", edge.Status)
	}
}

func TestRejectSelfEdge(t *testing.T) {
	db := setupTopoTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	_, err := svc.CreateOrUpdateEdge(CreateEdgeInput{
		FromNodeID: "nd_a", ToNodeID: "nd_a",
	})
	if err == nil {
		t.Error("expected error for self-edge")
	}
}

func TestGetMatrix(t *testing.T) {
	db := setupTopoTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	svc.CreateOrUpdateEdge(CreateEdgeInput{FromNodeID: "nd_a", ToNodeID: "nd_b"})
	svc.CreateOrUpdateEdge(CreateEdgeInput{FromNodeID: "nd_b", ToNodeID: "nd_c"})

	edges, err := svc.GetMatrix()
	if err != nil {
		t.Fatalf("get matrix: %v", err)
	}
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}
}

func TestGetPath(t *testing.T) {
	db := setupTopoTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	svc.CreateOrUpdateEdge(CreateEdgeInput{
		FromNodeID: "nd_a", ToNodeID: "nd_b",
		PrivateReachable: boolPtr(true),
	})

	path, err := svc.GetPath("nd_a", "nd_b")
	if err != nil {
		t.Fatalf("get path: %v", err)
	}
	if path.Status != StatusUnknown {
		t.Errorf("expected status unknown, got %s", path.Status)
	}
	if !path.PrivateReachable {
		t.Error("expected private_reachable = true")
	}
}

func TestGetPathUnknown(t *testing.T) {
	db := setupTopoTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	path, err := svc.GetPath("nd_a", "nd_z")
	if err != nil {
		t.Fatalf("get path: %v", err)
	}
	if path.Status != StatusUnknown {
		t.Errorf("expected status unknown for non-existent edge, got %s", path.Status)
	}
}

func TestUpdateEdgeStatus(t *testing.T) {
	db := setupTopoTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	svc.CreateOrUpdateEdge(CreateEdgeInput{FromNodeID: "nd_a", ToNodeID: "nd_b"})
	err := svc.SetEdgeStatus("nd_a", "nd_b", StatusVerified, "")
	if err != nil {
		t.Fatalf("set status: %v", err)
	}

	edge, _ := svc.GetEdge("nd_a", "nd_b")
	if edge == nil || edge.Status != StatusVerified {
		t.Errorf("expected status verified, got %v", edge)
	}
}

func TestListEdges(t *testing.T) {
	db := setupTopoTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	svc.CreateOrUpdateEdge(CreateEdgeInput{FromNodeID: "nd_a", ToNodeID: "nd_b"})
	svc.CreateOrUpdateEdge(CreateEdgeInput{FromNodeID: "nd_a", ToNodeID: "nd_c"})

	edges, err := svc.ListEdges()
	if err != nil {
		t.Fatalf("list edges: %v", err)
	}
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}
}

func TestCreateEdgeRequiresBothNodes(t *testing.T) {
	db := setupTopoTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	_, err := svc.CreateOrUpdateEdge(CreateEdgeInput{FromNodeID: "nd_a", ToNodeID: ""})
	if err == nil {
		t.Error("expected error for empty to_node_id")
	}
}

func boolPtr(b bool) *bool {
	return &b
}
