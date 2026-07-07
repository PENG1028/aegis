# Service API Reference

注册到 Aegis ServiceAuth 集群后，服务可以用自己的 Ed25519 票证直接调用 Aegis 管理 API，无需 admin token。

> **自用与对外同一套 API。** Aegis 面板内部的「快速创建域名」功能调的就是这些端点。所以它们稳定、完整、有 UI 参考案例（见 `/fabric/auth` 页面）。

---

## 快速开始

从零到一个域名映射，三步：

```bash
# 1. 注册到 ServiceAuth（只需一次）
POST /api/service-auth/v1/register
X-Content-Type: application/json
{ "service_name": "my-app", "host": "127.0.0.1", "port": 3000, "public_key": "<your_ed25519_pub>" }

→ { "service_id": "svc_xxx", "instances": [...], "sync_interval": 30 }

# 2. 创建域名映射（以后每次调这个就行）
POST /api/v1/actions/bind-http-domain
X-Service-Ticket: <ed25519_ticket>
X-Caller-Service: my-app
Content-Type: application/json

{ "domain": "myapp.example.com", "target_host": "127.0.0.1", "target_port": 3000 }

→ { "operation_id": "op_xxx", "status": "success", "message": "bound HTTP domain ..." }

# 3. 查看自己的资源
GET /api/v1/my/routes
X-Service-Ticket: <ed25519_ticket>
X-Caller-Service: my-app

→ { "routes": [...], "count": 1 }
```

---

## 认证方式

所有 API 通过 `X-Service-Ticket` header 认证，SDK 自动签名：

```go
resp, _ := client.CallAegis(ctx, "/api/v1/actions/bind-http-domain", "POST", body)
// CallAegis 自动：签 Ed25519 ticket → 设 X-Service-Ticket header → 发请求
```

### 手动 HTTP 调用（curl / 非 Go 服务）

```
POST /api/v1/actions/bind-http-domain
X-Service-Ticket: <base64(caller:target:api:expiry:ed25519_sig)>
X-Caller-Service: my-service-name
Content-Type: application/json

{ "domain": "myapp.example.com", ... }
```

### 如何构造 ticket（非 Go 服务）

见 [`docs/serviceauth-protocol.md`](serviceauth-protocol.md)：
1. payload = `caller_name:target_name:api_path:unix_timestamp`
2. signature = Ed25519.Sign(private_key, payload)
3. ticket = base64(payload + ":" + signature)
4. 设 header `X-Service-Ticket: <ticket>`

### 票证验证过程

1. Aegis 解码 ticket → 提取 `caller_name`
2. 查 `svc_auth_services` 中该 name 的 Ed25519 公钥
3. 本地验签（零网络开销）
4. 检查服务未被 block
5. 检查访问策略（policy）
6. 通过 → 所有操作视为该服务的空间范围

---

## 资源隔离

每个服务有自己的空间（space），以 `SpaceID=<service-name>` 标识：

```
             Admin                          服务 A                   服务 B
        ┌──────────────┐           ┌─────────────────┐      ┌─────────────────┐
        │  看到全部     │           │  只看自己的       │      │  只看自己的       │
        │  域名/路由/   │           │  域名/路由/       │      │  域名/路由/       │
        │  服务/规则    │           │  服务/规则        │      │  服务/规则        │
        └──────────────┘           └─────────────────┘      └─────────────────┘
              ↑                            ↑                       ↑
         Admin Cookie /              X-Service-Ticket        X-Service-Ticket
         Bearer Token                 (服务 A 的密钥)          (服务 B 的密钥)
```

- 服务只能操作自己创建的资源（`OwnerType="space"` + `OwnerID=<service-name>`）
- 不能跨空间访问
- 不能访问系统资源（`SpaceID=""`）
- admin 可以看到全部
- **域名全局唯一：** 一旦被一个空间绑定，其他空间不能再绑定同域名（返回 409）

---

## 端点清单

### 域名管理

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/actions/bind-http-domain` | 绑定 HTTP 域名到后端 |
| `POST` | `/api/v1/actions/bind-tls-backend` | 绑定 TLS 直通（TCP 转发） |
| `PATCH` | `/api/v1/actions/update-target` | 更新后端目标地址 |
| `POST` | `/api/v1/actions/disable-domain` | 禁用域名 |
| `DELETE` | `/api/v1/actions/domain` | 删除域名 |

#### `POST /api/v1/actions/bind-http-domain`

绑定 HTTP(S) 域名到后端。Aegis 自动创建 Service → Endpoint → Route → Edge Rule → Apply。

**请求：**
```json
{
  "domain": "myapp.example.com",
  "target_host": "127.0.0.1",
  "target_port": 3000,
  "gateway_link_id": "gl_xxx",
  "cert_id": "cert_xxx"
}
```

| 字段 | 必需 | 说明 |
|------|------|------|
| `domain` | 是 | 域名，全局唯一 |
| `target_host` | 是 | 后端 IP 或主机名 |
| `target_port` | 否 | 默认 80 |
| `gateway_link_id` | 否 | 网关链路 ID（跨机转发） |
| `cert_id` | 否 | 自定义 TLS 证书 ID |

**响应：**
```json
{
  "operation_id": "op_xxx",
  "status": "success",
  "message": "bound HTTP domain myapp.example.com -> 127.0.0.1:3000",
  "details": "service_id=svc_xxx route_id=rt_xxx"
}
```

**错误场景：**

| 错误码 | HTTP 状态 | 触发条件 |
|--------|-----------|----------|
| `SCOPE_DENIED` | 403 | ticket 无效或服务被 block |
| `DOMAIN_ALREADY_OWNED` | 409 | 域名已被其他空间绑定 |
| `TARGET_NOT_ALLOWED` | 400 | target_host 为空或端口不在 1-65535 范围 |
| `APPLY_LOCKED` | 423 | 另一个 Apply 正在进行，请重试 |
| `PROVIDER_MISSING` | 500 | 缺少必要的 Provider（Caddy/HAProxy） |
| `CONFIG_VALIDATE_FAILED` | 500 | Caddyfile 校验失败 |
| `RELOAD_FAILED` | 500 | Caddy 重载失败 |

**curl 示例：**
```bash
TICKET="<base64 ed25519 ticket>"

curl -X POST http://127.0.0.1:7380/api/v1/actions/bind-http-domain \
  -H "X-Service-Ticket: $TICKET" \
  -H "X-Caller-Service: my-app" \
  -H "Content-Type: application/json" \
  -d '{"domain": "myapp.example.com", "target_host": "127.0.0.1", "target_port": 3000}'
```

---

#### `POST /api/v1/actions/bind-tls-backend`

绑定 SNI 域名到 TCP 后端（TLS 透传，不终止 TLS）。

**请求：**
```json
{
  "sni_host": "db.example.com",
  "target_host": "192.168.1.10",
  "target_port": 5432
}
```

| 字段 | 必需 | 说明 |
|------|------|------|
| `sni_host` | 是 | SNI 域名 |
| `target_host` | 是 | 后端 IP |
| `target_port` | 否 | 默认 443 |

**错误场景：**

| 错误码 | HTTP 状态 | 触发条件 |
|--------|-----------|----------|
| `TARGET_NOT_ALLOWED` | 400 | sni_host/target_host 为空、端口超范围、target_port<1024（受限端口） |
| `SCOPE_DENIED` | 403 | 无权限 |
| `DOMAIN_ALREADY_OWNED` | 409 | SNI 域名已被占用 |
| `APPLY_LOCKED` | 423 | 另一个 Apply 正在进行 |

**curl 示例：**
```bash
curl -X POST http://127.0.0.1:7380/api/v1/actions/bind-tls-backend \
  -H "X-Service-Ticket: $TICKET" \
  -H "X-Caller-Service: my-app" \
  -H "Content-Type: application/json" \
  -d '{"sni_host": "db.example.com", "target_host": "10.0.0.5", "target_port": 5432}'
```

---

#### `PATCH /api/v1/actions/update-target`

更新已绑定的域名后端地址。更新后自动 Apply。

**请求：**
```json
{
  "resource_id": "svc_xxx_or_rt_xxx",
  "resource_type": "service",
  "target_host": "10.0.0.5",
  "target_port": 8080
}
```

| 字段 | 必需 | 说明 |
|------|------|------|
| `resource_id` | 是 | 资源 ID（service 或 edge_rule） |
| `resource_type` | 是 | `"service"` 或 `"edge_rule"` |
| `target_host` | 是 | 新的后端 IP |
| `target_port` | 是 | 新的后端端口 |

**错误场景：**

| 错误码 | HTTP 状态 | 触发条件 |
|--------|-----------|----------|
| `RESOURCE_NOT_FOUND` | 404 | resource_type 无效（不是 service 也不是 edge_rule） |
| `RESOURCE_NOT_OWNED` | 403 | 资源不属于本空间 |
| `APPLY_LOCKED` | 423 | 另一个 Apply 正在进行 |

**curl 示例：**
```bash
curl -X PATCH http://127.0.0.1:7380/api/v1/actions/update-target \
  -H "X-Service-Ticket: $TICKET" \
  -H "X-Caller-Service: my-app" \
  -H "Content-Type: application/json" \
  -d '{"resource_id": "svc_xxx", "resource_type": "service", "target_host": "10.0.0.5", "target_port": 8080}'
```

---

#### `POST /api/v1/actions/disable-domain`

禁用域名路由，不删除资源。

**请求：**
```json
{
  "domain": "myapp.example.com"
}
```

**错误场景：**

| 错误码 | HTTP 状态 | 触发条件 |
|--------|-----------|----------|
| `RESOURCE_NOT_FOUND` | 404 | 域名不存在 |
| `RESOURCE_NOT_OWNED` | 403 | 域名属于其他空间 |

**curl 示例：**
```bash
curl -X POST http://127.0.0.1:7380/api/v1/actions/disable-domain \
  -H "X-Service-Ticket: $TICKET" \
  -H "X-Caller-Service: my-app" \
  -H "Content-Type: application/json" \
  -d '{"domain": "myapp.example.com"}'
```

---

#### `DELETE /api/v1/actions/domain`

删除域名及所有关联资源（service + endpoint + route + edge rule）。

**请求：**
```json
{
  "domain": "myapp.example.com"
}
```

**错误场景：**

| 错误码 | HTTP 状态 | 触发条件 |
|--------|-----------|----------|
| `RESOURCE_NOT_FOUND` | 404 | 域名不存在 |
| `RESOURCE_NOT_OWNED` | 403 | 域名属于其他空间 |
| `APPLY_LOCKED` | 423 | 另一个 Apply 正在进行 |

**curl 示例：**
```bash
curl -X DELETE http://127.0.0.1:7380/api/v1/actions/domain \
  -H "X-Service-Ticket: $TICKET" \
  -H "X-Caller-Service: my-app" \
  -H "Content-Type: application/json" \
  -d '{"domain": "myapp.example.com"}'
```

---

### 资源查看

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/my/routes` | 查看自己的路由列表 |
| `GET` | `/api/v1/my/services` | 查看自己的服务列表 |
| `GET` | `/api/v1/my/edge-rules` | 查看自己的 SNI 规则 |
| `GET` | `/api/v1/my/operations` | 查看自己的操作记录 |

返回 scope = "space" 的资源。admin 调用显示全部。

**响应结构：**
```json
{
  "routes": [
    {
      "id": "rt_xxx",
      "domain": "myapp.example.com",
      "service_id": "svc_xxx",
      "status": "active",
      "tls_enabled": true,
      "created_at": "2026-07-07T..."
    }
  ],
  "count": 1
}
```

**curl 示例：**
```bash
curl http://127.0.0.1:7380/api/v1/my/routes \
  -H "X-Service-Ticket: $TICKET" \
  -H "X-Caller-Service: my-app"
```

---

## SDK 用法

```go
import "aegis/pkg/serviceauth"

func main() {
    client, _ := serviceauth.New(serviceauth.Config{
        ServiceName: "my-service",
        ServicePort: 8080,
    })
    client.Register(context.Background())
    defer client.Close()

    // 创建域名映射——零 admin token 配置
    body := map[string]interface{}{
        "domain":      "myapp.example.com",
        "target_host": "127.0.0.1",
        "target_port": 3000,
    }
    resp, err := client.CallAegis(ctx, "/api/v1/actions/bind-http-domain", "POST", body)
    // ↑ 自动签 Ed25519 ticket，自动设 X-Service-Ticket header

    // 查看自己的路由
    routes, _ := client.CallAegis(ctx, "/api/v1/my/routes", "GET", nil)
}
```

更多模式（内部调用、用户认证混合、API Key 混合、任意认证）见
[`docs/serviceauth-patterns.md`](serviceauth-patterns.md)。

---

## admin 操作 vs 服务操作

| 操作 | admin API | 服务 API |
|------|-----------|----------|
| 创建域名 | `POST /api/v1/actions/bind-http-domain` | 相同 |
| 删除域名 | `DELETE /api/v1/actions/domain` | 相同 |
| 列出所有服务 | `GET /api/admin/v1/service-auth/services` | ❌ |
| 创建服务组 | `POST /api/admin/v1/service-auth/groups` | ❌ |
| 查看系统状态 | `GET /api/system/status` | 相同（公开） |
| 基础设施检测 | `GET /api/admin/v1/infra/status` | ❌ |
| 查看自己的服务 | `GET /api/v1/my/services` | 相同 |

---

## 错误码速查

| 错误码 | HTTP 状态 | 含义 | 常见原因 |
|--------|-----------|------|----------|
| `SCOPE_DENIED` | 403 | 没有操作权限 | 缺少或无效的 ticket |
| `DOMAIN_ALREADY_OWNED` | 409 | 域名已被占用 | 域名已被其他服务绑定 |
| `TARGET_NOT_ALLOWED` | 400 | 后端地址不合法 | target_host 为空，端口超范围 |
| `PROVIDER_MISSING` | 500 | 缺少必要 Provider | Caddy/HAProxy 未配置 |
| `CONFIG_VALIDATE_FAILED` | 500 | Caddyfile 校验失败 | 配置语法错误 |
| `APPLY_LOCKED` | 423 | Apply 正在执行 | 并发冲突，稍后重试 |
| `RELOAD_FAILED` | 500 | Caddy/HAProxy 重载失败 | 配置应用但运行时加载失败 |
| `RUNTIME_VERIFY_FAILED` | 500 | 运行时验证失败 | 重载后健康检查不通过 |
| `LISTENER_CONFLICT` | 500 | 端口冲突 | 端口已被其他服务占用 |
| `RESOURCE_NOT_FOUND` | 404 | 资源不存在 | domain/resource_id 未找到 |
| `RESOURCE_NOT_OWNED` | 403 | 资源不属于本空间 | 尝试操作其他空间的资源 |
| `QUOTA_EXCEEDED` | 403 | 超过配额限制 | 域名数量/转发规则数量超限 |

所有错误响应格式：
```json
{
  "error": {
    "code": "SCOPE_DENIED",
    "message": "detailed description",
    "details": "optional_detail"
  }
}
```

---

## 限制

- 只能操作自己创建的资源（`requireOwnership` 校验）
- 不能访问 `/api/admin/v1/*` 端点（管理后台、节点管理、系统配置）
- 域名全局唯一，不能与其他空间冲突
- 服务重启后身份不变（按 name 匹配），Ed25519 私钥持久化在 `~/.aegis/keys/<name>.key`
- 同名不同 Ed25519 密钥 = 不同实例（Gray Deploy 支持）
- 僵尸实例：3 分钟无心跳 → inactive，1 小时无心跳 → 自动清除
- 同步间隔：注册时返回 `sync_interval`（默认 30 秒），用于拉取集群变更
- 请求体上限：1 MB
- 服务名称一旦注册不可更改（需 admin Rebind）

---

## 对外使用说明

这套 API 是为以下场景设计的：

1. **Aegis 自用：** 面板后台创建域名时调的正是这些端点。所以你看到的 `/fabric/auth` 页面既是 Aegis 的功能页面，也是给你的 UI 参考案例。
2. **服务自管理：** 你的服务注册后，可以用同样的端点管理自己的域名，不需要人工审批。
3. **外部系统集成：** 任何能签 Ed25519 票证的系统都可以调这些 API。

关键原则：**Aegis 不在你自己的服务和你的域名之间插入任何额外认证。** 服务注册的那一刻起，它就是自己域名的管理员。
