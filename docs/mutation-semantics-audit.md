# Mutation Semantics Audit — v1.7V

## Design Contract

The system has two mutation paths with different semantics:

| Path | Behavior | Source |
|------|----------|--------|
| **Action API** (`/api/v1/actions/*`) | Mutate → `safeApply()` → config written + provider reloaded | `action/service.go:125` |
| **Admin CRUD** (`/api/routes`, `/api/services`, etc.) | Mutate → `MarkPending()` → admin manually applies later | Design docs, PendingState interface |
| **Manual Apply** | `POST /api/admin/v1/system/apply` → `ClearPending()` on success | `apply/service.go:207-208` |

---

## Endpoint-by-Endpoint Audit

Legend:
- ✅ = correctly implemented
- ❌ = missing/broken
- ⚠️ = partially implemented

### Service Endpoints

| Endpoint | HTTP Method | Modifies State | Triggers safeApply | Calls MarkPending | Writes op_log | Requires Admin |
|----------|:---:|:---:|:---:|:---:|:---:|:---:|
| `POST /api/services` | POST | ✅ | ❌ | ❌ | ❌ | ❌ (public route) |
| `PATCH /api/services/{id}` | PATCH | ✅ | ❌ | ❌ | ❌ | ❌ |
| `POST /api/services/{id}/enable` | POST | ✅ | ❌ | ❌ | ❌ | ❌ |
| `POST /api/services/{id}/disable` | POST | ✅ | ❌ | ❌ | ❌ | ❌ |

**Source:** `internal/httpapi/handlers/service.go`
- `CreateService` (line 21): Calls `h.Service.CreateService()`, writes JSON response. No MarkPending, no safeApply, no op_log.
- `UpdateService`: Calls `h.Service.UpdateService()`. Same gaps.
- All service endpoints are registered on PUBLIC routes (`/api/services/...`), NOT admin-protected.

### Route Endpoints

| Endpoint | HTTP Method | Modifies State | Triggers safeApply | Calls MarkPending | Writes op_log | Requires Admin |
|----------|:---:|:---:|:---:|:---:|:---:|:---:|
| `POST /api/routes` | POST | ✅ | ❌ | ❌ | ❌ | ❌ (public route) |
| `PATCH /api/routes/{id}` | PATCH | ⚠️ (returns 501) | ❌ | ❌ | ❌ | ❌ |
| `POST /api/routes/{id}/enable` | POST | ✅ | ❌ | ❌ | ❌ | ❌ |
| `POST /api/routes/{id}/disable` | POST | ✅ | ❌ | ❌ | ❌ | ❌ |

**Source:** `internal/httpapi/handlers/route.go`
- `CreateRoute` (line 21): Calls `h.Route.CreateRoute()`. No MarkPending, no safeApply, no op_log.
- `UpdateRoute` (line 50): Returns HTTP 501 "not implemented yet".
- All route endpoints are on PUBLIC routes (`/api/routes/...`).

### Edge Rule Endpoints

| Endpoint | HTTP Method | Modifies State | Triggers safeApply | Calls MarkPending | Writes op_log | Requires Admin |
|----------|:---:|:---:|:---:|:---:|:---:|:---:|
| Edge rule CRUD (via `edgemux.AppService`) | — | ✅ (if called) | ❌ | ❌ | ❌ | ❌ |

**Source:** Edge rule CRUD is available via `internal/edgemux/` but not directly exposed as standalone HTTP endpoints. Edge rules are managed via Action API (bind-tls-backend) or route lifecycle sync.

### Listener Endpoints

| Endpoint | HTTP Method | Modifies State | Triggers safeApply | Calls MarkPending | Writes op_log | Requires Admin |
|----------|:---:|:---:|:---:|:---:|:---:|:---:|
| Listener updates | — | N/A (no HTTP CRUD) | N/A | N/A | N/A | N/A |

**Source:** Listeners are registered via `ListenerService.RegisterDefaults()` during bootstrap. No update/delete HTTP endpoints exist.

### Managed Domain Endpoints

| Endpoint | HTTP Method | Modifies State | Triggers safeApply | Calls MarkPending | Writes op_log | Requires Admin |
|----------|:---:|:---:|:---:|:---:|:---:|:---:|
| `POST /api/managed-domains` | POST | ✅ | ❌ | ❌ | ❌ | ❌ (public route) |
| `DELETE /api/managed-domains/{id}` | DELETE | ✅ | ❌ | ❌ | ❌ | ❌ |

**Source:** `internal/httpapi/handlers/managed_domain.go`

### Scope Endpoints

| Endpoint | HTTP Method | Modifies State | Triggers safeApply | Calls MarkPending | Writes op_log | Requires Admin |
|----------|:---:|:---:|:---:|:---:|:---:|:---:|
| `POST /api/admin/v1/scopes` | POST | ✅ | N/A | N/A | ❌ | ✅ |
| `PATCH /api/admin/v1/scopes/{id}` | PATCH | ✅ (if exists) | N/A | N/A | ❌ | ✅ |

**Note:** Scope changes don't affect provider config, so MarkPending/safeApply is N/A.

### API Key Endpoints

| Endpoint | HTTP Method | Modifies State | Triggers safeApply | Calls MarkPending | Writes op_log | Requires Admin |
|----------|:---:|:---:|:---:|:---:|:---:|:---:|
| `POST /api/admin/v1/scopes/{id}/api-keys` | POST | ✅ | N/A | N/A | ❌ | ✅ |
| `POST /api/admin/v1/api-keys/{id}/revoke` | POST | ✅ | N/A | N/A | ❌ | ✅ |
| `POST /api/admin/v1/api-keys/{id}/rotate` | POST | ✅ | N/A | N/A | ❌ | ✅ |

---

## Action API Endpoints

| Endpoint | Triggers safeApply | Code Evidence |
|----------|:---:|------|
| `POST /api/v1/actions/bind-http-domain` | ✅ | `action/bind_http_domain.go:134` |
| `POST /api/v1/actions/bind-tls-backend` | ✅ | `action/bind_tls_backend.go:91` |
| `PATCH /api/v1/actions/update-target` (service) | ✅ | `action/update_target.go:67` |
| `PATCH /api/v1/actions/update-target` (edge_rule) | ✅ | `action/update_target.go:105` |
| `POST /api/v1/actions/disable-domain` (route) | ✅ | `action/disable_domain.go:59` |
| `POST /api/v1/actions/disable-domain` (edge_rule) | ✅ | `action/disable_domain.go:100` |
| `DELETE /api/v1/actions/domain` (route) | ✅ | `action/delete_domain.go:33` |
| `DELETE /api/v1/actions/domain` (edge_rule) | ✅ | `action/delete_domain.go:64` |

**Status:** ✅ All Action API endpoints correctly trigger `safeApply()` after mutation.

---

## MarkPending Gap Analysis

### The Gap

**Expected behavior (from design docs):**
```
Admin CRUD mutation → MarkPending(reason) → admin manually triggers apply → ClearPending on success
```

**Actual behavior:**
```
Admin CRUD mutation → state changed in DB → nothing else happens
```

### Code Evidence

1. **MarkPending is defined** — `internal/cluster/pending_state.go:27`
2. **ClearPending is defined** — `internal/cluster/pending_state.go:52`
3. **MarkPending is tested** — `internal/cluster/pending_state_test.go` (5 tests)
4. **MarkPending is NEVER called outside tests** — `grep -rn "\.MarkPending" internal/` returns ONLY:
   - `internal/cluster/pending_state.go:27` (definition)
   - `internal/cluster/pending_state_test.go` (tests)
   - `internal/apply/service.go:27` (interface definition only)
   - `internal/action/` — NOT called (action API uses safeApply directly, which is correct)
5. **Admin CRUD handlers NEVER call MarkPending** — verified for all 5 handler files

### Impact

| Affected Endpoint | Impact |
|-------------------|--------|
| `POST /api/services` | Changed service not visible to provider until admin manually discovers and applies |
| `POST /api/routes` | Changed route not visible to provider |
| `POST /api/services/{id}/enable` | Enable/disable not effective until apply |
| `POST /api/managed-domains` | Domain change not applied |
| `POST /api/routes/{id}/disable` | Route still active in config |

The admin has NO signal that config is out of date. `pending_apply` stays `false` after admin CRUD changes because MarkPending is never called.

---

## Issue: Admin CRUD Routes Not Admin-Protected

**Finding:** Service, route, endpoint, managed domain, and exposure CRUD endpoints are registered on PUBLIC routes:
```go
// routes.go — these are NOT under /api/admin/v1/
mux.HandleFunc("POST /api/routes", h.CreateRoute)
mux.HandleFunc("POST /api/services", h.CreateService)
mux.HandleFunc("POST /api/managed-domains", h.CreateManagedDomain)
```

Any valid bearer token (including service keys) can call these endpoints. They are NOT protected by admin session auth.

**Note:** The admin middleware only wraps routes under `/api/admin/` prefix (see `isSystemRoute`). The CRUD endpoints at `/api/routes`, `/api/services`, etc. are authenticated but NOT admin-restricted.

---

## Fix Plan

### Fix 1: Wire MarkPending into Admin CRUD Handlers

Each config-affecting Admin CRUD handler must call `h.PendingState.MarkPending(reason)` after mutation.

**Affected files:**
- `internal/httpapi/handlers/service.go` — CreateService, UpdateService, EnableService, DisableService
- `internal/httpapi/handlers/route.go` — CreateRoute, EnableRoute, DisableRoute
- `internal/httpapi/handlers/managed_domain.go` — CreateManagedDomain, EnableManagedDomain, DisableManagedDomain, DeleteManagedDomain

### Fix 2: Operation Logging for Admin CRUD

Each mutation handler should call `h.Logs.Log()` to record the operation.

### Fix 3 (Deferred): Admin Route Protection

Move config-affecting CRUD under `/api/admin/v1/` prefix or add admin session check. This is a security boundary issue but changing route paths could break existing clients. Document as known limitation.

---

## Correctness Summary

| Semantics Contract | Action API | Admin CRUD |
|--------------------|:---:|:---:|
| Mutate desired state | ✅ | ✅ |
| Trigger safeApply | ✅ | ❌ (by design — manual) |
| MarkPending on mutation | N/A (auto-apply) | ❌ MISSING |
| ClearPending on apply success | ✅ (via safeApply) | ✅ (via manual apply) |
| Write operation_log | ✅ | ❌ MISSING |
| Admin-only access | ✅ (token scope) | ❌ (public route) |
