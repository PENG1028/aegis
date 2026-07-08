# ServiceAuth

服务间身份认证。一个服务注册后，用 Ed25519 签名证明"我是谁"。
其他服务通过调用方名字做权限判断。

---

## 概念

```
服务注册表    所有已注册服务的名单（名字 + 公钥）
ticket       Ed25519 签名的身份凭证，固定 5 分钟有效
Guard        验证 ticket 的中间件。验证通过后注入调用方名字
Post(url)    调用另一个服务时自动签 ticket
sync         客户端每 30s 拉取注册表变更（公钥、封禁列表）
```

**ServiceAuth 不处理权限。** 它只回答"谁在调你"。权限是服务自己的中间件决定的。

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
// 知道 URL 就行。ticket 自动签、自动放 X-Service-Ticket header
resp, err := client.Post(ctx, "http://auth-service:8080/api/verify", body)
resp, err := client.Get(ctx, "http://target-service/api/data")
resp, err := client.Put(ctx, "http://target-service/api/resource", body)
resp, err := client.Delete(ctx, "http://target-service/api/resource")

// 健康检查
if client.Healthy("http://auth-service:8080/healthz") {
    log.Println("auth-service 在线")
}
```

URL 可以是内部地址（`http://service-name:port/path`）或域名（走内部 DNS）。推荐域名。

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
GET /api/service-auth/v1/sync?bl_version=0&cat_version=0

→ 200 { "public_keys": {"auth-service": "<base64>", ...},
         "blocklist": [...], "groups": [...], "policies": [...] }
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
const resp = await fetch("http://auth-service:8080/api/verify", {
    method: "POST",
    headers: {
        "Content-Type": "application/json",
        "X-Service-Ticket": ticket,
        "X-Caller-Service": "my-service"
    },
    body: JSON.stringify({ token: "..." })
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

## 检测新服务 + 自动放置

这是提供服务侧最常见的问题：**一个新服务注册后，我怎么知道它来了？把它放哪？**

### 发现

发现靠 topology（调用日志聚合），**不依赖全局服务名单**。
只看有调用关系的服务——你调过谁、谁调过你——才是真正需要关心的。

```go
// 方案 A：topology API（推荐）
type TopologyEdge struct {
    Caller string `json:"caller"`
    Target string `json:"target"`
    Count  int64  `json:"count"`
}
// 返回的边集就是有调用关系的服务对，天然范围限制

// 调用方式（admin API）：
// GET /api/admin/v1/service-auth/topology?window=24h

// 方案 B：sync 数据的 groups/policies（范围有限，仅同组或策略关联）
// client.Groups()  — 与你同组的服务
// client.Policies() — 策略里引用了你的服务
```

### 放置

```go
func handleNewService(name string) {
    switch classifyService(name) {
    case "infra":
        // 基础设施服务（aegis, auth, gateway）→ 放到 admin 项目，自动放行
        project := getOrCreateAdminProject()
        bindingSvc.Bind(project.ID, name, name)
        policySvc.Create(Policy{Subject: name, Effect: "allow"})

    case "external":
        // 外部服务 → 放到对应项目，默认拒绝
        project := getOrCreateProjectForService(name)
        bindingSvc.Bind(project.ID, name, name)
        // 不创建 policy，默认 deny，等管理员确认

    case "unknown":
        // 未知服务 → UI 显示"待接入"
        queueForApproval(name)
    }
}

func classifyService(name string) string {
    // 服务自己决定分类规则
    infraPrefixes := []string{"aegis", "auth", "gateway"}
    for _, p := range infraPrefixes {
        if strings.HasPrefix(name, p) { return "infra" }
    }
    return "unknown"
}
```

### 在 UI 里的展示

```
┌── 服务注册表 ───────────────────────────────────┐
│                                                  │
│  服务名            状态    已绑定项目    操作      │
│  aetherion-authn  ● 在线  admin         —        │
│  privacy-policy   ● 在线  项目 A        配置     │
│  billing           ○ 离线  项目 B       配置     │
│                                                  │
│  ⏳ 待处理                                         │
│  frontend-app（新出现）  [确认放置] [忽略]         │
└──────────────────────────────────────────────────┘
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

## 三种典型提供策略

### 全局放行（Aegis 自己）

```go
// 任何服务注册进来，自动放行。不需要审批，不需要隔离。
func handleCall(w http.ResponseWriter, r *http.Request) {
    caller := CallerFromContext(r.Context())
    // 不检查权限，直接执行业务
    doOperation(w, r)
}
```

### 按项目隔离（aetherion）

```go
// 每个服务有一个归属项目。只能操作自己项目内的资源。
func handleCreateDomain(w http.ResponseWriter, r *http.Request) {
    caller := CallerFromContext(r.Context())

    // 第一次遇到这个服务 → 自动创建项目
    project := autoProvision(caller.ServiceName)
    // 资源自动归到这个项目
    createDomainUnderProject(project.ID, body)
}
```

### 按 API Key Scope（传统模式）

```go
// ServiceAuth 和 API Key 走同一个权限判断
func handleDeleteResource(w http.ResponseWriter, r *http.Request) {
    identity := r.Context().Value("identity").(*Identity)

    switch identity.Source {
    case "serviceauth":
        // 内部服务用 ServiceAuth → 检查服务级别的权限
        if !isInternalAllowed(identity.Name, "delete") {
            http.Error(w, "forbidden", 403); return
        }
    case "api_key":
        // 外部开发者用 API Key → 检查 key 的 scope
        if identity.Role != "admin" {
            http.Error(w, "forbidden", 403); return
        }
    }
}
```

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
旧 ticket（旧私钥签的，传输中）→ 验签失败 ❌（最多 30s）
↓
资源：C 之前创建的路由和域名不变（因为名字没变）
```

### 恢复步骤

```bash
# 1. 服务自动恢复（重新 Register），不需要人工操作
# 2. 资源不丢，因为资源绑定的是「名字」不是「公钥」
# 3. 如果有备份 .key 文件，直接放回去覆盖，重启即可
# 4. 没有备份 → 正常运行。只是旧 ticket 会失败几十秒
```

### 稳定性总结

| 场景 | 影响 | 恢复方式 |
|------|------|---------|
| 正常重启（密钥文件在） | 零影响 | 自动 |
| 密钥丢失 | 旧 ticket 失败 ≤30s | 自动重新注册 |
| 双机部署 | 共享密钥即可 | 复制 .key 文件 |
| 服务下线 | Post(url) 超时 | 调用方处理重试 |
| 灰度更新 | 当前不支持多公钥 | 需要改 Guard 遍历逻辑 |

---

## 安全模型

| 防护 | 能防 | 不能防 |
|------|------|--------|
| 服务端被黑 | ✅ 只有公钥，签不了 ticket | — |
| 重放攻击 | ✅ ticket 5 分钟过期 | — |
| 单机被黑 | ✅ 只泄露本机服务的密钥 | — |
| 密钥丢失 | ✅ 重新注册即可 | — |
| 内网攻击者注册同名服务 | — | ❌ 需要额外的身份溯源机制 |
| 请求体篡改 | — | ❌ ticket 不签请求体 |
| DDoS | — | ❌ 不是认证层的事 |

---

## 当前限制

| 限制 | 说明 |
|------|------|
| Exposure API 无空间隔离 | TCP/UDP 端口转发 API（`/api/exposures/*`）未实现 ServiceAuth 空间隔离，需要 admin 权限。外部服务请用 Action API 管理域名 |
| Apply API 无 ServiceAuth 检查 | `POST /api/apply` 当前仅受 AuthMiddleware 保护（Bearer/ticket 均可触发）。建议保持 admin token 调用 |
| 策略引擎已移除 | `EvaluatePolicy` 函数已删除。Guard 只验证身份，权限由服务自己的中间件决定。Groups/Policies 的 UI 管理界面保留供参考 |

---

## 术语

| 术语 | 含义 |
|------|------|
| ServiceAuth | 服务间身份认证系统 |
| 服务注册表 | 所有已注册服务的名单（名字 + 公钥） |
| ticket | Ed25519 签名的身份凭证 |
| Guard | 验证 ticket 的 HTTP 中间件 |
| Post/Get/Put/Delete | HTTP 方法，自动签 ticket |
| 调用方 | 发起请求的服务 |
| 接收方 | 处理请求的服务 |
| 封禁 | 将某个服务从注册表中禁用 |
| sync | 客户端每 30s 拉取注册表变更 |
| 放置 | 新服务检测到后，分配到项目/空间的过程 |
