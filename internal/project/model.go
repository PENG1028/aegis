package project

import "time"

// Project represents a project that groups related services.
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // active | archived
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateProjectInput is the input for creating a project.
type CreateProjectInput struct {
	Name        string
	Description string
}
