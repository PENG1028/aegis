# Gateway Mutation Bypass Audit — v1.7V

## Principle

> Gateway_* tables must NOT be a second source of truth. All writes go through ActionService → real tables (routes, edge_mux_rules, services, endpoints) → safe apply. Gateway_* tables are frozen schema artifacts.

## Search Results

### Direct SQL Writes

Search pattern: `INSERT INTO gateway_`, `UPDATE gateway_`, `DELETE FROM gateway_`

**File:** `internal/gateway/repository.go` — contains all gateway_* SQL:

| Line | SQL | Table |
|------|-----|-------|
| 18 | `INSERT INTO gateway_domains (...)` | gateway_domains |
| 59 | `UPDATE gateway_domains SET ...` | gateway_domains |
| 65 | `DELETE FROM gateway_domains WHERE id=?` | gateway_domains |
| 93 | `INSERT INTO gateway_routes (...)` | gateway_routes |
| 116 | `DELETE FROM gateway_routes WHERE id=?` | gateway_routes |
| 144 | `INSERT INTO gateway_listeners (...)` | gateway_listeners |

**Status:** Repository methods exist. But who calls them?

### Call Path Analysis

Search pattern: `gatewayRepo\.Create|gatewayRepo\.Update|gatewayRepo\.Delete` — **NO MATCHES**

Search pattern: `\.CreateDomain\(|\.AttachRoute\(|\.DetachRoute\(|\.UpdateTLSPolicy\(` — **NO MATCHES in Go source** (only in docs)

**Conclusion:** Gateway repository methods are defined but NEVER called from any handler, service, or CLI code.

### Gateway Service Call Path

**File:** `internal/gateway/service.go`

| Method | Calls Repository? | Called By Handler? |
|--------|:---:|:---:|
| `CreateDomain()` | ✅ (`s.domains.Create(d)`) | ❌ (handler returns 405) |
| `AttachRoute()` | ✅ (`s.routes.Create(rt)`) | ❌ (handler returns 405) |
| `DetachRoute()` | ✅ (`s.routes.Delete(routeID)`) | ❌ (handler returns 405) |
| `UpdateTLSPolicy()` | ✅ (`s.domains.Update(d)`) | ❌ (handler returns 405) |
| `ListDomains()` | ✅ (read-only) | ❌ (handler reads real tables) |
| `ListListeners()` | ✅ (read-only) | ❌ (handler reads real tables) |
| `HealthCheck()` | ✅ (read-only) | May be called for health summary |
| `DisabledActionsForNode()` | ✅ (read-only) | May be called for UI |

### Gateway Handler Status

**File:** `internal/httpapi/handlers/gateway_handler.go`

| Handler | HTTP Method | Returns | Status |
|---------|:---:|--------|--------|
| `CreateGatewayDomain` | POST | 405 GATEWAY_MUTATION_FROZEN | ✅ FROZEN |
| `AttachGatewayRoute` | POST | 405 GATEWAY_MUTATION_FROZEN | ✅ FROZEN |
| `DetachGatewayRoute` | DELETE | 405 GATEWAY_MUTATION_FROZEN | ✅ FROZEN |
| `UpdateTLSPolicy` | PUT | 405 GATEWAY_MUTATION_FROZEN | ✅ FROZEN |
| `ListGatewayDomains` | GET | Reads from routes + managed_domains | ✅ Read-only (real tables) |
| `ListGatewayListeners` | GET | Reads from listeners | ✅ Read-only (real tables) |

---

## Potential Bypass Vectors

### Vector 1: Direct DB Access

**Risk:** Anyone with DB access could execute raw SQL against gateway_* tables.
**Assessment:** This is outside Aegis scope. DB access = full control. Not a bypass vector within the API.

### Vector 2: GatewayService Methods Called from Non-Handler Code

**Search:** `gatewaySvc\.|GatewayService\.|\.CreateDomain|\.AttachRoute|\.DetachRoute`

**Result:** No callers found outside the gateway handler (which is frozen) and gateway service itself.

### Vector 3: Test Code

**Search:** `internal/gateway/` for test files — `internal/gateway/` has no `*_test.go` files.

### Vector 4: Future Code

**Risk:** Future developer could call `GatewayService.CreateDomain()` from a new handler.
**Mitigation:** The `CreateDomain` method signature exists. A comment in the handler states it's frozen, but the method itself has no compile-time guard. This is acceptable — any new handler would need explicit review.

### Vector 5: Migrations Creating gateway_* Tables

**Check:** Migration files create gateway_* tables but only INSERT defaults (if any). No data mutation.

---

## Verdict

| Check | Result |
|-------|--------|
| Gateway mutation handlers return 405 | ✅ All 4 frozen |
| Gateway read handlers use real tables | ✅ ListGatewayDomains → routes+managed_domains, ListGatewayListeners → listeners |
| No code path calls GatewayService mutators | ✅ No callers found |
| No code path writes directly to gateway_* tables | ✅ Repository methods defined but uncalled |
| Gateway repository SQL exists but unreachable | ✅ Dead code (frozen) |
| Gateway service mutator methods exist but unreachable | ✅ Dead code (frozen) |

**Overall:** ✅ **NO BYPASS FOUND.** Gateway_* table write paths are frozen. All mutation endpoints return 405. Read endpoints query real source-of-truth tables. Gateway repository and service write methods are dead code — defined but never invoked.

## Recommendation

The gateway repository write methods and gateway service mutator methods could be deleted to reduce dead code, but keeping them as "schema reference" is acceptable for documentation purposes.
