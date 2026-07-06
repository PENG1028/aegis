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
			writeGuardError(w, 403, "invalid ticket: "+err.Error())
			return
		}

		if claims.TargetService != c.cfg.ServiceName {
			writeGuardError(w, 403, "ticket target mismatch")
			return
		}

		if apiName != "" && claims.TargetAPI != apiName {
			writeGuardError(w, 403, "ticket api mismatch")
			return
		}

		blockedReason := c.isBlocked(claims.CallerService, apiName)
		if blockedReason != "" {
			writeGuardError(w, 403, blockedReason)
			return
		}

		if !c.isAllowed(claims.CallerService, c.cfg.ServiceName, apiName) {
			writeGuardError(w, 403, "access denied by policy")
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

// isAllowed evaluates access policies. If no policy matches, defaults to allow.
func (c *Client) isAllowed(callerName, targetService, apiName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, p := range c.policies {
		if !matchSubject(callerName, p.Subject) {
			continue
		}
		if p.TargetService != "*" && p.TargetService != targetService {
			continue
		}
		if p.Action != "*" && p.Action != apiName {
			continue
		}
		return p.Effect == "allow"
	}
	return true // default allow
}

func matchSubject(callerName, subject string) bool {
	if subject == "*" || subject == callerName {
		return true
	}
	// Check group membership (groups stored locally in c.groups)
	return false // group matching handled by caller via InGroup()
}

func writeGuardError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
