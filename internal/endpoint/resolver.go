package endpoint

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// Resolver resolves which endpoint to use for a service.
// Fixed priority: local -> private -> public -> fail
type Resolver struct {
	repo *Repository
}

// NewResolver creates a new endpoint resolver.
func NewResolver(repo *Repository) *Resolver {
	return &Resolver{repo: repo}
}

// Resolve finds the first available endpoint for a service.
// Tries endpoints in priority order (local > private > public).
// Performs a TCP connect check on each endpoint.
// Returns the first connectable endpoint, or an error if none are available.
func (r *Resolver) Resolve(ctx context.Context, serviceID string) (*Endpoint, error) {
	endpoints, err := r.repo.FindEnabledByServiceID(serviceID)
	if err != nil {
		return nil, fmt.Errorf("find endpoints for service %s: %w", serviceID, err)
	}

	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no enabled endpoints for service %s", serviceID)
	}

	// Try each endpoint in priority order
	for i := range endpoints {
		ep := &endpoints[i]
		if r.isReachable(ep.Address) {
			return ep, nil
		}
	}

	return nil, fmt.Errorf("no available endpoint for service %s (tried %d endpoints)", serviceID, len(endpoints))
}

// isReachable performs a TCP connect check with 2s timeout.
func (r *Resolver) isReachable(address string) bool {
	host, port, err := parseHostPort(address)
	if err != nil {
		return false
	}

	target := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", target, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// parseHostPort extracts host and port from an address string.
func parseHostPort(addr string) (host string, port string, err error) {
	cleaned := addr
	if len(cleaned) > 7 && strings.HasPrefix(cleaned, "http://") {
		cleaned = cleaned[7:]
	} else if len(cleaned) > 8 && strings.HasPrefix(cleaned, "https://") {
		cleaned = cleaned[8:]
	}

	h, p, e := net.SplitHostPort(cleaned)
	if e != nil {
		// No port; assume defaults
		if strings.HasPrefix(addr, "https://") {
			return cleaned, "443", nil
		}
		return cleaned, "80", nil
	}
	return h, p, nil
}
