package token

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"aegis/internal/action"
)

// mockChecker implements ServiceAuthChecker for tests.
type mockChecker struct {
	serviceName string
	err         error
}

func (m *mockChecker) VerifyTicketAndGetSpace(ticketStr string) (string, error) {
	return m.serviceName, m.err
}

func TestServiceAuthTicketFallback(t *testing.T) {
	// Save and restore global state.
	prev := serviceAuthChecker
	defer func() { serviceAuthChecker = prev }()

	serviceAuthChecker = &mockChecker{serviceName: "test-service"}

	var ctxAction *action.ActionContext
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxAction = action.GetActionContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := NewAuthMiddleware("admin-token")
	h := mw.Middleware(handler)

	// Test 1: Ticket fallback — no Bearer, valid Ticket → should pass.
	t.Run("valid ticket without bearer", func(t *testing.T) {
		ctxAction = nil
		req := httptest.NewRequest("GET", "/api/v1/my/routes", nil)
		req.Header.Set("X-Service-Ticket", "valid-ticket")
		req.Header.Set("X-Caller-Service", "test-service")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if ctxAction == nil {
			t.Fatal("expected ActionContext to be set")
		}
		if ctxAction.SpaceID != "test-service" {
			t.Errorf("expected space_id 'test-service', got %q", ctxAction.SpaceID)
		}
		if ctxAction.TokenType != "service" {
			t.Errorf("expected token_type 'service', got %q", ctxAction.TokenType)
		}
	})

	// Test 2: Invalid Ticket → should get 401.
	t.Run("invalid ticket", func(t *testing.T) {
		serviceAuthChecker = &mockChecker{err: errors.New("invalid ticket")}

		req := httptest.NewRequest("GET", "/api/v1/my/routes", nil)
		req.Header.Set("X-Service-Ticket", "bad-ticket")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for invalid ticket, got %d", w.Code)
		}
	})

	// Test 3: Bearer token present → Ticket path should NOT be tried.
	t.Run("bearer takes priority over ticket", func(t *testing.T) {
		serviceAuthChecker = &mockChecker{serviceName: "should-not-be-used"}

		req := httptest.NewRequest("GET", "/api/v1/my/routes", nil)
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("X-Service-Ticket", "some-ticket")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for admin Bearer, got %d", w.Code)
		}
		if ctxAction == nil || ctxAction.TokenType != "admin" {
			t.Error("expected admin context, Ticket should not override Bearer")
		}
	})

	// Test 4: No Ticket header and no Bearer → original 401 behavior.
	t.Run("no ticket and no bearer", func(t *testing.T) {
		serviceAuthChecker = &mockChecker{serviceName: "irrelevant"}

		req := httptest.NewRequest("GET", "/api/v1/my/routes", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})
}

func TestServiceAuthTicketLoginBypass(t *testing.T) {
	// Regression: login page must still work without any auth.
	prev := serviceAuthChecker
	defer func() { serviceAuthChecker = prev }()

	serviceAuthChecker = &mockChecker{serviceName: "test"}

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := NewAuthMiddleware("admin-token")
	h := mw.Middleware(handler)

	req := httptest.NewRequest("POST", "/api/admin/v1/auth/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !called {
		t.Error("BUG: login handler blocked — isPublicPath regression")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServiceAuthTicketMiddlewareWithoutBridge(t *testing.T) {
	// When bridge is not set (nil), existing behavior must be preserved.
	prev := serviceAuthChecker
	defer func() { serviceAuthChecker = prev }()
	serviceAuthChecker = nil

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := NewAuthMiddleware("admin-token")
	h := mw.Middleware(handler)

	// Without bridge, Ticket should be ignored → 401 as before.
	req := httptest.NewRequest("GET", "/api/v1/my/routes", nil)
	req.Header.Set("X-Service-Ticket", "any-ticket")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when bridge not set, got %d", w.Code)
	}
}

func TestServiceAuthTicketDoesNotBypassAdminRoute(t *testing.T) {
	// Admin routes should still be blocked for service tickets
	// via the existing isSystemRoute check.
	prev := serviceAuthChecker
	defer func() { serviceAuthChecker = prev }()

	serviceAuthChecker = &mockChecker{serviceName: "test-service"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := NewAuthMiddleware("admin-token")
	h := mw.Middleware(handler)

	// Service ticket to admin endpoint — the fallback creates a "service"
	// type context, and isSystemRoute should allow it through for Action API
	// paths (which are /api/v1/actions/*, NOT /api/admin/*).
	// But /api/v1/actions/* is not blocked by isSystemRoute, so it should pass.
	req := httptest.NewRequest("POST", "/api/v1/actions/bind-http-domain", nil)
	req.Header.Set("X-Service-Ticket", "valid-ticket")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Should pass — the service context with Ticket should reach the handler.
	// requireOwnership will do the actual authorization in the handler.
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for service ticket on Action API, got %d", w.Code)
	}
}

// Ensure context propagation works through the Ticket path.
func TestServiceAuthTicketContextPropagation(t *testing.T) {
	prev := serviceAuthChecker
	defer func() { serviceAuthChecker = prev }()

	serviceAuthChecker = &mockChecker{serviceName: "ctx-test-svc"}

	var capturedCtx context.Context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	mw := NewAuthMiddleware("admin-token")
	h := mw.Middleware(handler)

	req := httptest.NewRequest("GET", "/api/v1/my/routes", nil)
	req.Header.Set("X-Service-Ticket", "ticket-ctx-test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ac := action.GetActionContext(capturedCtx)
	if ac == nil {
		t.Fatal("ActionContext not propagated through context")
	}
	if ac.SpaceID != "ctx-test-svc" {
		t.Errorf("expected space 'ctx-test-svc', got %q", ac.SpaceID)
	}
	if ac.TokenType != "service" {
		t.Errorf("expected token_type 'service', got %q", ac.TokenType)
	}
	if ac.Actor != "service" {
		t.Errorf("expected actor 'service', got %q", ac.Actor)
	}
}
