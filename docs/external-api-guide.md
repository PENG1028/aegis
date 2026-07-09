# Aegis External API Guide

> 给外部服务/开发者参考的 Aegis HTTP API 文档。
> ServiceAuth 相关逻辑已标注，非强依赖——大部分功能用 admin token 即可调用。

---

## 认证方式

Aegis API 支持三种认证，优先级从高到低：

| 方式 | Header | 适用范围 | 说明 |
|------|--------|----------|------|
| Admin Session | Cookie: `aegis_admin_session` | `POST /api/admin/v1/auth/login` 登录后获得 | 浏览器用 |
| Bearer Token | `Authorization: Bearer <token>` | 全部 API | CLI/curl 用，**推荐** |
| ServiceAuth Ticket | `X-Service-Ticket: <ticket>` | Action API + My Resources | 见下方说明 |

**Admin Token 获取方式：** Aegis 启动时打印在控制台。如需重置，直连数据库修改 `admin_users` 表。

**ServiceAuth 说明：** 带上 Ed25519 签名的 service ticket 可以直接调 Action API，Aegis 会自动将你的服务名映射为一个独立空间。你只能管理自己空间内的资源。阅读 `docs/serviceauth.md` 了解如何生成 ticket。

---

## 快速参考

```
# 健康检查（无需认证）
curl http://host:7380/api/healthz

# 系统状态（无需认证）
curl http://host:7380/api/system/status

# 用 admin token 调所有 API
curl -H "Authorization: Bearer <token>" http://host:7380/api/admin/v1/...

# 用 ServiceAuth ticket 调 Action API
curl -H "X-Service-Ticket: <base64_ed25519_ticket>" http://host:7380/api/v1/actions/...
```

---

## 系统 API（公开，无需认证）

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/healthz` | GET | 存活探针，始终返回 200 |
| `/api/readyz` | GET | 就绪探针，检查 DB |
| `/api/system/status` | GET | 系统概览（项目/服务/路由数 + 版本） |
| `/api/system/runtime-mode` | GET | 运行时模式 + provider 绑定矩阵 |
| `/api/system/compositions` | GET | 组合能力定义列表 |

---

## Action API（服务自管理）

> ServiceAuth ticket 可调。Admin token 也可调（拥有全部空间的管理权限）。

**用于域名绑定和管理。** 每个服务注册到 ServiceAuth 后，用自己的 ticket 调这些端点来创建/查看/修改自己的域名映射。

| 端点 | 方法 | 用途 | Body |
|------|------|------|------|
| `/api/v1/actions/bind-http-domain` | POST | 绑定 HTTP 域名→后端 | `{domain, target_host, target_port}` |
| `/api/v1/actions/bind-tls-backend` | POST | 绑定 TLS SNI 后端 | `{sni_host, target_host, target_port}` |
| `/api/v1/actions/update-target` | PATCH | 更新目标地址 | `{resource_type, resource_id, target_host, target_port}` |
| `/api/v1/actions/disable-domain` | POST | 禁用域名 | `{domain, resource_type}` |
| `/api/v1/actions/domain` | DELETE | 删除域名 | `{domain}` |

**ServiceAuth 说明：** 调用时带上 `X-Service-Ticket` header，Aegis 解码服务名并设为资源空间。创建的资源（服务、路由）归该服务所有，其他服务不可见不可操作。

### 我的资源

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/v1/my/routes` | GET | 查看自己空间的 HTTP 路由 |
| `/api/v1/my/services` | GET | 查看自己空间的服务 |
| `/api/v1/my/edge-rules` | GET | 查看自己空间的 SNI 规则 |
| `/api/v1/my/operations` | GET | 查看自己的操作记录 |

---

## ServiceAuth 端点（服务注册用）

> 这些端点通过 IP 检查（`isInCluster`）保护，**不经过 AuthMiddleware**。
> 服务 SDK 自动调用，一般不需要手动调。

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/service-auth/v1/register` | POST | 注册服务（名 + 公钥 + 实例ID） |
| `/api/service-auth/v1/sync` | GET | 拉取集群公钥/封禁列表变更 |
| `/api/service-auth/v1/heartbeat` | POST | 更新最后在线时间 |
| `/api/service-auth/v1/report` | POST | 上报服务间调用记录 |

**Register 请求体：**
```json
{
  "service_name": "my-service",
  "public_key": "<base64 Ed25519 公钥>",
  "instance_id": "ins_a1b2c3d4e5f6g7h8"
}
```

**Register 返回（含其他服务的公钥列表）：**
```json
{
  "service_id": "svc_xxx",
  "public_keys": {"other-service": ["<pubkey>"]},
  "blocklist": [],
  "bl_version": 0,
  "sync_interval": 30,
  "warnings": []
}
```

---

## Admin API（后台管理）

> 需要 admin session cookie 或 Bearer token。ServiceAuth ticket **不能**调这些端点。

### 服务认证管理

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/admin/v1/service-auth/services` | GET | 列出所有注册服务 |
| `/api/admin/v1/service-auth/services/{id}` | GET | 查看服务详情 |
| `/api/admin/v1/service-auth/services/{id}/block` | POST | 封锁服务 |
| `/api/admin/v1/service-auth/blocklist/{id}/unblock` | POST | 解封 |
| `/api/admin/v1/service-auth/topology` | GET | 调用拓扑（`?window=1h`） |
| `/api/admin/v1/service-auth/call-logs` | GET | 调用日志（`?since=&limit=`） |
| `/api/admin/v1/service-auth/groups` | GET/POST | 服务组管理 |
| `/api/admin/v1/service-auth/policies` | GET/POST | 权限策略管理（⚠️ 仅展示，引擎已移除） |

### 管理员

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/admin/v1/auth/login` | POST | 登录（公开） |
| `/api/admin/v1/auth/me` | GET | 当前会话信息 |
| `/api/admin/v1/auth/logout` | POST | 登出 |
| `/api/admin/v1/auth/change-password` | POST | 修改密码 |

### 节点管理

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/admin/v1/nodes` | GET | 节点列表 |
| `/api/admin/v1/nodes/{id}` | GET | 节点详情 |
| `/api/admin/v1/nodes/{id}/delete` | POST | 删除节点 |
| `/api/admin/v1/nodes/{id}/update` | POST | 触发节点更新 |

### Provider 管理

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/admin/v1/providers` | GET | 已安装 provider 列表 |
| `/api/admin/v1/providers/{provider}/config` | GET/PUT | 配置查看/修改 |
| `/api/admin/v1/providers/{provider}/reload` | POST | 重载配置 |
| `/api/admin/v1/providers/{provider}/service` | POST | 服务控制 |
| `/api/admin/v1/providers/{provider}/drift` | GET | 配置偏移检测 |
| `/api/admin/v1/providers/{provider}` | DELETE | 卸载 provider |

### 配置下发

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/config/preview` | GET | 预览 Caddyfile/HAProxy 配置 |
| `/api/config/diff` | GET | 对比变更 |
| `/api/config/current` | GET | 当前生效配置 |
| `/api/apply` | POST | 执行配置下发 |
| `/api/apply/dry-run` | POST | 试运行 |
| `/api/apply/history` | GET | 下发历史 |
| `/api/rollback` | POST | 回滚到最近版本 |

### 流量管理（Exposure — TCP/UDP）

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/exposures` | GET | 列出所有端口转发 |
| `/api/exposures` | POST | 创建端口转发 |
| `/api/exposures/{id}` | GET | 查看详情 |
| `/api/exposures/{id}` | PATCH | 修改 |
| `/api/exposures/{id}/activate` | POST | 激活转发 |
| `/api/exposures/{id}/disable` | POST | 停用 |

> **注意：** Exposure API 是旧版 REST 接口，**未实现 ServiceAuth 空间隔离**。
> 调用者需要 admin 权限。外部服务请使用 Action API（`/api/v1/actions/*`）管理域名。

### 其他管理端点

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/admin/v1/projects` | GET/POST | 项目管理 |
| `/api/admin/v1/projects/{id}` | GET/PATCH/DELETE | 项目 CRUD |
| `/api/admin/v1/services` | GET/POST | 服务管理 |
| `/api/admin/v1/services/{id}` | GET/PATCH/DELETE | 服务 CRUD |
| `/api/admin/v1/endpoints` | GET/POST | 端点管理 |
| `/api/admin/v1/routes` | GET/POST | 路由管理 |
| `/api/admin/v1/routes/{id}` | GET/PATCH/DELETE | 路由 CRUD |
| `/api/admin/v1/credentials` | GET/POST | 凭据管理（加密连接串） |
| `/api/admin/v1/gateways` | GET/POST | 网关管理 |
| `/api/admin/v1/gateway-links` | GET/POST | 跨机链路管理 |
| `/api/admin/v1/domains/managed` | GET/POST | 受管域名 |
| `/api/admin/v1/system/overview` | GET | 系统概览 |

---

## 错误码

所有错误返回 JSON 格式：
```json
{"error": {"code": "ERROR_CODE", "message": "人类可读描述"}}
```

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| `UNAUTHORIZED` | 401 | 缺少或无效的认证 |
| `SCOPE_DENIED` | 403 | 无权操作该资源 |
| `RESOURCE_NOT_FOUND` | 404 | 资源不存在 |
| `RESOURCE_NOT_OWNED` | 403 | 资源不属于当前空间 |
| `DOMAIN_ALREADY_OWNED` | 409 | 域名已被其他空间绑定 |
| `APPLY_LOCKED` | 423 | 配置下发进行中，稍后重试 |
| `TARGET_NOT_ALLOWED` | 400 | 目标地址不允许 |
| `QUOTA_EXCEEDED` | 429 | 超过配额限制 |

---

## 认证流程总结

```
请求到达
  │
  ├─ isPublicPath? → 放行（healthz, login, service-auth SDK 等）
  │
  ├─ AdminSession Cookie? → ActionContext{admin} → 放行
  │
  ├─ Bearer Token 匹配? → ActionContext{admin} → 放行
  │
  ├─ X-Service-Ticket? → VerifyTicket() → ActionContext{service, space=服务名}
  │     │
  │     ├─ Action API? → requireOwnership() 检查空间归属 → 放行/拒绝
  │     │
  │     └─ Admin API? → AdminAuthMiddleware 拒绝（需要 cookie）
  │
  └─ 无认证 → 401
```

ServiceAuth ticket 只适用于 Action API 和 My Resources。
Admin API 和 Exposure API 需要 admin 权限。
