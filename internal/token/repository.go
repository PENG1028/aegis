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
		`INSERT INTO api_tokens (id, name, token_hash, scopes, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.TokenHash, scopes, t.Status,
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
		`SELECT id, name, token_hash, scopes, status, created_at, updated_at
		 FROM api_tokens WHERE token_hash = ? AND status = 'active'`, hash,
	).Scan(&t.ID, &t.Name, &t.TokenHash, &scopesStr, &t.Status, &createdAt, &updatedAt)
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
