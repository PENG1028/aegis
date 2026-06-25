package token

import "time"

// Token types.
const (
	TokenTypeAdmin = "admin"
	TokenTypeSpace = "space"
)

// APIToken represents a Bearer token for API authentication.
type APIToken struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	TokenHash string    `json:"-"`
	SpaceID   string    `json:"space_id"`
	TokenType string    `json:"token_type"` // admin | space
	Scopes    []string  `json:"scopes"`
	Status    string    `json:"status"` // active | disabled
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
