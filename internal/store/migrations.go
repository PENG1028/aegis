package store

import (
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// Migration represents a versioned database migration.
type Migration struct {
	Version string
	Name    string
	UpSQL   string
}

// AllMigrations returns all migrations in version order.
func AllMigrations() []Migration {
	return []Migration{
		{
			Version: "001",
			Name:    "initial_schema",
			UpSQL:   migration001,
		},
		{
			Version: "002",
			Name:    "add_indexes",
			UpSQL:   migration002,
		},
		{
			Version: "003",
			Name:    "add_exposures",
			UpSQL:   migration003,
		},
		{
			Version: "004",
			Name:    "path_routes",
			UpSQL:   migration004,
		},
		{
			Version: "005",
			Name:    "tcp_exposure_fields",
			UpSQL:   migration005,
		},
	}
}

// RunMigrations applies all pending migrations in order within transactions.
// Each migration runs in its own transaction. On failure, that transaction
// is rolled back and the error is returned.
func RunMigrations(db *sql.DB) error {
	// Create schema_migrations table first (not part of versioned migrations)
	if _, err := db.Exec(createSchemaMigrationsTable); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	// Get already-applied migrations
	applied, err := getAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("read applied migrations: %w", err)
	}

	migrations := AllMigrations()
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}

		// Run in transaction
		if err := runMigration(db, m); err != nil {
			return fmt.Errorf("migration %s (%s) failed: %w", m.Version, m.Name, err)
		}

		fmt.Printf("migration %s (%s) applied\n", m.Version, m.Name)
	}

	return nil
}

// runMigration executes a single migration in a transaction.
func runMigration(db *sql.DB, m Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if _, err := tx.Exec(m.UpSQL); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed (%v) after error: %w", rbErr, err)
		}
		return err
	}

	// Record migration
	now := time.Now().Format(time.RFC3339)
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		m.Version, m.Name, now,
	); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed (%v) after record error: %w", rbErr, err)
		}
		return fmt.Errorf("record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// getAppliedMigrations returns the set of already-applied migration versions.
func getAppliedMigrations(db *sql.DB) (map[string]bool, error) {
	// Check if schema_migrations exists
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&count)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return map[string]bool{}, nil
	}

	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// GetCurrentVersion returns the latest applied migration version.
func GetCurrentVersion(db *sql.DB) (string, error) {
	var version string
	err := db.QueryRow(`SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			return "none", nil
		}
		return "", err
	}
	return version, nil
}

// RunMigrationsLegacy runs all SQL statements without version tracking.
// Used for backward compatibility with the old Init path.
func RunMigrationsLegacy(db *sql.DB) error {
	return RunMigrations(db)
}

const createSchemaMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at TEXT NOT NULL
);
`

// migration001 creates the initial set of tables.
const migration001 = `
CREATE TABLE IF NOT EXISTS projects (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	description TEXT,
	status TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS services (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	name TEXT NOT NULL UNIQUE,
	kind TEXT NOT NULL DEFAULT 'http',
	env TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'active',
	note TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS endpoints (
	id TEXT PRIMARY KEY,
	service_id TEXT NOT NULL,
	type TEXT NOT NULL,
	address TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS routes (
	id TEXT PRIMARY KEY,
	domain TEXT NOT NULL UNIQUE,
	service_id TEXT NOT NULL,
	tls_enabled INTEGER NOT NULL DEFAULT 1,
	status TEXT NOT NULL DEFAULT 'active',
	maintenance_enabled INTEGER NOT NULL DEFAULT 0,
	maintenance_message TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS managed_domains (
	id TEXT PRIMARY KEY,
	domain TEXT NOT NULL UNIQUE,
	service_id TEXT NOT NULL,
	owner_ref TEXT NOT NULL,
	target_type TEXT,
	target_ref TEXT,
	verification_type TEXT NOT NULL,
	verification_name TEXT NOT NULL,
	verification_value TEXT NOT NULL,
	status TEXT NOT NULL,
	tls_status TEXT NOT NULL DEFAULT 'pending',
	last_check_message TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS health_checks (
	id TEXT PRIMARY KEY,
	service_id TEXT NOT NULL,
	endpoint_id TEXT,
	status TEXT NOT NULL,
	latency_ms INTEGER,
	message TEXT,
	checked_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS apply_versions (
	id TEXT PRIMARY KEY,
	version TEXT NOT NULL,
	config_path TEXT NOT NULL,
	backup_path TEXT,
	rendered_config TEXT,
	status TEXT NOT NULL,
	message TEXT,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS operation_logs (
	id TEXT PRIMARY KEY,
	action TEXT NOT NULL,
	target_type TEXT,
	target_id TEXT,
	result TEXT NOT NULL,
	message TEXT,
	actor TEXT,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS api_tokens (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	token_hash TEXT NOT NULL,
	scopes TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`

// migration002 adds performance indexes.
const migration002 = `
CREATE INDEX IF NOT EXISTS idx_services_project_id ON services(project_id);
CREATE INDEX IF NOT EXISTS idx_endpoints_service_id ON endpoints(service_id);
CREATE INDEX IF NOT EXISTS idx_endpoints_type ON endpoints(type);
CREATE INDEX IF NOT EXISTS idx_routes_service_id ON routes(service_id);
CREATE INDEX IF NOT EXISTS idx_routes_status ON routes(status);
CREATE INDEX IF NOT EXISTS idx_managed_domains_service_id ON managed_domains(service_id);
CREATE INDEX IF NOT EXISTS idx_managed_domains_owner_ref ON managed_domains(owner_ref);
CREATE INDEX IF NOT EXISTS idx_managed_domains_status ON managed_domains(status);
CREATE INDEX IF NOT EXISTS idx_health_checks_service_id ON health_checks(service_id);
CREATE INDEX IF NOT EXISTS idx_health_checks_endpoint_id ON health_checks(endpoint_id);
CREATE INDEX IF NOT EXISTS idx_health_checks_checked_at ON health_checks(checked_at);
CREATE INDEX IF NOT EXISTS idx_apply_versions_created_at ON apply_versions(created_at);
CREATE INDEX IF NOT EXISTS idx_apply_versions_status ON apply_versions(status);
CREATE INDEX IF NOT EXISTS idx_operation_logs_created_at ON operation_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_operation_logs_action ON operation_logs(action);
CREATE INDEX IF NOT EXISTS idx_api_tokens_token_hash ON api_tokens(token_hash);
`

// migration003 adds the exposures table.
const migration003 = `
CREATE TABLE IF NOT EXISTS exposures (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	mode TEXT NOT NULL DEFAULT 'private',
	host TEXT NOT NULL,
	port INTEGER DEFAULT 0,
	path TEXT,
	service_id TEXT NOT NULL,
	node_id TEXT,
	owner_ref TEXT NOT NULL,
	target_ref TEXT,
	status TEXT NOT NULL DEFAULT 'pending',
	message TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_exposures_owner_ref ON exposures(owner_ref);
CREATE INDEX IF NOT EXISTS idx_exposures_service_id ON exposures(service_id);
CREATE INDEX IF NOT EXISTS idx_exposures_type ON exposures(type);
CREATE INDEX IF NOT EXISTS idx_exposures_status ON exposures(status);
CREATE INDEX IF NOT EXISTS idx_exposures_type_status ON exposures(type, status);
`

// migration004 adds path_prefix and strip_prefix to routes.
// Recreates routes table to support multiple paths per domain (old UNIQUE on domain only is too restrictive).
const migration004 = `
CREATE TABLE IF NOT EXISTS routes_new (
	id TEXT PRIMARY KEY,
	domain TEXT NOT NULL,
	path_prefix TEXT NOT NULL DEFAULT '',
	strip_prefix INTEGER NOT NULL DEFAULT 0,
	service_id TEXT NOT NULL,
	tls_enabled INTEGER NOT NULL DEFAULT 1,
	status TEXT NOT NULL DEFAULT 'active',
	maintenance_enabled INTEGER NOT NULL DEFAULT 0,
	maintenance_message TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(domain, path_prefix)
);

INSERT OR IGNORE INTO routes_new (id, domain, path_prefix, strip_prefix, service_id, tls_enabled, status, maintenance_enabled, maintenance_message, created_at, updated_at)
	SELECT id, domain, '', 0, service_id, tls_enabled, status, maintenance_enabled, maintenance_message, created_at, updated_at
	FROM routes;

DROP TABLE routes;
ALTER TABLE routes_new RENAME TO routes;

CREATE INDEX IF NOT EXISTS idx_routes_domain ON routes(domain);
CREATE INDEX IF NOT EXISTS idx_routes_domain_path ON routes(domain, path_prefix);
CREATE INDEX IF NOT EXISTS idx_routes_service_id ON routes(service_id);
CREATE INDEX IF NOT EXISTS idx_routes_status ON routes(status);
`

// migration005 adds target_host, target_port, allow_public_tcp, project_id to exposures.
const migration005 = `
ALTER TABLE exposures ADD COLUMN target_host TEXT DEFAULT '';
ALTER TABLE exposures ADD COLUMN target_port INTEGER DEFAULT 0;
ALTER TABLE exposures ADD COLUMN allow_public_tcp INTEGER NOT NULL DEFAULT 0;
ALTER TABLE exposures ADD COLUMN project_id TEXT DEFAULT '';
`
