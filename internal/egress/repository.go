package egress

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides CRUD for EgressRule.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new egress rule repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new rule.
func (r *Repository) Create(rule *EgressRule) error {
	_, err := r.DB.Exec(
		`INSERT INTO egress_rules (id, type, match_type, match_value, priority, status, note, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.Type, rule.MatchType, rule.MatchValue,
		rule.Priority, rule.Status, rule.Note,
		rule.CreatedAt.Format(time.RFC3339),
		rule.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("egress: create rule: %w", err)
	}
	return nil
}

// FindAll returns all rules, ordered by priority.
func (r *Repository) FindAll() ([]EgressRule, error) {
	rows, err := r.DB.Query(
		`SELECT id, type, match_type, match_value, priority, status, note, created_at, updated_at
		 FROM egress_rules ORDER BY priority ASC`)
	if err != nil {
		return nil, fmt.Errorf("egress: list rules: %w", err)
	}
	defer rows.Close()
	return scanRules(rows)
}

// FindActive returns only active rules.
func (r *Repository) FindActive() ([]EgressRule, error) {
	rows, err := r.DB.Query(
		`SELECT id, type, match_type, match_value, priority, status, note, created_at, updated_at
		 FROM egress_rules WHERE status='active' ORDER BY priority ASC`)
	if err != nil {
		return nil, fmt.Errorf("egress: list active rules: %w", err)
	}
	defer rows.Close()
	return scanRules(rows)
}

// FindByID returns a single rule.
func (r *Repository) FindByID(id string) (*EgressRule, error) {
	row := r.DB.QueryRow(
		`SELECT id, type, match_type, match_value, priority, status, note, created_at, updated_at
		 FROM egress_rules WHERE id=?`, id)
	return scanRule(row)
}

// Update updates a rule.
func (r *Repository) Update(rule *EgressRule) error {
	_, err := r.DB.Exec(
		`UPDATE egress_rules SET type=?, match_type=?, match_value=?, priority=?, status=?, note=?, updated_at=? WHERE id=?`,
		rule.Type, rule.MatchType, rule.MatchValue, rule.Priority, rule.Status, rule.Note,
		rule.UpdatedAt.Format(time.RFC3339), rule.ID)
	if err != nil {
		return fmt.Errorf("egress: update rule: %w", err)
	}
	return nil
}

// Delete removes a rule.
func (r *Repository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM egress_rules WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("egress: delete rule: %w", err)
	}
	return nil
}

// ─── Scanners ───

func scanRule(row *sql.Row) (*EgressRule, error) {
	var r EgressRule
	var createdAt, updatedAt string
	var note sql.NullString
	err := row.Scan(&r.ID, &r.Type, &r.MatchType, &r.MatchValue,
		&r.Priority, &r.Status, &note, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	r.Note = note.String
	r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	r.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &r, nil
}

func scanRules(rows *sql.Rows) ([]EgressRule, error) {
	var out []EgressRule
	for rows.Next() {
		var r EgressRule
		var createdAt, updatedAt string
		var note sql.NullString
		if err := rows.Scan(&r.ID, &r.Type, &r.MatchType, &r.MatchValue,
			&r.Priority, &r.Status, &note, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("egress: scan rule: %w", err)
		}
		r.Note = note.String
		r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		r.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		out = append(out, r)
	}
	return out, rows.Err()
}
