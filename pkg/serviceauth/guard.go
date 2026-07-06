package serviceauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const ctxKeyCaller contextKey = "serviceauth-caller"

// Guard returns HTTP middleware that verifies every request carries a valid
// service ticket signed with the caller's Ed25519 private key. Verification is
// local — zero network calls.
func (c *Client) Guard(apiName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ticket := r.Header.Get("X-Service-Ticket")
		if ticket == "" {
			writeGuardError(w, 401, "missing service ticket")
			return
		}

		// Extract caller name from ticket to look up public key.
		callerName := callerNameFromTicket(ticket)
		if callerName == "" {
			writeGuardError(w, 403, "malformed ticket")
			return
		}

		c.mu.RLock()
		pubKey := c.publicKeys[callerName]
		c.mu.RUnlock()

		if pubKey == "" {
			writeGuardError(w, 403, "unknown caller: "+callerName)
			return
		}

		claims, err := VerifyTicket(ticket, pubKey)
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

// callerNameFromTicket extracts the caller service name from a ticket without verification.
func callerNameFromTicket(ticketStr string) string {
	decoded, err := base64.StdEncoding.DecodeString(ticketStr)
	if err != nil {
		return ""
	}
	parts := strings.SplitN(string(decoded), ":", 5)
	if len(parts) < 1 {
		return ""
	}
	return parts[0]
}

// isBlocked checks whether a service+API combination is blocked.
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
