package token

import "time"

// APIToken represents a Bearer token for API authentication.
type APIToken struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	TokenHash string    `json:"-"`
	Scopes    []string  `json:"scopes"`
	Status    string    `json:"status"` // active | disabled
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
