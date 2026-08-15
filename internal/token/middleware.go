package token

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"path"
	"strings"

	"aegis/internal/action"
	"aegis/internal/adminauth"
	"aegis/internal/logs"
)

// AuditLogger is the interface for writing audit log entries.
type AuditLogger interface {
	LogAudit(actorType, actorID, eventType, ip, userAgent, targetType, targetID, result, errorCode string)
}

var auditLog logs.AuditLogger

// SetAuditLogger sets the global audit logger for the auth middleware.
func SetAuditLogger(l logs.AuditLogger) {
	auditLog = l
}

// ServiceAuthChecker is an optional bridge that validates serviceauth tickets
// and maps the caller service to an Aegis space for Action API access.
// When set, requests without a Bearer token are checked for a valid
// X-Service-Ticket header and granted space-scoped access automatically.
type ServiceAuthChecker interface {
	VerifyTicketAndGetSpace(ticketStr string) (serviceName string, err error)
}

var serviceAuthChecker ServiceAuthChecker

// SetServiceAuthChecker injects a serviceauth bridge into the auth middleware.
func SetServiceAuthChecker(checker ServiceAuthChecker) {
	serviceAuthChecker = checker
}

// AuthMiddleware provides authentication for the HTTP API.
// It supports three auth methods (tried in order):
//
//  1. Admin session cookie (set by AdminAuthMiddleware) — AdminContext
//  2. Authorization: Bearer token (static admin token, CLI/curl)
//  3. X-Service-Ticket header (service-to-service, via serviceauth bridge)
type AuthMiddleware struct {
	adminToken string
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(adminToken string) *AuthMiddleware {
	return &AuthMiddleware{adminToken: adminToken}
}

// isPublicPath returns true for paths that don't require authentication.
// isPublicPath reports whether a path may be served without authentication.
//
// The caller passes the raw request path, which this function normalizes first.
// Without cleaning, a dot segment could claim a public prefix while denoting a
// protected route — e.g. "/api/service-auth/v1/../admin/v1/scopes" matches the
// SDK prefix below, and the exact-match exclusion for
// "/api/service-auth/v1/services" is sidestepped by
// "/api/service-auth/v1/./services". Normalizing makes every comparison here
// operate on the path that actually gets routed.
func isPublicPath(rawPath, method string) bool {
	path := normalizeGuardPath(rawPath)

	if path == "/api/admin/v1/auth/login" && method == "POST" {
		return true
	}
	// ACME validators cannot authenticate; only expose the read-only challenge namespace.
	if method == http.MethodGet && strings.HasPrefix(path, "/.well-known/acme-challenge/") {
		return true
	}
	// distnode transport RPC — protected by distnode's own HMAC shared-secret
	// auth inside Transport.Handler(). Distinct from the admin-protected
	// /api/admin/v1/distnode/* management endpoints.
	if strings.HasPrefix(path, "/api/distnode/v1/") {
		return true
	}
	if strings.HasPrefix(path, "/api/transparent/v1/") {
		return true
	}
	if strings.HasPrefix(path, "/__aegis/") {
		return true
	}
	// v1.9A: Service-Auth SDK endpoints — protected by isInCluster() IP check
	if strings.HasPrefix(path, "/api/service-auth/v1/") && path != "/api/service-auth/v1/services" {
		return true
	}
	if path == "/api/healthz" || path == "/api/readyz" {
		return true
	}
	if path == "/api/system/status" && method == "GET" {
		return true
	}
	if path == "/api/system/runtime-mode" && method == "GET" {
		return true
	}
	if path == "/api/system/compositions" && method == "GET" {
		return true
	}
	// Embedded UI
	if path == "/" || strings.HasPrefix(path, "/assets/") || path == "/favicon.ico" || path == "/favicon.svg" {
		return true
	}
	// SPA routes
	if !strings.HasPrefix(path, "/api/") && !strings.Contains(path, ".") {
		return true
	}
	return false
}

// isSystemRoute reports whether a path is an administrative/system surface that
// service callers must never reach.
//
// Service tickets authenticate a *workload*, not an operator. A workload may
// manage its own resources through the Action API, but it must not read cluster
// inventory, mutate providers, or drive Apply. Keeping this as a path predicate
// (rather than per-handler checks) means a newly added admin route is protected
// the moment it is registered — the previous per-handler approach left most
// admin handlers unguarded because each one had to remember to check.
func isSystemRoute(path string) bool {
	return strings.HasPrefix(path, "/api/admin/")
}

// serviceTicketAllowlist is the complete set of surfaces a ServiceAuth ticket
// may reach. This is an allowlist, not a denylist: a route that is not named
// here is denied for service callers.
//
// WHY an allowlist: the business API (/api/routes, /api/apply, /api/projects…)
// performs no space-ownership checks of its own. A service reaching those paths
// could create arbitrary routes and trigger Apply, bypassing the ownership
// enforcement that only exists inside the Action API handlers. Enumerating what
// is permitted keeps new business routes closed by default.
var serviceTicketAllowlist = []string{
	"/api/v1/actions/",      // self-service resource operations (ownership enforced per-handler)
	"/api/v1/my/",           // read-only view of caller's own resources
	"/api/service-auth/v1/", // SDK surface: sync, report, heartbeat, call, capabilities
}

// serviceTicketAllowed reports whether a service-ticket caller may reach path.
func serviceTicketAllowed(p string) bool {
	for _, prefix := range serviceTicketAllowlist {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// normalizeGuardPath resolves dot segments before the allowlist sees a path.
//
// This middleware runs ahead of http.ServeMux, so r.URL.Path is still exactly
// what the client sent: "/api/v1/my/../admin/v1/scopes" carries an allowed
// prefix but denotes an admin route. ServeMux happens to answer such requests
// with a 301 to the cleaned path (which then re-enters this guard and is
// denied), so prefix matching is not currently bypassable — but that safety
// belongs to the router, not to this check. Cleaning here keeps the guard
// correct on its own terms.
//
// path.Clean drops a trailing slash, so restore it: the allowlist entries are
// directory prefixes and "/api/v1/my/" must keep matching itself.
func normalizeGuardPath(raw string) string {
	if raw == "" {
		return "/"
	}
	cleaned := path.Clean(raw)
	if strings.HasSuffix(raw, "/") && !strings.HasSuffix(cleaned, "/") {
		cleaned += "/"
	}
	return cleaned
}

// Middleware returns an HTTP middleware that authenticates requests.
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SSRF guard: /api/service-auth/v1/call proxies to any registered
		// backend, so it requires a valid service ticket even though the SDK
		// prefix is otherwise public. Without this, an unauthenticated client
		// could drive gatewayed requests into every registered service.
		if normalizeGuardPath(r.URL.Path) == "/api/service-auth/v1/call" {
			m.authServiceTicket(w, r, next)
			return
		}

		// Public paths: no auth required
		if isPublicPath(r.URL.Path, r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		// ① Admin session cookie — AdminAuthMiddleware injects AdminContext for
		// any /api/ path, so this covers admin-only endpoints outside the
		// /api/admin/v1/ prefix too (/api/apply, /api/rollback, /api/config/*).
		if adminCtx := adminauth.GetAdminContext(r.Context()); adminCtx != nil {
			ac := &action.ActionContext{
				SpaceID:   "",
				TokenType: "admin",
				TokenID:   adminCtx.UserID,
				Actor:     "admin",
			}
			ctx := action.WithActionContext(r.Context(), ac)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// ③ Static admin Bearer token
		token := extractBearerToken(r)
		if token != "" && m.adminToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(m.adminToken)) == 1 {
			ac := &action.ActionContext{
				SpaceID:   "",
				TokenType: "admin",
				TokenID:   "",
				Actor:     "api",
			}
			ctx := action.WithActionContext(r.Context(), ac)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// ② Service-to-service Ticket (via serviceauth bridge)
		if !m.authServiceTicket(w, r, next) {
			return
		}
	})
}

// authServiceTicket authenticates a request via the X-Service-Ticket header
// (Ed25519, verified by the serviceauth bridge) and enforces the service
// allowlist. It writes the response itself; it returns false when the caller
// must not continue.
func (m *AuthMiddleware) authServiceTicket(w http.ResponseWriter, r *http.Request, next http.Handler) bool {
	if serviceAuthChecker == nil {
		logAuditEvent("service_key", "", "unauthorized_access", r, "", "missing_token", "failed", "UNAUTHORIZED")
		writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid auth")
		return false
	}
	ticket := r.Header.Get("X-Service-Ticket")
	if ticket == "" {
		logAuditEvent("service_key", "", "unauthorized_access", r, "", "missing_token", "failed", "UNAUTHORIZED")
		writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "service ticket required")
		return false
	}
	serviceName, err := serviceAuthChecker.VerifyTicketAndGetSpace(ticket)
	if err != nil {
		logAuditEvent("service_key", "", "unauthorized_access", r, "", "invalid_token", "failed", "UNAUTHORIZED")
		writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired service ticket")
		return false
	}
	// A valid ticket proves identity, not authority. Admin and unlisted
	// business routes stay closed to service callers. Match on the cleaned
	// path so dot segments cannot smuggle an admin route in behind an
	// allowed prefix.
	guardPath := normalizeGuardPath(r.URL.Path)
	if isSystemRoute(guardPath) || !serviceTicketAllowed(guardPath) {
		// actor_type matches the unauthorized_access call below and
		// docs/design/failure-matrix.md §3.1 so audit queries on
		// actor_type=service_key see both denial kinds.
		logAuditEvent("service_key", serviceName, "access_denied", r,
			"route", guardPath, "failed", "SCOPE_DENIED")
		writeAuthError(w, http.StatusForbidden, "SCOPE_DENIED",
			"service tickets may only access the Action API and their own resources")
		return false
	}
	ac := &action.ActionContext{
		SpaceID:   serviceName,
		TokenType: "service",
		TokenID:   serviceName,
		Actor:     "service",
	}
	ctx := action.WithActionContext(r.Context(), ac)
	next.ServeHTTP(w, r.WithContext(ctx))
	return true
}

// extractBearerToken extracts the Bearer token from an Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// logAuditEvent writes an audit log entry for auth failures.
func logAuditEvent(actorType, actorID, eventType string, r *http.Request, targetType, targetID, result, errorCode string) {
	if auditLog != nil {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}
		auditLog.LogAudit(actorType, actorID, eventType, ip, r.UserAgent(), targetType, targetID, result, errorCode)
	}
}
