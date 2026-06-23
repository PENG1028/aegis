package token

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// AuthMiddleware provides Bearer token authentication for HTTP API.
type AuthMiddleware struct {
	adminToken    string // from config
	tokenRepo     *Repository
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
			http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
			return
		}

		if !m.validateToken(token) {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validateToken validates a token against the admin token or database tokens.
func (m *AuthMiddleware) validateToken(token string) bool {
	// First check against admin token from config
	if m.adminToken != "" && token == m.adminToken {
		return true
	}

	// Then check against database tokens
	if m.tokenRepo != nil {
		hash := hashToken(token)
		t, err := m.tokenRepo.FindByTokenHash(hash)
		if err == nil && t != nil {
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
