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
		{
			Version: "006",
			Name:    "add_listeners",
			UpSQL:   migration006,
		},
		{
			Version: "007",
			Name:    "exposure_provider_fields",
			UpSQL:   migration007,
		},
		{
			Version: "008",
			Name:    "edge_mux_rules",
			UpSQL:   migration008,
		},
		{
			Version: "009",
			Name:    "edge_rule_managed_by",
			UpSQL:   migration009,
		},
		{
			Version: "010",
			Name:    "add_nodes",
			UpSQL:   migration010,
		},
		{
			Version: "011",
			Name:    "node_leader_fields",
			UpSQL:   migration011,
		},
		{
			Version: "012",
			Name:    "cluster_state_version",
			UpSQL:   migration012,
		},
		{
			Version: "013",
			Name:    "upgrade_sessions",
			UpSQL:   migration013,
		},
		{
			Version: "014",
			Name:    "add_spaces",
			UpSQL:   migration014,
		},
		{
			Version: "015",
			Name:    "alter_api_tokens_add_space",
			UpSQL:   migration015,
		},
		{
			Version: "016",
			Name:    "add_resource_ownership",
			UpSQL:   migration016,
		},
		{
			Version: "017",
			Name:    "add_admin_auth",
			UpSQL:   migration017,
		},
		{
			Version: "018",
			Name:    "add_apply_logs",
			UpSQL:   migration018,
		},
		{
			Version: "019",
			Name:    "add_audit_logs",
			UpSQL:   migration019,
		},
		{
			Version: "020",
			Name:    "add_node_events",
			UpSQL:   migration020,
		},
		{
			Version: "021",
			Name:    "add_node_capabilities",
			UpSQL:   migration021,
		},
		{
			Version: "022",
			Name:    "add_gateway_abstraction",
			UpSQL:   migration022,
		},
		{
			Version: "023",
			Name:    "add_deployments",
			UpSQL:   migration023,
		},
	{
			Version: "024",
			Name:    "add_trusted_gateways",
			UpSQL:   migration024,
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

// migration006 adds the listeners table.
const migration006 = `
CREATE TABLE IF NOT EXISTS listeners (
	id TEXT PRIMARY KEY,
	provider TEXT NOT NULL,
	protocol TEXT NOT NULL,
	bind_ip TEXT NOT NULL,
	port INTEGER NOT NULL,
	status TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(bind_ip, port, protocol)
);

CREATE INDEX IF NOT EXISTS idx_listeners_provider ON listeners(provider);
CREATE INDEX IF NOT EXISTS idx_listeners_port ON listeners(port);
`

// migration007 adds provider and listener_id to exposures.
const migration007 = `
ALTER TABLE exposures ADD COLUMN provider TEXT DEFAULT '';
ALTER TABLE exposures ADD COLUMN listener_id TEXT DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_exposures_provider ON exposures(provider);
CREATE INDEX IF NOT EXISTS idx_exposures_listener_id ON exposures(listener_id);
`

// migration008 adds edge_mux_rules and updates listener schema.
const migration008 = `
CREATE TABLE IF NOT EXISTS edge_mux_rules (
	id TEXT PRIMARY KEY,
	sni_host TEXT NOT NULL UNIQUE,
	declared_kind TEXT NOT NULL DEFAULT 'unknown_tls_backend',
	target_host TEXT NOT NULL,
	target_port INTEGER NOT NULL,
	service_id TEXT,
	status TEXT NOT NULL DEFAULT 'active',
	message TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_edge_mux_rules_sni ON edge_mux_rules(sni_host);
CREATE INDEX IF NOT EXISTS idx_edge_mux_rules_status ON edge_mux_rules(status);

ALTER TABLE listeners ADD COLUMN node_id TEXT DEFAULT '';
ALTER TABLE listeners ADD COLUMN purpose TEXT DEFAULT '';
`

// migration009 adds managed_by and source_ref to edge_mux_rules.
const migration009 = `
ALTER TABLE edge_mux_rules ADD COLUMN managed_by TEXT NOT NULL DEFAULT 'manual';
ALTER TABLE edge_mux_rules ADD COLUMN source_ref TEXT DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_edge_mux_rules_managed_by ON edge_mux_rules(managed_by);
CREATE INDEX IF NOT EXISTS idx_edge_mux_rules_source_ref ON edge_mux_rules(source_ref);
`

// migration010 adds the nodes table.
const migration010 = `
CREATE TABLE IF NOT EXISTS nodes (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL,
	hostname TEXT NOT NULL,
	local_ip TEXT NOT NULL DEFAULT '127.0.0.1',
	private_ip TEXT DEFAULT '',
	public_ip TEXT DEFAULT '',
	is_current INTEGER NOT NULL DEFAULT 0,
	ip_migrated INTEGER NOT NULL DEFAULT 0,
	last_seen TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_nodes_is_current ON nodes(is_current);
CREATE INDEX IF NOT EXISTS idx_nodes_node_id ON nodes(node_id);
`

// migration011 adds leader election fields to nodes.
const migration011 = `
ALTER TABLE nodes ADD COLUMN is_leader INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN leader_elected_at TEXT DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_nodes_is_leader ON nodes(is_leader);
`

// migration012 adds cluster_state table and state_version tracking.
const migration012 = `
CREATE TABLE IF NOT EXISTS cluster_state (
	key TEXT PRIMARY KEY,
	value INTEGER NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL DEFAULT ''
);

ALTER TABLE nodes ADD COLUMN state_version INTEGER NOT NULL DEFAULT 0;
`

// migration013 adds the upgrade_sessions table.
const migration013 = `
CREATE TABLE IF NOT EXISTS upgrade_sessions (
	id TEXT PRIMARY KEY,
	from_version TEXT NOT NULL,
	to_version TEXT NOT NULL,
	state_version_start INTEGER NOT NULL DEFAULT 0,
	state_version_end INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'running',
	error_message TEXT,
	steps TEXT DEFAULT '[]',
	start_time TEXT NOT NULL,
	end_time TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_upgrade_sessions_status ON upgrade_sessions(status);
CREATE INDEX IF NOT EXISTS idx_upgrade_sessions_start_time ON upgrade_sessions(start_time);
`

// migration014 adds the spaces table for logical isolation.
const migration014 = `
CREATE TABLE IF NOT EXISTS spaces (
	id TEXT PRIMARY KEY,
	space_id TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	max_routes INTEGER NOT NULL DEFAULT 50,
	max_edge_rules INTEGER NOT NULL DEFAULT 50,
	max_services INTEGER NOT NULL DEFAULT 20,
	max_apply_per_minute INTEGER NOT NULL DEFAULT 10,
	status TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`

// migration015 adds space_id and token_type to api_tokens.
const migration015 = `
ALTER TABLE api_tokens ADD COLUMN space_id TEXT NOT NULL DEFAULT '';
ALTER TABLE api_tokens ADD COLUMN token_type TEXT NOT NULL DEFAULT 'admin';
CREATE INDEX IF NOT EXISTS idx_api_tokens_space_id ON api_tokens(space_id);
`

// migration016 adds ownership fields to services, routes, and edge_mux_rules.
const migration016 = `
ALTER TABLE services ADD COLUMN space_id TEXT NOT NULL DEFAULT '';
ALTER TABLE services ADD COLUMN owner_type TEXT NOT NULL DEFAULT '';
ALTER TABLE services ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE services ADD COLUMN created_by_token_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_services_space_id ON services(space_id);

ALTER TABLE routes ADD COLUMN space_id TEXT NOT NULL DEFAULT '';
ALTER TABLE routes ADD COLUMN owner_type TEXT NOT NULL DEFAULT '';
ALTER TABLE routes ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE routes ADD COLUMN created_by_token_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_routes_space_id ON routes(space_id);

ALTER TABLE edge_mux_rules ADD COLUMN space_id TEXT NOT NULL DEFAULT '';
ALTER TABLE edge_mux_rules ADD COLUMN owner_type TEXT NOT NULL DEFAULT '';
ALTER TABLE edge_mux_rules ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE edge_mux_rules ADD COLUMN created_by_token_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_edge_mux_rules_space_id ON edge_mux_rules(space_id);
`

// migration017 adds admin authentication tables.
const migration017 = `
CREATE TABLE IF NOT EXISTS admin_users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_sessions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	session_hash TEXT NOT NULL UNIQUE,
	expires_at TEXT NOT NULL,
	revoked_at TEXT DEFAULT '',
	created_at TEXT NOT NULL,
	last_seen_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_hash ON admin_sessions(session_hash);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_user_id ON admin_sessions(user_id);
`

// migration018 adds the apply_logs table.
const migration018 = `
CREATE TABLE IF NOT EXISTS apply_logs (
	id TEXT PRIMARY KEY,
	operation_id TEXT NOT NULL,
	state_version INTEGER NOT NULL DEFAULT 0,
	config_hash_before TEXT DEFAULT '',
	config_hash_after TEXT DEFAULT '',
	provider TEXT NOT NULL DEFAULT '',
	validate_status TEXT NOT NULL DEFAULT 'pending',
	reload_status TEXT NOT NULL DEFAULT 'pending',
	runtime_verify_status TEXT NOT NULL DEFAULT 'pending',
	stderr TEXT DEFAULT '',
	step_log TEXT DEFAULT '[]',
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_apply_logs_operation_id ON apply_logs(operation_id);
CREATE INDEX IF NOT EXISTS idx_apply_logs_created_at ON apply_logs(created_at);
`

// migration019 adds the audit_logs table.
const migration019 = `
CREATE TABLE IF NOT EXISTS audit_logs (
	id TEXT PRIMARY KEY,
	actor_type TEXT NOT NULL DEFAULT '',
	actor_id TEXT NOT NULL DEFAULT '',
	event_type TEXT NOT NULL DEFAULT '',
	ip TEXT NOT NULL DEFAULT '',
	user_agent TEXT NOT NULL DEFAULT '',
	target_type TEXT NOT NULL DEFAULT '',
	target_id TEXT NOT NULL DEFAULT '',
	result TEXT NOT NULL DEFAULT '',
	error_code TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_event_type ON audit_logs(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor_type, actor_id);
`

// migration020 adds the node_events table.
const migration020 = `
CREATE TABLE IF NOT EXISTS node_events (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL DEFAULT '',
	event_type TEXT NOT NULL DEFAULT '',
	state_version INTEGER NOT NULL DEFAULT 0,
	severity TEXT NOT NULL DEFAULT 'info',
	message TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_node_events_node_id ON node_events(node_id);
CREATE INDEX IF NOT EXISTS idx_node_events_event_type ON node_events(event_type);
CREATE INDEX IF NOT EXISTS idx_node_events_created_at ON node_events(created_at);
`

// migration021 adds capabilities column to nodes.
const migration021 = `
ALTER TABLE nodes ADD COLUMN capabilities TEXT NOT NULL DEFAULT '{}';
`

// migration022 adds gateway abstraction tables.
const migration022 = `
CREATE TABLE IF NOT EXISTS gateway_domains (
	id TEXT PRIMARY KEY,
	domain TEXT NOT NULL UNIQUE,
	node_id TEXT NOT NULL DEFAULT '',
	tls_enabled INTEGER NOT NULL DEFAULT 0,
	tls_provider TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_gateway_domains_node_id ON gateway_domains(node_id);

CREATE TABLE IF NOT EXISTS gateway_routes (
	id TEXT PRIMARY KEY,
	domain_id TEXT NOT NULL,
	path TEXT NOT NULL DEFAULT '',
	target_service TEXT NOT NULL DEFAULT '',
	target_port INTEGER NOT NULL DEFAULT 0,
	protocol TEXT NOT NULL DEFAULT 'http',
	status TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_gateway_routes_domain_id ON gateway_routes(domain_id);

CREATE TABLE IF NOT EXISTS gateway_listeners (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL DEFAULT '',
	port INTEGER NOT NULL DEFAULT 0,
	tls_enabled INTEGER NOT NULL DEFAULT 0,
	protocol TEXT NOT NULL DEFAULT 'http',
	status TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_gateway_listeners_node_id ON gateway_listeners(node_id);
`

// migration023 adds deployment version tracking tables.
const migration024 = `
CREATE TABLE IF NOT EXISTS trusted_gateways (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT "",
	host TEXT NOT NULL DEFAULT "",
	private_ip TEXT NOT NULL DEFAULT "",
	port INTEGER NOT NULL DEFAULT 443,
	auth_type TEXT NOT NULL DEFAULT "shared_secret",
	auth_value TEXT NOT NULL DEFAULT "",
	gateway_type TEXT NOT NULL DEFAULT "upstream",
	auto_route INTEGER NOT NULL DEFAULT 1,
	status TEXT NOT NULL DEFAULT "active",
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`

const migration023 = `
CREATE TABLE IF NOT EXISTS deployments (
	id TEXT PRIMARY KEY,
	version TEXT NOT NULL DEFAULT '',
	service_id TEXT NOT NULL DEFAULT '',
	target_nodes TEXT NOT NULL DEFAULT '[]',
	rollout_strategy TEXT NOT NULL DEFAULT 'all',
	status TEXT NOT NULL DEFAULT 'pending',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_deployments_service_id ON deployments(service_id);
CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments(status);

CREATE TABLE IF NOT EXISTS deployment_instances (
	id TEXT PRIMARY KEY,
	deployment_id TEXT NOT NULL,
	node_id TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'pending',
	last_applied_version TEXT NOT NULL DEFAULT '',
	applied_at TEXT DEFAULT '',
	error_message TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_deployment_instances_deployment_id ON deployment_instances(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_instances_node_id ON deployment_instances(node_id);
`
