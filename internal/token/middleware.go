package token

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"aegis/internal/action"
	"aegis/internal/adminauth"
	"aegis/internal/logs"
)

// AuditLogger is the interface for writing audit log entries.
// Implemented by logs.AppService.
type AuditLogger interface {
	LogAudit(actorType, actorID, eventType, ip, userAgent, targetType, targetID, result, errorCode string)
}

// auditLog is the global audit logger for auth failures.
// Set via SetAuditLogger during initialization.
var auditLog logs.AuditLogger

// SetAuditLogger sets the global audit logger for the token middleware.
func SetAuditLogger(l logs.AuditLogger) {
	auditLog = l
}

// AuthMiddleware provides Bearer token authentication with scope checking.
type AuthMiddleware struct {
	adminToken string
	tokenRepo  *Repository
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(adminToken string, repo *Repository) *AuthMiddleware {
	return &AuthMiddleware{
		adminToken: adminToken,
		tokenRepo:  repo,
	}
}

// isPublicPath returns true for paths that don't require authentication.
// These include: login endpoint, node join, relay dispatch, health probes,
// and the embedded UI (login page + SPA assets).
func isPublicPath(path, method string) bool {
	if path == "/api/admin/v1/auth/login" && method == "POST" {
		return true
	}
	if path == "/api/node/v1/join" && method == "POST" {
		return true
	}
	if strings.HasPrefix(path, "/__aegis/") {
		return true
	}
	if path == "/api/healthz" || path == "/api/readyz" {
		return true
	}
	// System status — used by the login page to detect server version/health
	if path == "/api/system/status" && method == "GET" {
		return true
	}
	// Embedded UI — must be public so the login form loads
	if path == "/" || strings.HasPrefix(path, "/assets/") || path == "/favicon.ico" {
		return true
	}
	return false
}

// Middleware returns an HTTP middleware that validates Bearer tokens.
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public paths: no auth required
		if isPublicPath(r.URL.Path, r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		// If admin session is already validated (by AdminAuthMiddleware),
		// skip bearer token check and use admin context directly.
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

		token := extractBearerToken(r)
		if token == "" {
			logAuditEvent("service_key", "", "unauthorized_access", r, "", "missing_token", "failed", "UNAUTHORIZED")
			writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing Authorization header")
			return
		}

		scopes, tokenType, spaceID, tokenID, ok := m.validateTokenWithScopes(token)
		if !ok {
			logAuditEvent("service_key", "", "unauthorized_access", r, "", "invalid_token", "failed", "UNAUTHORIZED")
			writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
			return
		}

		// Check scope for this route
		requiredScope, hasScope := FindMatchingScope(r.Method, r.URL.Path)
		if hasScope && !HasScope(scopes, requiredScope) {
			logAuditEvent(tokenType, tokenID, "scope_violation", r, requiredScope, r.URL.Path, "failed", "FORBIDDEN")
			writeAuthError(w, http.StatusForbidden, "FORBIDDEN",
				"missing required scope: "+requiredScope)
			return
		}

		// For space tokens, block access to system-level routes
		if tokenType == "space" && isSystemRoute(r.URL.Path) {
			logAuditEvent(tokenType, tokenID, "service_key_denied_admin", r, "admin_route", r.URL.Path, "failed", "SCOPE_DENIED")
			writeAuthError(w, http.StatusForbidden, "SCOPE_DENIED",
				"service API keys cannot access admin routes")
			return
		}

		// Inject action context into request context
		ac := &action.ActionContext{
			SpaceID:   spaceID,
			TokenType: tokenType,
			TokenID:   tokenID,
			Actor:     "api",
		}
		ctx := action.WithActionContext(r.Context(), ac)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validateTokenWithScopes validates a token and returns its scopes, type, space_id, and token_id.
func (m *AuthMiddleware) validateTokenWithScopes(token string) (scopes []string, tokenType, spaceID, tokenID string, ok bool) {
	if m.adminToken != "" && token == m.adminToken {
		return []string{ScopeAdminAll}, "admin", "", "", true
	}

	if m.tokenRepo != nil {
		hash := hashToken(token)
		t, err := m.tokenRepo.FindByTokenHash(hash)
		if err == nil && t != nil {
			return t.Scopes, t.TokenType, t.SpaceID, t.ID, true
		}
	}

	return nil, "", "", "", false
}

// isSystemRoute returns true if the path targets system-level resources
// that space tokens should not access.
func isSystemRoute(path string) bool {
	systemPrefixes := []string{
		"/api/admin/",
		"/api/system/",
		"/api/config/",
		"/api/apply",
		"/api/rollback",
		"/api/diagnostics/",
		"/api/settings",
		"/api/health",
		"/api/logs",
		"/api/routes",
		"/api/services",
		"/api/managed-domains",
		"/api/endpoints",
		"/api/exposures",
		"/api/projects",
	}
	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
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

// hashToken returns the SHA-256 hex hash of a token.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
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

// logAuditEvent writes an audit log entry for auth failures using the global audit logger.
func logAuditEvent(actorType, actorID, eventType string, r *http.Request, targetType, targetID, result, errorCode string) {
	if auditLog != nil {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}
		auditLog.LogAudit(actorType, actorID, eventType, ip, r.UserAgent(), targetType, targetID, result, errorCode)
	}
}
