package store

import (
	"path/filepath"
	"testing"
)

func TestMigration043CanonicalizesLegacyNodeIDs(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "aegis.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(createSchemaMigrationsTable); err != nil {
		t.Fatal(err)
	}
	for _, m := range AllMigrations() {
		if m.Version == "043" {
			break
		}
		if err := runMigration(db, m); err != nil {
			t.Fatalf("migration %s: %v", m.Version, err)
		}
	}

	ts := "2026-07-21T00:00:00Z"
	_, err = db.Exec(`INSERT INTO nodes (id, node_id, hostname, last_seen, created_at, updated_at) VALUES
		('legacy', 'VM-0-4-ubuntu', 'VM-0-4-ubuntu', ?, ?, ?),
		('stable', 'node_VM-0-4-ubuntu', 'VM-0-4-ubuntu', ?, ?, ?)`, ts, ts, ts, ts, ts, ts)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO endpoints (id, service_id, type, address, node_id, created_at, updated_at)
		VALUES ('ep_legacy', 'svc', 'local', '127.0.0.1:9100', 'VM-0-4-ubuntu', ?, ?)`, ts, ts)
	if err != nil {
		t.Fatal(err)
	}

	if err := runMigration(db, Migration{Version: "043", Name: "canonical_node_ids", UpSQL: migration043}); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE node_id = 'VM-0-4-ubuntu'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("legacy node rows = %d, want 0", count)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE node_id = 'node_VM-0-4-ubuntu'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("stable node rows = %d, want 1", count)
	}
	var endpointNodeID string
	if err := db.QueryRow(`SELECT node_id FROM endpoints WHERE id = 'ep_legacy'`).Scan(&endpointNodeID); err != nil {
		t.Fatal(err)
	}
	if endpointNodeID != "node_VM-0-4-ubuntu" {
		t.Fatalf("endpoint node_id = %q, want stable node id", endpointNodeID)
	}
}
