package main

import (
	"database/sql"
	"fmt"
	"sort"
	"time"
)

type migration struct {
	version string
	name    string
	upSQL   string
}

func allMigrations() []migration {
	return []migration{
		{"001", "initial_service_auth_schema", migration001},
	}
}

func runMigrations(db *sql.DB) error {
	// Ensure tracking table exists.
	db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		name TEXT,
		applied_at TEXT
	)`)

	// Load applied migrations.
	applied := make(map[string]bool)
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err == nil {
		for rows.Next() {
			var v string
			rows.Scan(&v)
			applied[v] = true
		}
		rows.Close()
	}

	migrations := allMigrations()
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("migration %s begin: %w", m.version, err)
		}

		if _, err := tx.Exec(m.upSQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s (%s): %w", m.version, m.name, err)
		}

		if _, err := tx.Exec(
			`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			m.version, m.name, time.Now().Format(time.RFC3339),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s record: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %s commit: %w", m.version, err)
		}

		fmt.Printf("migration %s (%s) applied\n", m.version, m.name)
	}

	return nil
}

const migration001 = `
CREATE TABLE IF NOT EXISTS svc_auth_services (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    host        TEXT NOT NULL,
    port        INTEGER NOT NULL DEFAULT 0,
    node_host   TEXT NOT NULL DEFAULT '',
    apis_json   TEXT NOT NULL DEFAULT '[]',
    status      TEXT NOT NULL DEFAULT 'active',
    last_seen   TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT '',
    updated_at  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_svc_auth_svc_name ON svc_auth_services(name);
CREATE INDEX IF NOT EXISTS idx_svc_auth_svc_status ON svc_auth_services(status);

CREATE TABLE IF NOT EXISTS svc_auth_call_logs (
    id              TEXT PRIMARY KEY,
    caller_service  TEXT NOT NULL DEFAULT '',
    target_service  TEXT NOT NULL DEFAULT '',
    target_api      TEXT NOT NULL DEFAULT '',
    caller_host     TEXT NOT NULL DEFAULT '',
    target_host     TEXT NOT NULL DEFAULT '',
    allowed         INTEGER NOT NULL DEFAULT 1,
    latency_ms      INTEGER NOT NULL DEFAULT 0,
    error_msg       TEXT NOT NULL DEFAULT '',
    called_at       TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_svc_auth_logs_time ON svc_auth_call_logs(called_at);
CREATE INDEX IF NOT EXISTS idx_svc_auth_logs_caller ON svc_auth_call_logs(caller_service);
CREATE INDEX IF NOT EXISTS idx_svc_auth_logs_target ON svc_auth_call_logs(target_service);

CREATE TABLE IF NOT EXISTS svc_auth_blocklist (
    id          TEXT PRIMARY KEY,
    service_id  TEXT,
    api_name    TEXT NOT NULL DEFAULT '*',
    reason      TEXT NOT NULL DEFAULT '',
    version     INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_svc_auth_bl_version ON svc_auth_blocklist(version);
`
