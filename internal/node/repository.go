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

// Create inserts a new node record.
func (r *Repository) Create(n *NodeRecord) error {
	migrated := 0
	if n.IPMigrated { migrated = 1 }
	current := 0
	if n.IsCurrent { current = 1 }
	leader := 0
	if n.IsLeader { leader = 1 }
	leaderAt := ""
	if !n.LeaderElectedAt.IsZero() { leaderAt = n.LeaderElectedAt.Format(time.RFC3339) }
	_, err := r.DB.Exec(
		`INSERT INTO nodes (id, node_id, hostname, local_ip, private_ip, public_ip, is_current, is_leader, leader_elected_at, ip_migrated, last_seen, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.NodeID, n.Hostname, n.LocalIP, n.PrivateIP, n.PublicIP,
		current, leader, leaderAt, migrated,
		n.LastSeen.Format(time.RFC3339),
		n.CreatedAt.Format(time.RFC3339),
		n.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert node: %w", err)
	}
	return nil
}

// FindCurrent returns the current node record.
func (r *Repository) FindCurrent() (*NodeRecord, error) {
	var n NodeRecord
	var createdAt, updatedAt, lastSeen, leaderAt string
	var migrated, current, leader int
	err := r.DB.QueryRow(
		`SELECT id, node_id, hostname, local_ip, private_ip, public_ip, is_current, is_leader, leader_elected_at, ip_migrated, last_seen, created_at, updated_at
		 FROM nodes WHERE is_current = 1 LIMIT 1`,
	).Scan(&n.ID, &n.NodeID, &n.Hostname, &n.LocalIP, &n.PrivateIP, &n.PublicIP,
		&current, &leader, &leaderAt, &migrated, &lastSeen, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows { return nil, nil }
		return nil, fmt.Errorf("query current node: %w", err)
	}
	n.IsCurrent = current == 1
	n.IsLeader = leader == 1
	if leaderAt != "" { n.LeaderElectedAt, _ = time.Parse(time.RFC3339, leaderAt) }
	n.IPMigrated = migrated == 1
	n.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
	n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	n.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &n, nil
}

// FindAll returns all node records.
func (r *Repository) FindAll() ([]NodeRecord, error) {
	rows, err := r.DB.Query(
		`SELECT id, node_id, hostname, local_ip, private_ip, public_ip, is_current, is_leader, leader_elected_at, ip_migrated, last_seen, created_at, updated_at
		 FROM nodes ORDER BY last_seen DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanNodes(rows)
}

// UnsetCurrent marks all nodes as not current.
func (r *Repository) UnsetCurrent() error {
	_, err := r.DB.Exec(`UPDATE nodes SET is_current = 0`)
	return err
}

// Update updates a node record.
func (r *Repository) Update(n *NodeRecord) error {
	migrated := 0
	if n.IPMigrated { migrated = 1 }
	current := 0
	if n.IsCurrent { current = 1 }
	leader := 0
	if n.IsLeader { leader = 1 }
	leaderAt := ""
	if !n.LeaderElectedAt.IsZero() { leaderAt = n.LeaderElectedAt.Format(time.RFC3339) }
	_, err := r.DB.Exec(
		`UPDATE nodes SET hostname=?, local_ip=?, private_ip=?, public_ip=?, is_current=?, is_leader=?, leader_elected_at=?, ip_migrated=?, last_seen=?, updated_at=? WHERE id=?`,
		n.Hostname, n.LocalIP, n.PrivateIP, n.PublicIP, current, leader, leaderAt, migrated,
		n.LastSeen.Format(time.RFC3339),
		n.UpdatedAt.Format(time.RFC3339), n.ID,
	)
	if err != nil { return fmt.Errorf("update node: %w", err) }
	return nil
}

func scanNodes(rows *sql.Rows) ([]NodeRecord, error) {
	var nodes []NodeRecord
	for rows.Next() {
		var n NodeRecord
		var createdAt, updatedAt, lastSeen, leaderAt string
		var migrated, current, leader int
		if err := rows.Scan(&n.ID, &n.NodeID, &n.Hostname, &n.LocalIP, &n.PrivateIP, &n.PublicIP,
			&current, &leader, &leaderAt, &migrated, &lastSeen, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		n.IsCurrent = current == 1
		n.IsLeader = leader == 1
		if leaderAt != "" { n.LeaderElectedAt, _ = time.Parse(time.RFC3339, leaderAt) }
		n.IPMigrated = migrated == 1
		n.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
		n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		n.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
