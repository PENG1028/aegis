package serviceauth

import (
	"context"
	"encoding/json"
	"net/http"
)

type contextKey string

const ctxKeyCaller contextKey = "serviceauth-caller"

// Guard returns HTTP middleware that verifies every request carries a valid
// cluster ticket. Verification is local — zero network calls.
//
//	http.HandleFunc("POST /api/v1/projects", client.Guard("createProject", projectHandler))
//
// The apiName must match the APIDef.Name registered by this service. The Guard
// verifies that the ticket was issued FOR this service AND for this specific
// API, preventing cross-target ticket reuse. Pass "" to skip the API check
// (backward compatibility, not recommended).
func (c *Client) Guard(apiName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ticket := r.Header.Get("X-Service-Ticket")
		if ticket == "" {
			writeGuardError(w, 401, "missing service ticket")
			return
		}

		c.mu.RLock()
		secret := c.clusterSecret
		c.mu.RUnlock()

		claims, err := VerifyTicket(ticket, secret)
		if err != nil {
			writeGuardError(w, 403, "invalid ticket")
			return
		}

		// Reject tickets issued for a different target service.
		if claims.TargetService != c.cfg.ServiceName {
			writeGuardError(w, 403, "ticket target mismatch")
			return
		}

		// Reject tickets issued for a different API (when specified).
		if apiName != "" && claims.TargetAPI != apiName {
			writeGuardError(w, 403, "ticket api mismatch")
			return
		}

		if c.isBlocked(claims.CallerService) {
			writeGuardError(w, 403, "caller is blocked")
			return
		}

		// Note: X-Caller-Host is informational only — the true caller identity
		// is claims.CallerService, which was verified by HMAC.
		caller := CallerInfo{
			ServiceName: claims.CallerService,
			CallerHost:  r.Header.Get("X-Caller-Host"),
		}
		ctx := context.WithValue(r.Context(), ctxKeyCaller, caller)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CallerFromContext extracts the caller identity injected by Guard.
func CallerFromContext(ctx context.Context) CallerInfo {
	if info, ok := ctx.Value(ctxKeyCaller).(CallerInfo); ok {
		return info
	}
	return CallerInfo{}
}

// isBlocked checks whether a service (by name) appears in the blocklist.
// The blocklist stores service DB IDs; we match by correlating with the
// instances map to find the service name for a given ID.
func (c *Client) isBlocked(callerName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, entry := range c.blocklist {
		blockedName := c.serviceNameForID(entry.ServiceID)
		if blockedName == "" {
			continue
		}
		if blockedName == callerName && entry.APIName == "*" {
			return true
		}
	}
	return false
}

// serviceNameForID returns the service name for a DB record ID, or "".
func (c *Client) serviceNameForID(id string) string {
	for name, instances := range c.instances {
		for _, inst := range instances {
			if inst.Name == id || name == id {
				return name
			}
		}
	}
	return ""
}

func writeGuardError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
