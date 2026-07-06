package certstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Repository handles database operations for certificates.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a certificate repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new certificate record.
func (r *Repository) Create(cert *Certificate) error {
	_, err := r.db.Exec(
		`INSERT INTO certificates (id, domains, issuer, not_before, not_after, cert_path, key_path, note, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cert.ID, cert.Domains, cert.Issuer, cert.NotBefore, cert.NotAfter,
		cert.CertPath, cert.KeyPath, cert.Note,
		cert.CreatedAt.Format(time.RFC3339), cert.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

// FindAll returns all certificates ordered by creation time descending.
func (r *Repository) FindAll() ([]Certificate, error) {
	rows, err := r.db.Query(
		`SELECT id, domains, issuer, not_before, not_after, cert_path, key_path, note, created_at, updated_at
		 FROM certificates ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []Certificate
	for rows.Next() {
		var c Certificate
		var ca, ua string
		if err := rows.Scan(&c.ID, &c.Domains, &c.Issuer, &c.NotBefore, &c.NotAfter,
			&c.CertPath, &c.KeyPath, &c.Note, &ca, &ua); err != nil {
			return nil, fmt.Errorf("scan certificate: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		c.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		certs = append(certs, c)
	}
	if certs == nil {
		certs = []Certificate{}
	}
	return certs, rows.Err()
}

// FindByID returns a single certificate by ID, or nil.
func (r *Repository) FindByID(id string) (*Certificate, error) {
	var c Certificate
	var ca, ua string
	err := r.db.QueryRow(
		`SELECT id, domains, issuer, not_before, not_after, cert_path, key_path, note, created_at, updated_at
		 FROM certificates WHERE id = ?`, id,
	).Scan(&c.ID, &c.Domains, &c.Issuer, &c.NotBefore, &c.NotAfter,
		&c.CertPath, &c.KeyPath, &c.Note, &ca, &ua)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &c, nil
}

// FindByDomain returns all certificates covering the given domain.
func (r *Repository) FindByDomain(domain string) ([]Certificate, error) {
	all, err := r.FindAll()
	if err != nil {
		return nil, err
	}
	var matching []Certificate
	for _, c := range all {
		var domains []string
		if err := json.Unmarshal([]byte(c.Domains), &domains); err != nil {
			continue
		}
		for _, d := range domains {
			if matchDomain(d, domain) {
				matching = append(matching, c)
				break
			}
		}
	}
	return matching, nil
}

// Delete removes a certificate record by ID.
func (r *Repository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM certificates WHERE id = ?`, id)
	return err
}

// matchDomain checks if pattern (which may start with "*.") covers domain.
func matchDomain(pattern, domain string) bool {
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return strings.HasSuffix(domain, suffix)
	}
	return pattern == domain
}
