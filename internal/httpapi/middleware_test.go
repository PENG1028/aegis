package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aegis/internal/token"
)

// ── CORS middleware tests (H2 fix) ──

func TestCORS_SameOriginPasses(t *testing.T) {
	mw := NewMiddleware(nil, []string{"http://localhost:5173"})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := mw.CORS(handler)

	// Same-origin request (no Origin header)
	req := httptest.NewRequest("GET", "/api/projects", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("same-origin request should pass, got %d", w.Code)
	}
}

func TestCORS_AllowedOriginPasses(t *testing.T) {
	mw := NewMiddleware(nil, []string{"http://localhost:5173"})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := mw.CORS(handler)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("allowed origin should pass, got %d", w.Code)
	}
	aco := w.Header().Get("Access-Control-Allow-Origin")
	if aco != "http://localhost:5173" {
		t.Errorf("expected Allow-Origin 'http://localhost:5173', got %q", aco)
	}
}

func TestCORS_DisallowedOriginBlocked(t *testing.T) {
	mw := NewMiddleware(nil, []string{"http://localhost:5173"})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for disallowed origin")
	})

	h := mw.CORS(handler)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("disallowed origin should get 403, got %d", w.Code)
	}
}

func TestCORS_EmptyAllowedOriginsRejectsAll(t *testing.T) {
	mw := NewMiddleware(nil, []string{}) // production mode: no cross-origin allowed
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for cross-origin when no origins allowed")
	})

	h := mw.CORS(handler)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("empty allowed origins should reject all cross-origin, got %d", w.Code)
	}
}

func TestCORS_OptionsPreflightAllowed(t *testing.T) {
	mw := NewMiddleware(nil, []string{"http://localhost:5173"})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should NOT be called for OPTIONS preflight
		t.Error("handler should not be called for OPTIONS preflight")
	})

	h := mw.CORS(handler)

	req := httptest.NewRequest("OPTIONS", "/api/projects", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("OPTIONS preflight for allowed origin should return 200, got %d", w.Code)
	}
}

func TestCORS_OptionsPreflightBlocked(t *testing.T) {
	mw := NewMiddleware(nil, []string{"http://localhost:5173"})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for blocked OPTIONS")
	})

	h := mw.CORS(handler)

	req := httptest.NewRequest("OPTIONS", "/api/projects", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("OPTIONS from disallowed origin should return 403, got %d", w.Code)
	}
}

func TestCORS_NoWildcardWithCredentials(t *testing.T) {
	mw := NewMiddleware(nil, []string{"http://localhost:5173"})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := mw.CORS(handler)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	aco := w.Header().Get("Access-Control-Allow-Origin")
	if aco == "*" {
		t.Error("Access-Control-Allow-Origin must NOT be '*' when credentials are used")
	}
	creds := w.Header().Get("Access-Control-Allow-Credentials")
	if creds != "true" {
		t.Error("Access-Control-Allow-Credentials should be 'true'")
	}
}

func TestCORS_MultipleAllowedOrigins(t *testing.T) {
	mw := NewMiddleware(nil, []string{
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://localhost:7380",
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, origin := range mw.allowedOrigins {
		t.Run("origin="+origin, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/projects", nil)
			req.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			mw.CORS(handler).ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("origin %q should be allowed, got %d", origin, w.Code)
			}
		})
	}
}

// ── Recovery middleware tests (M4 fix) ──

func TestRecovery_CatchesPanic(t *testing.T) {
	mw := NewMiddleware(token.NewAuthMiddleware("test-token"), nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	h := mw.Recovery(handler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	// This must NOT panic
	h.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("recovery should return 500, got %d", w.Code)
	}
	body := w.Body.String()
	if body == "" {
		t.Error("recovery should return an error body")
	}
}

func TestRecovery_PassesNormalResponse(t *testing.T) {
	mw := NewMiddleware(token.NewAuthMiddleware("test-token"), nil)
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	})

	h := mw.Recovery(handler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !called {
		t.Error("normal handler should be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("normal response should be 200, got %d", w.Code)
	}
}

func TestRecovery_NilPointerPanic(t *testing.T) {
	mw := NewMiddleware(token.NewAuthMiddleware("test-token"), nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p *struct{ X int }
		_ = p.X // nil pointer dereference
	})

	h := mw.Recovery(handler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	// Must not crash
	h.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("nil pointer panic should return 500, got %d", w.Code)
	}
}

// ── isOriginAllowed tests ──

func TestIsOriginAllowed_EmptyList(t *testing.T) {
	mw := NewMiddleware(nil, []string{})
	if mw.isOriginAllowed("http://anything.com") {
		t.Error("empty allowed list should reject all origins")
	}
}

func TestIsOriginAllowed_ExactMatch(t *testing.T) {
	mw := NewMiddleware(nil, []string{"http://localhost:5173"})
	if !mw.isOriginAllowed("http://localhost:5173") {
		t.Error("exact match should be allowed")
	}
	// v1.8L-20: dev mode allows any localhost/127.0.0.1 origin regardless of port
	if !mw.isOriginAllowed("http://localhost:5174") {
		t.Error("dev mode should allow any localhost port")
	}
	if !mw.isOriginAllowed("http://127.0.0.1:3000") {
		t.Error("dev mode should allow 127.0.0.1")
	}
	// Non-localhost origins should still be rejected
	if mw.isOriginAllowed("http://example.com:5173") {
		t.Error("non-localhost origin should be rejected")
	}
}
