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

const nodeSelectCols = `id, node_id, hostname, local_ip, private_ip, public_ip, is_current, is_leader, leader_elected_at, ip_migrated, state_version, capabilities, last_seen, created_at, updated_at`

// Create inserts a new node record.
func (r *Repository) Create(n *NodeRecord) error {
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
	caps := n.Capabilities.ToJSON()

	_, err := r.DB.Exec(
		`INSERT INTO nodes (id, node_id, hostname, local_ip, private_ip, public_ip, is_current, is_leader, leader_elected_at, ip_migrated, state_version, capabilities, last_seen, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.NodeID, n.Hostname, n.LocalIP, n.PrivateIP, n.PublicIP,
		current, leader, leaderAt, migrated, n.StateVersion, caps,
		n.LastSeen.Format(time.RFC3339),
		n.CreatedAt.Format(time.RFC3339),
		n.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert node: %w", err)
	}
	return nil
}

// FindCurrent returns the current node record (FIXED: now scans state_version + capabilities).
func (r *Repository) FindCurrent() (*NodeRecord, error) {
	var n NodeRecord
	var createdAt, updatedAt, lastSeen, leaderAt, capsStr string
	var migrated, current, leader int
	var stateVersion uint64
	err := r.DB.QueryRow(
		`SELECT `+nodeSelectCols+` FROM nodes WHERE is_current = 1 LIMIT 1`,
	).Scan(&n.ID, &n.NodeID, &n.Hostname, &n.LocalIP, &n.PrivateIP, &n.PublicIP,
		&current, &leader, &leaderAt, &migrated, &stateVersion, &capsStr, &lastSeen, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query current node: %w", err)
	}
	n.IsCurrent = current == 1
	n.IsLeader = leader == 1
	if leaderAt != "" {
		n.LeaderElectedAt, _ = time.Parse(time.RFC3339, leaderAt)
	}
	n.StateVersion = stateVersion
	n.IPMigrated = migrated == 1
	n.Capabilities = ParseCapabilities(capsStr)
	n.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
	n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	n.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &n, nil
}

// FindAll returns all node records.
func (r *Repository) FindAll() ([]NodeRecord, error) {
	rows, err := r.DB.Query(
		`SELECT ` + nodeSelectCols + ` FROM nodes ORDER BY last_seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// FindByNodeID returns a node by its logical node_id.
func (r *Repository) FindByNodeID(nodeID string) (*NodeRecord, error) {
	var n NodeRecord
	var createdAt, updatedAt, lastSeen, leaderAt, capsStr string
	var migrated, current, leader int
	var stateVersion uint64
	err := r.DB.QueryRow(
		`SELECT `+nodeSelectCols+` FROM nodes WHERE node_id = ?`, nodeID,
	).Scan(&n.ID, &n.NodeID, &n.Hostname, &n.LocalIP, &n.PrivateIP, &n.PublicIP,
		&current, &leader, &leaderAt, &migrated, &stateVersion, &capsStr, &lastSeen, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query node by node_id: %w", err)
	}
	n.IsCurrent = current == 1
	n.IsLeader = leader == 1
	if leaderAt != "" {
		n.LeaderElectedAt, _ = time.Parse(time.RFC3339, leaderAt)
	}
	n.StateVersion = stateVersion
	n.IPMigrated = migrated == 1
	n.Capabilities = ParseCapabilities(capsStr)
	n.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
	n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	n.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &n, nil
}

// UnsetCurrent marks all nodes as not current.
func (r *Repository) UnsetCurrent() error {
	_, err := r.DB.Exec(`UPDATE nodes SET is_current = 0`)
	return err
}

// Update updates a node record.
func (r *Repository) Update(n *NodeRecord) error {
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
	caps := n.Capabilities.ToJSON()

	_, err := r.DB.Exec(
		`UPDATE nodes SET hostname=?, local_ip=?, private_ip=?, public_ip=?, is_current=?, is_leader=?, leader_elected_at=?, ip_migrated=?, state_version=?, capabilities=?, last_seen=?, updated_at=? WHERE id=?`,
		n.Hostname, n.LocalIP, n.PrivateIP, n.PublicIP, current, leader, leaderAt, migrated,
		n.StateVersion, caps,
		n.LastSeen.Format(time.RFC3339),
		n.UpdatedAt.Format(time.RFC3339), n.ID,
	)
	if err != nil {
		return fmt.Errorf("update node: %w", err)
	}
	return nil
}

func scanNodes(rows *sql.Rows) ([]NodeRecord, error) {
	var nodes []NodeRecord
	for rows.Next() {
		var n NodeRecord
		var createdAt, updatedAt, lastSeen, leaderAt, capsStr string
		var migrated, current, leader int
		var stateVersion uint64
		if err := rows.Scan(&n.ID, &n.NodeID, &n.Hostname, &n.LocalIP, &n.PrivateIP, &n.PublicIP,
			&current, &leader, &leaderAt, &migrated, &stateVersion, &capsStr, &lastSeen, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		n.IsCurrent = current == 1
		n.IsLeader = leader == 1
		if leaderAt != "" {
			n.LeaderElectedAt, _ = time.Parse(time.RFC3339, leaderAt)
		}
		n.StateVersion = stateVersion
		n.IPMigrated = migrated == 1
		n.Capabilities = ParseCapabilities(capsStr)
		n.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
		n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		n.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// UpdateCapabilities updates only the capabilities column for a node.
func (r *Repository) UpdateCapabilities(nodeID string, caps NodeCapabilities) error {
	_, err := r.DB.Exec(`UPDATE nodes SET capabilities=? WHERE id=? OR node_id=?`,
		caps.ToJSON(), nodeID, nodeID)
	return err
}
