package token

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestServiceTicketDeniedOnSystemAndBusinessRoutes is the regression test for the
// auth bypass where a valid X-Service-Ticket reached admin and business handlers.
//
// The previous test suite asserted this protection by re-implementing the path
// predicate inside the test, so it passed while production had no such check.
// These cases drive the real middleware instead.
func TestServiceTicketDeniedOnSystemAndBusinessRoutes(t *testing.T) {
	prev := serviceAuthChecker
	defer func() { serviceAuthChecker = prev }()
	serviceAuthChecker = &mockChecker{serviceName: "test-service"}

	reached := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	h := NewAuthMiddleware("admin-token").Middleware(handler)

	denied := []struct{ method, path string }{
		{"GET", "/api/admin/v1/scopes"},       // admin inventory
		{"GET", "/api/admin/v1/routes"},       // admin route list
		{"GET", "/api/admin/v1/certificates"}, // cert store
		{"POST", "/api/admin/v1/providers/caddy/reload"},
		{"GET", "/api/admin/v1/credentials"}, // decryptable secrets
		{"POST", "/api/routes"},              // business CRUD, no ownership check
		{"POST", "/api/apply"},               // config push
		{"POST", "/api/rollback"},            // config rollback
		{"GET", "/api/projects"},
		{"POST", "/api/exposures"},
		{"PATCH", "/api/admin/v1/settings"},
	}

	for _, tc := range denied {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			reached = false
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-Service-Ticket", "valid-ticket")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403, got %d", w.Code)
			}
			if reached {
				t.Error("handler was reached — service ticket bypassed the guard")
			}
		})
	}
}

// TestServiceTicketAllowedOnOwnSurfaces guards against over-blocking: the fix
// must not break the surfaces services legitimately depend on.
func TestServiceTicketAllowedOnOwnSurfaces(t *testing.T) {
	prev := serviceAuthChecker
	defer func() { serviceAuthChecker = prev }()
	serviceAuthChecker = &mockChecker{serviceName: "test-service"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := NewAuthMiddleware("admin-token").Middleware(handler)

	allowed := []struct{ method, path string }{
		{"POST", "/api/v1/actions/bind-http-domain"},
		{"POST", "/api/v1/actions/bind-tls-backend"},
		{"PATCH", "/api/v1/actions/update-target"},
		{"DELETE", "/api/v1/actions/domain"},
		{"GET", "/api/v1/my/routes"},
		{"GET", "/api/v1/my/services"},
		{"GET", "/api/v1/my/operations"},
		{"GET", "/api/service-auth/v1/services"},
	}

	for _, tc := range allowed {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-Service-Ticket", "valid-ticket")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d — legitimate service surface blocked", w.Code)
			}
		})
	}
}

// TestAdminPathsUnaffectedByServiceGuard confirms the guard is scoped to service
// callers only: admin Bearer tokens must still reach admin routes.
func TestAdminPathsUnaffectedByServiceGuard(t *testing.T) {
	prev := serviceAuthChecker
	defer func() { serviceAuthChecker = prev }()
	serviceAuthChecker = &mockChecker{serviceName: "test-service"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := NewAuthMiddleware("admin-token").Middleware(handler)

	// Admin Bearer on an admin route, with a service ticket also present:
	// Bearer wins (checked first) and must not be downgraded to service scope.
	req := httptest.NewRequest("GET", "/api/admin/v1/scopes", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("X-Service-Ticket", "valid-ticket")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for admin Bearer on admin route, got %d", w.Code)
	}
}
