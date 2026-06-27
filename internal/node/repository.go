package node

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for node records.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new node repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

const nodeSelectCols = `id, node_id, name, role, status, hostname, local_ip, private_ip, public_ip, region, network_id, os, arch, agent_version, last_heartbeat_at, last_error, is_current, is_leader, leader_elected_at, ip_migrated, state_version, capabilities, last_seen, created_at, updated_at`

// scanNode scans a single node row into a NodeRecord.
func scanNode(scanner interface {
	Scan(dest ...interface{}) error
}, n *NodeRecord) error {
	var createdAt, updatedAt, lastSeen, leaderAt, lastHBAt, capsStr string
	var migrated, current, leader int
	var stateVersion uint64

	err := scanner.Scan(
		&n.ID, &n.NodeID, &n.Name, &n.Role, &n.Status,
		&n.Hostname, &n.LocalIP, &n.PrivateIP, &n.PublicIP,
		&n.Region, &n.NetworkID, &n.OS, &n.Arch, &n.AgentVersion,
		&lastHBAt, &n.LastError,
		&current, &leader, &leaderAt, &migrated, &stateVersion,
		&capsStr, &lastSeen, &createdAt, &updatedAt,
	)
	if err != nil {
		return err
	}

	n.IsCurrent = current == 1
	n.IsLeader = leader == 1
	n.IPMigrated = migrated == 1
	n.StateVersion = stateVersion
	n.Capabilities = ParseCapabilities(capsStr)

	if leaderAt != "" {
		n.LeaderElectedAt, _ = time.Parse(time.RFC3339, leaderAt)
	}
	if lastHBAt != "" {
		n.LastHeartbeatAt, _ = time.Parse(time.RFC3339, lastHBAt)
	}
	n.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
	n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	n.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return nil
}

// nodeRowValues returns the values for INSERT/UPDATE, excluding the auto-managed fields.
func nodeRowValues(n *NodeRecord) []interface{} {
	migrated := 0
	if n.IPMigrated {
		migrated = 1
	}
	current := 0
	if n.IsCurrent {
		current = 1
	}
	leader := 0
	if n.IsLeader {
		leader = 1
	}
	leaderAt := ""
	if !n.LeaderElectedAt.IsZero() {
		leaderAt = n.LeaderElectedAt.Format(time.RFC3339)
	}
	lastHBAt := ""
	if !n.LastHeartbeatAt.IsZero() {
		lastHBAt = n.LastHeartbeatAt.Format(time.RFC3339)
	}
	caps := n.Capabilities.ToJSON()

	return []interface{}{
		n.ID, n.NodeID, n.Name, n.Role, n.Status,
		n.Hostname, n.LocalIP, n.PrivateIP, n.PublicIP,
		n.Region, n.NetworkID, n.OS, n.Arch, n.AgentVersion,
		lastHBAt, n.LastError,
		current, leader, leaderAt, migrated, n.StateVersion,
		caps,
		n.LastSeen.Format(time.RFC3339),
		n.CreatedAt.Format(time.RFC3339),
		n.UpdatedAt.Format(time.RFC3339),
	}
}

// Create inserts a new node record.
func (r *Repository) Create(n *NodeRecord) error {
	_, err := r.DB.Exec(
		`INSERT INTO nodes (id, node_id, name, role, status, hostname, local_ip, private_ip, public_ip,
		 region, network_id, os, arch, agent_version, last_heartbeat_at, last_error,
		 is_current, is_leader, leader_elected_at, ip_migrated, state_version, capabilities,
		 last_seen, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nodeRowValues(n)...,
	)
	if err != nil {
		return fmt.Errorf("insert node: %w", err)
	}
	return nil
}

// FindCurrent returns the current node record.
func (r *Repository) FindCurrent() (*NodeRecord, error) {
	var n NodeRecord
	err := scanNode(
		r.DB.QueryRow(`SELECT `+nodeSelectCols+` FROM nodes WHERE is_current = 1 LIMIT 1`),
		&n,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query current node: %w", err)
	}
	return &n, nil
}

// FindAll returns all node records.
func (r *Repository) FindAll() ([]NodeRecord, error) {
	rows, err := r.DB.Query(`SELECT ` + nodeSelectCols + ` FROM nodes ORDER BY last_seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// FindByNodeID returns a node by its logical node_id.
func (r *Repository) FindByNodeID(nodeID string) (*NodeRecord, error) {
	var n NodeRecord
	err := scanNode(
		r.DB.QueryRow(`SELECT `+nodeSelectCols+` FROM nodes WHERE node_id = ?`, nodeID),
		&n,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query node by node_id: %w", err)
	}
	return &n, nil
}

// FindByID returns a node by its internal DB ID.
func (r *Repository) FindByID(id string) (*NodeRecord, error) {
	var n NodeRecord
	err := scanNode(
		r.DB.QueryRow(`SELECT `+nodeSelectCols+` FROM nodes WHERE id = ?`, id),
		&n,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query node by id: %w", err)
	}
	return &n, nil
}

// UnsetCurrent marks all nodes as not current.
func (r *Repository) UnsetCurrent() error {
	_, err := r.DB.Exec(`UPDATE nodes SET is_current = 0`)
	return err
}

// Update updates a full node record.
func (r *Repository) Update(n *NodeRecord) error {
	vals := nodeRowValues(n)
	vals = append(vals, n.ID) // for WHERE
	_, err := r.DB.Exec(
		`UPDATE nodes SET id=?, node_id=?, name=?, role=?, status=?, hostname=?, local_ip=?, private_ip=?, public_ip=?,
		 region=?, network_id=?, os=?, arch=?, agent_version=?, last_heartbeat_at=?, last_error=?,
		 is_current=?, is_leader=?, leader_elected_at=?, ip_migrated=?, state_version=?, capabilities=?,
		 last_seen=?, created_at=?, updated_at=?
		 WHERE id=?`,
		vals...,
	)
	if err != nil {
		return fmt.Errorf("update node: %w", err)
	}
	return nil
}

// UpdateHeartbeat updates node status from a heartbeat report.
func (r *Repository) UpdateHeartbeat(nodeID, status, agentVersion, publicIP, privateIP, hostname, lastError string, now time.Time) error {
	nowStr := now.Format(time.RFC3339)
	_, err := r.DB.Exec(
		`UPDATE nodes SET status=?, agent_version=?, public_ip=?, private_ip=?, hostname=?,
		 last_heartbeat_at=?, last_error=?, last_seen=?, updated_at=?
		 WHERE node_id=?`,
		status, agentVersion, publicIP, privateIP, hostname,
		nowStr, lastError, nowStr, nowStr, nodeID,
	)
	if err != nil {
		return fmt.Errorf("update node heartbeat: %w", err)
	}
	return nil
}

// SetStatus updates the node's status field.
func (r *Repository) SetStatus(nodeID, status, lastError string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.DB.Exec(
		`UPDATE nodes SET status=?, last_error=?, updated_at=? WHERE node_id=?`,
		status, lastError, now, nodeID,
	)
	return err
}

// UpdateCapabilities updates only the capabilities column for a node.
func (r *Repository) UpdateCapabilities(nodeID string, caps NodeCapabilities) error {
	_, err := r.DB.Exec(`UPDATE nodes SET capabilities=? WHERE node_id=?`,
		caps.ToJSON(), nodeID)
	return err
}

func scanNodes(rows *sql.Rows) ([]NodeRecord, error) {
	var nodes []NodeRecord
	for rows.Next() {
		var n NodeRecord
		if err := scanNode(rows, &n); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
