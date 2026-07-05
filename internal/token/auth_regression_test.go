package token

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// v1.7Y Bug 1: Login blocked by Bearer middleware.
// Regression: POST /api/admin/v1/auth/login must NOT require Bearer token.

func TestAuthMiddlewareBypassesLoginPath(t *testing.T) {
	// Create a simple test handler that records if it was called
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := NewAuthMiddleware("test-admin-token")
	h := mw.Middleware(handler)

	// Test: login POST without Bearer token — should pass through
	req := httptest.NewRequest("POST", "/api/admin/v1/auth/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !called {
		t.Error("Bug 1 regression: login handler was NOT called — middleware blocked it with 'missing Authorization header'")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	t.Log("Bug 1 regression PASS: login bypasses Bearer token check")
}

func TestAuthMiddlewareBlocksNonLoginWithoutToken(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	mw := NewAuthMiddleware("test-admin-token")
	h := mw.Middleware(handler)

	// Non-login path without token — should be rejected
	req := httptest.NewRequest("GET", "/api/admin/v1/scopes", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if called {
		t.Error("non-login path without token should NOT reach handler")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	t.Log("Bug 1 regression PASS: non-admin paths correctly require Bearer token")
}

func TestAuthMiddlewareLoginWrongPassword(t *testing.T) {
	// The login handler itself should return "invalid credentials" not "missing Authorization header"
	// This tests that the middleware lets the request through to the handler
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// Simulate login handler behavior with wrong credentials
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid credentials"}`))
	})

	mw := NewAuthMiddleware("test-admin-token")
	h := mw.Middleware(handler)

	req := httptest.NewRequest("POST", "/api/admin/v1/auth/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !called {
		t.Fatal("Bug 1 regression: login handler not reached")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", w.Code)
	}
	body := w.Body.String()
	if body != `{"error":"invalid credentials"}` {
		t.Errorf("expected invalid credentials error, got: %s", body)
	}
	t.Log("Bug 1 regression PASS: wrong password returns 'invalid credentials', not 'missing Authorization header'")
}

// C1 fix: Node join endpoint must bypass Bearer token middleware.
// New nodes have no credentials — the join token in the request body proves eligibility.
func TestAuthMiddlewareBypassesNodeJoin(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := NewAuthMiddleware("test-admin-token")
	h := mw.Middleware(handler)

	// POST /api/node/v1/join without Bearer token — must pass through
	req := httptest.NewRequest("POST", "/api/node/v1/join", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !called {
		t.Error("C1 regression: node join handler was NOT called — middleware blocked it")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	t.Log("C1 fix PASS: node join bypasses Bearer token check")
}

func TestAuthMiddlewareBypassesNodeJoinOnlyPOST(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	mw := NewAuthMiddleware("test-admin-token")
	h := mw.Middleware(handler)

	// GET /api/node/v1/join — should NOT bypass (only POST is allowed)
	req := httptest.NewRequest("GET", "/api/node/v1/join", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if called {
		t.Error("GET /api/node/v1/join should NOT bypass Bearer token check")
	}
	// Should get 401 (missing token), not reach handler
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for GET /api/node/v1/join without token, got %d", w.Code)
	}
	t.Log("C1 fix PASS: only POST /api/node/v1/join bypasses auth, GET is blocked")
}
