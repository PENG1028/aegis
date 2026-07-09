package nodestate

import (
	"database/sql"
	"encoding/json"
	"testing"

	_ "modernc.org/sqlite"

)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func createTables(t *testing.T, db *sql.DB) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS node_desired_states (
			id TEXT PRIMARY KEY, node_id TEXT NOT NULL, revision INTEGER NOT NULL DEFAULT 0,
			state_hash TEXT NOT NULL DEFAULT '', state_json TEXT NOT NULL DEFAULT '{}',
			status TEXT NOT NULL DEFAULT 'active', reason TEXT NOT NULL DEFAULT '',
			created_by TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, superseded_at TEXT DEFAULT '',
			UNIQUE(node_id, revision)
		);
		CREATE TABLE IF NOT EXISTS node_actual_states (
			id TEXT PRIMARY KEY, node_id TEXT NOT NULL UNIQUE, applied_revision INTEGER NOT NULL DEFAULT 0,
			state_hash TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'unknown',
			last_apply_at TEXT DEFAULT '', last_success_at TEXT DEFAULT '', last_error TEXT DEFAULT '',
			provider_status TEXT DEFAULT '{}', relay_status TEXT DEFAULT '{}', gateway_status TEXT DEFAULT '{}',
			diagnostics_status TEXT DEFAULT '{}', reported_at TEXT DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("create tables: %v", err)
	}
}

func validStateJSON() string {
	return `{"version":1,"node_id":"nd_test","gateways":[],"diagnostics":{"enabled":true}}`
}

func TestCreateDesiredStateRevision1(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	ds, err := svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_test", StateJSON: validStateJSON(), Reason: "initial", CreatedBy: "admin",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if ds.Revision != 1 {
		t.Errorf("expected revision 1, got %d", ds.Revision)
	}
	if ds.StateHash == "" {
		t.Error("expected non-empty state_hash")
	}
}

func TestCreateDesiredStateRevision2(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_test", StateJSON: validStateJSON(), Reason: "v1", CreatedBy: "admin",
	})
	ds2, err := svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_test", StateJSON: validStateJSON(), Reason: "v2", CreatedBy: "admin",
	})
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}
	if ds2.Revision != 2 {
		t.Errorf("expected revision 2, got %d", ds2.Revision)
	}
}

func TestStateHashStable(t *testing.T) {
	h1, err := ComputeStateHash(validStateJSON())
	if err != nil {
		t.Fatal(err)
	}
	h2, err := ComputeStateHash(validStateJSON())
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Error("expected stable hash for same content")
	}
}

func TestMalformedStateRejected(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	_, err := svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_test", StateJSON: "{invalid json}", Reason: "bad", CreatedBy: "admin",
	})
	if err == nil {
		t.Error("expected error for malformed state_json")
	}
}

func TestGetLatestDesiredState(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_test", StateJSON: validStateJSON(), Reason: "v1", CreatedBy: "admin",
	})
	svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_test", StateJSON: `{"version":2}`, Reason: "v2", CreatedBy: "admin",
	})

	latest, err := svc.GetLatestDesiredState("nd_test")
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if latest == nil {
		t.Fatal("expected desired state")
	}
	if latest.Revision != 2 {
		t.Errorf("expected revision 2, got %d", latest.Revision)
	}
}

func TestGetDesiredStateByRevision(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	ds1, _ := svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_test", StateJSON: validStateJSON(), Reason: "v1", CreatedBy: "admin",
	})

	found, err := svc.GetDesiredStateByRevision("nd_test", ds1.Revision)
	if err != nil {
		t.Fatalf("get by revision: %v", err)
	}
	if found == nil || found.ID != ds1.ID {
		t.Error("expected to find desired state by revision")
	}
}

func TestActualStateReport(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	as, err := svc.ReportActualState("nd_test", 1, "hash123", ASStatusApplied, "", "{}", "{}", "{}", "{}")
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if as.AppliedRevision != 1 {
		t.Errorf("expected revision 1, got %d", as.AppliedRevision)
	}

	// Get actual state
	got, err := svc.GetActualState("nd_test")
	if err != nil {
		t.Fatalf("get actual: %v", err)
	}
	if got == nil {
		t.Fatal("expected actual state")
	}
	if got.Status != ASStatusApplied {
		t.Errorf("expected applied, got %s", got.Status)
	}
}

func TestSyncStatusInSync(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	ds, _ := svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_test", StateJSON: validStateJSON(), Reason: "v1", CreatedBy: "admin",
	})
	svc.ReportActualState("nd_test", ds.Revision, ds.StateHash, ASStatusApplied, "", "{}", "{}", "{}", "{}")

	ss, err := svc.GetSyncStatus("nd_test")
	if err != nil {
		t.Fatalf("sync status: %v", err)
	}
	if ss.Status != SyncInSync {
		t.Errorf("expected in_sync, got %s", ss.Status)
	}
}

func TestSyncStatusOutdated(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_test", StateJSON: validStateJSON(), Reason: "v1", CreatedBy: "admin",
	})
	svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_test", StateJSON: `{"version":2}`, Reason: "v2", CreatedBy: "admin",
	})
	svc.ReportActualState("nd_test", 1, "old_hash", ASStatusApplied, "", "{}", "{}", "{}", "{}")

	ss, err := svc.GetSyncStatus("nd_test")
	if err != nil {
		t.Fatalf("sync status: %v", err)
	}
	if ss.Status != SyncOutdated {
		t.Errorf("expected outdated, got %s", ss.Status)
	}
}

func TestSyncStatusNoDesiredState(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	ss, err := svc.GetSyncStatus("nd_missing")
	if err != nil {
		t.Fatalf("sync status: %v", err)
	}
	if ss.Status != SyncNoDesiredState {
		t.Errorf("expected no_desired_state, got %s", ss.Status)
	}
}

func TestSyncStatusNoActualState(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_noact", StateJSON: validStateJSON(), Reason: "v1", CreatedBy: "admin",
	})

	ss, err := svc.GetSyncStatus("nd_noact")
	if err != nil {
		t.Fatalf("sync status: %v", err)
	}
	if ss.Status != SyncNoActualState {
		t.Errorf("expected no_actual_state, got %s", ss.Status)
	}
}

func TestSyncStatusFailed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_fail", StateJSON: validStateJSON(), Reason: "v1", CreatedBy: "admin",
	})
	svc.ReportActualState("nd_fail", 1, "hash", ASStatusFailed, "error: apply failed", "{}", "{}", "{}", "{}")

	ss, err := svc.GetSyncStatus("nd_fail")
	if err != nil {
		t.Fatalf("sync status: %v", err)
	}
	if ss.Status != SyncFailed {
		t.Errorf("expected failed, got %s", ss.Status)
	}
	if ss.LastError != "error: apply failed" {
		t.Errorf("expected error message, got %s", ss.LastError)
	}
}

func TestSyncStatusDegraded(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_degr", StateJSON: validStateJSON(), Reason: "v1", CreatedBy: "admin",
	})
	svc.ReportActualState("nd_degr", 1, "hash", ASStatusDegraded, "degraded: provider error", "{}", "{}", "{}", "{}")

	ss, err := svc.GetSyncStatus("nd_degr")
	if err != nil {
		t.Fatalf("sync status: %v", err)
	}
	if ss.Status != SyncDegraded {
		t.Errorf("expected degraded, got %s", ss.Status)
	}
}

func TestCompareNodeRevision(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	latest, avail, outdated, err := svc.CompareNodeRevision("nd_test", 0)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if latest != 0 {
		t.Errorf("expected latest 0, got %d", latest)
	}
	if avail {
		t.Error("expected no desired state available")
	}
	if outdated {
		t.Error("expected not outdated")
	}

	// After creating desired state
	svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_test", StateJSON: validStateJSON(), Reason: "v1", CreatedBy: "admin",
	})
	latest2, avail2, outdated2, _ := svc.CompareNodeRevision("nd_test", 1)
	if latest2 != 1 {
		t.Errorf("expected latest 1, got %d", latest2)
	}
	if !avail2 {
		t.Error("expected desired state available")
	}
	if outdated2 {
		t.Error("expected not outdated when applied_revision == latest")
	}

	_, _, outdated3, _ := svc.CompareNodeRevision("nd_test", 0)
	if !outdated3 {
		t.Error("expected outdated when applied_revision < latest")
	}
}

func TestNoRawTokensInStateJSON(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTables(t, db)
	repo := NewRepository(db)
	svc := NewService(repo)

	state := `{"version":1,"node_id":"nd_test","secrets":[],"gateway_links":[]}`
	ds, err := svc.CreateDesiredState(CreateDesiredStateInput{
		NodeID: "nd_test", StateJSON: state, Reason: "no-leak", CreatedBy: "admin",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal([]byte(ds.StateJSON), &parsed)

	// Check secrets are empty or absent
	if secrets, ok := parsed["secrets"]; ok {
		if arr, ok := secrets.([]interface{}); ok && len(arr) > 0 {
			t.Error("state_json should not contain secrets in v1.8C-2")
		}
	}

	// Check no raw token fields
	for _, key := range []string{"raw_token", "node_token", "join_token", "gateway_token"} {
		if _, ok := parsed[key]; ok {
			t.Errorf("state_json should not contain field '%s'", key)
		}
	}
}

func TestNormalizeJSON(t *testing.T) {
	a := `{"b":1,"a":2}`
	b := `{"a":2,"b":1}`
	normA, _ := NormalizeJSON(a)
	normB, _ := NormalizeJSON(b)
	if normA != normB {
		t.Error("normalized JSON should be identical regardless of key order")
	}
}

func TestMustComputeHash(t *testing.T) {
	h := MustComputeHash(validStateJSON())
	if h == "" {
		t.Error("expected non-empty hash")
	}
}
