package credential

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for credentials.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new credential repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new credential.
func (r *Repository) Create(c *Credential) error {
	_, err := r.DB.Exec(
		`INSERT INTO credentials (id, alias, encrypted_conn_string, secret_version, secret_created_at, secret_rotated_at, scheme, masked_uri, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Alias, c.EncryptedConnString, c.SecretVersion,
		c.SecretCreatedAt, c.SecretRotatedAt,
		c.Scheme, c.MaskedURI, c.Description,
		c.CreatedAt.Format(time.RFC3339), c.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert credential: %w", err)
	}
	return nil
}

// FindByID returns a credential by ID.
func (r *Repository) FindByID(id string) (*Credential, error) {
	c := &Credential{}
	var createdAt, updatedAt string
	var secretRotatedAt sql.NullString
	err := r.DB.QueryRow(
		`SELECT id, alias, encrypted_conn_string, secret_version, secret_created_at, secret_rotated_at, scheme, masked_uri, description, created_at, updated_at
		 FROM credentials WHERE id = ?`, id,
	).Scan(&c.ID, &c.Alias, &c.EncryptedConnString, &c.SecretVersion,
		&c.SecretCreatedAt, &secretRotatedAt,
		&c.Scheme, &c.MaskedURI, &c.Description,
		&createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find credential: %w", err)
	}
	c.SecretRotatedAt = secretRotatedAt.String
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return c, nil
}

// FindByAlias returns a credential by alias.
func (r *Repository) FindByAlias(alias string) (*Credential, error) {
	c := &Credential{}
	var createdAt, updatedAt string
	var secretRotatedAt sql.NullString
	err := r.DB.QueryRow(
		`SELECT id, alias, encrypted_conn_string, secret_version, secret_created_at, secret_rotated_at, scheme, masked_uri, description, created_at, updated_at
		 FROM credentials WHERE alias = ?`, alias,
	).Scan(&c.ID, &c.Alias, &c.EncryptedConnString, &c.SecretVersion,
		&c.SecretCreatedAt, &secretRotatedAt,
		&c.Scheme, &c.MaskedURI, &c.Description,
		&createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find credential by alias: %w", err)
	}
	c.SecretRotatedAt = secretRotatedAt.String
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return c, nil
}

// FindAll returns all credentials (encrypted fields excluded for display).
func (r *Repository) FindAll() ([]Credential, error) {
	rows, err := r.DB.Query(
		`SELECT id, alias, encrypted_conn_string, secret_version, secret_created_at, secret_rotated_at, scheme, masked_uri, description, created_at, updated_at
		 FROM credentials ORDER BY alias`)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	defer rows.Close()

	var list []Credential
	for rows.Next() {
		var c Credential
		var createdAt, updatedAt string
		var secretRotatedAt sql.NullString
		if err := rows.Scan(&c.ID, &c.Alias, &c.EncryptedConnString, &c.SecretVersion,
			&c.SecretCreatedAt, &secretRotatedAt,
			&c.Scheme, &c.MaskedURI, &c.Description,
			&createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		c.SecretRotatedAt = secretRotatedAt.String
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		list = append(list, c)
	}
	if list == nil {
		list = []Credential{}
	}
	return list, rows.Err()
}

// Update updates a credential (alias, encrypted fields on rotate, description).
func (r *Repository) Update(c *Credential) error {
	_, err := r.DB.Exec(
		`UPDATE credentials SET alias=?, encrypted_conn_string=?, secret_version=?, secret_rotated_at=?, scheme=?, masked_uri=?, description=?, updated_at=? WHERE id=?`,
		c.Alias, c.EncryptedConnString, c.SecretVersion, c.SecretRotatedAt,
		c.Scheme, c.MaskedURI, c.Description,
		c.UpdatedAt.Format(time.RFC3339), c.ID,
	)
	if err != nil {
		return fmt.Errorf("update credential: %w", err)
	}
	return nil
}

// Delete removes a credential.
func (r *Repository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM credentials WHERE id = ?`, id)
	return err
}
