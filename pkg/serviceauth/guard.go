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

		blockedReason := c.isBlocked(claims.CallerService, apiName)
		if blockedReason != "" {
			writeGuardError(w, 403, blockedReason)
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

// isBlocked checks whether a service+API combination is blocked.
// Returns the reason string if blocked, or "" if allowed.
// Rules: service-level block (api_name="*") blocks all APIs; per-API block
// only blocks that specific API for that service.
func (c *Client) isBlocked(callerName, apiName string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, entry := range c.blocklist {
		blockedName := c.serviceNameForID(entry.ServiceID)
		if blockedName == "" || blockedName != callerName {
			continue
		}
		if entry.APIName == "*" {
			return "caller is blocked (service-level)"
		}
		if apiName != "" && entry.APIName == apiName {
			return "caller is blocked (API-level): " + apiName
		}
	}
	return ""
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
