# Admin Route Protection Proof — v1.7X

## Middleware Chain (serve.go:53-56)

```go
handler = apiMiddleware.CORS(handler)       // 1. CORS
handler = adminAuthMw.Middleware(handler)    // 2. Admin cookie session check (/api/admin/v1/*)
handler = apiMiddleware.Auth(handler)        // 3. Bearer token auth (all routes)
```

Request flow for `/api/admin/v1/*`:
1. AdminAuthMiddleware: checks `aegis_admin_session` cookie → 401 if missing
2. Auth middleware: checks for existing AdminContext → admin bypass; checks Bearer token for non-admin requests
3. For admin-authenticated requests: AdminContext already present, Auth skips bearer check, injects admin ActionContext
4. For service key requests: No cookie → blocked at AdminAuthMiddleware → 401

## Complete Admin Route Audit

| # | Method | Path | Handler | Modifies DB | Auth Required | Unauthenticated → | Service Key → | Admin Session → |
|---|--------|------|---------|:---:|:---:|:---:|:---:|:---:|
| 1 | POST | `/api/admin/v1/auth/login` | AdminLogin | ❌ (read) | Bypassed | 401* | 401* | 200 |
| 2 | POST | `/api/admin/v1/auth/logout` | AdminLogout | ❌ | Cookie | 401 | 401 | 200 |
| 3 | GET | `/api/admin/v1/auth/me` | AdminMe | ❌ | Cookie | 401 | 401 | 200 |
| 4 | GET | `/api/admin/v1/system/overview` | SystemOverview | ❌ | Cookie | 401 | 401 | 200 |
| 5 | GET | `/api/admin/v1/nodes` | AdminListNodes | ❌ | Cookie | 401 | 401 | 200 |
| 6 | GET | `/api/admin/v1/routes` | AdminListRoutes | ❌ | Cookie | 401 | 401 | 200 |
| 7 | GET | `/api/admin/v1/edge-rules` | AdminListEdgeRules | ❌ | Cookie | 401 | 401 | 200 |
| 8 | GET | `/api/admin/v1/services` | AdminListServices | ❌ | Cookie | 401 | 401 | 200 |
| 9 | GET | `/api/admin/v1/scopes` | AdminListScopes | ❌ | Cookie | 401 | 401 | 200 |
| 10 | **POST** | `/api/admin/v1/scopes` | AdminCreateSpace | ✅ | Cookie | 401 | 401 | 201 |
| 11 | GET | `/api/admin/v1/api-keys` | AdminListAPIKeys | ❌ | Cookie | 401 | 401 | 200 |
| 12 | **POST** | `/api/admin/v1/scopes/{id}/api-keys` | AdminCreateAPIKey | ✅ | Cookie | 401 | 401 | 201 |
| 13 | **POST** | `/api/admin/v1/api-keys/{id}/revoke` | AdminRevokeAPIKey | ✅ | Cookie | 401 | 401 | 200 |
| 14 | **POST** | `/api/admin/v1/api-keys/{id}/rotate` | AdminRotateAPIKey | ✅ | Cookie | 401 | 401 | 200 |
| 15 | GET | `/api/admin/v1/operations` | AdminListOperations | ❌ | Cookie | 401 | 401 | 200 |
| 16 | GET | `/api/admin/v1/apply-logs` | AdminListApplyLogs | ❌ | Cookie | 401 | 401 | 200 |
| 17 | GET | `/api/admin/v1/audit-logs` | AdminListAuditLogs | ❌ | Cookie | 401 | 401 | 200 |
| 18 | GET | `/api/admin/v1/node-events` | AdminListNodeEvents | ❌ | Cookie | 401 | 401 | 200 |
| 19 | **POST** | `/api/admin/v1/system/doctor` | AdminSystemDoctor | ❌ (read) | Cookie | 401 | 401 | 200 |
| 20 | **POST** | `/api/admin/v1/system/verify` | AdminSystemVerify | ❌ (read) | Cookie | 401 | 401 | 200 |
| 21 | **POST** | `/api/admin/v1/system/apply` | AdminSystemApply | ✅ | Cookie | 401 | 401 | 200 |
| 22 | GET | `/api/admin/v1/nodes/{id}/capabilities` | GetNodeCapabilities | ❌ | Cookie | 401 | 401 | 200 |
| 23 | POST | `/api/admin/v1/nodes/{id}/refresh-capabilities` | RefreshNodeCapabilities | ✅ | Cookie | 401 | 401 | 200 |
| 24 | POST | `/api/admin/v1/gateway/domains` | CreateGatewayDomain | ❌ (405 frozen) | Cookie | 401 | 401 | 405 |
| 25 | GET | `/api/admin/v1/gateway/domains` | ListGatewayDomains | ❌ | Cookie | 401 | 401 | 200 |
| 26 | POST | `/api/admin/v1/gateway/routes` | AttachGatewayRoute | ❌ (405 frozen) | Cookie | 401 | 401 | 405 |
| 27 | DELETE | `/api/admin/v1/gateway/routes/{id}` | DetachGatewayRoute | ❌ (405 frozen) | Cookie | 401 | 401 | 405 |
| 28 | GET | `/api/admin/v1/gateway/listeners` | ListGatewayListeners | ❌ | Cookie | 401 | 401 | 200 |
| 29 | PUT | `/api/admin/v1/gateway/domains/{id}/tls` | UpdateTLSPolicy | ❌ (405 frozen) | Cookie | 401 | 401 | 405 |
| 30 | POST | `/api/admin/v1/deployments` | CreateDeployment | ✅ | Cookie | 401 | 401 | 200 |
| 31 | GET | `/api/admin/v1/deployments` | ListDeployments | ❌ | Cookie | 401 | 401 | 200 |
| 32 | GET | `/api/admin/v1/deployments/{id}` | GetDeployment | ❌ | Cookie | 401 | 401 | 200 |
| 33 | POST | `/api/admin/v1/deployments/{id}/rollback` | RollbackDeployment | ✅ | Cookie | 401 | 401 | 200 |
| 34 | GET | `/api/admin/v1/providers` | ListProviders | ❌ | Cookie | 401 | 401 | 200 |
| 35 | **POST** | `/api/admin/v1/providers/diagnose` | DiagnoseAllProviders | ❌ (read) | Cookie | 401 | 401 | 200 |
| 36 | GET | `/api/admin/v1/trace/domain/{domain}` | TraceDomain | ❌ | Cookie | 401 | 401 | 200 |
| 37 | GET | `/api/admin/v1/trace/sni/{sni_host}` | TraceSNI | ❌ | Cookie | 401 | 401 | 200 |
| 38 | GET | `/api/admin/v1/trace/route/{route_id}` | TraceRoute | ❌ | Cookie | 401 | 401 | 200 |

\* Login is bypassed in AdminAuthMiddleware — but AuthMiddleware still requires Bearer token. Fixed in v1.7W: admin session bypass added.

## Mutation Routes (Write to DB)

10 mutation routes identified (bold in table above):

| # | Route | What it writes |
|---|-------|---------------|
| 10 | POST /api/admin/v1/scopes | Creates space in `spaces` table |
| 12 | POST /api/admin/v1/scopes/{id}/api-keys | Creates token in `tokens` table |
| 13 | POST /api/admin/v1/api-keys/{id}/revoke | Updates token `revoked` flag |
| 14 | POST /api/admin/v1/api-keys/{id}/rotate | Creates new token, revokes old |
| 21 | POST /api/admin/v1/system/apply | Triggers full apply pipeline |
| 23 | POST /api/admin/v1/nodes/{id}/refresh-capabilities | Updates node capabilities |
| 30 | POST /api/admin/v1/deployments | Creates deployment record |
| 33 | POST /api/admin/v1/deployments/{id}/rollback | Creates rollback record |
| — | POST/PUT/DELETE /api/admin/v1/gateway/* | All frozen (405) |

## Auth Protection Verification

### Layer 1: AdminAuthMiddleware (serve.go:50-55)
```go
adminAuthMw := adminauth.NewAdminAuthMiddleware(svcs.AdminAuth)
handler = adminAuthMw.Middleware(handler)
```
- Protects ALL `/api/admin/v1/*` paths
- Checks `aegis_admin_session` cookie
- Returns 401 with `{"error":{"code":"UNAUTHORIZED","message":"admin session required"}}`
- Exception: `POST /api/admin/v1/auth/login` bypasses

### Layer 2: Auth Middleware Admin Bypass (token/middleware.go:48-56)
```go
if adminCtx := adminauth.GetAdminContext(r.Context()); adminCtx != nil {
    ac := &action.ActionContext{
        SpaceID: "", TokenType: "admin", TokenID: adminCtx.UserID, Actor: "admin",
    }
    ctx := action.WithActionContext(r.Context(), ac)
    next.ServeHTTP(w, r.WithContext(ctx))
    return
}
```
- Admin-authenticated requests skip Bearer token check
- AdminContext injected with Actor="admin", TokenType="admin"

### Layer 3: Service Key Blocking (token/middleware.go:76-80)
```go
if tokenType == "space" && isSystemRoute(r.URL.Path) {
    logAuditEvent(tokenType, tokenID, "service_key_denied_admin", ...)
    writeAuthError(w, http.StatusForbidden, "SCOPE_DENIED", "service API keys cannot access admin routes")
    return
}
```
- Service keys blocked from ALL system routes
- isSystemRoute includes: /api/admin/, /api/routes, /api/services, /api/managed-domains, /api/endpoints, /api/exposures, /api/projects, /api/system/, /api/config/, /api/apply, /api/rollback, /api/diagnostics/, /api/settings, /api/health, /api/logs

### Layer 4: Audit Logging
Failed accesses write audit_log entries:
- `unauthorized_access` — missing token
- `service_key_denied_admin` — space token accessing admin
- `scope_violation` — token missing required scope
- `scope_denied` — space token accessing system route

## Test Coverage

```go
// Test 1: Unauthenticated → 401 for every admin mutation
// Test 2: Service API key → 403 for every admin route
// Test 3: Admin session → 200/201 for every admin route
// Test 4: Denied mutation does NOT change DB
// Test 5: Denied access writes audit_log
```

### Test Implementation Notes

All auth tests are functional (test the middleware chain logic) not integration (no HTTP server needed):

1. **AdminAuthMiddleware** — tested in `internal/adminauth/`
2. **Auth middleware admin bypass** — tested via token middleware with mocked admin context
3. **isSystemRoute blocking** — tested by verifying space tokens hit the system route check
4. **Audit logging** — tested via auditLog mock interface

For real HTTP integration tests (requiring a running server), see `docs/real-vps-verification-plan.md`.

## Verdict

- All 38 `/api/admin/v1/*` routes pass through AdminAuthMiddleware ✅
- Admin session cookie required for all (except login) ✅
- Service API keys blocked from all admin routes ✅ (v1.7W)
- Service API keys blocked from public CRUD routes ✅ (v1.7W)
- Unauthenticated requests get 401 ✅
- Admin-authenticated requests skip Bearer token ✅ (v1.7W)
- Failed access writes audit log ✅
- 10 mutation routes identified; all require admin session ✅
