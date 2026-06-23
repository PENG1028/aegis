package project

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for projects.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new project repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new project.
func (r *Repository) Create(p *Project) error {
	_, err := r.DB.Exec(
		`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.Status,
		p.CreatedAt.Format(time.RFC3339),
		p.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

// FindAll returns all projects ordered by name.
func (r *Repository) FindAll() ([]Project, error) {
	rows, err := r.DB.Query(
		`SELECT id, name, description, status, created_at, updated_at
		 FROM projects ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// FindByID returns a project by ID.
func (r *Repository) FindByID(id string) (*Project, error) {
	var p Project
	var createdAt, updatedAt string
	err := r.DB.QueryRow(
		`SELECT id, name, description, status, created_at, updated_at
		 FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Status, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query project by id: %w", err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &p, nil
}

// FindByName returns a project by name.
func (r *Repository) FindByName(name string) (*Project, error) {
	var p Project
	var createdAt, updatedAt string
	err := r.DB.QueryRow(
		`SELECT id, name, description, status, created_at, updated_at
		 FROM projects WHERE name = ?`, name,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Status, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query project by name: %w", err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &p, nil
}

// Update updates a project.
func (r *Repository) Update(p *Project) error {
	_, err := r.DB.Exec(
		`UPDATE projects SET name=?, description=?, status=?, updated_at=? WHERE id=?`,
		p.Name, p.Description, p.Status,
		p.UpdatedAt.Format(time.RFC3339), p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}
