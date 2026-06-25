# API Boundary Audit — v1.7R

## Executive Summary

**CRITICAL FINDING**: The admin auth middleware (`AdminAuthMiddleware`) is defined but **never wired** into the HTTP server middleware chain. All routes are protected by Bearer token auth only. Admin endpoints lack scope restrictions in `RequiredScopes` and `/api/admin/` is not in `isSystemRoute()`, meaning space tokens can access admin endpoints.

---

## 1. Middleware Chain Analysis

### Current chain (serve.go:49-51)

```
handler = CORS → Bearer Token Auth → mux
```

### What's missing

```
handler = CORS → Admin Cookie Auth (for /api/admin/v1/*) → Bearer Token Auth → mux
```

The `AdminAuthMiddleware.Middleware()` exists in `internal/adminauth/middleware.go` but is **never instantiated or applied** in the server setup.

---

## 2. Complete Endpoint Inventory

### 2.1 Admin-Only Endpoints (`/api/admin/v1/*`)

These MUST require admin cookie session. Service API keys MUST be denied.

| Method | Path | Handler | Current Scope Check | Auth Bypass Risk |
|--------|------|---------|-------------------|------------------|
| POST | /api/admin/v1/auth/login | AdminLogin | None | Public (as intended) |
| POST | /api/admin/v1/auth/logout | AdminLogout | None | Needs admin session |
| GET | /api/admin/v1/auth/me | AdminMe | None | Needs admin session |
| GET | /api/admin/v1/system/overview | SystemOverview | None | **EXPOSED** — any valid token |
| GET | /api/admin/v1/nodes | AdminListNodes | None | **EXPOSED** |
| GET | /api/admin/v1/routes | AdminListRoutes | None | **EXPOSED** |
| GET | /api/admin/v1/edge-rules | AdminListEdgeRules | None | **EXPOSED** |
| GET | /api/admin/v1/services | AdminListServices | None | **EXPOSED** |
| GET | /api/admin/v1/scopes | AdminListScopes | None | **EXPOSED** |
| POST | /api/admin/v1/scopes | AdminCreateSpace | None | **EXPOSED** |
| GET | /api/admin/v1/api-keys | AdminListAPIKeys | None | **EXPOSED** |
| POST | /api/admin/v1/scopes/{id}/api-keys | AdminCreateAPIKey | None | **EXPOSED** |
| POST | /api/admin/v1/api-keys/{id}/revoke | AdminRevokeAPIKey | None | **EXPOSED** |
| POST | /api/admin/v1/api-keys/{id}/rotate | AdminRotateAPIKey | None | **EXPOSED** |
| GET | /api/admin/v1/operations | AdminListOperations | None | **EXPOSED** |
| GET | /api/admin/v1/apply-logs | AdminListApplyLogs | None | **EXPOSED** |
| GET | /api/admin/v1/audit-logs | AdminListAuditLogs | None | **EXPOSED** |
| GET | /api/admin/v1/node-events | AdminListNodeEvents | None | **EXPOSED** |
| POST | /api/admin/v1/system/doctor | AdminSystemDoctor | None | **EXPOSED** |
| POST | /api/admin/v1/system/verify | AdminSystemVerify | None | **EXPOSED** |
| POST | /api/admin/v1/system/apply | AdminSystemApply | None | **EXPOSED** (triggers safe apply) |
| GET | /api/admin/v1/nodes/{id}/capabilities | GetNodeCapabilities | None | **EXPOSED** |
| POST | /api/admin/v1/nodes/{id}/refresh-capabilities | RefreshNodeCapabilities | None | **EXPOSED** |
| POST | /api/admin/v1/gateway/domains | CreateGatewayDomain | None | **EXPOSED** |
| GET | /api/admin/v1/gateway/domains | ListGatewayDomains | None | **EXPOSED** |
| POST | /api/admin/v1/gateway/routes | AttachGatewayRoute | None | **EXPOSED** |
| DELETE | /api/admin/v1/gateway/routes/{id} | DetachGatewayRoute | None | **EXPOSED** |
| GET | /api/admin/v1/gateway/listeners | ListGatewayListeners | None | **EXPOSED** |
| PUT | /api/admin/v1/gateway/domains/{id}/tls | UpdateTLSPolicy | None | **EXPOSED** |
| POST | /api/admin/v1/deployments | CreateDeployment | None | **EXPOSED** |
| GET | /api/admin/v1/deployments | ListDeployments | None | **EXPOSED** |
| GET | /api/admin/v1/deployments/{id} | GetDeployment | None | **EXPOSED** |
| POST | /api/admin/v1/deployments/{id}/rollback | RollbackDeployment | None | **EXPOSED** |

**Total: 33 admin endpoints, 31 exposed (only login + logout are auth endpoints themselves)**

### 2.2 Service API Endpoints

These are for service API keys (space tokens). Must deny admin tokens conceptually (admin can access everything).

| Method | Path | Handler | Required Scope | Safe Apply? |
|--------|------|---------|---------------|-------------|
| GET | /api/system/status | SystemStatus | system:read | No |
| GET | /api/projects | ListProjects | project:read | No |
| POST | /api/projects | CreateProject | project:write | No |
| GET | /api/projects/{id} | GetProject | project:read | No |
| PATCH | /api/projects/{id} | UpdateProject | project:write | No |
| POST | /api/projects/{id}/archive | ArchiveProject | project:write | No |
| GET | /api/services | ListServices | service:read | No |
| POST | /api/services | CreateService | service:write | No |
| GET | /api/services/{id} | GetService | service:read | No |
| PATCH | /api/services/{id} | UpdateService | service:write | No |
| POST | /api/services/{id}/enable | EnableService | service:write | No |
| POST | /api/services/{id}/disable | DisableService | service:write | No |
| GET | /api/services/{id}/endpoints | ListEndpoints | endpoint:read | No |
| POST | /api/services/{id}/endpoints | CreateEndpoint | endpoint:write | No |
| PATCH | /api/endpoints/{id} | UpdateEndpoint | endpoint:write | No |
| POST | /api/endpoints/{id}/enable | EnableEndpoint | endpoint:write | No |
| POST | /api/endpoints/{id}/disable | DisableEndpoint | endpoint:write | No |
| DELETE | /api/endpoints/{id} | DeleteEndpoint | endpoint:write | No |
| GET | /api/routes | ListRoutes | route:read | No |
| POST | /api/routes | CreateRoute | route:write | **No** |
| GET | /api/routes/{id} | GetRoute | route:read | No |
| PATCH | /api/routes/{id} | UpdateRoute | route:write | **No** |
| POST | /api/routes/{id}/enable | EnableRoute | route:write | **No** |
| POST | /api/routes/{id}/disable | DisableRoute | route:write | **No** |
| POST | /api/routes/{id}/switch-service | SwitchRouteService | route:write | **No** |
| POST | /api/routes/{id}/maintenance-on | RouteMaintenanceOn | route:write | **No** |
| POST | /api/routes/{id}/maintenance-off | RouteMaintenanceOff | route:write | **No** |
| GET | /api/managed-domains | ListManagedDomains | managed_domain:read | No |
| POST | /api/managed-domains | CreateManagedDomain | managed_domain:write | **No** |
| GET | /api/managed-domains/{id} | GetManagedDomain | managed_domain:read | No |
| POST | /api/managed-domains/{id}/verify | VerifyManagedDomain | managed_domain:verify | No |
| POST | /api/managed-domains/{id}/enable | EnableManagedDomain | managed_domain:write | **No** |
| POST | /api/managed-domains/{id}/disable | DisableManagedDomain | managed_domain:write | **No** |
| DELETE | /api/managed-domains/{id} | DeleteManagedDomain | managed_domain:write | **No** |
| GET | /api/config/current | ConfigCurrent | config:read | No |
| GET | /api/config/preview | ConfigPreview | config:read | No |
| GET | /api/config/diff | ConfigDiff | config:read | No |
| POST | /api/apply | ApplyConfig | apply:run | **Yes** |
| POST | /api/apply/dry-run | ApplyDryRun | apply:run | No (dry run) |
| POST | /api/rollback | Rollback | rollback:run | **Yes** |
| GET | /api/apply/history | ApplyHistory | config:read | No |
| GET | /api/exposures | ListExposures | exposure:read | No |
| POST | /api/exposures | CreateExposure | exposure:write | **No** |
| GET | /api/exposures/{id} | GetExposure | exposure:read | No |
| PATCH | /api/exposures/{id} | UpdateExposure | exposure:write | **No** |
| POST | /api/exposures/{id}/activate | ActivateExposure | exposure:write | **No** |
| POST | /api/exposures/{id}/disable | DisableExposure | exposure:write | **No** |
| GET | /api/diagnostics/export | DiagnosticsExport | admin:* only | No |
| GET | /api/health | GetHealth | health:read | No |
| POST | /api/health/check-all | CheckAllHealth | health:run | No |
| GET | /api/health/services/{id} | GetServiceHealth | health:read | No |
| GET | /api/logs | GetLogs | logs:read | No |
| GET | /api/settings | GetSettings | settings:read | No |
| PATCH | /api/settings | UpdateSettings | settings:write | No |

### 2.3 Action API Endpoints (v1.6)

| Method | Path | Handler | Required Scope | Safe Apply? |
|--------|------|---------|---------------|-------------|
| POST | /api/v1/actions/bind-http-domain | BindHTTPDomain | domain:bind | **Yes** ✅ |
| POST | /api/v1/actions/bind-tls-backend | BindTLSBackend | edge:create | **Yes** ✅ |
| PATCH | /api/v1/actions/update-target | UpdateTarget | domain:update | **Yes** ✅ |
| POST | /api/v1/actions/disable-domain | DisableDomain | domain:disable | **Yes** ✅ |
| DELETE | /api/v1/actions/domain | DeleteDomain | domain:disable | **Yes** ✅ |

### 2.4 My Resources (v1.6)

| Method | Path | Handler | Required Scope | Safe Apply? |
|--------|------|---------|---------------|-------------|
| GET | /api/v1/my/routes | ListMyRoutes | read:own | No |
| GET | /api/v1/my/services | ListMyServices | read:own | No |
| GET | /api/v1/my/edge-rules | ListMyEdgeRules | read:own | No |
| GET | /api/v1/my/operations | ListMyOperations | read:own | No |

---

## 3. isSystemRoute Coverage Gap

Current `isSystemRoute()` prefixes:
```
/api/system/, /api/config/, /api/apply, /api/rollback,
/api/diagnostics/, /api/settings, /api/health, /api/logs
```

**MISSING**: `/api/admin/` is NOT blocked for space tokens. Space API keys could theoretically access admin endpoints if they have a valid bearer token. Since admin endpoints have no scope requirements in `RequiredScopes`, the scope check passes vacuously, and since `/api/admin/` is not a system route, space tokens are not blocked.

---

## 4. Fix Plan

### Fix 1: Wire AdminAuthMiddleware into serve.go

The admin cookie auth middleware must run BEFORE the bearer token middleware for admin routes. Since `AdminAuthMiddleware.Middleware()` already passes through non-admin paths, it can be stacked:

```go
adminAuthMw := adminauth.NewAdminAuthMiddleware(svcs.AdminAuth)
handler = apiMiddleware.CORS(handler)
handler = adminAuthMw.Middleware(handler)  // NEW: protects /api/admin/v1/*
handler = apiMiddleware.Auth(handler)       // then Bearer token for everything else
```

### Fix 2: Add /api/admin/ to isSystemRoute

Space tokens must be blocked from admin endpoints at the Bearer token level:
```go
"/api/admin/",
```

### Fix 3: Add admin scope requirements to RequiredScopes

Admin endpoints should require `admin:*` scope as defense-in-depth:
```go
"GET /api/admin/v1/":  ScopeAdminAll,
"POST /api/admin/v1/": ScopeAdminAll,
"PUT /api/admin/v1/":  ScopeAdminAll,
"DELETE /api/admin/v1/": ScopeAdminAll,
```

### Fix 4: Service route handlers that mutate config must trigger safe apply

Several service endpoints (CreateRoute, UpdateRoute, EnableRoute, DisableRoute, etc.) change desired state without triggering safe apply. These need to either:
- Always trigger safe apply after mutation
- Or clearly document that apply must be called separately

---

## 5. Expected End State

After fixes:

```
┌──────────────────────────────────────┐
│  /api/admin/v1/*                     │
│  Auth: Cookie session (admin only)   │
│  + Bearer token (admin:* only)       │
│  Denied: service API keys            │
├──────────────────────────────────────┤
│  /api/v1/actions/*                   │
│  Auth: Bearer token                  │
│  Allowed: admin + service keys       │
│  Space isolation: scope + ownership  │
│  Triggers safe apply: YES            │
├──────────────────────────────────────┤
│  /api/* (service endpoints)          │
│  Auth: Bearer token                  │
│  Allowed: admin + service keys       │
│  Space isolation: scope + ownership  │
│  Triggers safe apply: NO (explicit)  │
└──────────────────────────────────────┘
```
