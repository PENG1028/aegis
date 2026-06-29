package httpapi

import (
	"aegis/internal/token"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
)

// Middleware wraps the token auth middleware for the HTTP API.
type Middleware struct {
	auth           *token.AuthMiddleware
	allowedOrigins []string
}

// NewMiddleware creates API middleware.
func NewMiddleware(auth *token.AuthMiddleware, allowedOrigins []string) *Middleware {
	return &Middleware{auth: auth, allowedOrigins: allowedOrigins}
}

// Auth returns the auth middleware handler.
func (m *Middleware) Auth(next http.Handler) http.Handler {
	return m.auth.Middleware(next)
}

// CORS adds CORS headers, validating the Origin header against allowed origins.
// If no origins are configured, reflects the request origin (embedded UI mode).
// Never returns wildcard "*" with credentials.
func (m *Middleware) CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// Same-origin request, no CORS header needed
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		allowed := m.isOriginAllowed(origin)
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == "OPTIONS" {
			if allowed {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusForbidden)
			}
			return
		}

		if !allowed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":{"code":"CORS_DENIED","message":"origin not allowed"}}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isOriginAllowed checks if the origin is in the configured allowed list.
// Empty list (production embedded UI) rejects all cross-origin requests.
func (m *Middleware) isOriginAllowed(origin string) bool {
	for _, o := range m.allowedOrigins {
		if o == origin {
			return true
		}
	}
	return false
}

// RequestID generates or propagates a correlation ID for cross-node tracing.
// Uses incoming X-Request-ID header if present, otherwise generates a new one.
// The ID is added to response headers and available via request context.
func (m *Middleware) RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			var b [8]byte
			rand.Read(b[:])
			reqID = hex.EncodeToString(b[:])
		}
		w.Header().Set("X-Request-ID", reqID)
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID, reqID))
		next.ServeHTTP(w, r)
	})
}

type contextKey string

// Recovery returns a middleware that recovers from panics in downstream handlers.
// Without this, a single nil-dereference or type-assertion panic crashes the entire server.
func (m *Middleware) Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// Log the panic for debugging
				fmt.Printf("[PANIC] %s %s: %v\n", r.Method, r.URL.Path, rec)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"internal server error"}}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

const ctxKeyRequestID contextKey = "request_id"

// GetRequestID extracts the request ID from context.
func GetRequestID(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}
