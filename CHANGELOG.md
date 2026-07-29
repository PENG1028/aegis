# Changelog

## Unreleased — ServiceAuth ticket 授权边界修复 + admin cookie 作用域修复 + 陈旧代码/文档清理

### 修复：UI 的 Apply / Rollback / Changes 页面全程 401

admin session cookie 的 `Path=/api/admin/v1`，而 UI 走纯 cookie 认证（`credentials: 'include'`，无 Bearer）。
浏览器按 RFC 6265 §5.1.4 的路径边界前缀匹配发送 cookie，`/api/apply` 不匹配 `/api/admin/v1` —— cookie 根本没发出去。
受影响的 admin 专属端点（均在 `/api/admin/v1/` 之外，全部由 UI 实际调用）：
`/api/apply`、`/api/apply/dry-run`、`/api/apply/history`、`/api/rollback`、
`/api/config/current|diff|preview`、`/api/exposures`、`/api/health`、`/api/routes/{id}`、`/api/services/{id}`。
调用点：`pages/release/Apply.tsx:44`、`pages/release/Rollback.tsx:15`、`hooks/useDiff.ts:10`、`pages/release/Changes.tsx:20`。

不受影响：`/api/v1/my/routes`、`/api/v1/my/services`。它们经 `getWithTicket()` 调用，
走 `X-Service-Ticket` 头且**不带** `credentials`，从来不依赖 cookie。

**两侧都有问题，改一边无效：**
1. cookie `Path` 收窄到 `/api/admin/v1` → 浏览器不发送。改为 `/api`。
2. `adminauth.Middleware` 的闸门只匹配 `/api/admin/v1/` → 即使 cookie 送达也不会注入 `AdminContext`。改为覆盖 `/api/`。

配套处理：
- `Path` 提取为 `adminauth.SessionCookiePath` 常量，`SetSessionCookie` / `ClearSessionCookie` 共用 ——
  两者 Path 不一致会导致登出无法删除 cookie（浏览器要求 name/Path/Domain 全等才删）。
- 闸门放宽后，过期 cookie 会进入校验分支。**公开端点（`/api/healthz`、`/api/system/status`、
  service-auth SDK 面）改为放行而非 401**，否则持有陈旧 cookie 的调用方会被打挂；
  `/api/admin/v1/*` 保留显式 401，让前端能区分"会话过期"与"未登录"。
- `handlers/admin_auth.go` 两处硬编码 cookie 名改用常量。

回归测试（均经变异测试反证 —— 改回原状必然失败）：
- `internal/adminauth/cookie_scope_test.go`：以 RFC 6265 路径匹配断言 cookie 能达到全部受影响端点；
  set/clear 的 Path 一致性；HttpOnly / SameSite=Strict / Secure 不被削弱。
- `internal/adminauth/middleware_scope_test.go`：真实 session + 真实中间件，逐条端点断言 `AdminContext` 注入；
  陈旧 cookie 在公开端点放行、在 admin 端点 401。
- `internal/httpapi/auth_chain_test.go`：按 `serve.go` 的顺序叠两层中间件。
  **bug 就在这道接缝里 —— 两个包各自的测试全程通过，只有组合起来才暴露**，故测试放在 httpapi。
  另附匿名请求在这些路径上仍须 401 的反向断言，确认修复只放宽了已认证 admin 的可达范围。

### 安全修复

- **有效 `X-Service-Ticket` 可达 admin 与业务 handler**。`isSystemRoute()` 被四份文档
  （`external-api-guide.md`、`service-api-boundary.md`、`failure-matrix.md`、
  `capability-verification-matrix.md`）描述为已生效的防线，其中一份还标了
  `single_node_real_verified` 证据 —— 但该函数在生产中间件中**从不存在**，仅出现在测试与注释里。
  链路：`adminauth.Middleware` 无 cookie 时放行（只负责注入 AdminContext，不做判定）→
  `token.Middleware` ticket 分支验签通过即注入 `TokenType="service"` 并放行。
  暴露面除全部 `/api/admin/v1/*` 外，还包括 `/api/routes`(POST)、`/api/apply`、`/api/rollback`、
  `/api/projects`、`/api/services`、`/api/exposures` —— 这些端点**自身不做归属检查**，
  service 可绕过 Action API 的 `requireOwnership` 直接建任意路由并触发 apply。
  现由 `isSystemRoute()` + `serviceTicketAllowed()` 白名单封闭，返回 403 `SCOPE_DENIED` 并写审计日志。
  白名单只含 `/api/v1/actions/`、`/api/v1/my/`、`/api/service-auth/v1/` —— 加法语义，新增业务路由默认关闭。
- **`isPublicPath()` 可被点段绕过**（连带发现）。它在未清理的原始路径上做前缀匹配，
  `/api/service-auth/v1/../admin/v1/scopes` 与 `/assets/../api/admin/v1/scopes` 均被判为公开路径，
  **完全跳过认证**；`!= "/api/service-auth/v1/services"` 这个精确排除也被 `/./services` 规避。
  现统一经 `normalizeGuardPath()` 清理后再比较。

### 测试

- 新增 `internal/token/service_scope_test.go`：11 条原可达路径断言 403，8 条合法路径断言 200，
  外加 admin Bearer 不受影响的回归。以变异测试反证有效性（去掉 guard 后必然失败）。
- 新增 `internal/token/path_traversal_test.go`：点段穿越、`normalizeGuardPath` 边界、清理后合法路径仍通。
- 删除 `TestGatewayMutationFrozen`：它构造 `SmokeResult` 字面量、置 `Passed=true`、再断言 `Passed` 为真，
  从未发出请求；所声称的端点已不存在。同时删除 failure matrix 中 `return true` 的对应用例。
- 重写 `safety_wiring_test.go` 的 `TestSafetyEndpointsAreAdminRoutes`：原版在测试内**自行重新实现**
  一份 `isSystemRoute` 闭包并断言该副本，因此在生产无防护的情况下依然通过并打印
  "protected by isSystemRoute()"。

### 死代码

- `buildCompositions(modeID)` 删除从未被读取的 `modeID` 参数（两处调用点分别传 `"legacy"`/`"edge_mux"`，
  函数体内直接忽略），并移除只是复制 `def` 字段的中间变量。
- `IsTransparentForwardTarget()` 修正自相矛盾的文档：注释称"仅 HTTP 组合符合条件"，实现恒返回 `true`。
- `provider.go` 移除指向不存在计划的注释：`lifecycle.Manager` / "Phase 4" —— `internal/lifecycle` 与
  `internal/hostdep/lifecycle` 均不存在，`lifecycle.Manager` 全库零引用。

### 文档漂移

- `.claude/rules/distnode-architecture.md`：5 个"待删除/冻结"组件已物理删除，改为"已清除，勿重建"。
- `service-api-boundary.md`：标明 per-action scope 机制（`domain:bind` 等 6 种）**在代码中不存在**
  —— 无 token 仓库、无 scope 校验、`/api/admin/v1/api-keys*` 未注册；ticket 是全有或全无。
- `external-api-guide.md`：删除从未注册的 `service-auth/groups`、`service-auth/policies`；
  修正认证流程图 —— 强制点是 `token.Middleware`，不是 `AdminAuthMiddleware`。
- `failure-matrix.md`、`gateway-boundary-closure.md`、`release-freeze-rules.md`：
  `/api/admin/v1/gateway/*` 已删除，返回 404 而非 405 `GATEWAY_MUTATION_FROZEN`（该码已不存在）。
- `control-plane-ux.md`、`admin-ui-boundary.md`：更正"UI 尚不存在 / Not implemented, CLI-only"
  —— 实际有 100 个 `.tsx`、8 个路由组，经 `go:embed` 打进二进制。
- `gateway-link.md`、`traffic-routing.md`：修正指向已删除包的实现路径
  （`internal/gateway_link/`→`internal/gateway/`，`internal/proxy/caddy/`→`internal/hostdep/provider/`，
  `internal/apply/planner.go` 与 `internal/noderuntime/`→`internal/topology/`）。
- `CLAUDE.md`：补录第 3 类对外表面 `internal/aegisgateway/`（能力注册表，此前不在 50 包清单中）。

---

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
