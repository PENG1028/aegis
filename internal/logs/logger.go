package logs

import "context"

// Logger is the application logging interface.
//
// All AppService methods satisfy this interface. Implementations can write
// to database, stdout, files, external services, or any combination of sinks.
//
// Replace the concrete *AppService with this interface in all consumer
// structs to enable swapping the logging backend without touching call sites.
//
// Read methods (List*) are included because several consumers need to query
// log history — a future implementation could read from a different store.
type Logger interface {
	// ── Write methods (best-effort, must not fail the caller) ──

	// Log records an operational event.
	Log(ctx context.Context, action, targetType, targetID, result, message, actor string)

	// LogAudit records a security-relevant audit event.
	LogAudit(actorType, actorID, eventType, ip, userAgent, targetType, targetID, result, errorCode string)

	// LogApply records a configuration apply step.
	LogApply(l *ApplyLog)

	// LogNodeEvent records a cluster node lifecycle event.
	LogNodeEvent(e *NodeEvent)

	// ── Read methods ──

	// ListLogs returns recent operation logs, optionally filtered.
	ListLogs(ctx context.Context, action, targetID string) ([]OperationLog, error)

	// ListApplyLogs returns recent apply logs.
	ListApplyLogs(limit int) ([]ApplyLog, error)

	// ListAuditLogs returns recent audit logs.
	ListAuditLogs(limit int) ([]AuditLog, error)

	// ListNodeEvents returns recent node events.
	ListNodeEvents(limit int) ([]NodeEvent, error)
}

// AuditLogger is the minimal audit-only interface used by security-sensitive
// packages (token, adminauth) that only need audit trail writes.
type AuditLogger interface {
	LogAudit(actorType, actorID, eventType, ip, userAgent, targetType, targetID, result, errorCode string)
}

// Compile-time check: *AppService satisfies Logger.
var _ Logger = (*AppService)(nil)

// Compile-time check: *AppService satisfies AuditLogger.
var _ AuditLogger = (*AppService)(nil)
