package serviceauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

type contextKey string

const ctxKeyCaller contextKey = "serviceauth-caller"

// Guard returns HTTP middleware that verifies every request carries a valid
// service ticket signed with the caller's Ed25519 private key. Verification is
// local — zero network calls.
//
// Before verifying the ticket, Guard checks the caller's IP against the
// client's IPChecker (default: cluster-only). External IPs can be allowed
// via temporary whitelist entries synced from Aegis (max 24h).
//
// The caller's identity is injected into the request context and can be
// retrieved with CallerFromContext(). The service's own code is responsible
// for permission checks — Guard only verifies identity.
func (c *Client) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// IP 检查（默认：仅允许内网 IP）
		remoteIP := extractIP(r)
		c.mu.RLock()
		checker := c.ipChecker
		c.mu.RUnlock()
		if !checker.Allow(remoteIP) {
			writeGuardError(w, 403, "request from untrusted IP")
			return
		}

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
			writeGuardError(w, 403, "unknown caller")
			return
		}

		claims, err := VerifyTicket(ticket, pubKey)
		if err != nil {
			writeGuardError(w, 403, "invalid ticket")
			return
		}

		blockedReason := c.isBlocked(claims.CallerService)
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

func callerNameFromTicket(ticketStr string) string {
	decoded, err := base64.StdEncoding.DecodeString(ticketStr)
	if err != nil {
		return ""
	}
	parts := strings.SplitN(string(decoded), ":", 3)
	if len(parts) < 1 {
		return ""
	}
	return parts[0]
}

func (c *Client) isBlocked(callerName string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, entry := range c.blocklist {
		if entry.ServiceID == "*" || entry.ServiceID == callerName {
			if entry.APIName == "*" {
				return "caller is blocked (service-level)"
			}
			return "caller is blocked: " + entry.APIName
		}
	}
	return ""
}

func extractIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeGuardError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
