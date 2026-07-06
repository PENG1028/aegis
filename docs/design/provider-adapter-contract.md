# Provider Adapter Contract — v1.7T

## Overview

This document defines the **unified boundary** that every Aegis provider adapter must satisfy. Currently three adapters exist: `caddy_http`, `haproxy_edge_mux`, `haproxy_tcp`.

---

## 1. Provider Adapter Responsibilities

Every provider adapter is responsible for these 7 capabilities:

### 1.1 Capability Report
- **Interface**: `Info() Info`
- **Returns**: provider name, protocol, status (ready/degraded/unavailable), message, config_path
- **Caddy**: checks `caddy` binary in PATH
- **HAProxy**: checks `haproxy` binary in PATH

### 1.2 Config Render
- **Interface**: `Render(routes []proxy.RouteConfig) ([]byte, error)`
- **Input**: abstract route configs (provider-agnostic)
- **Output**: provider-specific config file content
- **Caddy**: generates Caddyfile with reverse_proxy directives
- **HAProxy EdgeMux**: generates HAProxy config with SNI frontends/backends
- **HAProxy TCP**: generates HAProxy config with TCP frontends/backends

### 1.3 Config Validate
- **Interface**: `Validate(configPath string) error`
- **Input**: path to rendered config file
- **Action**: runs provider's native validation command
- **Returns**: error with stderr captured
- **Caddy**: `caddy validate --config <path>`
- **HAProxy**: `haproxy -c -f <path>`

### 1.4 Config Apply
- **Interface**: `Backup() (string, error)` + `Restore(backupPath string) error`
- **Backup**: saves current config to timestamped backup file
- **Restore**: restores config from backup path
- **Apply pipeline** orchestrates: render → write temp → validate → backup → hash compare → replace → reload

### 1.5 Reload
- **Interface**: `Reload() error`
- **Action**: gracefully reloads the provider with new config
- **Caddy**: `caddy reload --config <path>` (or systemctl reload caddy)
- **HAProxy**: `systemctl reload haproxy` (fallback: `haproxy -sf`)

### 1.6 Runtime Verify
- **Implementation**: provider-specific (not a formal interface method for all adapters)
- **Caddy**: curl check against localhost:80/443
- **HAProxy**: TCP connect check against listening port
- **Used by**: `Diagnose()` method

### 1.7 Diagnose
- **Interface**: `Diagnose() ProviderDiagnostic` (via `Diagnoser` interface)
- **Returns**: structured diagnostic covering all 7 error codes
- **Error codes**: PROVIDER_MISSING, PROVIDER_VERSION_UNSUPPORTED, CONFIG_FILE_MISSING, CONFIG_VALIDATE_FAILED, SERVICE_NOT_RUNNING, LISTENER_CONFLICT, RUNTIME_VERIFY_FAILED
- **Caddy**: ✅ implements `Diagnoser`
- **HAProxy (both)**: ✅ implements `Diagnoser`

---

## 2. Provider Adapter Does NOT Handle

These concerns belong to **other Aegis layers**, never to provider adapters:

| Concern | Handled By |
|---------|-----------|
| Scope permission | `token.AuthMiddleware` |
| Action ownership | `action.ActionService` |
| API key authentication | `token.AuthMiddleware` |
| Route ownership / space isolation | `action.ActionService` |
| Business-level action decisions | `action.ActionService` |
| Safe apply orchestration | `apply.AppService` |
| Config backup rotation | `apply.Executor` |
| State version tracking | `cluster.StateVersion` |
| Pending apply state | `cluster.PendingState` |
| Operation logging | `logs.AppService` |
| Audit logging | `logs.AppService` |

---

## 3. Provider Interface (actual Go interface)

```go
// provider/provider.go
type Provider interface {
    Info() Info
    Render(routes []proxy.RouteConfig) ([]byte, error)
    Validate(configPath string) error
    Reload() error
    Backup() (string, error)
    Restore(backupPath string) error
    GetCurrentConfig() (string, error)
}

// provider/diagnostic.go
type Diagnoser interface {
    Diagnose() ProviderDiagnostic
}
```

All three providers implement `Provider`. All three implement `Diagnoser` (v1.7S).

---

## 4. Current Implementations Alignment

| Capability | CaddyHTTPProvider | HAProxyEdgeMuxProvider | HAProxyTCPProvider |
|-----------|------------------|----------------------|-------------------|
| Info() | ✅ | ✅ | ✅ |
| Render() | ✅ (Caddyfile) | ✅ (HAProxy SNI cfg) | ✅ (HAProxy TCP cfg) |
| Validate() | ✅ `caddy validate` | ✅ `haproxy -c` | ✅ `haproxy -c` |
| Reload() | ✅ `systemctl reload` | ✅ `systemctl reload` + fallback | ✅ `systemctl reload` + fallback |
| Backup() | ✅ | ✅ | ✅ |
| Restore() | ✅ | ✅ | ✅ |
| GetCurrentConfig() | ✅ | ✅ | ✅ |
| Diagnose() | ✅ (v1.7S) | ✅ (v1.7S) | ✅ (v1.7S) |
| Runtime Verify | ✅ curl localhost | ❌ (not implemented) | ❌ (not implemented) |

---

## 5. Diagnostic ↔ Access Path Trace Relationship

The `AccessPathTrace` from `internal/trace/` includes provider diagnostic steps:

```
Trace step: provider → haproxy_diag
  Status: matched | error
  Detail: "HAProxy: available (v2.4.22)" | "HAProxy: config_invalid: ..."
```

The trace service calls `provider.CheckHAProxyStatus()` and `provider.CheckCaddyStatus()` to get diagnostic information inline. When providers implement `Diagnoser`, the trace can also call `Diagnose()` for structured output.

---

## 6. Adding a New Provider (Future Reference)

To add a new provider, implement:
1. `Provider` interface (7 methods)
2. `Diagnoser` interface (1 method)
3. Register with `provider.Registry`
4. Add listener defaults in `listener.Service`
5. Wire into `cmd/aegis/main.go`
6. Add to provider diagnostic handler
7. Add to trace service (if applicable)

**Current constraint**: No new providers in 1.x. This contract is for reference only.

---

## 7. Contract Enforcement

- **Compile-time**: `var _ Provider = (*CaddyHTTPProvider)(nil)` ensures interface satisfaction
- **Compile-time**: `var _ Diagnoser = (*CaddyHTTPProvider)(nil)` ensures diagnostic capability
- **Test-time**: FakeProvider tests verify the contract for all 7 error codes
- **Runtime**: `POST /api/admin/v1/providers/diagnose` returns structured diagnostics

---

## 8. Design Principle

> Provider adapters translate between Aegis abstract models and provider-specific config files. They know HOW to render, validate, and reload configs — but they don't know WHO is allowed to make changes or WHY a change is being made. Business logic lives in ActionService; config mechanics live in providers.
