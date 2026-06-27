package nodeauth

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Repository provides database access for node authentication data.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new nodeauth repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// ============================================================================
// Join Tokens
// ============================================================================

// CreateJoinToken inserts a new join token.
func (r *Repository) CreateJoinToken(t *JoinToken) error {
	rolesJSON := "[]"
	if len(t.AllowedRoles) > 0 {
		b, _ := json.Marshal(t.AllowedRoles)
		rolesJSON = string(b)
	}
	_, err := r.DB.Exec(
		`INSERT INTO node_join_tokens (id, token_hash, name, allowed_roles, expected_node_name,
		 allowed_source_cidr, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.TokenHash, t.Name, rolesJSON, t.ExpectedNodeName,
		t.AllowedSourceCIDR, t.ExpiresAt.Format(time.RFC3339), t.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert join token: %w", err)
	}
	return nil
}

// FindJoinTokenByHash finds a join token by its SHA-256 hash.
func (r *Repository) FindJoinTokenByHash(hash string) (*JoinToken, error) {
	var t JoinToken
	var expiresAt, createdAt, usedAt, revokedAt string
	var rolesJSON string

	err := r.DB.QueryRow(
		`SELECT id, token_hash, name, allowed_roles, expected_node_name, allowed_source_cidr,
		 expires_at, used_at, used_by_node_id, revoked_at, created_at
		 FROM node_join_tokens WHERE token_hash = ?`, hash,
	).Scan(&t.ID, &t.TokenHash, &t.Name, &rolesJSON, &t.ExpectedNodeName,
		&t.AllowedSourceCIDR, &expiresAt, &usedAt, &t.UsedByNodeID, &revokedAt, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query join token: %w", err)
	}

	t.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if usedAt != "" {
		t.UsedAt, _ = time.Parse(time.RFC3339, usedAt)
	}
	if revokedAt != "" {
		t.RevokedAt, _ = time.Parse(time.RFC3339, revokedAt)
	}
	if rolesJSON != "" && rolesJSON != "[]" {
		json.Unmarshal([]byte(rolesJSON), &t.AllowedRoles)
	}

	return &t, nil
}

// FindJoinTokenByID finds a join token by its DB ID.
func (r *Repository) FindJoinTokenByID(id string) (*JoinToken, error) {
	var t JoinToken
	var expiresAt, createdAt, usedAt, revokedAt string
	var rolesJSON string

	err := r.DB.QueryRow(
		`SELECT id, token_hash, name, allowed_roles, expected_node_name, allowed_source_cidr,
		 expires_at, used_at, used_by_node_id, revoked_at, created_at
		 FROM node_join_tokens WHERE id = ?`, id,
	).Scan(&t.ID, &t.TokenHash, &t.Name, &rolesJSON, &t.ExpectedNodeName,
		&t.AllowedSourceCIDR, &expiresAt, &usedAt, &t.UsedByNodeID, &revokedAt, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query join token by id: %w", err)
	}

	t.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if usedAt != "" {
		t.UsedAt, _ = time.Parse(time.RFC3339, usedAt)
	}
	if revokedAt != "" {
		t.RevokedAt, _ = time.Parse(time.RFC3339, revokedAt)
	}
	if rolesJSON != "" && rolesJSON != "[]" {
		json.Unmarshal([]byte(rolesJSON), &t.AllowedRoles)
	}

	return &t, nil
}

// ListJoinTokens returns all join tokens (without raw token values).
func (r *Repository) ListJoinTokens() ([]JoinToken, error) {
	rows, err := r.DB.Query(
		`SELECT id, token_hash, name, allowed_roles, expected_node_name, allowed_source_cidr,
		 expires_at, used_at, used_by_node_id, revoked_at, created_at
		 FROM node_join_tokens ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []JoinToken
	for rows.Next() {
		var t JoinToken
		var expiresAt, createdAt, usedAt, revokedAt string
		var rolesJSON string
		if err := rows.Scan(&t.ID, &t.TokenHash, &t.Name, &rolesJSON, &t.ExpectedNodeName,
			&t.AllowedSourceCIDR, &expiresAt, &usedAt, &t.UsedByNodeID, &revokedAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan join token: %w", err)
		}
		t.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if usedAt != "" {
			t.UsedAt, _ = time.Parse(time.RFC3339, usedAt)
		}
		if revokedAt != "" {
			t.RevokedAt, _ = time.Parse(time.RFC3339, revokedAt)
		}
		if rolesJSON != "" && rolesJSON != "[]" {
			json.Unmarshal([]byte(rolesJSON), &t.AllowedRoles)
		}
		tokens = append(tokens, t)
	}
	if tokens == nil {
		tokens = []JoinToken{}
	}
	return tokens, rows.Err()
}

// MarkJoinTokenUsed marks a join token as used by a node.
func (r *Repository) MarkJoinTokenUsed(id, nodeID string, now time.Time) error {
	nowStr := now.Format(time.RFC3339)
	_, err := r.DB.Exec(
		`UPDATE node_join_tokens SET used_at=?, used_by_node_id=? WHERE id=? AND used_at=''`,
		nowStr, nodeID, id,
	)
	return err
}

// RevokeJoinToken revokes a join token before its use.
func (r *Repository) RevokeJoinToken(id string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.DB.Exec(
		`UPDATE node_join_tokens SET revoked_at=? WHERE id=? AND used_at=''`,
		now, id,
	)
	return err
}

// ============================================================================
// Node Credentials
// ============================================================================

// CreateNodeCredential inserts a new node credential.
func (r *Repository) CreateNodeCredential(c *NodeCredential) error {
	_, err := r.DB.Exec(
		`INSERT INTO node_credentials (id, node_id, token_hash, created_at)
		 VALUES (?, ?, ?, ?)`,
		c.ID, c.NodeID, c.TokenHash, c.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert node credential: %w", err)
	}
	return nil
}

// FindNodeCredentialByNodeID finds the active credential for a node.
func (r *Repository) FindNodeCredentialByNodeID(nodeID string) (*NodeCredential, error) {
	var c NodeCredential
	var createdAt, lastUsedAt, revokedAt string

	err := r.DB.QueryRow(
		`SELECT id, node_id, token_hash, created_at, last_used_at, revoked_at
		 FROM node_credentials WHERE node_id = ? AND revoked_at = ''
		 ORDER BY created_at DESC LIMIT 1`, nodeID,
	).Scan(&c.ID, &c.NodeID, &c.TokenHash, &createdAt, &lastUsedAt, &revokedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query node credential: %w", err)
	}

	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if lastUsedAt != "" {
		c.LastUsedAt, _ = time.Parse(time.RFC3339, lastUsedAt)
	}
	if revokedAt != "" {
		c.RevokedAt, _ = time.Parse(time.RFC3339, revokedAt)
	}

	return &c, nil
}

// FindNodeCredentialByTokenHash finds a credential by its SHA-256 hash.
func (r *Repository) FindNodeCredentialByTokenHash(hash string) (*NodeCredential, error) {
	var c NodeCredential
	var createdAt, lastUsedAt, revokedAt string

	err := r.DB.QueryRow(
		`SELECT id, node_id, token_hash, created_at, last_used_at, revoked_at
		 FROM node_credentials WHERE token_hash = ? AND revoked_at = ''
		 LIMIT 1`, hash,
	).Scan(&c.ID, &c.NodeID, &c.TokenHash, &createdAt, &lastUsedAt, &revokedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query credential by hash: %w", err)
	}

	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if lastUsedAt != "" {
		c.LastUsedAt, _ = time.Parse(time.RFC3339, lastUsedAt)
	}
	if revokedAt != "" {
		c.RevokedAt, _ = time.Parse(time.RFC3339, revokedAt)
	}

	return &c, nil
}

// UpdateNodeCredentialLastUsed updates the last_used_at timestamp.
func (r *Repository) UpdateNodeCredentialLastUsed(id string, now time.Time) error {
	_, err := r.DB.Exec(
		`UPDATE node_credentials SET last_used_at=? WHERE id=?`,
		now.Format(time.RFC3339), id,
	)
	return err
}

// RevokeNodeCredential revokes a node's credential.
func (r *Repository) RevokeNodeCredential(id string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.DB.Exec(
		`UPDATE node_credentials SET revoked_at=? WHERE id=?`,
		now, id,
	)
	return err
}

// RevokeNodeCredentialsByNodeID revokes all credentials for a node.
func (r *Repository) RevokeNodeCredentialsByNodeID(nodeID string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.DB.Exec(
		`UPDATE node_credentials SET revoked_at=? WHERE node_id=? AND revoked_at=''`,
		now, nodeID,
	)
	return err
}
