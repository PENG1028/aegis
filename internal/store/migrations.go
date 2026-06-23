package store

// GetMigrations returns the list of SQL migrations to run in order.
func GetMigrations() []string {
	return []string{
		createProjectsTable,
		createServicesTable,
		createEndpointsTable,
		createRoutesTable,
		createManagedDomainsTable,
		createHealthChecksTable,
		createApplyVersionsTable,
		createOperationLogsTable,
		createAPITokensTable,
		createIndexes,
	}
}

const createProjectsTable = `
CREATE TABLE IF NOT EXISTS projects (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	description TEXT,
	status TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`

const createServicesTable = `
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
`

const createEndpointsTable = `
CREATE TABLE IF NOT EXISTS endpoints (
	id TEXT PRIMARY KEY,
	service_id TEXT NOT NULL,
	type TEXT NOT NULL,
	address TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`

const createRoutesTable = `
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
`

const createManagedDomainsTable = `
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
`

const createHealthChecksTable = `
CREATE TABLE IF NOT EXISTS health_checks (
	id TEXT PRIMARY KEY,
	service_id TEXT NOT NULL,
	endpoint_id TEXT,
	status TEXT NOT NULL,
	latency_ms INTEGER,
	message TEXT,
	checked_at TEXT NOT NULL
);
`

const createApplyVersionsTable = `
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
`

const createOperationLogsTable = `
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
`

const createAPITokensTable = `
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

const createIndexes = `
CREATE INDEX IF NOT EXISTS idx_services_project_id ON services(project_id);
CREATE INDEX IF NOT EXISTS idx_endpoints_service_id ON endpoints(service_id);
CREATE INDEX IF NOT EXISTS idx_routes_service_id ON routes(service_id);
CREATE INDEX IF NOT EXISTS idx_managed_domains_service_id ON managed_domains(service_id);
CREATE INDEX IF NOT EXISTS idx_managed_domains_owner_ref ON managed_domains(owner_ref);
CREATE INDEX IF NOT EXISTS idx_managed_domains_status ON managed_domains(status);
CREATE INDEX IF NOT EXISTS idx_health_checks_service_id ON health_checks(service_id);
CREATE INDEX IF NOT EXISTS idx_health_checks_checked_at ON health_checks(checked_at);
CREATE INDEX IF NOT EXISTS idx_apply_versions_created_at ON apply_versions(created_at);
CREATE INDEX IF NOT EXISTS idx_operation_logs_created_at ON operation_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_operation_logs_action ON operation_logs(action);
`
