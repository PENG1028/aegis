package space

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for spaces.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new space repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new space.
func (r *Repository) Create(s *Space) error {
	_, err := r.DB.Exec(
		`INSERT INTO spaces (id, space_id, name, max_routes, max_edge_rules, max_services, max_apply_per_minute, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.SpaceID, s.Name,
		s.Quotas.MaxRoutes, s.Quotas.MaxEdgeRules, s.Quotas.MaxServices, s.Quotas.MaxApplyPerMinute,
		s.Status,
		s.CreatedAt.Format(time.RFC3339),
		s.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert space: %w", err)
	}
	return nil
}

// FindAll returns all spaces.
func (r *Repository) FindAll() ([]*Space, error) {
	rows, err := r.DB.Query(
		`SELECT id, space_id, name, max_routes, max_edge_rules, max_services, max_apply_per_minute, status, created_at, updated_at
		 FROM spaces ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query spaces: %w", err)
	}
	defer rows.Close()
	return scanSpaces(rows)
}

// FindByID returns a space by primary key.
func (r *Repository) FindByID(id string) (*Space, error) {
	var s Space
	var createdAt, updatedAt string
	var maxRoutes, maxEdgeRules, maxServices, maxApplyPerMin int
	err := r.DB.QueryRow(
		`SELECT id, space_id, name, max_routes, max_edge_rules, max_services, max_apply_per_minute, status, created_at, updated_at
		 FROM spaces WHERE id = ?`, id,
	).Scan(&s.ID, &s.SpaceID, &s.Name, &maxRoutes, &maxEdgeRules, &maxServices, &maxApplyPerMin, &s.Status, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query space by id: %w", err)
	}
	s.Quotas.MaxRoutes = maxRoutes
	s.Quotas.MaxEdgeRules = maxEdgeRules
	s.Quotas.MaxServices = maxServices
	s.Quotas.MaxApplyPerMinute = maxApplyPerMin
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &s, nil
}

// FindBySpaceID returns a space by its logical space_id.
func (r *Repository) FindBySpaceID(spaceID string) (*Space, error) {
	var s Space
	var createdAt, updatedAt string
	var maxRoutes, maxEdgeRules, maxServices, maxApplyPerMin int
	err := r.DB.QueryRow(
		`SELECT id, space_id, name, max_routes, max_edge_rules, max_services, max_apply_per_minute, status, created_at, updated_at
		 FROM spaces WHERE space_id = ?`, spaceID,
	).Scan(&s.ID, &s.SpaceID, &s.Name, &maxRoutes, &maxEdgeRules, &maxServices, &maxApplyPerMin, &s.Status, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query space by space_id: %w", err)
	}
	s.Quotas.MaxRoutes = maxRoutes
	s.Quotas.MaxEdgeRules = maxEdgeRules
	s.Quotas.MaxServices = maxServices
	s.Quotas.MaxApplyPerMinute = maxApplyPerMin
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &s, nil
}

// Update updates a space.
func (r *Repository) Update(s *Space) error {
	_, err := r.DB.Exec(
		`UPDATE spaces SET name=?, max_routes=?, max_edge_rules=?, max_services=?, max_apply_per_minute=?, status=?, updated_at=?
		 WHERE id=?`,
		s.Name, s.Quotas.MaxRoutes, s.Quotas.MaxEdgeRules, s.Quotas.MaxServices, s.Quotas.MaxApplyPerMinute,
		s.Status, s.UpdatedAt.Format(time.RFC3339), s.ID,
	)
	if err != nil {
		return fmt.Errorf("update space: %w", err)
	}
	return nil
}

// Delete removes a space.
func (r *Repository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM spaces WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete space: %w", err)
	}
	return nil
}

func scanSpaces(rows *sql.Rows) ([]*Space, error) {
	var spaces []*Space
	for rows.Next() {
		var s Space
		var createdAt, updatedAt string
		var maxRoutes, maxEdgeRules, maxServices, maxApplyPerMin int
		if err := rows.Scan(&s.ID, &s.SpaceID, &s.Name, &maxRoutes, &maxEdgeRules, &maxServices, &maxApplyPerMin, &s.Status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan space: %w", err)
		}
		s.Quotas.MaxRoutes = maxRoutes
		s.Quotas.MaxEdgeRules = maxEdgeRules
		s.Quotas.MaxServices = maxServices
		s.Quotas.MaxApplyPerMinute = maxApplyPerMin
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		spaces = append(spaces, &s)
	}
	return spaces, rows.Err()
}
