package httpapi

import (
	"aegis/internal/token"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// Middleware wraps the token auth middleware for the HTTP API.
type Middleware struct {
	auth *token.AuthMiddleware
}

// NewMiddleware creates API middleware.
func NewMiddleware(auth *token.AuthMiddleware) *Middleware {
	return &Middleware{auth: auth}
}

// Auth returns the auth middleware handler.
func (m *Middleware) Auth(next http.Handler) http.Handler {
	return m.auth.Middleware(next)
}

// CORS adds basic CORS headers for development.
func (m *Middleware) CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
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

const ctxKeyRequestID contextKey = "request_id"

// GetRequestID extracts the request ID from context.
func GetRequestID(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}
