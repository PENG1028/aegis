package topology

import (
	"database/sql"
	"fmt"
	"time"

	"aegis/internal/id"
)

// Repository provides database access for topology edges.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new topology repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// CreateOrUpdateEdge creates or updates a topology edge.
func (r *Repository) CreateOrUpdateEdge(e *TopologyEdge) error {
	if e.ID == "" {
		e.ID = id.New("te")
	}
	e.UpdatedAt = time.Now()

	existing, err := r.findByNodes(e.FromNodeID, e.ToNodeID)
	if err != nil {
		return err
	}
	if existing != nil {
		e.ID = existing.ID
		e.CreatedAt = existing.CreatedAt
		return r.update(e)
	}
	e.CreatedAt = time.Now()
	return r.create(e)
}

func (r *Repository) create(e *TopologyEdge) error {
	_, err := r.DB.Exec(
		`INSERT INTO topology_edges (id, from_node_id, to_node_id, private_reachable, public_reachable,
		 preferred_gateway_id, gateway_link_id, status, last_error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.FromNodeID, e.ToNodeID,
		boolToInt(e.PrivateReachable), boolToInt(e.PublicReachable),
		e.PreferredGatewayID, e.GatewayLinkID, e.Status, e.LastError,
		e.CreatedAt.Format(time.RFC3339), e.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert edge: %w", err)
	}
	return nil
}

func (r *Repository) update(e *TopologyEdge) error {
	_, err := r.DB.Exec(
		`UPDATE topology_edges SET private_reachable=?, public_reachable=?,
		 preferred_gateway_id=?, gateway_link_id=?, status=?, last_error=?, updated_at=?
		 WHERE id=?`,
		boolToInt(e.PrivateReachable), boolToInt(e.PublicReachable),
		e.PreferredGatewayID, e.GatewayLinkID, e.Status, e.LastError,
		e.UpdatedAt.Format(time.RFC3339), e.ID)
	return err
}

// GetEdge returns the edge between two nodes.
func (r *Repository) GetEdge(fromNodeID, toNodeID string) (*TopologyEdge, error) {
	return r.findByNodes(fromNodeID, toNodeID)
}

// ListEdges returns all topology edges.
func (r *Repository) ListEdges() ([]TopologyEdge, error) {
	rows, err := r.DB.Query(
		`SELECT id, from_node_id, to_node_id, private_reachable, public_reachable,
		 preferred_gateway_id, gateway_link_id, status, last_verified_at, last_error, created_at, updated_at
		 FROM topology_edges ORDER BY from_node_id, to_node_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEdges(rows)
}

// SetStatus updates the status for an edge.
func (r *Repository) SetStatus(id, status, lastError string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.DB.Exec(
		`UPDATE topology_edges SET status=?, last_error=?, updated_at=? WHERE id=?`,
		status, lastError, now, id)
	return err
}

func (r *Repository) findByNodes(fromNodeID, toNodeID string) (*TopologyEdge, error) {
	var e TopologyEdge
	var lv, ca, ua string
	var priv, pub int
	err := r.DB.QueryRow(
		`SELECT id, from_node_id, to_node_id, private_reachable, public_reachable,
		 preferred_gateway_id, gateway_link_id, status, last_verified_at, last_error, created_at, updated_at
		 FROM topology_edges WHERE from_node_id=? AND to_node_id=?`, fromNodeID, toNodeID,
	).Scan(&e.ID, &e.FromNodeID, &e.ToNodeID, &priv, &pub,
		&e.PreferredGatewayID, &e.GatewayLinkID, &e.Status, &lv, &e.LastError, &ca, &ua)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query edge: %w", err)
	}
	e.PrivateReachable = priv == 1
	e.PublicReachable = pub == 1
	if lv != "" {
		e.LastVerifiedAt, _ = time.Parse(time.RFC3339, lv)
	}
	e.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	e.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &e, nil
}

func scanEdges(rows *sql.Rows) ([]TopologyEdge, error) {
	var edges []TopologyEdge
	for rows.Next() {
		var e TopologyEdge
		var lv, ca, ua string
		var priv, pub int
		if err := rows.Scan(&e.ID, &e.FromNodeID, &e.ToNodeID, &priv, &pub,
			&e.PreferredGatewayID, &e.GatewayLinkID, &e.Status, &lv, &e.LastError, &ca, &ua); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		e.PrivateReachable = priv == 1
		e.PublicReachable = pub == 1
		if lv != "" {
			e.LastVerifiedAt, _ = time.Parse(time.RFC3339, lv)
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		e.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		edges = append(edges, e)
	}
	if edges == nil {
		edges = []TopologyEdge{}
	}
	return edges, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
