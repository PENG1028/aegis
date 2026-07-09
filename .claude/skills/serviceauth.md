---
name: serviceauth
description: ServiceAuth — 服务间认证系统。注册、调用、Guard、IP检查、双路径认证、故障排查
model: sonnet
---

# ServiceAuth Skill

ServiceAuth 是 Aegis 内置的服务间认证系统，不是独立服务。
每个服务注册 Ed25519 公钥，调用时用私钥签 ticket，接收方验 ticket 即可确认"谁在调我"。

**核心哲学：ServiceAuth 是基础设施，不是 SDK。** 业务 SDK 不包含 ServiceAuth 逻辑——认证由调用方通过 `http.Client` 注入，业务代码只关心 HTTP 请求。

---

## 核心概念

- **服务注册表**：存 `name → [public_keys]`，不存 URL/IP/端口
- **ticket**：Ed25519 签名 `base64(caller:expiry:signature)`，5 分钟有效
- **Guard**：先检查 IP（默认仅内网），再验证 ticket，注入 `CallerInfo{ServiceName}`
- **Post/Get/Put/Delete**：自动签 ticket，直接调 URL
- **Do() 自动报告**：2xx 响应后自动 POST `/api/service-auth/v1/report`，填充拓扑调用关系
- **sync**：每 30s 拉公钥、blocklist（不拉 groups/policies — 已移除）
- **WrapHTTP**：把 ServiceAuth 认证注入标准 `http.Client`，业务 SDK 零感知

---

## 注册

```go
client, _ := serviceauth.New(Config{ServiceName: "my-service"})
client.Register(ctx)
// 生成 Ed25519 密钥 → 存 ~/.aegis/keys/my-service.key → 交公钥
```

---

## 调用

```go
// 方式 A：直接用 client 调（ticket 自动签）
client.Post(ctx, "http://target-service:8080/api/action", body)
client.Get(ctx, "http://target-service:8080/api/data")

// 方式 B：WrapHTTP — 把认证注入 http.Client，业务 SDK 无感
httpClient := client.WrapHTTP(http.DefaultClient)
sdk := usermgmt.New("http://user-mgmt:8080", sdk.WithHTTPClient(httpClient))
// sdk.CheckUser() 发出的每个请求自动带 ticket，SDK 本身不知道 ServiceAuth
```

---

## 提供服务（Guard）

```go
mux.Handle("POST /api/verify", client.Guard(handler))
// handler 里：
caller := serviceauth.CallerFromContext(r.Context())
// caller.ServiceName —— 就一个名字，权限自己决定
```

---

## 双路径认证

ServiceAuth 支持两种认证方式，服务端可同时启用：

```go
// 服务端同时接受 ticket 和 API Key
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 路径 A：ServiceAuth ticket（内部服务间调用）
        if ticket := r.Header.Get("X-Service-Ticket"); ticket != "" {
            caller := serviceauth.VerifyTicket(ticket, publicKeys)
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

---

## IP 白名单（默认仅内网）

```go
// 开发模式：加一个临时外网 IP，最长 24h 硬编码
checker := serviceauth.NewWhitelistChecker(serviceauth.AllowCluster())
checker.SetWhitelist(map[string]time.Time{
    "114.114.114.114": time.Now().Add(2 * time.Hour),
})
client.SetIPChecker(checker)

// 完全不限制（不安全，仅调试用）
client.SetIPChecker(serviceauth.AllowAll())
```

---

## 多公钥支持

Guard 会遍历所有匹配 name 的公钥验签。同名多 key（灰度、密钥轮换、多实例）自动支持，不需要改代码。

## 注册警告

Register 返回 `Warnings` 字段：
- 同名已有不同公钥 → "可能是密钥轮换或多实例"
- 同公钥用于不同名字 → "两个服务共享同一私钥"

---

## 调用关系查询

Scoped API（服务自己查）：`GET /api/service-auth/v1/services?window=24h`
- 返回该服务的调用方（谁调了我）和依赖方（我调了谁）
- 需要 X-Service-Ticket 认证

Admin API（全景查看）：`GET /api/admin/v1/service-auth/topology?window=24h`
- 返回全部拓扑边
- 需要 admin session 认证

---

## 常见故障

| 问题 | 原因 | 解决 |
|------|------|------|
| Guard 返回 403 "unknown caller" | 对方 sync 没拉到你的公钥 | 等 ≤30s sync |
| Guard 返回 403 "invalid ticket" | 密钥丢了重新生成，旧 ticket 还在传输 | 等几秒重试 |
| Guard 返回 403 "untrusted IP" | 调用方从外网 IP 调过来 | 加白名单或走内网 |
| Post 超时 | 目标服务挂了或地址不对 | 检查 URL，配置正确 |
| Register 返回 warnings | 同名多 key 或同 key 多 name | 确认是否预期行为 |

## 密钥丢失恢复

```bash
# 自动重新注册（名不变，资源不丢，≤30s 恢复）
# 备份：~/.aegis/keys/<name>.key
# 双机：scp ~/.aegis/keys/<name>.key user@第二台:~/.aegis/keys/
```

---

## 非 Go 服务

```typescript
// 注册
POST /api/service-auth/v1/register
{ "service_name": "my-service", "public_key": "<base64 Ed25519 pub>" }

// 同步公钥
GET /api/service-auth/v1/sync?bl_version=0

// 调用（构造 ticket）
const ticket = base64(callerName + ":" + expiry + ":" + base64(signature))
POST /api/xxx
X-Service-Ticket: <ticket>

// 验证（Guard 等价实现）
// 1. 解码 ticket → get caller name
// 2. 查 sync 拉到的 publicKeys[name] → [pubkey1, pubkey2, ...]
// 3. 遍历验签，任一通过即放行
```

---

## 关键文件

| 文件 | 内容 |
|------|------|
| `pkg/serviceauth/client.go` | SDK 客户端 |
| `pkg/serviceauth/guard.go` | Guard 中间件 |
| `pkg/serviceauth/ipcheck.go` | IP 白名单 |
| `pkg/serviceauth/ticket.go` | Ed25519 签名 |
| `internal/serviceauth/service.go` | 服务端注册逻辑 |
| `docs/serviceauth.md` | 完整文档 |
| `docs/service-api-reference.md` | API 参考 |
