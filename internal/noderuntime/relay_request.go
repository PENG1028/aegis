package noderuntime

import "fmt"

// RelayRequestPlan describes how to dispatch a request through the relay.
type RelayRequestPlan struct {
	Method     string            `json:"method"`
	GatewayURL string            `json:"gateway_url"`
	Headers    map[string]string `json:"headers"`
	PreserveHost bool            `json:"preserve_host"`
	RouteID    string            `json:"route_id,omitempty"`
	ServiceID  string            `json:"service_id,omitempty"`
	Available  bool              `json:"available"`
	Reason     string            `json:"reason,omitempty"`
}

// RelayPlanBuilder builds relay request plans from routing decisions.
type RelayPlanBuilder struct{}

// NewRelayPlanBuilder creates a new relay plan builder.
func NewRelayPlanBuilder() *RelayPlanBuilder {
	return &RelayPlanBuilder{}
}

// BuildPlan builds a relay request plan from a routing decision.
// This is a dry-run plan builder — it does not execute the request.
func (b *RelayPlanBuilder) BuildPlan(decision *RoutingDecision, originalMethod string) *RelayRequestPlan {
	if decision.Status != "available" || decision.SelectedCandidate == nil {
		return &RelayRequestPlan{
			Method:    originalMethod,
			Available: false,
			Reason:    decision.UnavailableReason,
		}
	}

	candidate := decision.SelectedCandidate

	plan := &RelayRequestPlan{
		Method:      originalMethod,
		GatewayURL:  candidate.GatewayURL + "/__aegis/relay",
		RouteID:     decision.RouteID,
		ServiceID:   decision.ServiceID,
		Available:   true,
		PreserveHost: true,
		Headers: map[string]string{
			"X-Aegis-Route-ID":    decision.RouteID,
			"X-Aegis-Hop":         "1",
		},
	}

	// Add gateway link ID if present
	if candidate.GatewayLinkID != "" && candidate.RequiresGatewayLink {
		plan.Headers["X-Aegis-Gateway-Link-ID"] = candidate.GatewayLinkID
		// Note: raw GatewayLink token is NOT included in the plan.
		// Token injection is deferred to the relay client layer.
	}

	return plan
}

// SafeString returns a log-safe representation of the plan (no token).
func (p *RelayRequestPlan) SafeString() string {
	if !p.Available {
		return fmt.Sprintf("RelayPlan{unavailable: %s}", p.Reason)
	}
	// Show method and URL, not headers (might contain sensitive data)
	return fmt.Sprintf("RelayPlan{%s %s route=%s}", p.Method, p.GatewayURL, p.RouteID)
}
