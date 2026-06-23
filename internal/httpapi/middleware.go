package httpapi

import (
	"aegis/internal/token"
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
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
