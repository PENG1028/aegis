package endpoint

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// ResolveResult holds the result of an endpoint resolution.
type ResolveResult struct {
	Endpoint *Endpoint          `json:"endpoint"`
	Attempts []EndpointAttempt  `json:"attempts"`
}

// EndpointAttempt records a single endpoint connection attempt.
type EndpointAttempt struct {
	EndpointID string `json:"endpoint_id"`
	Type       string `json:"type"`
	Address    string `json:"address"`
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	LatencyMS  int64  `json:"latency_ms"`
}

// Resolver resolves which endpoint to use for a service.
type Resolver struct {
	repo *Repository
}

// NewResolver creates a new endpoint resolver.
func NewResolver(repo *Repository) *Resolver {
	return &Resolver{repo: repo}
}

// Resolve finds the first available endpoint (legacy API).
func (r *Resolver) Resolve(ctx context.Context, serviceID string) (*Endpoint, error) {
	result := r.ResolveWithResult(ctx, serviceID)
	if result.Endpoint == nil {
		return nil, fmt.Errorf("no available endpoint for service %s (tried %d endpoints)", serviceID, len(result.Attempts))
	}
	return result.Endpoint, nil
}

// ResolveWithResult performs endpoint resolution and returns detailed results.
// Priority: local → private → public → fail
func (r *Resolver) ResolveWithResult(ctx context.Context, serviceID string) *ResolveResult {
	result := &ResolveResult{}

	endpoints, err := r.repo.FindEnabledByServiceID(serviceID)
	if err != nil {
		return result
	}

	for i := range endpoints {
		ep := &endpoints[i]
		addr := NormalizeAddress(ep.Address)
		start := time.Now()

		reachable, msg := r.checkTCP(addr)
		latency := time.Since(start).Milliseconds()

		attempt := EndpointAttempt{
			EndpointID: ep.ID,
			Type:       ep.Type,
			Address:    addr,
			Success:    reachable,
			Message:    msg,
			LatencyMS:  latency,
		}
		result.Attempts = append(result.Attempts, attempt)

		if reachable {
			// Return a copy with normalized address
			resolved := *ep
			resolved.Address = addr
			result.Endpoint = &resolved
			return result
		}
	}

	return result
}

// checkTCP performs a TCP connect check with 2s timeout.
// For Unix socket addresses, the check is skipped (not applicable).
func (r *Resolver) checkTCP(address string) (bool, string) {
	host, port, err := parseHostPort(address)
	if err != nil {
		return false, fmt.Sprintf("invalid address: %v", err)
	}

	// Unix socket paths — skip TCP check (not applicable)
	if strings.HasPrefix(host, "/") || strings.HasPrefix(address, "unix://") {
		return true, "Unix socket (skip TCP check)"
	}

	target := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", target, 2*time.Second)
	if err != nil {
		return false, fmt.Sprintf("TCP connect failed: %v", err)
	}
	conn.Close()
	return true, "TCP connect OK"
}

// NormalizeAddress normalizes an endpoint address to http://host:port format.
func NormalizeAddress(addr string) string {
	cleaned := strings.TrimSpace(addr)

	// Check if it already has a scheme
	hasScheme := strings.HasPrefix(cleaned, "http://") || strings.HasPrefix(cleaned, "https://")

	if !hasScheme {
		// Bare host:port — add http://
		if strings.Contains(cleaned, ":") {
			cleaned = "http://" + cleaned
		} else {
			// Host only — add http:// and default port 80
			cleaned = "http://" + cleaned + ":80"
		}
	}

	return cleaned
}

// parseHostPort extracts host and port from an address string.
// NOTE: Unlike the canonical safety.SplitHostPort, this function:
//   - Strips "http://" and "https://" URL prefixes
//   - Returns port as string (not int)
//   - Defaults to "80" or "443" based on scheme
// For simple "host:port" splitting, use safety.SplitHostPort instead.
func parseHostPort(addr string) (host string, port string, err error) {
	cleaned := addr
	if len(cleaned) > 7 && strings.HasPrefix(cleaned, "http://") {
		cleaned = cleaned[7:]
	} else if len(cleaned) > 8 && strings.HasPrefix(cleaned, "https://") {
		cleaned = cleaned[8:]
	}

	h, p, e := net.SplitHostPort(cleaned)
	if e != nil {
		if strings.HasPrefix(addr, "https://") {
			return cleaned, "443", nil
		}
		return cleaned, "80", nil
	}
	return h, p, nil
}
