package service

import "time"

// Service represents a backend service managed by Aegis.
// Upstream addresses are managed by Endpoints, not stored directly on Service.
type Service struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"project_id"`
	Name             string    `json:"name"`
	Kind             string    `json:"kind"` // http | tcp | file
	Env              string    `json:"env"`  // dev | preview | prod
	Status           string    `json:"status"` // active | disabled | error
	Note             string    `json:"note"`
	SpaceID          string    `json:"space_id"`
	OwnerType        string    `json:"owner_type"`         // space | admin
	OwnerID          string    `json:"owner_id"`           // space_id when owner_type=space
	CreatedByTokenID string    `json:"created_by_token_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreateServiceInput is the input for creating a service.
type CreateServiceInput struct {
	ProjectID   string
	ProjectName string
	Name        string
	Kind        string
	Env         string
}

// UpdateServiceInput is the input for updating a service.
type UpdateServiceInput struct {
	Kind *string
	Env  *string
	Note *string
}
