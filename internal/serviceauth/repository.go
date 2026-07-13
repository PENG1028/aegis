package serviceauth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// Repository provides database access for the serviceauth package.
type Repository struct {
	DB *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// ─── Service records ──────────────────────────────────────────────────────

const svcCols = "id, name, host, port, listen_port, node_host, apis_json, public_key, status, instance_id, last_seen, created_at, updated_at"

func (r *Repository) UpsertService(s *ServiceRecord) error {
	result, err := r.DB.Exec(
		`UPDATE svc_auth_services SET status=?, last_seen=?, updated_at=?, instance_id=?, host=?, port=?, listen_port=?, node_host=?
			 WHERE name=? AND public_key=?`,
		s.Status, s.LastSeen.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339),
		s.InstanceID, s.Host, s.Port, s.ListenPort, s.NodeHost,
		s.Name, s.PublicKey,
	)
	if err != nil {
		return fmt.Errorf("upsert service: update: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		return nil
	}
	_, err = r.DB.Exec(
		`INSERT INTO svc_auth_services (id, name, host, port, listen_port, node_host, apis_json, public_key, instance_id, status, last_seen, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, '', ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Host, s.Port, s.ListenPort, s.NodeHost,
		s.PublicKey, s.InstanceID, s.Status,
		s.LastSeen.Format(time.RFC3339), s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

// Heartbeat updates last_seen for a specific instance by name+instance_id.
func (r *Repository) Heartbeat(name, instanceID string, now time.Time) error {
	_, err := r.DB.Exec(
		`UPDATE svc_auth_services SET last_seen=?, updated_at=? WHERE name=? AND instance_id=?`,
		now.Format(time.RFC3339), now.Format(time.RFC3339), name, instanceID,
	)
	return err
}

// CountOnlineByService returns how many instances per service have heartbeated recently.
func (r *Repository) CountOnlineByService(since time.Time) (map[string]int, error) {
	rows, err := r.DB.Query(
		`SELECT name, COUNT(*) as cnt FROM svc_auth_services WHERE status='active' AND last_seen > ? GROUP BY name ORDER BY name`,
		since.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var name string
		var cnt int
		if err := rows.Scan(&name, &cnt); err != nil {
			return nil, err
		}
		out[name] = cnt
	}
	return out, rows.Err()
}

func (r *Repository) FindByName(name string) ([]ServiceRecord, error) {
	rows, err := r.DB.Query(`SELECT `+svcCols+` FROM svc_auth_services WHERE name=?`, name)
	if err != nil {
		return nil, fmt.Errorf("find service by name: %w", err)
	}
	defer rows.Close()
	return scanServices(rows)
}

func (r *Repository) FindByPublicKey(pubKey string) ([]ServiceRecord, error) {
	rows, err := r.DB.Query(`SELECT `+svcCols+` FROM svc_auth_services WHERE public_key=? AND status='active'`, pubKey)
	if err != nil {
		return nil, fmt.Errorf("find by public key: %w", err)
	}
	defer rows.Close()
	return scanServices(rows)
}

func (r *Repository) FindByID(id string) (*ServiceRecord, error) {
	row := r.DB.QueryRow(`SELECT `+svcCols+` FROM svc_auth_services WHERE id=?`, id)
	return scanService(row)
}

func (r *Repository) ListActive() ([]ServiceRecord, error) {
	rows, err := r.DB.Query(`SELECT `+svcCols+` FROM svc_auth_services WHERE status='active' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list active services: %w", err)
	}
	defer rows.Close()
	return scanServices(rows)
}

func (r *Repository) ListAll() ([]ServiceRecord, error) {
	rows, err := r.DB.Query(`SELECT `+svcCols+` FROM svc_auth_services ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list all services: %w", err)
	}
	defer rows.Close()
	return scanServices(rows)
}

func (r *Repository) UpdateStatus(id, status string) error {
	_, err := r.DB.Exec(`UPDATE svc_auth_services SET status=?, updated_at=? WHERE id=?`,
		status, time.Now().Format(time.RFC3339), id)
	return err
}

func (r *Repository) UpdateLastSeen(id string, t time.Time) error {
	_, err := r.DB.Exec(`UPDATE svc_auth_services SET last_seen=? WHERE id=?`,
		t.Format(time.RFC3339), id)
	return err
}

func (r *Repository) MarkStale(threshold time.Time) (int, error) {
	result, err := r.DB.Exec(
		`UPDATE svc_auth_services SET status='inactive', updated_at=? WHERE last_seen < ? AND status='active'`,
		time.Now().Format(time.RFC3339), threshold.Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

func (r *Repository) DeleteService(id string) error {
	_, err := r.DB.Exec(`DELETE FROM svc_auth_services WHERE id=?`, id)
	return err
}

func (r *Repository) DeleteStale(before time.Time) (int, error) {
	result, err := r.DB.Exec(`DELETE FROM svc_auth_services WHERE last_seen < ?`, before.Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

func (r *Repository) ListPublicKeys() (map[string][]string, error) {
	rows, err := r.DB.Query(
		`SELECT name, public_key FROM svc_auth_services WHERE status='active' AND public_key != ''`)
	if err != nil {
		return nil, fmt.Errorf("list public keys: %w", err)
	}
	defer rows.Close()
	out := make(map[string][]string)
	for rows.Next() {
		var name, key string
		if err := rows.Scan(&name, &key); err != nil {
			return nil, err
		}
		out[name] = append(out[name], key)
	}
	return out, rows.Err()
}

// ─── Call logs ────────────────────────────────────────────────────────────

func (r *Repository) InsertCallLog(log *CallLog) error {
	allowedInt := 0
	if log.Allowed {
		allowedInt = 1
	}
	_, err := r.DB.Exec(
		`INSERT INTO svc_auth_call_logs (id, caller_service, target_service, target_api, caller_host, target_host, allowed, latency_ms, error_msg, called_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.CallerService, log.TargetService, log.TargetAPI,
		log.CallerHost, log.TargetHost, allowedInt, log.LatencyMs, log.ErrorMsg,
		log.CalledAt.Format(time.RFC3339),
	)
	return err
}

func (r *Repository) QueryCallLogs(since time.Time, limit int) ([]CallLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.DB.Query(
		`SELECT id, caller_service, target_service, target_api, caller_host, target_host, allowed, latency_ms, error_msg, called_at FROM svc_auth_call_logs WHERE called_at >= ? ORDER BY called_at DESC LIMIT ?`,
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

func (r *Repository) TopologyEdges(since time.Time) ([]TopologyEdge, error) {
	rows, err := r.DB.Query(
		`SELECT caller_service, target_service, target_api, COUNT(*) as cnt, MAX(called_at) as last_seen FROM svc_auth_call_logs WHERE called_at >= ? GROUP BY caller_service, target_service, target_api ORDER BY cnt DESC`,
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

// CallersOf returns services that have called the given service, aggregated.
func (r *Repository) CallersOf(name string, since time.Time) ([]TopologyEdge, error) {
	rows, err := r.DB.Query(
		`SELECT caller_service, target_service, target_api, COUNT(*) as cnt, MAX(called_at) as last_seen FROM svc_auth_call_logs WHERE target_service=? AND called_at >= ? GROUP BY caller_service, target_api ORDER BY cnt DESC`,
		name, since.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("callers of %s: %w", name, err)
	}
	defer rows.Close()
	var edges []TopologyEdge
	for rows.Next() {
		var e TopologyEdge
		var lastSeen string
		if err := rows.Scan(&e.Caller, &e.Target, &e.API, &e.Count, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan caller edge: %w", err)
		}
		e.LastSeen = lastSeen
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// DepsOf returns services that the given service has called, aggregated.
func (r *Repository) DepsOf(name string, since time.Time) ([]TopologyEdge, error) {
	rows, err := r.DB.Query(
		`SELECT caller_service, target_service, target_api, COUNT(*) as cnt, MAX(called_at) as last_seen FROM svc_auth_call_logs WHERE caller_service=? AND called_at >= ? GROUP BY target_service, target_api ORDER BY cnt DESC`,
		name, since.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("deps of %s: %w", name, err)
	}
	defer rows.Close()
	var edges []TopologyEdge
	for rows.Next() {
		var e TopologyEdge
		var lastSeen string
		if err := rows.Scan(&e.Caller, &e.Target, &e.API, &e.Count, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan dep edge: %w", err)
		}
		e.LastSeen = lastSeen
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// ─── Blocklist ────────────────────────────────────────────────────────────

func (r *Repository) AddBlock(entry *BlocklistEntry) error {
	_, err := r.DB.Exec(
		`INSERT INTO svc_auth_blocklist (id, service_id, api_name, reason, version, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.ServiceID, entry.APIName, entry.Reason,
		entry.Version, time.Now().Format(time.RFC3339),
	)
	return err
}

func (r *Repository) RemoveBlock(id string) error {
	_, err := r.DB.Exec(`DELETE FROM svc_auth_blocklist WHERE id=?`, id)
	return err
}

func (r *Repository) GetBlocklist() ([]BlocklistEntry, error) {
	rows, err := r.DB.Query(`SELECT id, service_id, api_name, reason, version FROM svc_auth_blocklist ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("get blocklist: %w", err)
	}
	defer rows.Close()
	return scanBlocklist(rows)
}

func (r *Repository) GetBlocklistSince(version int64) ([]BlocklistEntry, error) {
	rows, err := r.DB.Query(`SELECT id, service_id, api_name, reason, version FROM svc_auth_blocklist WHERE version > ? ORDER BY version`, version)
	if err != nil {
		return nil, fmt.Errorf("get blocklist since: %w", err)
	}
	defer rows.Close()
	return scanBlocklist(rows)
}

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

// ─── Scan helpers ─────────────────────────────────────────────────────────

func scanService(row *sql.Row) (*ServiceRecord, error) {
	var s ServiceRecord
	var lastSeen, createdAt, updatedAt string
	err := row.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.ListenPort, &s.NodeHost, &s.APIsJSON,
		&s.PublicKey, &s.Status, &s.InstanceID, &lastSeen, &createdAt, &updatedAt)
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
		if err := rows.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.ListenPort, &s.NodeHost, &s.APIsJSON,
			&s.PublicKey, &s.Status, &s.InstanceID, &lastSeen, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		s.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		out = append(out, s)
	}
	return out, nil
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
	return out, nil
}


func DefaultIDGen() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("svc_%x", time.Now().UnixNano())
	}
	return "svc_" + hex.EncodeToString(b)
}

// ─── Groups ───────────────────────────────────────────────────────────────



// ─── Policies ─────────────────────────────────────────────────────────────

