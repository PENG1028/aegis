package space

import (
	"fmt"
	"time"

	"aegis/internal/core"
)

// Space is a logical isolation unit (tenant-like).
type Space struct {
	ID        string    `json:"id"`
	SpaceID   string    `json:"space_id"`
	Name      string    `json:"name"`
	Quotas    Quota     `json:"quotas"`
	Status    string    `json:"status"` // active | disabled
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Quota defines resource limits for a space.
type Quota struct {
	MaxRoutes         int `json:"max_routes"`
	MaxEdgeRules      int `json:"max_edge_rules"`
	MaxServices       int `json:"max_services"`
	MaxApplyPerMinute int `json:"max_apply_per_minute"`
}

// DefaultQuota returns safe defaults.
func DefaultQuota() Quota {
	return Quota{
		MaxRoutes:         50,
		MaxEdgeRules:      50,
		MaxServices:       20,
		MaxApplyPerMinute: 10,
	}
}

// NewSpace creates a new space with defaults.
func NewSpace(name string) *Space {
	now := time.Now()
	return &Space{
		ID:        core.NewID("space"),
		SpaceID:   fmt.Sprintf("space_%s", name),
		Name:      name,
		Quotas:    DefaultQuota(),
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsExceeded checks if any quota limit is exceeded given current counts.
func (q *Quota) IsExceeded(routes, edgeRules, services int) (bool, string) {
	if routes > q.MaxRoutes {
		return true, fmt.Sprintf("QUOTA_EXCEEDED: routes %d/%d", routes, q.MaxRoutes)
	}
	if edgeRules > q.MaxEdgeRules {
		return true, fmt.Sprintf("QUOTA_EXCEEDED: edge_rules %d/%d", edgeRules, q.MaxEdgeRules)
	}
	if services > q.MaxServices {
		return true, fmt.Sprintf("QUOTA_EXCEEDED: services %d/%d", services, q.MaxServices)
	}
	return false, ""
}
