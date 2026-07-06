package serviceauth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Repository provides database access for the serviceauth package.
// It depends only on *sql.DB — no Aegis packages.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// ============================================================================
// Service records
// ============================================================================

// UpsertService inserts or updates a service by its unique name.
// Host/Port/NodeHost are locators that change on restart/migration.
func (r *Repository) UpsertService(s *ServiceRecord) error {
	result, err := r.DB.Exec(
		`UPDATE svc_auth_services
		 SET host=?, port=?, node_host=?, apis_json=?, public_key=?, status=?, last_seen=?, updated_at=?
		 WHERE name=?`,
		s.Host, s.Port, s.NodeHost, s.APIsJSON, s.PublicKey, s.Status,
		s.LastSeen.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339),
		s.Name,
	)
	if err != nil {
		return fmt.Errorf("upsert service: update: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		return nil
	}

	// New service.
	_, err = r.DB.Exec(
		`INSERT INTO svc_auth_services (id, name, host, port, node_host, apis_json, public_key, status, last_seen, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Host, s.Port, s.NodeHost, s.APIsJSON, s.PublicKey, s.Status,
		s.LastSeen.Format(time.RFC3339), s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("upsert service: insert: %w", err)
	}
	return nil
}

// FindByName returns all instances registered under the given service name.
func (r *Repository) FindByName(name string) ([]ServiceRecord, error) {
	rows, err := r.DB.Query(
		`SELECT id, name, host, port, node_host, apis_json, status, last_seen, created_at, updated_at
		 FROM svc_auth_services WHERE name=?`, name)
	if err != nil {
		return nil, fmt.Errorf("find service by name: %w", err)
	}
	defer rows.Close()
	return scanServices(rows)
}

// FindByID returns a single service record.
func (r *Repository) FindByID(id string) (*ServiceRecord, error) {
	row := r.DB.QueryRow(
		`SELECT id, name, host, port, node_host, apis_json, status, last_seen, created_at, updated_at
		 FROM svc_auth_services WHERE id=?`, id)
	return scanService(row)
}

// ListActive returns all services whose status is not "blocked".
func (r *Repository) ListActive() ([]ServiceRecord, error) {
	rows, err := r.DB.Query(
		`SELECT id, name, host, port, node_host, apis_json, status, last_seen, created_at, updated_at
		 FROM svc_auth_services WHERE status='active' ORDER BY name, host`)
	if err != nil {
		return nil, fmt.Errorf("list active services: %w", err)
	}
	defer rows.Close()
	return scanServices(rows)
}

// ListAll returns every service record regardless of status.
func (r *Repository) ListAll() ([]ServiceRecord, error) {
	rows, err := r.DB.Query(
		`SELECT id, name, host, port, node_host, apis_json, status, last_seen, created_at, updated_at
		 FROM svc_auth_services ORDER BY name, host`)
	if err != nil {
		return nil, fmt.Errorf("list all services: %w", err)
	}
	defer rows.Close()
	return scanServices(rows)
}

// UpdateStatus sets the status for a service.
func (r *Repository) UpdateStatus(id, status string) error {
	_, err := r.DB.Exec(
		`UPDATE svc_auth_services SET status=?, updated_at=? WHERE id=?`,
		status, time.Now().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update service status: %w", err)
	}
	return nil
}

// UpdateLastSeen refreshes the last_seen timestamp.
func (r *Repository) UpdateLastSeen(id string, t time.Time) error {
	_, err := r.DB.Exec(
		`UPDATE svc_auth_services SET last_seen=? WHERE id=?`,
		t.Format(time.RFC3339), id)
	return err
}

// DeleteService removes a service record by ID.
func (r *Repository) DeleteService(id string) error {
	_, err := r.DB.Exec(`DELETE FROM svc_auth_services WHERE id=?`, id)
	return err
}

// DeleteStale removes services that haven't been seen since the given time.
func (r *Repository) DeleteStale(before time.Time) (int, error) {
	result, err := r.DB.Exec(
		`DELETE FROM svc_auth_services WHERE last_seen < ?`,
		before.Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// ListPublicKeys returns a map of service name → Ed25519 public key for all active services.
func (r *Repository) ListPublicKeys() (map[string]string, error) {
	rows, err := r.DB.Query(
		`SELECT name, public_key FROM svc_auth_services WHERE status='active' AND public_key != ''`)
	if err != nil {
		return nil, fmt.Errorf("list public keys: %w", err)
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var name, key string
		if err := rows.Scan(&name, &key); err != nil {
			return nil, err
		}
		out[name] = key
	}
	return out, rows.Err()
}

// ============================================================================
// Call logs
// ============================================================================

// InsertCallLog writes one call record.
func (r *Repository) InsertCallLog(log *CallLog) error {
	allowedInt := 0
	if log.Allowed {
		allowedInt = 1
	}
	_, err := r.DB.Exec(
		`INSERT INTO svc_auth_call_logs (id, caller_service, target_service, target_api, caller_host, target_host, allowed, latency_ms, error_msg, called_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.CallerService, log.TargetService, log.TargetAPI,
		log.CallerHost, log.TargetHost, allowedInt, log.LatencyMs, log.ErrorMsg,
		log.CalledAt.Format(time.RFC3339),
	)
	return err
}

// QueryCallLogs returns recent call logs, ordered by most recent first.
func (r *Repository) QueryCallLogs(since time.Time, limit int) ([]CallLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.DB.Query(
		`SELECT id, caller_service, target_service, target_api, caller_host, target_host, allowed, latency_ms, error_msg, called_at
		 FROM svc_auth_call_logs WHERE called_at >= ? ORDER BY called_at DESC LIMIT ?`,
		since.Format(time.RFC3339), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query call logs: %w", err)
	}
	defer rows.Close()

	var logs []CallLog
	for rows.Next() {
		var l CallLog
		var calledAt string
		var allowedInt int
		var errorMsg sql.NullString
		if err := rows.Scan(&l.ID, &l.CallerService, &l.TargetService, &l.TargetAPI,
			&l.CallerHost, &l.TargetHost, &allowedInt, &l.LatencyMs, &errorMsg, &calledAt); err != nil {
			return nil, fmt.Errorf("scan call log: %w", err)
		}
		l.Allowed = allowedInt == 1
		l.ErrorMsg = errorMsg.String
		l.CalledAt, _ = time.Parse(time.RFC3339, calledAt)
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// TopologyEdges returns aggregated call counts between service pairs.
func (r *Repository) TopologyEdges(since time.Time) ([]TopologyEdge, error) {
	rows, err := r.DB.Query(
		`SELECT caller_service, target_service, target_api, COUNT(*) as cnt, MAX(called_at) as last_seen
		 FROM svc_auth_call_logs
		 WHERE called_at >= ?
		 GROUP BY caller_service, target_service, target_api
		 ORDER BY cnt DESC`,
		since.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("query topology: %w", err)
	}
	defer rows.Close()

	var edges []TopologyEdge
	for rows.Next() {
		var e TopologyEdge
		var lastSeen string
		if err := rows.Scan(&e.Caller, &e.Target, &e.API, &e.Count, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan topology edge: %w", err)
		}
		e.LastSeen = lastSeen
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// ============================================================================
// Blocklist
// ============================================================================

// AddBlock inserts a blocklist entry and returns it.
func (r *Repository) AddBlock(entry *BlocklistEntry) error {
	_, err := r.DB.Exec(
		`INSERT INTO svc_auth_blocklist (id, service_id, api_name, reason, version, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.ServiceID, entry.APIName, entry.Reason,
		entry.Version, time.Now().Format(time.RFC3339),
	)
	return err
}

// RemoveBlock deletes a blocklist entry by ID.
func (r *Repository) RemoveBlock(id string) error {
	_, err := r.DB.Exec(`DELETE FROM svc_auth_blocklist WHERE id=?`, id)
	return err
}

// GetBlocklist returns all active blocklist entries.
func (r *Repository) GetBlocklist() ([]BlocklistEntry, error) {
	rows, err := r.DB.Query(
		`SELECT id, service_id, api_name, reason, version
		 FROM svc_auth_blocklist ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("get blocklist: %w", err)
	}
	defer rows.Close()
	return scanBlocklist(rows)
}

// GetBlocklistSince returns entries with version greater than the given value.
func (r *Repository) GetBlocklistSince(version int64) ([]BlocklistEntry, error) {
	rows, err := r.DB.Query(
		`SELECT id, service_id, api_name, reason, version
		 FROM svc_auth_blocklist WHERE version > ? ORDER BY version`, version)
	if err != nil {
		return nil, fmt.Errorf("get blocklist since: %w", err)
	}
	defer rows.Close()
	return scanBlocklist(rows)
}

// GetBlocklistVersion returns the maximum blocklist version, or 0 if empty.
func (r *Repository) GetBlocklistVersion() (int64, error) {
	var v sql.NullInt64
	err := r.DB.QueryRow(`SELECT MAX(version) FROM svc_auth_blocklist`).Scan(&v)
	if err != nil {
		return 0, err
	}
	if v.Valid {
		return v.Int64, nil
	}
	return 0, nil
}

// ============================================================================
// Helpers
// ============================================================================

func scanService(row *sql.Row) (*ServiceRecord, error) {
	var s ServiceRecord
	var lastSeen, createdAt, updatedAt string
	err := row.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.NodeHost, &s.APIsJSON,
		&s.PublicKey, &s.Status, &lastSeen, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	s.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &s, nil
}

func scanServices(rows *sql.Rows) ([]ServiceRecord, error) {
	var out []ServiceRecord
	for rows.Next() {
		var s ServiceRecord
		var lastSeen, createdAt, updatedAt string
		if err := rows.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.NodeHost, &s.APIsJSON,
			&s.PublicKey, &s.Status, &lastSeen, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		s.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		out = append(out, s)
	}
	return out, rows.Err()
}

func scanBlocklist(rows *sql.Rows) ([]BlocklistEntry, error) {
	var out []BlocklistEntry
	for rows.Next() {
		var b BlocklistEntry
		if err := rows.Scan(&b.ID, &b.ServiceID, &b.APIName, &b.Reason, &b.Version); err != nil {
			return nil, fmt.Errorf("scan blocklist: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// JoinStrings is a helper for building the APIs JSON list display string.
func (r *Repository) JoinStrings(ss []string, sep string) string {
	return strings.Join(ss, sep)
}

// DefaultIDGen returns a random hex ID using crypto/rand.
// This is used by the standalone binary. When integrated into Aegis,
// id.GenerateID is preferred.
func DefaultIDGen() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should never fail on a modern OS; fall back to time.
		return fmt.Sprintf("svc_%x", time.Now().UnixNano())
	}
	return "svc_" + hex.EncodeToString(b)
}
