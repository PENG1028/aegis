package gateway

import (
	"database/sql"
	"fmt"
	"time"
)

// Gateway status constants.
const (
	GWStatusUnknown  = "unknown"
	GWStatusOnline   = "online"
	GWStatusOffline  = "offline"
	GWStatusDegraded = "degraded"
)

// GatewayType constants.
const (
	GWTypeLocal   = "local"
	GWTypePrivate = "private"
	GWTypePublic  = "public"
)

// GatewayProvider constants.
const (
	GWProviderCaddy   = "caddy"
	GWProviderHAProxy = "haproxy"
	GWProviderAegis   = "aegis"
)

// GatewayScheme constants.
const (
	GWSchemeHTTP  = "http"
	GWSchemeHTTPS = "https"
)

// GatewayInventory represents a node's gateway capability (v1.8C).
type GatewayInventory struct {
	GatewayID         string    `json:"gateway_id"`
	NodeID            string    `json:"node_id"`
	Name              string    `json:"name"`
	Type              string    `json:"type"`   // local | private | public
	Provider          string    `json:"provider"` // caddy | haproxy | aegis
	BindAddr          string    `json:"bind_addr"`
	Host              string    `json:"host"`
	Port              int       `json:"port"`
	Scheme            string    `json:"scheme"` // http | https
	PublicAccessible  bool      `json:"public_accessible"`
	PrivateAccessible bool      `json:"private_accessible"`
	Enabled           bool      `json:"enabled"`
	Priority          int       `json:"priority"`
	Status            string    `json:"status"` // unknown | online | offline | degraded
	LastVerifiedAt    time.Time `json:"last_verified_at,omitempty"`
	LastError         string    `json:"last_error,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CreateGatewayInput is the input for creating a gateway.
type CreateGatewayInput struct {
	NodeID            string `json:"node_id"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	Provider          string `json:"provider"`
	BindAddr          string `json:"bind_addr"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Scheme            string `json:"scheme"`
	PublicAccessible  bool   `json:"public_accessible"`
	PrivateAccessible bool   `json:"private_accessible"`
	Priority          int    `json:"priority"`
}

// UpdateGatewayInput is the input for updating a gateway.
type UpdateGatewayInput struct {
	Name              *string `json:"name,omitempty"`
	Type              *string `json:"type,omitempty"`
	Provider          *string `json:"provider,omitempty"`
	BindAddr          *string `json:"bind_addr,omitempty"`
	Host              *string `json:"host,omitempty"`
	Port              *int    `json:"port,omitempty"`
	Scheme            *string `json:"scheme,omitempty"`
	PublicAccessible  *bool   `json:"public_accessible,omitempty"`
	PrivateAccessible *bool   `json:"private_accessible,omitempty"`
	Enabled           *bool   `json:"enabled,omitempty"`
	Priority          *int    `json:"priority,omitempty"`
}

// ============================================================================
// GatewayInventory Repository
// ============================================================================

// InventoryRepository provides database access for gateway inventory.
type InventoryRepository struct {
	DB *sql.DB
}

// NewInventoryRepository creates a new gateway inventory repository.
func NewInventoryRepository(db *sql.DB) *InventoryRepository {
	return &InventoryRepository{DB: db}
}

// Create inserts a new gateway.
func (r *InventoryRepository) Create(g *GatewayInventory) error {
	_, err := r.DB.Exec(
		`INSERT INTO gateways (gateway_id, node_id, name, type, provider, bind_addr, host, port, scheme,
		 public_accessible, private_accessible, enabled, priority, status, last_error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.GatewayID, g.NodeID, g.Name, g.Type, g.Provider, g.BindAddr, g.Host, g.Port, g.Scheme,
		boolToInt(g.PublicAccessible), boolToInt(g.PrivateAccessible), boolToInt(g.Enabled),
		g.Priority, g.Status, g.LastError,
		g.CreatedAt.Format(time.RFC3339), g.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert gateway: %w", err)
	}
	return nil
}

// FindByID finds a gateway by ID.
func (r *InventoryRepository) FindByID(id string) (*GatewayInventory, error) {
	var g GatewayInventory
	var lv, ca, ua string
	var pubAcc, privAcc, en int
	err := r.DB.QueryRow(
		`SELECT gateway_id, node_id, name, type, provider, bind_addr, host, port, scheme,
		 public_accessible, private_accessible, enabled, priority, status, last_verified_at, last_error, created_at, updated_at
		 FROM gateways WHERE gateway_id = ?`, id,
	).Scan(&g.GatewayID, &g.NodeID, &g.Name, &g.Type, &g.Provider, &g.BindAddr, &g.Host, &g.Port, &g.Scheme,
		&pubAcc, &privAcc, &en, &g.Priority, &g.Status, &lv, &g.LastError, &ca, &ua)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query gateway: %w", err)
	}
	g.PublicAccessible = pubAcc == 1
	g.PrivateAccessible = privAcc == 1
	g.Enabled = en == 1
	if lv != "" {
		g.LastVerifiedAt, _ = time.Parse(time.RFC3339, lv)
	}
	g.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	g.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &g, nil
}

// FindByNodeID returns all gateways for a node.
func (r *InventoryRepository) FindByNodeID(nodeID string) ([]GatewayInventory, error) {
	rows, err := r.DB.Query(
		`SELECT gateway_id, node_id, name, type, provider, bind_addr, host, port, scheme,
		 public_accessible, private_accessible, enabled, priority, status, last_verified_at, last_error, created_at, updated_at
		 FROM gateways WHERE node_id = ? ORDER BY priority`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGateways(rows)
}

// FindAll returns all gateways.
func (r *InventoryRepository) FindAll() ([]GatewayInventory, error) {
	rows, err := r.DB.Query(
		`SELECT gateway_id, node_id, name, type, provider, bind_addr, host, port, scheme,
		 public_accessible, private_accessible, enabled, priority, status, last_verified_at, last_error, created_at, updated_at
		 FROM gateways ORDER BY node_id, priority`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGateways(rows)
}

// Update updates a gateway's mutable fields.
func (r *InventoryRepository) Update(g *GatewayInventory) error {
	_, err := r.DB.Exec(
		`UPDATE gateways SET name=?, type=?, provider=?, bind_addr=?, host=?, port=?, scheme=?,
		 public_accessible=?, private_accessible=?, enabled=?, priority=?, status=?, last_error=?, updated_at=?
		 WHERE gateway_id=?`,
		g.Name, g.Type, g.Provider, g.BindAddr, g.Host, g.Port, g.Scheme,
		boolToInt(g.PublicAccessible), boolToInt(g.PrivateAccessible), boolToInt(g.Enabled),
		g.Priority, g.Status, g.LastError, g.UpdatedAt.Format(time.RFC3339), g.GatewayID)
	return err
}

// SetStatus updates just the status and last_error for a gateway.
func (r *InventoryRepository) SetStatus(gatewayID, status, lastError string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.DB.Exec(
		`UPDATE gateways SET status=?, last_error=?, updated_at=? WHERE gateway_id=?`,
		status, lastError, now, gatewayID)
	return err
}

// UpsertByNodeAndName creates or updates a gateway by node_id + name.
func (r *InventoryRepository) UpsertByNodeAndName(g *GatewayInventory) error {
	existing, err := r.findByNameAndNode(g.Name, g.NodeID)
	if err != nil {
		return err
	}
	if existing != nil {
		g.GatewayID = existing.GatewayID
		g.CreatedAt = existing.CreatedAt
		g.UpdatedAt = time.Now()
		return r.Update(g)
	}
	g.CreatedAt = time.Now()
	g.UpdatedAt = time.Now()
	return r.Create(g)
}

func (r *InventoryRepository) findByNameAndNode(name, nodeID string) (*GatewayInventory, error) {
	var g GatewayInventory
	var lv, ca, ua string
	var pubAcc, privAcc, en int
	err := r.DB.QueryRow(
		`SELECT gateway_id, node_id, name, type, provider, bind_addr, host, port, scheme,
		 public_accessible, private_accessible, enabled, priority, status, last_verified_at, last_error, created_at, updated_at
		 FROM gateways WHERE name = ? AND node_id = ?`, name, nodeID,
	).Scan(&g.GatewayID, &g.NodeID, &g.Name, &g.Type, &g.Provider, &g.BindAddr, &g.Host, &g.Port, &g.Scheme,
		&pubAcc, &privAcc, &en, &g.Priority, &g.Status, &lv, &g.LastError, &ca, &ua)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &g, nil
}

// Delete removes a gateway.
func (r *InventoryRepository) Delete(gatewayID string) error {
	_, err := r.DB.Exec(`DELETE FROM gateways WHERE gateway_id=?`, gatewayID)
	return err
}

func scanGateways(rows *sql.Rows) ([]GatewayInventory, error) {
	var gws []GatewayInventory
	for rows.Next() {
		var g GatewayInventory
		var lv, ca, ua string
		var pubAcc, privAcc, en int
		if err := rows.Scan(&g.GatewayID, &g.NodeID, &g.Name, &g.Type, &g.Provider, &g.BindAddr, &g.Host, &g.Port, &g.Scheme,
			&pubAcc, &privAcc, &en, &g.Priority, &g.Status, &lv, &g.LastError, &ca, &ua); err != nil {
			return nil, fmt.Errorf("scan gateway: %w", err)
		}
		g.PublicAccessible = pubAcc == 1
		g.PrivateAccessible = privAcc == 1
		g.Enabled = en == 1
		if lv != "" {
			g.LastVerifiedAt, _ = time.Parse(time.RFC3339, lv)
		}
		g.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		g.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		gws = append(gws, g)
	}
	if gws == nil {
		gws = []GatewayInventory{}
	}
	return gws, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
