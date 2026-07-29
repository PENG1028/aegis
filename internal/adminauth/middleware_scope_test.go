package adminauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// loginForTest creates an admin user, logs in, and returns the session token.
func loginForTest(t *testing.T) (*Service, string) {
	t.Helper()
	db := setupTestDB(t)
	t.Cleanup(func() { db.Close() })

	userRepo := NewAdminUserRepository(db)
	sessionRepo := NewAdminSessionRepository(db)
	svc := NewService(userRepo, sessionRepo)

	user, err := NewAdminUser("admin", "secure-password-123")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := userRepo.Create(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	result, err := svc.Login("admin", "secure-password-123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return svc, result.SessionToken
}

// TestMiddlewareInjectsAdminContextOnNonAdminPaths drives the real middleware
// with a real session over the paths the UI calls.
//
// This is the half of the 401 bug that the cookie-Path test cannot see: even with
// the cookie correctly delivered, the gate matched only /api/admin/v1/ and so
// never injected AdminContext for /api/apply and friends. Reverting the gate must
// make this test fail.
func TestMiddlewareInjectsAdminContextOnNonAdminPaths(t *testing.T) {
	svc, sessionToken := loginForTest(t)

	var gotAdmin *AdminContext
	h := NewAdminAuthMiddleware(svc).Middleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAdmin = GetAdminContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

	for _, p := range []string{
		"/api/apply",
		"/api/apply/dry-run",
		"/api/rollback",
		"/api/apply/history",
		"/api/config/current",
		"/api/config/diff",
		"/api/config/preview",
		"/api/exposures",
		"/api/health",
		"/api/routes/rt_1",
		"/api/admin/v1/scopes",
	} {
		t.Run(p, func(t *testing.T) {
			gotAdmin = nil
			req := httptest.NewRequest("POST", p, nil)
			req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionToken})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			if gotAdmin == nil {
				t.Fatal("AdminContext was not injected — endpoint will 401 downstream")
			}
			if gotAdmin.Username != "admin" {
				t.Errorf("expected username admin, got %q", gotAdmin.Username)
			}
		})
	}
}

// TestStaleCookieDoesNotBreakNonAdminPaths covers the regression risk introduced
// by widening the gate to all /api/ paths.
//
// Now that the middleware inspects every /api/ request, an expired cookie reaches
// the validation branch on paths that previously skipped it entirely. Returning
// 401 there would break public endpoints (/api/healthz, /api/system/status, the
// service-auth SDK surface) for anyone holding a stale cookie, so those must fall
// through and let the inner middleware decide. Admin paths keep the explicit 401.
func TestStaleCookieDoesNotBreakNonAdminPaths(t *testing.T) {
	svc, _ := loginForTest(t)

	reached := false
	h := NewAdminAuthMiddleware(svc).Middleware(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached = true
			w.WriteHeader(http.StatusOK)
		}))

	// Falls through: the inner auth middleware owns the decision.
	fallThrough := []string{
		"/api/healthz",
		"/api/readyz",
		"/api/system/status",
		"/api/service-auth/v1/sync",
		"/api/apply",
	}
	for _, p := range fallThrough {
		t.Run("fallthrough "+p, func(t *testing.T) {
			reached = false
			req := httptest.NewRequest("GET", p, nil)
			req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "expired-garbage"})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if !reached {
				t.Errorf("expected fall-through for stale cookie on %s, got %d", p, w.Code)
			}
		})
	}

	// Admin paths still answer 401 directly so the UI can tell "expired" from
	// "never logged in".
	t.Run("401 on admin path", func(t *testing.T) {
		reached = false
		req := httptest.NewRequest("GET", "/api/admin/v1/scopes", nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "expired-garbage"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for stale cookie on admin path, got %d", w.Code)
		}
		if reached {
			t.Error("handler should not be reached with an invalid session")
		}
	})
}
