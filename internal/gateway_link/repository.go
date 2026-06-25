package gatewaylink

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for trusted gateways.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new gateway link repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new trusted gateway.
func (r *Repository) Create(g *TrustedGateway) error {
	_, err := r.DB.Exec(
		`INSERT INTO trusted_gateways
		 (id, name, host, private_ip, port, auth_type, auth_value, gateway_type, auto_route, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Name, g.Host, g.PrivateIP, g.Port,
		g.AuthType, g.AuthValue, g.GatewayType, g.AutoRoute, g.Status,
		g.CreatedAt.Format(time.RFC3339), g.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert trusted_gateway: %w", err)
	}
	return nil
}

// FindAll returns all trusted gateways.
func (r *Repository) FindAll() ([]TrustedGateway, error) {
	rows, err := r.DB.Query(
		`SELECT id, name, host, private_ip, port, auth_type, gateway_type, auto_route, status, created_at, updated_at
		 FROM trusted_gateways ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query trusted_gateways: %w", err)
	}
	defer rows.Close()
	return scanGateways(rows)
}

// FindByID returns a trusted gateway by ID.
func (r *Repository) FindByID(id string) (*TrustedGateway, error) {
	row := r.DB.QueryRow(
		`SELECT id, name, host, private_ip, port, auth_type, auth_value, gateway_type, auto_route, status, created_at, updated_at
		 FROM trusted_gateways WHERE id = ?`, id)
	g, err := scanGateway(row)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// FindByType returns gateways of a specific type.
func (r *Repository) FindByType(gatewayType string) ([]TrustedGateway, error) {
	rows, err := r.DB.Query(
		`SELECT id, name, host, private_ip, port, auth_type, gateway_type, auto_route, status, created_at, updated_at
		 FROM trusted_gateways WHERE gateway_type = ? ORDER BY name`, gatewayType)
	if err != nil {
		return nil, fmt.Errorf("query trusted_gateways by type: %w", err)
	}
	defer rows.Close()
	return scanGateways(rows)
}

// UpdateHost updates the host and private IP of a gateway.
func (r *Repository) UpdateHost(id, host, privateIP string) error {
	_, err := r.DB.Exec(
		`UPDATE trusted_gateways SET host = ?, private_ip = ?, updated_at = ? WHERE id = ?`,
		host, privateIP, time.Now().Format(time.RFC3339), id)
	return err
}

// UpdateStatus updates the status of a gateway.
func (r *Repository) UpdateStatus(id, status string) error {
	_, err := r.DB.Exec(
		`UPDATE trusted_gateways SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().Format(time.RFC3339), id)
	return err
}

// RotateSecret updates the auth secret.
func (r *Repository) RotateSecret(id, newAuthValue string) error {
	_, err := r.DB.Exec(
		`UPDATE trusted_gateways SET auth_value = ?, updated_at = ? WHERE id = ?`,
		newAuthValue, time.Now().Format(time.RFC3339), id)
	return err
}

// Delete removes a trusted gateway.
func (r *Repository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM trusted_gateways WHERE id = ?`, id)
	return err
}

func scanGateways(rows *sql.Rows) ([]TrustedGateway, error) {
	var gateways []TrustedGateway
	for rows.Next() {
		var g TrustedGateway
		var createdAt, updatedAt string
		var privateIP, authType sql.NullString
		err := rows.Scan(&g.ID, &g.Name, &g.Host, &privateIP, &g.Port,
			&authType, &g.GatewayType, &g.AutoRoute, &g.Status,
			&createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan trusted_gateway: %w", err)
		}
		g.PrivateIP = privateIP.String
		g.AuthType = authType.String
		g.AuthValue = "" // never returned
		g.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		g.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		gateways = append(gateways, g)
	}
	return gateways, rows.Err()
}

func scanGateway(row *sql.Row) (*TrustedGateway, error) {
	var g TrustedGateway
	var createdAt, updatedAt string
	var privateIP, authType, authValue sql.NullString
	err := row.Scan(&g.ID, &g.Name, &g.Host, &privateIP, &g.Port,
		&authType, &authValue, &g.GatewayType, &g.AutoRoute, &g.Status,
		&createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan trusted_gateway: %w", err)
	}
	g.PrivateIP = privateIP.String
	g.AuthType = authType.String
	g.AuthValue = authValue.String
	g.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	g.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &g, nil
}
