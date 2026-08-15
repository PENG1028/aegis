package store

import (
	"database/sql"
	"testing"
)

// TestMigration050DropsDeadTables is a regression test: 13 tables with no
// read/write code (nodeagent heartbeats, desired/actual state, join tokens,
// gateway inventory, deployment records, service-auth groups, upgrade
// sessions) stayed in the schema forever, letting the catalog drift from
// reality. Migration 050 must drop them all.
func TestMigration050DropsDeadTables(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	dead := []string{
		"gateway_domains", "gateway_routes", "gateway_listeners",
		"deployments", "deployment_instances",
		"node_join_tokens", "node_credentials",
		"node_desired_states", "node_actual_states",
		"svc_auth_groups", "svc_auth_group_members", "svc_auth_policies",
		"upgrade_sessions",
	}
	for _, table := range dead {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != sql.ErrNoRows {
			t.Errorf("dead table %q still exists after migration 050 (err=%v)", table, err)
		}
	}

	// Live tables must survive.
	live := []string{"operation_logs", "health_checks", "nodes", "routes"}
	for _, table := range live {
		var name string
		if err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name); err != nil {
			t.Errorf("live table %q missing after migration 050: %v", table, err)
		}
	}
}
