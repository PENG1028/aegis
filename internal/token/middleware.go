package token

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

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
		token := extractBearerToken(r)
		if token == "" {
			writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing Authorization header")
			return
		}

		scopes, ok := m.validateTokenWithScopes(token)
		if !ok {
			writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
			return
		}

		// Check scope for this route
		requiredScope, hasScope := FindMatchingScope(r.Method, r.URL.Path)
		if hasScope && !HasScope(scopes, requiredScope) {
			writeAuthError(w, http.StatusForbidden, "FORBIDDEN",
				"missing required scope: "+requiredScope)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validateTokenWithScopes validates a token and returns its scopes.
func (m *AuthMiddleware) validateTokenWithScopes(token string) ([]string, bool) {
	// Admin token from config: full admin scope
	if m.adminToken != "" && token == m.adminToken {
		return []string{ScopeAdminAll}, true
	}

	// Check database tokens
	if m.tokenRepo != nil {
		hash := hashToken(token)
		t, err := m.tokenRepo.FindByTokenHash(hash)
		if err == nil && t != nil {
			return t.Scopes, true
		}
	}

	return nil, false
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
