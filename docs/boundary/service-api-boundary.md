# Service API Boundary

> **认证方式：ServiceAuth ticket（`X-Service-Ticket`）。**
> 本文早期版本描述的"space-scoped API key + per-action scope"机制**在代码中不存在** —— 没有 token 仓库，没有 scope 校验，`/api/admin/v1/api-keys*` 路由未注册。
> 调用方身份来自 `internal/token/middleware.go` 的 ticket 校验：服务名 → `ActionContext.SpaceID`，`TokenType="service"`。
>
> **粒度现状：ticket 是全有或全无的。** 持有有效 ticket 即可调用下表全部 Action API，无法只授予其中一部分。下方 Scope 列是**目标设计，尚未实现**。真正生效的约束只有两层：路径白名单（中间件）+ 归属检查（handler 内 `requireOwnership`）。

## What a Service Ticket Can Do

| Action | Endpoint | Scope（未实现） | Triggers Apply | Operation Log | Ownership Check |
|--------|----------|:---:|:---:|:---:|:---:|
| bind-http-domain | `POST /api/v1/actions/bind-http-domain` | `domain:bind` | ✅ | ✅ | ✅ domain + space |
| bind-tls-backend | `POST /api/v1/actions/bind-tls-backend` | `domain:bind` | ✅ | ✅ | ✅ space |
| update-target (service) | `PATCH /api/v1/actions/update-target` | `service:update` | ✅ | ✅ | ✅ space+ownership |
| update-target (edge) | `PATCH /api/v1/actions/update-target` | `edge:update` | ✅ | ✅ | ✅ space+ownership |
| disable-domain (route) | `POST /api/v1/actions/disable-domain` | `domain:disable` | ✅ | ✅ | ✅ space+ownership |
| disable-domain (edge) | `POST /api/v1/actions/disable-domain` | `domain:disable` | ✅ | ✅ | ✅ space+ownership |
| delete-domain (route) | `DELETE /api/v1/actions/domain` | `domain:delete` | ✅ | ✅ | ✅ space+ownership |
| delete-domain (edge) | `DELETE /api/v1/actions/domain` | `domain:delete` | ✅ | ✅ | ✅ space+ownership |
| list my routes | `GET /api/v1/my/routes` | `read:own` | ❌ | ❌ | ✅ space |
| list my services | `GET /api/v1/my/services` | `read:own` | ❌ | ❌ | ✅ space |
| list my edge rules | `GET /api/v1/my/edge-rules` | `read:own` | ❌ | ❌ | ✅ space |
| list my operations | `GET /api/v1/my/operations` | `read:own` | ❌ | ❌ | ✅ space |

## What a Service Ticket CANNOT Do

强制点在 `internal/token/middleware.go`：ticket 分支先过 `isSystemRoute()`，再过 `serviceTicketAllowed()` 白名单，不在白名单的路径返回 `403 SCOPE_DENIED` 并写审计日志。白名单只含 `/api/v1/actions/`、`/api/v1/my/`、`/api/service-auth/v1/`。

| Action | Why |
|--------|-----|
| Access any `/api/admin/v1/*` endpoint | `isSystemRoute()` → 403 SCOPE_DENIED |
| Access CRUD endpoints (`/api/routes`, `/api/services`, `/api/projects`, `/api/exposures`) | 不在 `serviceTicketAllowed()` 白名单 → 403。**这些端点自身不做归属检查**，白名单是唯一防线 |
| Trigger `/api/apply` or `/api/rollback` | 同上 → 403。配置推送与回滚是 admin 动作 |
| Manage nodes / providers / listeners / cluster / upgrades | 均在 `/api/admin/v1/*` 之下 → 403 |
| Access resources belonging to other spaces | handler 内 `requireOwnership()` |
| Access system-owned resources (space_id="") | 非 admin token 被拒 |

> 回归测试：`internal/token/service_scope_test.go`。新增业务路由默认关闭 —— 白名单是加法，不是减法。

## Endpoint Table

| Method | Path | Action | Scope | Admin | Service Key |
|--------|------|--------|:---:|:---:|:---:|
| POST | `/api/v1/actions/bind-http-domain` | bind-http-domain | `domain:bind` | ❌ | ✅ |
| POST | `/api/v1/actions/bind-tls-backend` | bind-tls-backend | `domain:bind` | ❌ | ✅ |
| PATCH | `/api/v1/actions/update-target` | update-target | `service:update` | ❌ | ✅ |
| POST | `/api/v1/actions/disable-domain` | disable-domain | `domain:disable` | ❌ | ✅ |
| DELETE | `/api/v1/actions/domain` | delete-domain | `domain:delete` | ❌ | ✅ |
| GET | `/api/v1/my/routes` | my routes | `read:own` | ❌ | ✅ |
| GET | `/api/v1/my/services` | my services | `read:own` | ❌ | ✅ |
| GET | `/api/v1/my/edge-rules` | my edge rules | `read:own` | ❌ | ✅ |
| GET | `/api/v1/my/operations` | my operations | `read:own` | ❌ | ✅ |
| GET/POST/PATCH/DELETE | `/api/admin/v1/*` | admin operations | admin:* | ✅ | ❌ |
| GET/POST | `/api/routes`, `/api/services`, etc. | CRUD | — | ✅ | ❌ |

## Current Gaps

| Gap | Impact |
|-----|--------|
| **无 per-action scope** | ticket 全有或全无，无法只授予 `read:own`。上表 Scope 列是目标设计 |
| Service 无法在绑定前列出可用域名 | 必须预先知道域名 |
| No pagination on list endpoints | 大结果集未处理（注意：CLAUDE.md 称 admin 列表支持 `?limit=&offset=`，My Resources 不支持） |
| No bulk operations | 每个域名绑定都是独立请求 |
| 错误形状不统一 | `writeError` 返回 `{"error":"..."}`，`writeAuthError` 返回 `{"error":{"code","message"}}` |
| 无幂等键 | 重复 bind 的行为未定义，`APPLY_LOCKED`(423) 无重试约定 |
