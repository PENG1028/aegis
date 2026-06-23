package edgemux

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for edge mux rules.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new edge mux rule repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new edge mux rule.
func (r *Repository) Create(rule *Rule) error {
	_, err := r.DB.Exec(
		`INSERT INTO edge_mux_rules (id, sni_host, declared_kind, target_host, target_port, service_id, status, message, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.SNIHost, rule.DeclaredKind, rule.TargetHost, rule.TargetPort,
		rule.ServiceID, rule.Status, rule.Message,
		rule.CreatedAt.Format(time.RFC3339),
		rule.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert edge_mux_rule: %w", err)
	}
	return nil
}

// FindAll returns all rules ordered by sni_host.
func (r *Repository) FindAll() ([]Rule, error) {
	rows, err := r.DB.Query(
		`SELECT id, sni_host, declared_kind, target_host, target_port, service_id, status, message, created_at, updated_at
		 FROM edge_mux_rules ORDER BY sni_host`)
	if err != nil {
		return nil, fmt.Errorf("query edge_mux_rules: %w", err)
	}
	defer rows.Close()
	return scanRules(rows)
}

// FindActive returns all active rules.
func (r *Repository) FindActive() ([]Rule, error) {
	rows, err := r.DB.Query(
		`SELECT id, sni_host, declared_kind, target_host, target_port, service_id, status, message, created_at, updated_at
		 FROM edge_mux_rules WHERE status = 'active' ORDER BY sni_host`)
	if err != nil {
		return nil, fmt.Errorf("query active edge_mux_rules: %w", err)
	}
	defer rows.Close()
	return scanRules(rows)
}

// FindByID returns a rule by ID.
func (r *Repository) FindByID(id string) (*Rule, error) {
	var rule Rule
	var createdAt, updatedAt string
	var serviceID, message sql.NullString
	err := r.DB.QueryRow(
		`SELECT id, sni_host, declared_kind, target_host, target_port, service_id, status, message, created_at, updated_at
		 FROM edge_mux_rules WHERE id = ?`, id,
	).Scan(&rule.ID, &rule.SNIHost, &rule.DeclaredKind, &rule.TargetHost, &rule.TargetPort,
		&serviceID, &rule.Status, &message, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	rule.ServiceID = serviceID.String
	rule.Message = message.String
	rule.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	rule.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &rule, nil
}

// FindBySNIHost returns a rule by SNI hostname.
func (r *Repository) FindBySNIHost(sniHost string) (*Rule, error) {
	var rule Rule
	var createdAt, updatedAt string
	var serviceID, message sql.NullString
	err := r.DB.QueryRow(
		`SELECT id, sni_host, declared_kind, target_host, target_port, service_id, status, message, created_at, updated_at
		 FROM edge_mux_rules WHERE sni_host = ?`, sniHost,
	).Scan(&rule.ID, &rule.SNIHost, &rule.DeclaredKind, &rule.TargetHost, &rule.TargetPort,
		&serviceID, &rule.Status, &message, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	rule.ServiceID = serviceID.String
	rule.Message = message.String
	rule.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	rule.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &rule, nil
}

// Update updates a rule.
func (r *Repository) Update(rule *Rule) error {
	_, err := r.DB.Exec(
		`UPDATE edge_mux_rules SET sni_host=?, declared_kind=?, target_host=?, target_port=?, service_id=?, status=?, message=?, updated_at=? WHERE id=?`,
		rule.SNIHost, rule.DeclaredKind, rule.TargetHost, rule.TargetPort,
		rule.ServiceID, rule.Status, rule.Message,
		rule.UpdatedAt.Format(time.RFC3339), rule.ID,
	)
	if err != nil {
		return fmt.Errorf("update edge_mux_rule: %w", err)
	}
	return nil
}

// Delete removes a rule.
func (r *Repository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM edge_mux_rules WHERE id = ?`, id)
	return err
}

func scanRules(rows *sql.Rows) ([]Rule, error) {
	var rules []Rule
	for rows.Next() {
		var rule Rule
		var createdAt, updatedAt string
	var serviceID, message sql.NullString
		if err := rows.Scan(&rule.ID, &rule.SNIHost, &rule.DeclaredKind, &rule.TargetHost, &rule.TargetPort,
			&serviceID, &rule.Status, &message, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan edge_mux_rule: %w", err)
		}
		rule.ServiceID = serviceID.String
		rule.Message = message.String
		rule.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		rule.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}
