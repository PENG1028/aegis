# Service API Reference

注册到 Aegis ServiceAuth 集群后，服务可以用自己的 Ed25519 票证直接调用 Aegis 管理 API，无需 admin token。

## 认证方式

所有 API 通过 `X-Service-Ticket` header 认证，SDK 自动签名：

```go
resp, _ := client.CallAegis(ctx, "/api/v1/actions/bind-http-domain", "POST", body)
// CallAegis 自动：签 Ed25519 ticket → 设 X-Service-Ticket header → 发请求
```

手动使用（非 Go 服务）：

```
POST /api/v1/actions/bind-http-domain
X-Service-Ticket: <base64(caller:target:api:expiry:ed25519_sig)>
X-Caller-Service: my-service-name
Content-Type: application/json

{ "domain": "myapp.example.com", ... }
```

票证验证过程：
1. Aegis 解码 ticket → 提取 `caller_name`
2. 查 `svc_auth_services` 中该 name 的 Ed25519 公钥
3. 本地验签（零网络开销）
4. 检查服务未被 block
5. 通过 → 所有操作视为该服务的空间范围

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

请求：
```json
{
  "domain": "myapp.example.com",
  "target_host": "127.0.0.1",
  "target_port": 3000
}
```

响应：
```json
{
  "operation_id": "op_xxx",
  "status": "success",
  "message": "bound HTTP domain myapp.example.com -> 127.0.0.1:3000",
  "details": "service_id=svc_xxx route_id=rt_xxx"
}
```

自动完成：
- 创建 Service → 创建 Endpoint → 创建 Route → 创建 SNI 规则 → Apply Caddyfile

#### `POST /api/v1/actions/bind-tls-backend`

```json
{
  "sni_host": "db.example.com",
  "target_host": "192.168.1.10",
  "target_port": 5432
}
```

仅创建 SNI 规则，不终止 TLS，适用于 TCP 透传。

#### `PATCH /api/v1/actions/update-target`

```json
{
  "domain": "myapp.example.com",
  "target_host": "10.0.0.5",
  "target_port": 8080
}
```

更新后自动 Apply。

#### `POST /api/v1/actions/disable-domain`

```json
{
  "domain": "myapp.example.com"
}
```

禁用路由，不影响其他资源。

#### `DELETE /api/v1/actions/domain`

```json
{
  "domain": "myapp.example.com"
}
```

删除域名及所有关联资源（service + endpoint + route + edge rule）。

### 资源查看

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/my/routes` | 查看自己的路由列表 |
| `GET` | `/api/v1/my/services` | 查看自己的服务列表 |
| `GET` | `/api/v1/my/edge-rules` | 查看自己的 SNI 规则 |
| `GET` | `/api/v1/my/operations` | 查看自己的操作记录 |

返回 scope = "space" 的资源。admin 调用显示全部。

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

## admin 操作 vs 服务操作

| 操作 | admin API | 服务 API |
|------|-----------|---------|
| 创建域名 | `POST /api/v1/actions/bind-http-domain` | 相同 |
| 删除域名 | `DELETE /api/v1/actions/domain` | 相同 |
| 列出所有服务 | `GET /api/admin/v1/service-auth/services` | ❌ |
| 创建服务组 | `POST /api/admin/v1/service-auth/groups` | ❌ |
| 查看系统状态 | `GET /api/system/status` | 相同（公开） |
| 基础设施检测 | `GET /api/admin/v1/infra/status` | ❌ |
| 查看自己的服务 | `GET /api/v1/my/services` | 相同 |

## 错误码

| 状态码 | 含义 |
|--------|------|
| `200` | 成功 |
| `400` | 请求格式错误 |
| `401` | 缺少 ticket 或 ticket 无效 |
| `403` | 无权限（非本空间资源） |
| `404` | 资源不存在 |
| `409` | 冲突（域名已被占用） |
| `500` | 服务端错误 |

## 限制

- 只能操作自己创建的资源（`requireOwnership` 校验）
- 不能访问 `/api/admin/v1/*` 端点
- 域名不能与其他空间冲突
- 更新二进制后重新注册，身份不变（按 name 匹配）
