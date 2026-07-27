package gateway

import (
	"database/sql"
	"fmt"
	"time"
)

// LinkRepository provides database access for trusted gateways.
type LinkRepository struct {
	DB *sql.DB
}

// NewLinkRepository creates a new gateway link repository.
func NewLinkRepository(db *sql.DB) *LinkRepository {
	return &LinkRepository{DB: db}
}

const gwSelectCols = `id, name, host, private_ip, port, auth_type, gateway_type, auto_route, target_node_id, status, created_at, updated_at`

const gwSelectColsWithAuth = `id, name, host, private_ip, port, auth_type, auth_value, gateway_type, auto_route, target_node_id, status, created_at, updated_at`

const gwSelectColsEncrypted = `id, name, host, private_ip, port, auth_type, auth_value, gateway_type, auto_route, target_node_id, encrypted_secret, secret_nonce, secret_version, secret_created_at, secret_rotated_at, status, created_at, updated_at`

// Create inserts a new trusted gateway.
func (r *LinkRepository) Create(g *TrustedGateway) error {
	_, err := r.DB.Exec(
		`INSERT INTO trusted_gateways
		 (id, name, host, private_ip, port, auth_type, auth_value, gateway_type, auto_route,
		  target_node_id, encrypted_secret, secret_nonce, secret_version, secret_created_at, secret_rotated_at, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Name, g.Host, g.PrivateIP, g.Port,
		g.AuthType, g.AuthValue, g.GatewayType, g.AutoRoute, g.TargetNodeID,
		g.EncryptedSecret, g.SecretNonce, g.SecretVersion, g.SecretCreatedAt, g.SecretRotatedAt, g.Status,
		g.CreatedAt.Format(time.RFC3339), g.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert trusted_gateway: %w", err)
	}
	return nil
}

// FindAll returns all trusted gateways (no auth_value or encrypted secret).
func (r *LinkRepository) FindAll() ([]TrustedGateway, error) {
	rows, err := r.DB.Query(
		`SELECT ` + gwSelectCols + ` FROM trusted_gateways ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query trusted_gateways: %w", err)
	}
	defer rows.Close()
	return scanGwLinkGateways(rows)
}

// FindByID returns a trusted gateway by ID (includes auth_value and encrypted fields).
func (r *LinkRepository) FindByID(id string) (*TrustedGateway, error) {
	var g TrustedGateway
	var createdAt, updatedAt string
	var privateIP, authType, authValue sql.NullString
	var targetNodeID sql.NullString
	var encryptedSecret, secretNonce, secretCreatedAt, secretRotatedAt sql.NullString
	var secretVersion sql.NullInt64
	err := r.DB.QueryRow(
		`SELECT `+gwSelectColsEncrypted+` FROM trusted_gateways WHERE id = ?`, id,
	).Scan(&g.ID, &g.Name, &g.Host, &privateIP, &g.Port,
		&authType, &authValue, &g.GatewayType, &g.AutoRoute, &targetNodeID,
		&encryptedSecret, &secretNonce, &secretVersion, &secretCreatedAt, &secretRotatedAt,
		&g.Status, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query trusted_gateway by id: %w", err)
	}
	g.PrivateIP = privateIP.String
	g.AuthType = authType.String
	g.AuthValue = authValue.String
	g.TargetNodeID = targetNodeID.String
	g.EncryptedSecret = encryptedSecret.String
	g.SecretNonce = secretNonce.String
	if secretVersion.Valid {
		g.SecretVersion = int(secretVersion.Int64)
	}
	g.SecretCreatedAt = secretCreatedAt.String
	g.SecretRotatedAt = secretRotatedAt.String
	g.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	g.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &g, nil
}

// FindByType returns gateways of a specific type (no auth_value or encrypted secret).
func (r *LinkRepository) FindByType(gatewayType string) ([]TrustedGateway, error) {
	rows, err := r.DB.Query(
		`SELECT `+gwSelectCols+` FROM trusted_gateways WHERE gateway_type = ? ORDER BY name`, gatewayType)
	if err != nil {
		return nil, fmt.Errorf("query trusted_gateways by type: %w", err)
	}
	defer rows.Close()
	return scanGwLinkGateways(rows)
}

// FindByTargetNodeID returns gateways targeting a specific node (no auth_value).
func (r *LinkRepository) FindByTargetNodeID(nodeID string) ([]TrustedGateway, error) {
	rows, err := r.DB.Query(
		`SELECT `+gwSelectCols+` FROM trusted_gateways WHERE target_node_id = ? ORDER BY name`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query trusted_gateways by target_node_id: %w", err)
	}
	defer rows.Close()
	return scanGwLinkGateways(rows)
}

// UpdateHost updates the host and private IP of a gateway.
func (r *LinkRepository) UpdateHost(id, host, privateIP string) error {
	_, err := r.DB.Exec(
		`UPDATE trusted_gateways SET host = ?, private_ip = ?, updated_at = ? WHERE id = ?`,
		host, privateIP, time.Now().Format(time.RFC3339), id)
	return err
}

// UpdateStatus updates the status of a gateway.
func (r *LinkRepository) UpdateStatus(id, status string) error {
	_, err := r.DB.Exec(
		`UPDATE trusted_gateways SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().Format(time.RFC3339), id)
	return err
}

// RotateSecret updates the auth secret (legacy HMAC path).
func (r *LinkRepository) RotateSecret(id, newAuthValue string) error {
	_, err := r.DB.Exec(
		`UPDATE trusted_gateways SET auth_value = ?, updated_at = ? WHERE id = ?`,
		newAuthValue, time.Now().Format(time.RFC3339), id)
	return err
}

// RotateSecretEncrypted rotates the secret with encrypted storage (v1.8B-5).
func (r *LinkRepository) RotateSecretEncrypted(id, encryptedSecret, secretNonce string, secretVersion int, secretRotatedAt string) error {
	_, err := r.DB.Exec(
		`UPDATE trusted_gateways SET encrypted_secret = ?, secret_nonce = ?, secret_version = ?,
		 secret_rotated_at = ?, updated_at = ? WHERE id = ?`,
		encryptedSecret, secretNonce, secretVersion, secretRotatedAt,
		time.Now().Format(time.RFC3339), id)
	return err
}

// Delete removes a trusted gateway.
func (r *LinkRepository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM trusted_gateways WHERE id = ?`, id)
	return err
}

func scanGwLinkGateways(rows *sql.Rows) ([]TrustedGateway, error) {
	var gateways []TrustedGateway
	for rows.Next() {
		var g TrustedGateway
		var createdAt, updatedAt string
		var privateIP, authType sql.NullString
		var targetNodeID sql.NullString
		err := rows.Scan(&g.ID, &g.Name, &g.Host, &privateIP, &g.Port,
			&authType, &g.GatewayType, &g.AutoRoute, &targetNodeID, &g.Status,
			&createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan trusted_gateway: %w", err)
		}
		g.PrivateIP = privateIP.String
		g.AuthType = authType.String
		g.TargetNodeID = targetNodeID.String
		g.AuthValue = "" // never returned in list queries
		g.EncryptedSecret = ""
		g.SecretNonce = ""
		g.SecretVersion = 0
		g.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		g.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		gateways = append(gateways, g)
	}
	return gateways, rows.Err()
}
