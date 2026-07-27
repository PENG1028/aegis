# Provider Boundary — v1.7AA

## Current Providers

| Provider | Type | Status |
|----------|------|--------|
| `caddy_http` | HTTP/HTTPS reverse proxy | ✅ single_node_real_verified |
| `haproxy_edge_mux` | TLS SNI passthrough mux | ✅ single_node_real_verified |
| `haproxy_tcp` | TCP proxy | ✅ verified (code) |

## Provider Responsibilities (7 methods)

| Method | Interface | What It Does |
|--------|-----------|-------------|
| `Info()` | Provider | Return name, protocol, status, config path |
| `Render()` | Provider | Generate provider-specific config from abstract route configs |
| `Validate()` | Provider | Run provider's native validation command |
| `Reload()` | Provider | Gracefully reload provider with new config |
| `Backup()` | Provider | Save current config to timestamped backup |
| `Restore()` | Provider | Restore config from backup path |
| `GetCurrentConfig()` | Provider | Read current config file |
| `Diagnose()` | Diagnoser | Full diagnostic (7 error codes) |

## What Providers Do NOT Handle

| Concern | Handled By |
|---------|-----------|
| Scope permission / API key auth | `token.AuthMiddleware` |
| Action ownership / space isolation | `action.ActionService` |
| Business action decisions | `action.ActionService` |
| Safe apply orchestration | `apply.AppService` |
| Config backup rotation | `apply.Executor` |
| State version tracking | `cluster.StateVersion` |
| Pending apply state | `cluster.PendingState` |
| Operation/audit logging | `logs.AppService` |

## Design Principle

> Aegis is NOT useless without Caddy/HAProxy.
> The core value is in action/state/trace/diagnose/apply abstractions.
> Caddy and HAProxy are the current **execution providers** — they translate
> Aegis abstract models into running configs. Other providers can be added
> without changing the action/apply/trace layers.

## Provider Adapter Contract

See `docs/provider-adapter-contract.md` for full contract specification.
