package token

import "strings"

// Scope constants.
const (
	ScopeSystemRead  = "system:read"

	ScopeProjectRead  = "project:read"
	ScopeProjectWrite = "project:write"

	ScopeServiceRead  = "service:read"
	ScopeServiceWrite = "service:write"

	ScopeEndpointRead  = "endpoint:read"
	ScopeEndpointWrite = "endpoint:write"

	ScopeRouteRead  = "route:read"
	ScopeRouteWrite = "route:write"

	ScopeManagedDomainRead   = "managed_domain:read"
	ScopeManagedDomainWrite  = "managed_domain:write"
	ScopeManagedDomainVerify = "managed_domain:verify"

	ScopeConfigRead = "config:read"
	ScopeApplyRun   = "apply:run"
	ScopeRollbackRun = "rollback:run"

	ScopeHealthRead = "health:read"
	ScopeHealthRun  = "health:run"

	ScopeLogsRead = "logs:read"

	ScopeSettingsRead  = "settings:read"
	ScopeSettingsWrite = "settings:write"

	ScopeExposureRead  = "exposure:read"
	ScopeExposureWrite = "exposure:write"

	ScopeAdminAll = "admin:*"
)

// RequiredScopes maps API path patterns to required scopes.
// The value is a comma-separated list of required scopes (at least one must match).
var RequiredScopes = map[string]string{
	// System
	"GET /api/system/status": ScopeSystemRead,

	// Projects
	"GET /api/projects":         ScopeProjectRead,
	"POST /api/projects":        ScopeProjectWrite,
	"GET /api/projects/":        ScopeProjectRead,
	"PATCH /api/projects/":      ScopeProjectWrite,
	"POST /api/projects/archive": ScopeProjectWrite,

	// Services
	"GET /api/services":        ScopeServiceRead,
	"POST /api/services":       ScopeServiceWrite,
	"GET /api/services/":       ScopeServiceRead,
	"PATCH /api/services/":     ScopeServiceWrite,
	"POST /api/services/enable":  ScopeServiceWrite,
	"POST /api/services/disable": ScopeServiceWrite,

	// Endpoints
	"GET /api/services/endpoints":  ScopeEndpointRead,
	"POST /api/services/endpoints": ScopeEndpointWrite,
	"PATCH /api/endpoints/":        ScopeEndpointWrite,
	"POST /api/endpoints/enable":   ScopeEndpointWrite,
	"POST /api/endpoints/disable":  ScopeEndpointWrite,
	"DELETE /api/endpoints/":       ScopeEndpointWrite,

	// Routes
	"GET /api/routes":          ScopeRouteRead,
	"POST /api/routes":         ScopeRouteWrite,
	"GET /api/routes/":         ScopeRouteRead,
	"PATCH /api/routes/":       ScopeRouteWrite,
	"POST /api/routes/enable":    ScopeRouteWrite,
	"POST /api/routes/disable":   ScopeRouteWrite,
	"POST /api/routes/switch":    ScopeRouteWrite,
	"POST /api/routes/maintenance": ScopeRouteWrite,

	// Managed Domains
	"GET /api/managed-domains":  ScopeManagedDomainRead,
	"POST /api/managed-domains": ScopeManagedDomainWrite,
	"GET /api/managed-domains/": ScopeManagedDomainRead,
	"POST /api/managed-domains/verify": ScopeManagedDomainVerify,
	"POST /api/managed-domains/enable": ScopeManagedDomainWrite,
	"POST /api/managed-domains/disable":ScopeManagedDomainWrite,
	"DELETE /api/managed-domains/":     ScopeManagedDomainWrite,

	// Config / Apply
	"GET /api/config/":    ScopeConfigRead,
	"POST /api/apply":     ScopeApplyRun,
	"POST /api/rollback":  ScopeRollbackRun,
	"GET /api/apply/":     ScopeConfigRead,

	// Health
	"GET /api/health":         ScopeHealthRead,
	"POST /api/health/":       ScopeHealthRun,
	"GET /api/health/services/": ScopeHealthRead,

	// Logs
	"GET /api/logs": ScopeLogsRead,

	// Settings
	"GET /api/settings":  ScopeSettingsRead,
	"PATCH /api/settings": ScopeSettingsWrite,

	// Exposures
	"GET /api/exposures":      ScopeExposureRead,
	"POST /api/exposures":     ScopeExposureWrite,
	"GET /api/exposures/":     ScopeExposureRead,
	"PATCH /api/exposures/":   ScopeExposureWrite,
	"POST /api/exposures/activate":  ScopeExposureWrite,
	"POST /api/exposures/disable":   ScopeExposureWrite,
	"DELETE /api/exposures/":  ScopeExposureWrite,

	// Diagnostics (needs admin or system+logs+config)
	"GET /api/diagnostics/": ScopeAdminAll,
}

// HasScope checks if a token has a specific scope.
// admin:* grants all scopes.
func HasScope(scopes []string, required string) bool {
	for _, s := range scopes {
		if s == ScopeAdminAll {
			return true
		}
		if strings.EqualFold(s, required) {
			return true
		}
	}
	return false
}

// FindMatchingScope finds the required scope for an HTTP method and path.
// Returns the required scope string and whether a match was found.
func FindMatchingScope(method, path string) (string, bool) {
	// Try exact match first
	if scope, ok := RequiredScopes[method+" "+path]; ok {
		return scope, true
	}

	// Try prefix matching for paths with IDs
	for pattern, scope := range RequiredScopes {
		parts := strings.SplitN(pattern, " ", 2)
		if len(parts) != 2 || parts[0] != method {
			continue
		}
		patternPath := parts[1]
		// Check if path starts with the pattern path prefix
		if strings.HasPrefix(path, patternPath) {
			return scope, true
		}
	}

	return "", false
}
