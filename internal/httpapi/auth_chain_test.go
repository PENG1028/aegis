package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aegis/internal/action"
	"aegis/internal/adminauth"
	"aegis/internal/token"
)

// stubSessionValidator lets the chain be exercised without a database.
type stubAdminAuth struct{}

// TestAuthChainAdminSessionReachesNonAdminEndpoints wires AdminAuthMiddleware and
// token.AuthMiddleware in the same order as internal/cli/serve.go and checks that
// a logged-in admin can reach admin-only endpoints outside /api/admin/v1/.
//
// The 401 bug lived in the seam between these two middlewares: the cookie Path
// stopped the browser from sending the session, and the AdminAuth gate would not
// have injected AdminContext even if it had arrived. Each package's own tests
// passed throughout — only the composition shows the failure, which is why this
// test lives here rather than in either package.
func TestAuthChainAdminSessionReachesNonAdminEndpoints(t *testing.T) {
	var gotCtx *action.ActionContext
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCtx = action.GetActionContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Compose as serve.go does: token.Auth inside, AdminAuth outside.
	authMw := token.NewAuthMiddleware("")
	var h http.Handler = handler
	h = authMw.Middleware(h)
	h = injectAdminContext(h)

	uiPaths := []struct{ method, path string }{
		{"POST", "/api/apply"},
		{"POST", "/api/rollback"},
		{"GET", "/api/apply/history"},
		{"GET", "/api/config/current"},
		{"GET", "/api/config/diff"},
		{"GET", "/api/config/preview"},
		{"GET", "/api/exposures"},
		{"GET", "/api/admin/v1/scopes"},
	}

	for _, tc := range uiPaths {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			gotCtx = nil
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 for logged-in admin, got %d", w.Code)
			}
			if gotCtx == nil {
				t.Fatal("no ActionContext reached the handler")
			}
			if gotCtx.TokenType != "admin" {
				t.Errorf("expected token_type admin, got %q", gotCtx.TokenType)
			}
		})
	}
}

// injectAdminContext stands in for AdminAuthMiddleware with a valid session,
// which is what that middleware produces on success. Using the real middleware
// would require a live session store; the value under test is what the inner
// middleware does with the context it receives.
func injectAdminContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := adminauth.WithAdminContext(r.Context(), &adminauth.AdminContext{
			UserID:   "u_test",
			Username: "admin",
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TestAuthChainRejectsAnonymousOnAdminOnlyPaths is the companion check: the paths
// above must still be closed without a session, so the fix widened reach for
// authenticated admins only.
func TestAuthChainRejectsAnonymousOnAdminOnlyPaths(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := token.NewAuthMiddleware("").Middleware(handler)

	for _, p := range []string{
		"/api/apply",
		"/api/rollback",
		"/api/config/current",
		"/api/exposures",
		"/api/admin/v1/scopes",
	} {
		req := httptest.NewRequest("POST", p, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401 without a session, got %d", p, w.Code)
		}
	}
}
