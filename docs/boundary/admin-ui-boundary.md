# Admin/UI Boundary

> **⚠️ 本文的 "Page UI: ❌ Not implemented / CLI-only" 标注已全面过期。**
> UI 已实现并嵌入二进制：100 个 `.tsx`，8 个路由组在 `ui/src/pages/`（`command-center`、`fabric`、`runtime`、`access`、`release`、`observe`、`exposure`、`settings`），
> 由 `internal/uiassets`（`go:embed dist`）提供，SPA 路由经 `routes.go` 的 `/{path...}` 兜底。
>
> 下表的 **API 列仍然有效**（可作为端点→控件的映射速查），**UI 状态列不要采信** —— 请直接查 `ui/src/lib/real-api-client.ts` 确认某端点是否已被前端调用。

## Dashboard Page

| Widget | API | Status | Notes |
|--------|-----|--------|-------|
| System overview | `GET /api/system/status` | ✅ API exists | Returns leader, state_version, pending_apply |
| Provider health | `POST /api/admin/v1/providers/diagnose` | ✅ API exists | Full 7-code diagnostic |
| Pending apply | From system status | ✅ Available | `pending_apply.pending` field |
| Last apply result | `GET /api/admin/v1/apply-logs` | ✅ API exists | Step-level trace |
| State version | From system status | ✅ Available | Monotonic counter |
| Page UI | — | ❌ Not implemented | CLI-only |

## Routes / Domains Page

| Widget | API | Status | Notes |
|--------|-----|--------|-------|
| List routes | `GET /api/admin/v1/routes` | ✅ API exists | All routes with ownership |
| Create route | `POST /api/routes` | ✅ API exists | Admin CRUD |
| Delete/disable route | `POST /api/routes/{id}/disable` | ✅ API exists | |
| Trace domain | `GET /api/admin/v1/trace/domain/{domain}` | ✅ API exists | 8-step access path |
| Manual apply | `POST /api/admin/v1/system/apply` | ✅ API exists | |
| Page UI | — | ❌ Not implemented | CLI-only |

## Scopes / API Keys Page

| Widget | API | Status | Notes |
|--------|-----|--------|-------|
| List scopes | `GET /api/admin/v1/scopes` | ✅ API exists | |
| Create scope | `POST /api/admin/v1/scopes` | ✅ API exists | With quotas |
| List API keys | `GET /api/admin/v1/api-keys` | ✅ API exists | |
| Create API key | `POST /api/admin/v1/scopes/{id}/api-keys` | ✅ API exists | Returns raw token once |
| Revoke API key | `POST /api/admin/v1/api-keys/{id}/revoke` | ✅ API exists | |
| Rotate API key | `POST /api/admin/v1/api-keys/{id}/rotate` | ✅ API exists | |
| Page UI | — | ❌ Not implemented | CLI-only |

## Nodes Page

| Widget | API | Status | Notes |
|--------|-----|--------|-------|
| List nodes | `GET /api/admin/v1/nodes` | ✅ API exists | |
| Node capabilities | `GET /api/admin/v1/nodes/{id}/capabilities` | ✅ API exists | |
| Refresh capabilities | `POST /api/admin/v1/nodes/{id}/refresh-capabilities` | ✅ API exists | |
| Node events | `GET /api/admin/v1/node-events` | ✅ API exists | |
| Page UI | — | ❌ Not implemented | CLI-only |

## Diagnostics Page

| Widget | API | Status | Notes |
|--------|-----|--------|-------|
| Doctor (CLI) | `aegis doctor` | ✅ CLI exists | |
| Provider diagnose | `POST /api/admin/v1/providers/diagnose` | ✅ API exists | |
| Trace domain | `GET /api/admin/v1/trace/domain/{domain}` | ✅ API exists | |
| Trace SNI | `GET /api/admin/v1/trace/sni/{sni_host}` | ✅ API exists | |
| Trace route | `GET /api/admin/v1/trace/route/{route_id}` | ✅ API exists | |
| Apply logs | `GET /api/admin/v1/apply-logs` | ✅ API exists | |
| Audit logs | `GET /api/admin/v1/audit-logs` | ✅ API exists | |
| Operation logs | `GET /api/admin/v1/operations` | ✅ API exists | |
| Page UI | — | ❌ Not implemented | CLI-only |

## Operations Page

| Widget | API | Status | Notes |
|--------|-----|--------|-------|
| Pending apply | From system status | ✅ Available | |
| Manual apply | `POST /api/admin/v1/system/apply` | ✅ API exists | |
| Rollback | `POST /api/rollback` | ✅ API exists | Config backup restore |
| Page UI | — | ❌ Not implemented | CLI-only |

## Summary

| Category | API Count | UI Count |
|----------|:---:|:---:|
| Dashboard | 4 | 0 |
| Routes/Domains | 6 | 0 |
| Scopes/API Keys | 6 | 0 |
| Nodes | 4 | 0 |
| Diagnostics | 7 | 0 |
| Operations | 3 | 0 |
| **Total** | **30 APIs** | **0 UI pages** |
