package adminauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

// context key type for admin session info.
type adminCtxKey struct{}

// AdminContext carries the authenticated admin user info.
type AdminContext struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// WithAdminContext injects admin context into a request context.
func WithAdminContext(ctx context.Context, ac *AdminContext) context.Context {
	return context.WithValue(ctx, adminCtxKey{}, ac)
}

// GetAdminContext extracts admin context from a request context.
func GetAdminContext(ctx context.Context) *AdminContext {
	ac, _ := ctx.Value(adminCtxKey{}).(*AdminContext)
	return ac
}

// AdminAuthMiddleware provides cookie-based admin session authentication.
type AdminAuthMiddleware struct {
	service *Service
}

// NewAdminAuthMiddleware creates a new admin auth middleware.
func NewAdminAuthMiddleware(service *Service) *AdminAuthMiddleware {
	return &AdminAuthMiddleware{service: service}
}

// Middleware returns an HTTP middleware that validates admin session cookies.
// It only protects routes under /api/admin/v1/ — all other paths pass through.
func (m *AdminAuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only protect admin routes
		if !strings.HasPrefix(r.URL.Path, "/api/admin/v1/") {
			// Skip auth endpoints themselves
			next.ServeHTTP(w, r)
			return
		}

		// Allow login endpoint without session
		if r.URL.Path == "/api/admin/v1/auth/login" && r.Method == "POST" {
			next.ServeHTTP(w, r)
			return
		}

		// Extract session cookie
		cookie, err := r.Cookie("aegis_admin_session")
		if err != nil || cookie.Value == "" {
			next.ServeHTTP(w, r)
			return
		}

		sessionHash := HashSessionToken(cookie.Value)
		user, err := m.service.ValidateSession(sessionHash)
		if err != nil || user == nil {
			writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired session")
			return
		}

		// Inject admin context
		ac := &AdminContext{
			UserID:   user.ID,
			Username: user.Username,
		}
		ctx := WithAdminContext(r.Context(), ac)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HashSessionToken hashes a session token for storage comparison.
func HashSessionToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// writeAdminError writes a JSON error response.
func writeAdminError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// SetSessionCookie sets the admin session cookie on a response.
func SetSessionCookie(w http.ResponseWriter, token string, expiresAt string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "aegis_admin_session",
		Value:    token,
		Path:     "/api/admin/v1",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
		MaxAge:   int(24 * 60 * 60), // 24 hours
	})
}

// ClearSessionCookie removes the admin session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "aegis_admin_session",
		Value:    "",
		Path:     "/api/admin/v1",
		HttpOnly: true,
		MaxAge:   -1,
	})
}
