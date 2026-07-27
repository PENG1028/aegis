// Package egress provides allow/block rule management for outbound traffic.
// Rules determine which domains/IPs should bypass the gateway (allow)
// or be blocked entirely (block).
package egress

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Rule type constants.
const (
	TypeAllow = "allow"
	TypeBlock = "block"
)

// Match type constants.
const (
	MatchDomain = "domain" // exact domain or *-wildcard suffix
	MatchIP     = "ip"
	MatchCIDR   = "cidr"
)

// EgressRule defines one allow/block rule for the egress gateway.
type EgressRule struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`        // "allow" | "block"
	MatchType  string    `json:"match_type"`  // "domain" | "ip" | "cidr"
	MatchValue string    `json:"match_value"` // e.g. "github.com", "10.0.0.0/8"
	Priority   int       `json:"priority"`    // 0=highest
	Status     string    `json:"status"`      // "active" | "disabled"
	Note       string    `json:"note,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Validate checks the rule for basic correctness.
func (r *EgressRule) Validate() error {
	if r.Type != TypeAllow && r.Type != TypeBlock {
		return fmt.Errorf("%w: type must be allow or block", ErrInvalidRule)
	}
	if r.MatchType != MatchDomain && r.MatchType != MatchIP && r.MatchType != MatchCIDR {
		return fmt.Errorf("%w: match_type must be domain, ip, or cidr", ErrInvalidRule)
	}
	if r.MatchValue == "" {
		return fmt.Errorf("%w: match_value is required", ErrInvalidRule)
	}
	if r.MatchType == MatchDomain {
		// Normalize: trim trailing dot, lowercase
		r.MatchValue = strings.TrimSuffix(strings.ToLower(r.MatchValue), ".")
	}
	return nil
}

// MatchesDomain checks whether a domain matches this rule.
// Supports exact match and wildcard suffix (*.example.com).
func (r *EgressRule) MatchesDomain(domain string) bool {
	if r.MatchType != MatchDomain {
		return false
	}
	if r.Status != "active" {
		return false
	}
	d := strings.TrimSuffix(strings.ToLower(domain), ".")
	m := strings.TrimSuffix(r.MatchValue, ".")

	if d == m {
		return true
	}
	// Wildcard: *.example.com → match any subdomain
	if strings.HasPrefix(m, "*.") {
		suffix := m[1:] // ".example.com"
		return strings.HasSuffix(d, suffix)
	}
	return false
}

// Sentinel errors.
var (
	ErrInvalidRule = errors.New("egress: invalid rule")
	ErrNotFound    = errors.New("egress: rule not found")
)
