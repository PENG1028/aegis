# Changelog

## v1.7AC-3 (2026-06-26) — Gateway Link Final Closure

**两节点真实验收通过**

### 修复
- `ResolveValidateCommand` now accepts `configPath` argument (was hardcoded to `CaddyfilePath`)
- Caddy render uses `--adapter caddyfile` for `.tmp` extension files
- `header_up` rendered inside `reverse_proxy { }` block (Caddy v2 syntax)
- `renderSimpleBlock` and multi-route handler now emit `ExtraHeaders`
- TraceService populates `GatewayLinkInfo` in trace output
- `bind-http-domain` accepts `gateway_link_id` in input

### 验收
- Server A (<SERVER_A_IP>) → Server B (<SERVER_B_IP>:80) verifier chain
- No token → HTTP 401 | Correct token → HTTP 200 | Wrong token → HTTP 403
- Trace shows `gateway_link.link_id`, `enabled`, `header_injected`, `verification_mode`
- Raw token NOT in trace/list/get/log (0 hex matches)

---

## v1.7AC-2 (2026-06-25) — Gateway Link Real Two-node

- Two-node gateway-to-gateway acceptance on port 80
- Verifier example app (`examples/gateway-link-verifier/`)
- CORS: fixed verifier to bind `0.0.0.0` instead of `127.0.0.1`
- `CLAUDE.md` created with project guide
- Port boundary docs: only 80/443 open

---

## v1.7AC (2026-06-25) — Gateway Link Acceptance

- GatewayLink verification documentation (static token mode)
- Route→Link binding lifecycle documented
- Secret handling audit (token storage risk documented)
- Rotate flow documented

---

## v1.7AB (2026-06-25) — Gateway Link Wiring

- Planner reads GatewayLink and injects `ExtraHeaders` into `RouteConfig`
- Caddy render emits `header_up` for Gateway Link headers
- Gateway Link API: `POST/GET/DELETE /api/admin/v1/gateway-links`
- Rotate API: `POST /api/admin/v1/gateway-links/{id}/rotate`
- Migration 025: `gateway_link_id` column on routes table
- 15 gateway_link unit tests

---

## v1.7AA (2026-06-25) — Boundary Definition

- 7 boundary documents in `docs/boundary/`
- Two-node acceptance plan + result
- Cross-VPC network analysis
- `ValidateTarget` allows public IPs (removed arbitrary restriction)
- Hot update drill result

---

## v1.7Z-RC (2026-06-25) — Controlled Pilot

- Pilot domain bind on real VPS (python3 http.server :3000)
- Restart drill: 10/10 PASS (data plane survives, state recovers)
- Rollback drill documented (NOT_EXECUTED)
- Observation report template

---

## v1.7Z (2026-06-25) — Release Lockdown

- 16 regression tests for 5 v1.7Y bugs
- Single-node production boundary doc
- Install runbook, rollback runbook
- Release freeze rules (8 rules)
- Capability matrix: 67 items, 67% verified/real

---

## v1.7Y (2026-06-25) — Real VPS Acceptance

**第一版真实 VPS 验收通过**

- Ubuntu 24.04, Caddy 2.6.2, HAProxy 2.8.16
- 24/24 capabilities PASS
- 5 bugs fixed during acceptance
- Full chain: bootstrap → login → bind → apply → trace → diagnose

### Bugs Found & Fixed
- Login blocked by Bearer middleware → added bypass
- Default admin never created → `EnsureAdmin("admin","admin")`
- Middleware order reversed → AdminAuth → Auth → CORS
- Config path mismatch → added subdirectory paths to config loader
- httpSvcs/service missing fields → expanded struct literals

---

## v1.7X (2026-06-24) — Pre-Real-Deploy Verification

- Smoke command verification audit
- Action chain proof (BindHTTPDomain full code trace)
- Mutation semantics audit
- Gateway mutation bypass audit
- Provider diagnoser command proof
- Trace verification audit
- Restart safety proof
- Multi-node verification audit
- Capability verification matrix

---

## v1.7W (2026-06-24) — Critical Closure

- Admin CRUD auth: admin session bypass in Auth middleware
- Service keys blocked from CRUD routes via `isSystemRoute()`
- `MarkPending()` wired into admin CRUD handlers (route.go, service.go)
- TraceDomain checks target connectivity (EndpointRepo lookup)
- Trace uses `DiagnoseHAProxy()` / `DiagnoseCaddy()` static functions
- Apply pipeline writes step-level logs (8 phases)
- GatewayLinkInfo type in trace model

---

## v1.7V (2026-06-24) — Verification Gate

- 8 audit documents
- Admin route protection: 38 routes audited, 10 mutations identified
- Action chain: BindHTTPDomain fully traced with code evidence
- CRUD semantics: MarkPending gap found and fixed
- Gateway bypass: no write paths found (all frozen)
- Provider Diagnoser: 5/7 REAL, 2 REAL_MISSING
- Trace: Target connectivity bug found (TraceDomain never called checkTargetConnectivity)
- Restart safety: architectural claim, not test-verified
- Multi-node: all FAKE_ONLY or UNTESTED
- 60 capabilities classified (50% verified)

---

## v1.7U (2026-06-24) — Runtime Acceptance & Failure Matrix

- 5 runbook documents
- Smoke CLI: `aegis smoke golden/provider/trace/failure-matrix/restart-check`
- smoke package with 20 tests
- Real VPS verification plan
- Logging acceptance scenarios (8 scenarios)

---

## v1.7T (2026-06-24) — Access Path Trace & Adapter Boundary

- Access Path Trace engine: `TraceDomain`, `TraceSNI`, `TraceRoute`
- 3 API endpoints + CLI commands
- Runtime target connectivity check (`net.DialTimeout`)
- Gateway boundary closure document
- Provider adapter contract document
- TraceStep model with `ProviderDiagnostic` field

---

## v1.7S (2026-06-24) — Diagnostics & Mutation Closure

- Real CaddyHTTPProvider.Diagnose() (5/7 codes)
- Real HAProxyEdgeMuxProvider.Diagnose() (5/7 codes)
- `DiagnoseHAProxy()` and `DiagnoseCaddy()` static functions
- `pending_apply` / dirty state mechanism (`PendingState`)
- Provider diagnostic API endpoints
- Audit logging for denied access
- Enhanced operation log coverage
- FakeProvider with all 7 diagnostic failure modes

---

## v1.7R (2026-06-24) — Reality Audit & Control Lockdown

- API boundary audit: 33 admin endpoints classified
- Gateway abstraction frozen: all mutation endpoints return 405
- AdminAuthMiddleware wired (was missing from middleware chain)
- Gateway handlers read from real tables (routes, managed_domains, listeners)
- Deployment model frozen: tracking-only, no execution
- Fake harness with complete fault injection matrix
- 5 audit documents

---

## Pre-v1.7

v0.x through v1.6 series: initial implementation of routes, services, endpoints,
providers, Caddy/HAProxy adapters, apply pipeline, state versioning, cluster
leadership, space isolation, action API, admin auth, API keys, and diagnostics.
