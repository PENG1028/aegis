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

// Middleware returns an HTTP middleware that validates Bearer tokens.
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Login endpoint: bypass both admin session and bearer token check
		if r.URL.Path == "/api/admin/v1/auth/login" && r.Method == "POST" {
			next.ServeHTTP(w, r)
			return
		}

		// Node join endpoint: uses join token in request body, not Bearer token.
		// New nodes have no credentials yet — the join token proves eligibility.
		if r.URL.Path == "/api/node/v1/join" && r.Method == "POST" {
			next.ServeHTTP(w, r)
			return
		}

		// Relay handler: uses GatewayLink-based auth, not Bearer token
		if strings.HasPrefix(r.URL.Path, "/__aegis/") {
			next.ServeHTTP(w, r)
			return
		}

		// If admin session is already validated (by AdminAuthMiddleware),
		// skip bearer token check and use admin context directly.
		if adminCtx := adminauth.GetAdminContext(r.Context()); adminCtx != nil {
			ac := &action.ActionContext{
				SpaceID:   "",           // admin has no space constraint
				TokenType: "admin",      // admin type
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
	// Admin token from config: full admin scope
	if m.adminToken != "" && token == m.adminToken {
		return []string{ScopeAdminAll}, "admin", "", "", true
	}

	// Check database tokens
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
		// v1.7W: Block service keys from direct CRUD — use Action API instead
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
