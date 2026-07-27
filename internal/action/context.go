package action

import "context"

// ActionContext carries the caller's identity and permissions through the action pipeline.
type ActionContext struct {
	SpaceID   string `json:"space_id"`
	TokenType string `json:"token_type"` // admin | service | space
	TokenID   string `json:"token_id"`
	Actor     string `json:"actor"` // cli | api
}

// IsAdmin returns true if the caller has admin privileges.
func (ac *ActionContext) IsAdmin() bool {
	return ac.TokenType == "admin"
}

// IsSpace returns true if the caller is scoped to a space.
func (ac *ActionContext) IsSpace() bool {
	return ac.TokenType == "space"
}

// IsService returns true if the caller is a registered service (via ServiceAuth ticket).
func (ac *ActionContext) IsService() bool {
	return ac.TokenType == "service"
}

// NewAdminContext creates an admin context for CLI/internal operations.
func NewAdminContext() *ActionContext {
	return &ActionContext{
		SpaceID:   "",
		TokenType: "admin",
		Actor:     "cli",
	}
}

// NewSpaceContext creates a space-scoped context.
func NewSpaceContext(spaceID, tokenID string) *ActionContext {
	return &ActionContext{
		SpaceID:   spaceID,
		TokenType: "space",
		TokenID:   tokenID,
		Actor:     "api",
	}
}

type ctxKey struct{}

// WithActionContext embeds an ActionContext into a context.Context.
func WithActionContext(ctx context.Context, ac *ActionContext) context.Context {
	return context.WithValue(ctx, ctxKey{}, ac)
}

// GetActionContext extracts the ActionContext from a context.Context.
// Returns nil if no context is set.
func GetActionContext(ctx context.Context) *ActionContext {
	ac, _ := ctx.Value(ctxKey{}).(*ActionContext)
	return ac
}
