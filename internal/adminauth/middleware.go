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

// Middleware validates the admin session cookie and injects AdminContext.
//
// It does not itself reject unauthenticated requests: absent or unreadable
// sessions fall through so the inner auth middleware (internal/token) makes the
// call. Its job is to establish *who* the caller is, not *whether* they may pass.
//
// Scope is every /api/ path, not just /api/admin/v1/. Admin-only endpoints exist
// outside that prefix (/api/apply, /api/rollback, /api/config/*, /api/exposures),
// and while the gate matched only /api/admin/v1/ those endpoints could never see
// an AdminContext — the UI's Apply, Rollback and Changes pages 401'd. Widening
// the cookie Path alone would not have fixed it; both sides had to move.
func (m *AdminAuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Non-API paths (SPA shell, assets) never carry the cookie.
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Allow login endpoint without session
		if r.URL.Path == "/api/admin/v1/auth/login" && r.Method == "POST" {
			next.ServeHTTP(w, r)
			return
		}

		// Extract session cookie
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || cookie.Value == "" {
			next.ServeHTTP(w, r)
			return
		}

		sessionHash := HashSessionToken(cookie.Value)
		user, err := m.service.ValidateSession(sessionHash)
		if err != nil || user == nil {
			// An expired session on an admin route gets an explicit 401 so the UI
			// can distinguish "session expired" from "not logged in".
			//
			// Elsewhere it falls through instead: public endpoints (/api/healthz,
			// /api/system/status, the service-auth SDK surface) must keep working
			// for a caller holding a stale cookie. Protected non-admin paths still
			// end in 401 — just from the token middleware rather than here.
			if strings.HasPrefix(r.URL.Path, "/api/admin/v1/") {
				writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired session")
				return
			}
			next.ServeHTTP(w, r)
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

// SessionCookieName and SessionCookiePath define the admin session cookie.
//
// WHY Path is "/api" and not "/api/admin/v1": a browser sends a cookie only when
// the cookie's Path is a prefix of the request path on a segment boundary.
// Scoped to "/api/admin/v1", the session was never sent to the admin-only
// endpoints living outside that prefix — /api/apply, /api/rollback,
// /api/config/*, /api/exposures, /api/routes/{id}, /api/services/{id} — so the
// UI's Apply, Rollback and Changes pages received 401 while looking correctly
// wired (the client sends credentials: 'include' and no Bearer token).
//
// Both setter and clearer must use the identical Path: a browser deletes a
// cookie only when name, Path and Domain all match, so a mismatch would leave
// logout unable to clear the session.
const (
	SessionCookieName = "aegis_admin_session"
	SessionCookiePath = "/api"
)

// SetSessionCookie sets the admin session cookie on a response.
func SetSessionCookie(w http.ResponseWriter, token string, expiresAt string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     SessionCookiePath,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
		MaxAge:   int(24 * 60 * 60), // 24 hours
	})
}

// ClearSessionCookie removes the admin session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     SessionCookiePath,
		HttpOnly: true,
		MaxAge:   -1,
	})
}
