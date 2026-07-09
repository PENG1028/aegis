package token

import (
	"encoding/json"
	"net/http"
	"strings"

	"aegis/internal/action"
	"aegis/internal/adminauth"
	"aegis/internal/logs"
)

// AuditLogger is the interface for writing audit log entries.
type AuditLogger interface {
	LogAudit(actorType, actorID, eventType, ip, userAgent, targetType, targetID, result, errorCode string)
}

var auditLog logs.AuditLogger

// SetAuditLogger sets the global audit logger for the auth middleware.
func SetAuditLogger(l logs.AuditLogger) {
	auditLog = l
}

// ServiceAuthChecker is an optional bridge that validates serviceauth tickets
// and maps the caller service to an Aegis space for Action API access.
// When set, requests without a Bearer token are checked for a valid
// X-Service-Ticket header and granted space-scoped access automatically.
type ServiceAuthChecker interface {
	VerifyTicketAndGetSpace(ticketStr string) (serviceName string, err error)
}

var serviceAuthChecker ServiceAuthChecker

// SetServiceAuthChecker injects a serviceauth bridge into the auth middleware.
func SetServiceAuthChecker(checker ServiceAuthChecker) {
	serviceAuthChecker = checker
}

// AuthMiddleware provides authentication for the HTTP API.
// It supports three auth methods (tried in order):
//
//  1. Admin session cookie (set by AdminAuthMiddleware) — AdminContext
//  2. Authorization: Bearer token (static admin token, CLI/curl)
//  3. X-Service-Ticket header (service-to-service, via serviceauth bridge)
type AuthMiddleware struct {
	adminToken string
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(adminToken string) *AuthMiddleware {
	return &AuthMiddleware{adminToken: adminToken}
}

// isPublicPath returns true for paths that don't require authentication.
func isPublicPath(path, method string) bool {
	if path == "/api/admin/v1/auth/login" && method == "POST" {
		return true
	}
	// Node API — protected by handler-level authenticateNodeRequest()
	if strings.HasPrefix(path, "/api/node/v1/") {
		return true
	}
	if strings.HasPrefix(path, "/__aegis/") {
		return true
	}
	// v1.9A: Service-Auth SDK endpoints — protected by isInCluster() IP check
	if strings.HasPrefix(path, "/api/service-auth/v1/") && path != "/api/service-auth/v1/services" {
		return true
	}
	if path == "/api/healthz" || path == "/api/readyz" {
		return true
	}
	if path == "/api/system/status" && method == "GET" {
		return true
	}
	if path == "/api/system/runtime-mode" && method == "GET" {
		return true
	}
	if path == "/api/system/compositions" && method == "GET" {
		return true
	}
	// Embedded UI
	if path == "/" || strings.HasPrefix(path, "/assets/") || path == "/favicon.ico" || path == "/favicon.svg" {
		return true
	}
	// SPA routes
	if !strings.HasPrefix(path, "/api/") && !strings.Contains(path, ".") {
		return true
	}
	return false
}

// Middleware returns an HTTP middleware that authenticates requests.
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public paths: no auth required
		if isPublicPath(r.URL.Path, r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		// ① Admin session cookie (set by AdminAuthMiddleware for /api/admin/v1/*)
		if adminCtx := adminauth.GetAdminContext(r.Context()); adminCtx != nil {
			ac := &action.ActionContext{
				SpaceID:   "",
				TokenType: "admin",
				TokenID:   adminCtx.UserID,
				Actor:     "admin",
			}
			ctx := action.WithActionContext(r.Context(), ac)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// ③ Static admin Bearer token
		token := extractBearerToken(r)
		if token != "" && m.adminToken != "" && token == m.adminToken {
			ac := &action.ActionContext{
				SpaceID:   "",
				TokenType: "admin",
				TokenID:   "",
				Actor:     "api",
			}
			ctx := action.WithActionContext(r.Context(), ac)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// ② Service-to-service Ticket (via serviceauth bridge)
		if serviceAuthChecker != nil {
			ticket := r.Header.Get("X-Service-Ticket")
			if ticket != "" {
				serviceName, err := serviceAuthChecker.VerifyTicketAndGetSpace(ticket)
				if err == nil {
					ac := &action.ActionContext{
						SpaceID:   serviceName,
						TokenType: "service",
						TokenID:   serviceName,
						Actor:     "service",
					}
					ctx := action.WithActionContext(r.Context(), ac)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
		}


		logAuditEvent("service_key", "", "unauthorized_access", r, "", "missing_token", "failed", "UNAUTHORIZED")
		writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid auth")
	})
}

// extractBearerToken extracts the Bearer token from an Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// logAuditEvent writes an audit log entry for auth failures.
func logAuditEvent(actorType, actorID, eventType string, r *http.Request, targetType, targetID, result, errorCode string) {
	if auditLog != nil {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}
		auditLog.LogAudit(actorType, actorID, eventType, ip, r.UserAgent(), targetType, targetID, result, errorCode)
	}
}
