package token

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Repository provides database access for API tokens.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new token repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new API token.
func (r *Repository) Create(t *APIToken) error {
	scopes := strings.Join(t.Scopes, ",")
	_, err := r.DB.Exec(
		`INSERT INTO api_tokens (id, name, token_hash, space_id, token_type, scopes, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.TokenHash, t.SpaceID, t.TokenType, scopes, t.Status,
		t.CreatedAt.Format(time.RFC3339),
		t.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert api_token: %w", err)
	}
	return nil
}

// FindByTokenHash returns an API token by its hash.
func (r *Repository) FindByTokenHash(hash string) (*APIToken, error) {
	var t APIToken
	var createdAt, updatedAt, scopesStr string
	err := r.DB.QueryRow(
		`SELECT id, name, token_hash, space_id, token_type, scopes, status, created_at, updated_at
		 FROM api_tokens WHERE token_hash = ? AND status = 'active'`, hash,
	).Scan(&t.ID, &t.Name, &t.TokenHash, &t.SpaceID, &t.TokenType, &scopesStr, &t.Status, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query api_token by hash: %w", err)
	}
	if scopesStr != "" {
		t.Scopes = strings.Split(scopesStr, ",")
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &t, nil
}

// FindBySpaceID returns all tokens for a space.
func (r *Repository) FindBySpaceID(spaceID string) ([]*APIToken, error) {
	rows, err := r.DB.Query(
		`SELECT id, name, token_hash, space_id, token_type, scopes, status, created_at, updated_at
		 FROM api_tokens WHERE space_id = ? ORDER BY name`, spaceID)
	if err != nil {
		return nil, fmt.Errorf("query api_tokens by space_id: %w", err)
	}
	defer rows.Close()
	return scanTokens(rows)
}

// FindAll returns all tokens.
func (r *Repository) FindAll() ([]*APIToken, error) {
	rows, err := r.DB.Query(
		`SELECT id, name, token_hash, space_id, token_type, scopes, status, created_at, updated_at
		 FROM api_tokens ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query api_tokens: %w", err)
	}
	defer rows.Close()
	return scanTokens(rows)
}

func scanTokens(rows *sql.Rows) ([]*APIToken, error) {
	var tokens []*APIToken
	for rows.Next() {
		var t APIToken
		var createdAt, updatedAt, scopesStr string
		if err := rows.Scan(&t.ID, &t.Name, &t.TokenHash, &t.SpaceID, &t.TokenType, &scopesStr, &t.Status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan api_token: %w", err)
		}
		if scopesStr != "" {
			t.Scopes = strings.Split(scopesStr, ",")
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		tokens = append(tokens, &t)
	}
	return tokens, rows.Err()
}

// FindByID returns a token by primary key.
func (r *Repository) FindByID(id string) (*APIToken, error) {
	var t APIToken
	var createdAt, updatedAt, scopesStr string
	err := r.DB.QueryRow(
		`SELECT id, name, token_hash, space_id, token_type, scopes, status, created_at, updated_at
		 FROM api_tokens WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.TokenHash, &t.SpaceID, &t.TokenType, &scopesStr, &t.Status, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query api_token by id: %w", err)
	}
	if scopesStr != "" {
		t.Scopes = strings.Split(scopesStr, ",")
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &t, nil
}

// Update updates an API token.
func (r *Repository) Update(t *APIToken) error {
	scopes := strings.Join(t.Scopes, ",")
	_, err := r.DB.Exec(
		`UPDATE api_tokens SET name=?, space_id=?, token_type=?, scopes=?, status=?, updated_at=? WHERE id=?`,
		t.Name, t.SpaceID, t.TokenType, scopes, t.Status,
		t.UpdatedAt.Format(time.RFC3339), t.ID,
	)
	if err != nil {
		return fmt.Errorf("update api_token: %w", err)
	}
	return nil
}

// UpdateLastUsed updates the updated_at timestamp for a token.
func (r *Repository) UpdateLastUsed(id string) error {
	_, err := r.DB.Exec(
		`UPDATE api_tokens SET updated_at=? WHERE id=?`,
		time.Now().Format(time.RFC3339), id,
	)
	return err
}

// Delete removes an API token.
func (r *Repository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM api_tokens WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete api_token: %w", err)
	}
	return nil
}
