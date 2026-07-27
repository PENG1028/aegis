# ServiceAuth

服务间身份认证。一个服务注册后，用 Ed25519 签名证明"我是谁"。
其他服务通过调用方名字做权限判断。

**ServiceAuth 不处理权限。** 它只回答"谁在调你"。权限是服务自己的中间件决定的。

---

## 概念

```
服务注册表    所有已注册服务的名单（名字 + 公钥）
ticket       Ed25519 签名的身份凭证，固定 5 分钟有效
Guard        验证 ticket 的中间件。验证通过后注入调用方名字
Post(url)    调用另一个服务时自动签 ticket
Do()         自动报告调用日志 → 拓扑数据
sync         客户端每 30s 拉取注册表变更（公钥、封禁列表）
```

---

## 注册

```go
client, _ := serviceauth.New(Config{
    ServiceName: "privacy-policy",
})
client.Register(ctx)
// 做了：生成 Ed25519 密钥对 → 存在 ~/.aegis/keys/privacy-policy.key
//       只交公钥 → 服务器存下 → 返回其他服务的公钥
//       启动 sync 每 30s 拉更新
```

**注册只需要名字。** 不需要端口、不需要路径、不需要暴露的 API 列表。

---

## 调用另一个服务

```go
// 方式 A：直接用 client 调（ticket 自动签）
resp, err := client.Post(ctx, "http://target-service:8080/api/action", body)
resp, err := client.Get(ctx, "http://target-service/api/data")
resp, err := client.Put(ctx, "http://target-service/api/resource", body)
resp, err := client.Delete(ctx, "http://target-service/api/resource")

// 健康检查
if client.Healthy("http://target-service:8080/healthz") {
    log.Println("target-service 在线")
}
```

URL 可以是内部地址（`http://service-name:port/path`）或域名（走内部 DNS）。推荐域名。

---

## 服务端双路径认证

一个服务可以同时接受两种认证，外部调用方看不到 ServiceAuth 的存在：

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 路径 A：ServiceAuth ticket（内部服务间调用）
        if ticket := r.Header.Get("X-Service-Ticket"); ticket != "" {
            caller := verifyTicket(ticket, publicKeys)
            if caller != "" {
                ctx := context.WithValue(r.Context(), "caller", caller)
                next.ServeHTTP(w, r.WithContext(ctx))
                return
            }
        }
        // 路径 B：API Key（外部调用）
        if key := r.Header.Get("X-API-Key"); key != "" {
            if validateAPIKey(key) {
                next.ServeHTTP(w, r)
                return
            }
        }
        http.Error(w, "unauthorized", http.StatusUnauthorized)
    })
}
```

两种认证在同一层——ServiceAuth 和 API Key 是并列关系，不是包含关系。

---

## 提供服务

```go
mux.Handle("POST /api/verify", client.Guard(
    http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        caller := serviceauth.CallerFromContext(r.Context())
        // caller.ServiceName == "privacy-policy"
        // 就一个名字。权限你自己决定
    }),
))
```

---

## 自动调用报告

`client.Do()` 在每次 2xx 响应后，会自动 POST `/api/service-auth/v1/report` 记录调用关系。
这是拓扑数据的来源——谁调了谁、调了多少次、最后调用时间。

不需要额外配置。如果不需要报告调用日志，直接用标准 `http.Client` 代替。

---

## 调用关系查询（Callers / Deps）

每个服务可以查"谁调了我"和"我调了谁"：

```go
// 谁调了我（返回 CallerEdge 列表）
callers, _ := client.Callers(ctx, "24h")

// 我调了谁（返回 DepEdge 列表）
deps, _ := client.Deps(ctx, "24h")
```

数据来源：`client.Do()` 自动报告的调用日志。只返回有实际调用关系的服务。

Scoped API（服务自己查）：`GET /api/service-auth/v1/services?window=24h`（需 X-Service-Ticket）

Admin API（全景查看）：`GET /api/admin/v1/service-auth/topology?window=24h`（需 admin session）

---

## 管理自己的域名和服务（Action API）

带上 ServiceAuth ticket 可以调 Aegis Action API 来管理域名映射。
你的服务名会自动映射为一个独立空间，你只能操作自己空间内的资源。

```go
// 创建 HTTP 域名映射
resp, err := client.Post(ctx, aegisURL+"/api/v1/actions/bind-http-domain", body)
// body: {"domain": "myapp.example.com", "target_host": "127.0.0.1", "target_port": 3000}

// 查看自己空间的资源
routes, _ := client.Get(ctx, aegisURL+"/api/v1/my/routes")
services, _ := client.Get(ctx, aegisURL+"/api/v1/my/services")
```

不需要 admin token。ticket 自动证明身份，Aegis 自动按空间隔离。

可用端点：

| 端点 | 用途 |
|------|------|
| `POST /api/v1/actions/bind-http-domain` | 绑定 HTTP 域名 → 后端 |
| `POST /api/v1/actions/bind-tls-backend` | 绑定 TLS/SNI 后端 |
| `PATCH /api/v1/actions/update-target` | 更新已绑定的目标 |
| `POST /api/v1/actions/disable-domain` | 禁用域名 |
| `DELETE /api/v1/actions/domain` | 删除域名 |
| `GET /api/v1/my/routes` | 查看我的路由 |
| `GET /api/v1/my/services` | 查看我的服务 |
| `GET /api/v1/my/edge-rules` | 查看我的 SNI 规则 |
| `GET /api/v1/my/operations` | 查看我的操作记录 |

---

## 非 Go 服务接入（Next.js / Python / 任意语言）

### 注册

```bash
POST /api/service-auth/v1/register
Content-Type: application/json

{
  "service_name": "my-service",
  "public_key": "<base64 Ed25519 公钥>"
}

→ 200 { "service_id": "svc_xxx", "public_keys": {...}, "sync_interval": 30 }
```

### 获取当前所有服务的公钥

```bash
# sync 端点在注册时返回的 sync_interval 秒内调用一次即可
GET /api/service-auth/v1/sync?bl_version=0

→ 200 { "public_keys": {"auth-service": "<base64>", ...},
         "blocklist": [...] }
→ 304 （无变更）
```

### 构造 ticket

```javascript
// Next.js 示例
import nacl from "tweetnacl"
import { encodeBase64, decodeBase64 } from "@std/encoding"

function signTicket(serviceName, privateKeyBase64) {
    const expiresAt = Math.floor(Date.now() / 1000) + 300 // 5分钟
    const payload = `${serviceName}:${expiresAt}`
    const signature = nacl.sign.detached(
        new TextEncoder().encode(payload),
        decodeBase64(privateKeyBase64)
    )
    return encodeBase64(
        new TextEncoder().encode(payload + ":" + encodeBase64(signature))
    )
}
```

### 调用其他服务

```typescript
// Next.js Route Handler
const ticket = signTicket("my-service", process.env.SERVICE_PRIVATE_KEY!)
const resp = await fetch("http://target-service:8080/api/action", {
    method: "POST",
    headers: {
        "Content-Type": "application/json",
        "X-Service-Ticket": ticket,
    },
    body: JSON.stringify({ ... })
})
```

### 验证传入请求（即在 Next.js 里做 Guard 的事）

```typescript
// middleware.ts 或 Route Handler 里
function verifyIncomingTicket(req: Request, publicKeys: Record<string, string>) {
    const ticket = req.headers.get("X-Service-Ticket")
    if (!ticket) return null

    const decoded = decodeBase64(ticket)
    const parts = new TextDecoder().decode(decoded).split(":")
    // parts = [callerName, expiry, signature]

    const pubKey = publicKeys[parts[0]]
    if (!pubKey) return null

    const payload = `${parts[0]}:${parts[1]}`
    const isValid = nacl.sign.detached.verify(
        new TextEncoder().encode(payload),
        decodeBase64(parts[2]),
        decodeBase64(pubKey)
    )
    return isValid ? parts[0] : null
}
```

---

## 检测新服务

发现靠 topology（调用日志聚合），**不依赖全局服务名单**。
只看有调用关系的服务——你调过谁、谁调过你——才是真正需要关心的。

```go
// Topology API — 返回有调用关系的服务对
// Admin API: GET /api/admin/v1/service-auth/topology?window=24h
```

---

## 权限模型

**ServiceAuth 不替代你的权限系统。它只是多了一种证明身份的方式。**

```go
// ─── 你原本的权限中间件 ───

type Identity struct {
    Source string // "serviceauth" | "api_key" | "session" | "public"
    Name   string // 服务名 / key 持有者 / 用户 ID
    Role   string // "admin" | "member" | "viewer"
}

func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var identity *Identity

        // 第一种身份：ServiceAuth ticket
        if caller := tryServiceAuth(r); caller != "" {
            identity = &Identity{Source: "serviceauth", Name: caller}
        }

        // 第二种身份：API Key
        if key := r.Header.Get("X-API-Key"); key != "" && identity == nil {
            if info, err := validateAPIKey(key); err == nil {
                identity = &Identity{Source: "api_key", Name: info.Name, Role: info.Role}
            }
        }

        // 第三种身份：Session Cookie
        if cookie := r.Header.Get("Cookie"); cookie != "" && identity == nil {
            if user := validateSession(cookie); user != nil {
                identity = &Identity{Source: "session", Name: user.ID, Role: user.Role}
            }
        }

        if identity == nil {
            http.Error(w, `{"error":"unauthorized"}`, 401)
            return
        }

        ctx := context.WithValue(r.Context(), "identity", identity)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**ServiceAuth 和 API Key 在同一层次。** 你的中间件可以：

| 策略 | 做法 |
|------|------|
| 只收 ticket | 不收 API Key。纯内部网络 |
| 只收 API Key | 不收 ticket。传统对外 API |
| 两者都收 | ticket 优先，API Key 降级。外部和内部走同一端点 |
| 公开端点 | 什么都不收。健康检查等 |

---

## 密钥丢失和身份恢复

### 场景

```
服务 C 运行中，密钥在 ~/.aegis/keys/c-service.key
↓
机器故障，密钥文件丢失
↓
C 重启 → 检测到无密钥 → 生成新密钥对
↓
C 重新 Register({Name: "c-service", PublicKey: 新公钥})
↓
服务器：INSERT 新行（c-service + 新公钥），旧行仍然在 DB
↓
其他服务 sync：ListPublicKeys 返回 name→pubkey map
         → 新公钥覆盖旧公钥
↓
C 的新 ticket（新私钥签的）→ 其他服务验签通过 ✅
```

---

## 多公钥支持

Guard 会遍历所有匹配 name 的公钥验签。同名多 key（灰度、密钥轮换、多实例）自动支持，不需要改代码。

## 注册警告

Register 返回 `Warnings` 字段：
- 同名已有不同公钥 → "可能是密钥轮换或多实例"
- 同公钥用于不同名字 → "两个服务共享同一私钥"
