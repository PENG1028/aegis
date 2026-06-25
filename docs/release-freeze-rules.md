# Release Freeze Rules — v1.7Z

## Purpose

These rules prevent accidental breakage of the verified single-node production path.
Any change that violates these rules requires explicit review and regression test updates before merging.

---

## Rule 1: No Gateway Mutation Bypass

> Do NOT add new write paths to `gateway_*` tables.

**Why:** Gateway_* tables are frozen schema artifacts. All domain/route/listener mutations must go through ActionService → real tables (routes, edge_mux_rules, services, endpoints).

**Checklist:**
- [ ] No `INSERT INTO gateway_domains` outside migration
- [ ] No `INSERT INTO gateway_routes` outside migration
- [ ] No `INSERT INTO gateway_listeners` outside migration
- [ ] No handler that writes to gateway_* tables
- [ ] Gateway mutation handlers must return 405 GATEWAY_MUTATION_FROZEN

**Regression guard:** `TestGatewayMutationFrozen` in smoke tests

---

## Rule 2: No Service API Bypass of ActionService

> Do NOT add new direct mutation endpoints for service keys that bypass ActionService.

**Why:** ActionService enforces scope, ownership, and safe apply. Direct CRUD bypasses all three.

**Checklist:**
- [ ] New mutation endpoint for service keys → must go through ActionService
- [ ] New admin CRUD endpoint → must call MarkPending
- [ ] New admin CRUD endpoint → must write operation_log
- [ ] New admin CRUD endpoint → must require admin session

**Regression guard:** `isSystemRoute` covers all CRUD paths; admin middleware covers `/api/admin/v1/*`

---

## Rule 3: No Admin Route Without AdminAuth

> Do NOT add new `/api/admin/v1/*` routes without AdminAuthMiddleware protection.

**Why:** The AdminAuth middleware checks for a valid session cookie. Routes outside this check are accessible with any Bearer token.

**Checklist:**
- [ ] New handler registered under `/api/admin/v1/*`
- [ ] Handler receives AdminContext via `adminauth.GetAdminContext()`
- [ ] AdminAuthMiddleware runs before Auth middleware for this path
- [ ] Service key denied access to this path

**Regression guard:** `TestAuthMiddlewareBlocksNonLoginWithoutToken`

---

## Rule 4: Config Path Consistency

> Do NOT change bootstrap config path without updating config loader.

**Why:** If bootstrap saves to path X but the config loader reads from path Y, the server runs with default config instead of user config.

**Checklist:**
- [ ] Bootstrap `config.Save()` path matches a path in `main.go`'s `defaultPaths`
- [ ] At least one `defaultPaths` entry covers the nested `config/config.yaml` format
- [ ] At least one `defaultPaths` entry covers the flat `config.yaml` format

**Regression guard:** `TestConfigWriteThenLoad`, `TestConfigDefaultPathsAccessible`

---

## Rule 5: Middleware Order Invariant

> Do NOT change middleware registration order in serve.go.

**Why:** The correct order (AdminAuth → Auth → CORS) ensures:
1. AdminAuth injects AdminContext from cookie
2. Auth checks AdminContext first, then falls back to Bearer token
3. CORS sets headers last

**Checklist:**
- [ ] `adminAuthMw.Middleware(handler)` is the outermost wrap
- [ ] `apiMiddleware.Auth(handler)` wraps inside adminAuth
- [ ] `apiMiddleware.CORS(handler)` is the innermost wrap

**Regression guard:** Login bypass and cookie auth tests

---

## Rule 6: Provider Diagnose + Trace Integration

> Do NOT add a new provider without implementing both Diagnose() and Trace integration.

**Why:** Provider health and trace visibility are required for operational use. A provider that can't be diagnosed or traced is invisible to operators.

**Checklist:**
- [ ] Provider implements `Diagnoser` interface
- [ ] `Diagnose()` covers all 7 error codes
- [ ] Provider appears in `GET /api/admin/v1/providers` list
- [ ] TraceService includes provider diagnostic step
- [ ] Provider has `var _ Diagnoser = (*ProviderType)(nil)` compile-time check

---

## Rule 7: Fake/Real Classification Accuracy

> Do NOT mark `fake_only` or `real_env_required` capabilities as `verified`.

**Why:** Misclassification leads to false confidence. A capability tested only via FakeProvider is not proven to work in production.

**Classification rules:**
- `single_node_real_verified` — tested on real VPS with real Caddy/HAProxy
- `verified` — tested with unit tests against real code paths
- `fake_only` — tested only with FakeProvider/FakeCluster mocks
- `real_env_required` — requires real VPS with real providers
- `doc_only` — documented but not automated
- `unsupported` — explicitly not supported in production

---

## Rule 8: No Unreviewed Dependency Injection

> Do NOT add new service dependencies to httpSvcs or cliSvcs without wiring in main.go.

**Why:** An unset dependency (`nil`) causes runtime panics. Every field in `Services` structs must be assigned in main.go.

**Checklist:**
- [ ] New field in `httpapi.Services` → assigned in `main.go`
- [ ] New field in `cli.Services` → assigned in `main.go`
- [ ] Handler that uses new field → test that field is non-nil

**Regression guard:** `TestServicesStructHasAdminAuthField`

---

## Appendix: Verified Invariants (Tested on Real VPS)

| Invariant | Test | Status |
|-----------|------|:---:|
| Login without Bearer token | auth regression | ✅ |
| Admin user created on bootstrap | admin regression | ✅ |
| Cookie auth success after login | admin regression | ✅ |
| Config write → load round-trip | config regression | ✅ |
| All Services fields non-nil | wiring regression | ✅ |
| Gateway mutation returns 405 | smoke test | ✅ |
| Service key denied admin route | VPS acceptance | ✅ |
| Bind domain creates service+route+edge | VPS acceptance | ✅ |
| Safe apply writes step logs | VPS acceptance | ✅ |
| Trace shows complete path | VPS acceptance | ✅ |

---

*Last updated: 2026-06-25, v1.7Z Release Lockdown*
