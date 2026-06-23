package service

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for services.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new service repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new service.
func (r *Repository) Create(s *Service) error {
	_, err := r.DB.Exec(
		`INSERT INTO services (id, project_id, name, kind, env, status, note, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ProjectID, s.Name, s.Kind, s.Env, s.Status, s.Note,
		s.CreatedAt.Format(time.RFC3339),
		s.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert service: %w", err)
	}
	return nil
}

// FindAll returns all services ordered by name.
func (r *Repository) FindAll() ([]Service, error) {
	rows, err := r.DB.Query(
		`SELECT id, project_id, name, kind, env, status, note, created_at, updated_at
		 FROM services ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query services: %w", err)
	}
	defer rows.Close()
	return scanServices(rows)
}

// FindByID returns a service by ID.
func (r *Repository) FindByID(id string) (*Service, error) {
	var s Service
	var createdAt, updatedAt string
	var note sql.NullString
	err := r.DB.QueryRow(
		`SELECT id, project_id, name, kind, env, status, note, created_at, updated_at
		 FROM services WHERE id = ?`, id,
	).Scan(&s.ID, &s.ProjectID, &s.Name, &s.Kind, &s.Env, &s.Status, &note, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query service by id: %w", err)
	}
	s.Note = note.String
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &s, nil
}

// FindByName returns a service by name.
func (r *Repository) FindByName(name string) (*Service, error) {
	var s Service
	var createdAt, updatedAt string
	var note sql.NullString
	err := r.DB.QueryRow(
		`SELECT id, project_id, name, kind, env, status, note, created_at, updated_at
		 FROM services WHERE name = ?`, name,
	).Scan(&s.ID, &s.ProjectID, &s.Name, &s.Kind, &s.Env, &s.Status, &note, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query service by name: %w", err)
	}
	s.Note = note.String
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &s, nil
}

// FindByProjectID returns all services for a project.
func (r *Repository) FindByProjectID(projectID string) ([]Service, error) {
	rows, err := r.DB.Query(
		`SELECT id, project_id, name, kind, env, status, note, created_at, updated_at
		 FROM services WHERE project_id = ? ORDER BY name`, projectID)
	if err != nil {
		return nil, fmt.Errorf("query services by project: %w", err)
	}
	defer rows.Close()
	return scanServices(rows)
}

// Update updates a service.
func (r *Repository) Update(s *Service) error {
	_, err := r.DB.Exec(
		`UPDATE services SET project_id=?, name=?, kind=?, env=?, status=?, note=?, updated_at=? WHERE id=?`,
		s.ProjectID, s.Name, s.Kind, s.Env, s.Status, s.Note,
		s.UpdatedAt.Format(time.RFC3339), s.ID,
	)
	if err != nil {
		return fmt.Errorf("update service: %w", err)
	}
	return nil
}

// FindActive returns all active services.
func (r *Repository) FindActive() ([]Service, error) {
	rows, err := r.DB.Query(
		`SELECT id, project_id, name, kind, env, status, note, created_at, updated_at
		 FROM services WHERE status = 'active' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query active services: %w", err)
	}
	defer rows.Close()
	return scanServices(rows)
}

func scanServices(rows *sql.Rows) ([]Service, error) {
	var services []Service
	for rows.Next() {
		var s Service
		var createdAt, updatedAt string
		var note sql.NullString
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Name, &s.Kind, &s.Env, &s.Status, &note, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		s.Note = note.String
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		services = append(services, s)
	}
	return services, rows.Err()
}
