package token

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestServiceTicketCannotTraversalIntoAdmin checks that the allowlist cannot be
// defeated with a dot-segment path.
//
// The guard runs in middleware, i.e. BEFORE http.ServeMux cleans the path, so
// r.URL.Path here is whatever the client sent. A path like
// "/api/v1/my/../admin/v1/scopes" literally starts with an allowed prefix while
// resolving to an admin route — so prefix matching alone must not be the last
// word. Anything that is not denied outright must at minimum not reach an admin
// handler with a service identity attached.
func TestServiceTicketCannotTraversalIntoAdmin(t *testing.T) {
	prev := serviceAuthChecker
	defer func() { serviceAuthChecker = prev }()
	serviceAuthChecker = &mockChecker{serviceName: "test-service"}

	// Stand in for the real router: registers an admin route so a successful
	// traversal would visibly land on it.
	mux := http.NewServeMux()
	adminHit := false
	mux.HandleFunc("GET /api/admin/v1/scopes", func(w http.ResponseWriter, _ *http.Request) {
		adminHit = true
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /api/v1/my/routes", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := NewAuthMiddleware("admin-token").Middleware(mux)

	// Each of these carries an allowlisted prefix literally but resolves onto the
	// admin surface (or onto nothing routable) once dot segments are applied.
	traversals := []string{
		"/api/v1/my/../../../api/admin/v1/scopes",
		"/api/v1/actions/../../../api/admin/v1/scopes",
		"/api/service-auth/v1/../../../api/admin/v1/scopes",
		"/api/v1/my/../admin/v1/scopes",      // → /api/v1/admin/... (not allowlisted)
		"/api/v1/my/./../../admin/v1/scopes", // mixed . and ..
	}

	for _, p := range traversals {
		t.Run(p, func(t *testing.T) {
			adminHit = false
			req := httptest.NewRequest("GET", p, nil)
			req.Header.Set("X-Service-Ticket", "valid-ticket")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if adminHit {
				t.Errorf("admin handler reached via traversal %q (status %d)", p, w.Code)
			}
			// The guard now cleans the path itself, so these are denied outright
			// rather than relying on the mux's 301-to-clean-path behavior.
			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 for traversal %q, got %d", p, w.Code)
			}
		})
	}
}

// TestNormalizeGuardPathPreservesDirectoryPrefixes pins the trailing-slash
// behavior the allowlist depends on: entries are directory prefixes, and
// path.Clean would otherwise strip the slash off "/api/v1/my/".
func TestNormalizeGuardPathPreservesDirectoryPrefixes(t *testing.T) {
	cases := map[string]string{
		"/api/v1/my/":       "/api/v1/my/",
		"/api/v1/my/routes": "/api/v1/my/routes",
		// ".." removes the single preceding segment ("my"), so this resolves to
		// /api/v1/admin/... — not an admin route, and not in the allowlist.
		"/api/v1/my/../admin/v1/scopes": "/api/v1/admin/v1/scopes",
		// Enough ".." to actually reach the admin surface.
		"/api/v1/my/../../../api/admin/v1/scopes": "/api/admin/v1/scopes",
		"/api/service-auth/v1/../admin/v1/scopes": "/api/service-auth/admin/v1/scopes",
		"/api/v1/my/./routes":                     "/api/v1/my/routes",
		"/api/v1/my//routes":                      "/api/v1/my/routes",
		"":                                        "/",
	}

	for in, want := range cases {
		if got := normalizeGuardPath(in); got != want {
			t.Errorf("normalizeGuardPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestAllowedPathsStillPassAfterNormalization guards against the cleaning step
// breaking legitimate service traffic.
func TestAllowedPathsStillPassAfterNormalization(t *testing.T) {
	prev := serviceAuthChecker
	defer func() { serviceAuthChecker = prev }()
	serviceAuthChecker = &mockChecker{serviceName: "test-service"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := NewAuthMiddleware("admin-token").Middleware(handler)

	for _, p := range []string{
		"/api/v1/my/routes",
		"/api/v1/actions/bind-http-domain",
		"/api/service-auth/v1/sync",
	} {
		req := httptest.NewRequest("GET", p, nil)
		req.Header.Set("X-Service-Ticket", "valid-ticket")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200 after normalization, got %d", p, w.Code)
		}
	}
}
