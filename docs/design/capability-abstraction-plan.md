# Aegis Capability Abstraction Compliance Plan

> Created 2026-07-26. Audit found 47 violations across Go + TypeScript.
> Guiding principle: **check capabilities, not provider names.**

---

## 1. Design Rules

| Rule | Wrong | Right |
|------|-------|-------|
| Provider identity | `if p.ID == "caddy"` | `if p.State().HasCapability("auto_cert")` |
| Provider list | `for _, name := range []string{"caddy", "haproxy"}` | `for _, p := range reg.ListAll()` |
| Provider name | `"Caddy HTTP"` (hardcoded) | `p.State().Name` (from API) |
| Config field | `CaddyfilePath`, `CaddyBinary` | `ConfigPath`, `ProviderBinary` |
| Diagnostic | `DiagnoseCaddy()`, `DiagnoseHAProxy()` | `p.Diagnose()` (universal) |
| Error message | `"Caddy: config invalid"` | `fmt.Sprintf("%s: %v", p.State().Name, err)` |
| Icons in UI | Emoji `🔄` `🔑` | SVG `<CapabilityLabel cap="auto_cert" />` |
| Optional fields | No indicator | Label with "（选填）" + hint text |

---

## 2. SVG Icon System

Each capability gets a 16×16 inline SVG path, rendered via `<CapabilityLabel>`.

```tsx
// ui/src/components/shared/CapabilityLabel.tsx
interface Props { cap: string; size?: 'sm' | 'md' }
// Renders SVG icon + Chinese label from capMeta lookup.
```

All 31 capabilities map to a minimal geometric icon (circle, lock, key, arrow, etc.)
No external sprite file — inline `<svg>` per instance, zero build pipeline changes.

---

## 3. Capability Label Map

`ui/src/lib/capability-labels.ts` — single source of truth:

| Key | Label | Layer |
|------|-------|-------|
| `auto_cert` | 自动证书 | L7 |
| `load_cert` | 加载证书 | L7 |
| `tls_terminate` | TLS 终结 | L5 |
| `tls_passthrough` | TLS 直通 | L5 |
| `sni_preread` | SNI 分流 | L5 |
| `mtls_terminate` | mTLS 认证 | L5 |
| `tls_masquerade` | TLS 卸载 | L5 |
| `listen_tcp` | TCP 监听 | L4 |
| `listen_udp` | UDP 监听 | L4 |
| `upstream_tcp` | TCP 转发 | L4 |
| `upstream_udp` | UDP 转发 | L4 |
| `route_host` | 域名路由 | L7 |
| `route_path` | 路径路由 | L7 |
| `hot_reload` | 热重载 | — |
| `validate_cfg` | 配置校验 | — |
| `health_check` | 健康检查 | L7 |
| `load_balance` | 负载均衡 | L7 |
| `rate_limit` | 限流 | L7 |
| `http1` | HTTP/1.1 | L7 |
| `http2` | HTTP/2 | L7 |
| `http3` | HTTP/3 | L7 |
| `grpc` | gRPC | L7 |
| `ws` | WebSocket | L7 |
| `connect` | CONNECT | L7 |
| `alpn_match` | ALPN 匹配 | L6 |
| `ocsp_stapling` | OCSP 装订 | L6 |
| `tcp_splice` | TCP 拼接 | L4 |
| `embedded` | 嵌入式 | — |
| `config_read` | 配置读取 | — |
| `config_write` | 配置写入 | — |
| `can_install` | 可安装 | — |

---

## 4. Page Redesigns

### 4.1 InfraManagement.tsx (`/fabric/infra`)

**Before:** 6 columns (name/type/status/version/path/actions), `p.id === 'caddy'` exceptions.
**After:** 7 columns — adds "核心能力" column with `<CapabilityLabel>` badges.

Actions driven by capabilities:
- Install: `canInstall && !installed` (no more `p.id !== 'caddy'`)
- Reload: `hasCap('hot_reload')`
- Service: always present for providers

### 4.2 Panel.tsx (`/settings`)

Email label changes:
- `"通知邮箱"` → `"Let's Encrypt 注册邮箱（选填）"`
- Hint: `"仅用于 ACME 证书注册和到期通知。可随时补充，不影响面板正常运行。"`

### 4.3 TlsSettings.tsx (`/settings/tls`)

**Removed:** PEM textarea paste, file path inputs, "保存证书配置" button.

**Added:** Certificate selector dropdown from `certApi.list()`, with bind-to-panel action.

---

## 5. Backend Audit Map

### 5.1 Field Renames (Breaking — YAML aliases for backward compat)

| Old | New |
|------|------|
| `CaddyfilePath` / `caddyfile_path` | `ConfigPath` / `config_path` |
| `CaddyBinary` / `caddy_binary` | `ProviderBinary` / `provider_binary` |

Affected: `config.go`, `config/paths.go`, `apply/service.go`, `apply/workflow.go`,
`deploy_node.go`, `cli/bootstrap.go`, `cli/doctor.go`, `cli/settings.go`,
`cli/diagnostics.go`, `listener/listener.go`, all provider files.

### 5.2 Critical Hardcodes

| File | Line | Current | Target |
|------|------|---------|--------|
| `deploy_node.go` | 662 | `!= "caddy" && != "haproxy"` | `provReg.Get(expected) == nil` |
| `deploy_node.go` | 748 | `switch providerID { case "haproxy": apt-get install...` | `LifecycleProvider.Install()` |
| `deploy_node.go` | 1347 | `switch provider { case "haproxy": proxy.CaddyfilePath...` | `p.State().ConfigPath` |
| `doctor.go` | 54 | `checkBinary("haproxy"); checkBinary("caddy")` | Registry iteration |
| `bootstrap.go` | 93-151 | Caddy-only production setup | Provider-driven |
| `preflight.go` | 65 | `for name in caddy haproxy; do` | Dynamic from registry |
| `node/capability.go` | 48 | `LookPath("caddy")` | Provider binary path discovery |

### 5.3 Deprecated Functions to Remove

`internal/hostdep/provider/diagnostic.go`:
- `CheckCaddyStatus()`, `CheckHAProxyStatus()`
- `DiagnoseCaddy()`, `DiagnoseHAProxy()`
- `quickDiagnoseCaddy()`, `quickDiagnoseHAProxy()`

### 5.4 Callers Already Fixed (Batch 0 — done in v1.8L-110~111)

- `trace/service.go` — uses `ProvReg.ListAll().Diagnose()` ✅
- `smoke/service.go` — uses `ProvReg.ListAll().Diagnose()` ✅
- `handlers/settings.go` — uses `ProvReg.FindByCapability(CapAutoCert)` ✅
- `handlers/certificates.go` — already capability-based ✅

---

## 6. Execution Order

### Batch 1 — Frontend + Labels (this commit)
```
□ ui/src/lib/capability-labels.ts              NEW
□ ui/src/components/shared/CapabilityLabel.tsx  NEW
□ ui/src/pages/settings/Panel.tsx               email → optional
□ ui/src/pages/settings/TlsSettings.tsx          paste → selector
□ ui/src/pages/fabric/InfraManagement.tsx        +capability column
```

### Batch 2 — Backend Core
```
□ config.go, config/paths.go                    field renames
□ deploy_node.go                                3 critical hardcodes
□ node/capability.go                            registry-based detection
□ listener/listener.go                          dynamic provider IDs
□ apply/                                       adapt to field renames
□ cli/                                         adapt to field renames
```

### Batch 3 — CLI + Cleanup
```
□ doctor.go, bootstrap.go, preflight.go, edge.go
□ diagnostic.go                                 delete deprecated funcs
□ All test files                                adapt to field renames
```

---

## 7. Commit History

| Commit | Scope |
|------|------|
| `fb032cf` | fix: panel domain save without TLS breaks IP access |
| `2e3298d` | refactor: replace hardcoded provider names with capability-based registry lookups (trace, smoke, settings) |
| `e3108ea` | feat: redesign settings main page with capability-aware TLS status |
| *(this batch)* | feat: capability icon system + InfraManagement + TLS cert selector |
| *(batch 2)* | refactor: rename CaddyfilePath→ConfigPath + deploy_node capability-based |
| *(batch 3)* | refactor: CLI diagnostics + bootstrap + deprecated removal |
