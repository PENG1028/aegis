# ServiceAuth

服务间认证系统。一个服务注册身份后，用 Ed25519 签名证明"我是谁"。

---

## 概念

```
服务注册表    所有注册服务的名单（名字 + 公钥）
ticket       Ed25519 签名，证明"我是 XXX"
Guard        验证 ticket 的中间件，通过后注入调用方名字
Post(url)    调用另一个服务，自动签 ticket
sync         每 30s 拉一次服务注册表变化
```

---

## 完整流程

### 1. 注册

```go
client, _ := serviceauth.New(Config{
    ServiceName: "privacy-policy",
})
client.Register(ctx)
// 做了：
//   1. 生成 Ed25519 密钥对，存 ~/.aegis/keys/privacy-policy.key
//   2. POST {name, public_key} 到服务器
//   3. 服务器存下来，返回其他服务的公钥
//   4. 启动 sync 每 30s 拉更新
```

### 2. 调用另一个服务

```go
// 知道 URL 就行。ticket 自动签、自动放 header。
resp, err := client.Post(ctx, "http://auth-service:8080/api/verify", body)
// 也可以调外部 URL（走内部 DNS + 透明代理）
resp, err := client.Post(ctx, "https://auth.internal.example.com/api/verify", body)

// 其他方法
client.Get(ctx, url)
client.Put(ctx, url, body)
client.Delete(ctx, url)

// 健康检查
if client.Healthy("http://auth-service:8080/healthz") {
    // 在线
}
```

### 3. 接收调用（提供服务）

```go
mux.Handle("POST /api/verify", client.Guard(
    http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        caller := serviceauth.CallerFromContext(r.Context())
        // caller.ServiceName == "privacy-policy"
        // 就一个名字，其他自己决定
    }),
))
```

### 4. 发现新服务 + 自动放置

```go
go func() {
    for {
        names := client.KnownServices() // 当前注册表里所有名字
        for _, name := range names {
            if !seen[name] {
                autoProvision(name) // 你自己的逻辑
                seen[name] = true
            }
        }
        time.Sleep(30 * time.Second)
    }
}()
```

---

## 注册表说明

| 内容 | 说明 |
|------|------|
| 服务名 | 唯一。注册时指定，不可变 |
| 公钥 | 注册时提交。重启后如果密钥文件还在，公钥不变 |
| 状态 | active / blocked / inactive |
| last_seen | 每次注册/心跳刷新 |

> 注册表里**不存 URL、不存端口、不存 IP**。调用方自己配目标地址。

---

## 异常处理

### 密钥丢了

`~/.aegis/keys/<name>.key` 被删除 → 下次启动会生成新密钥 → 公钥变了 → 旧 ticket 全部失效。

**最多 30s**（sync 间隔）后其他服务拿到新公钥，恢复。

### 如何避免

- 密钥文件是自动持久化的，正常重启不会丢
- 备份 `.aegis/keys/` 目录
- 多实例用同一把密钥（复制 .key 文件），身份一致

### 灰度更新

多版本部署时，新旧版本的公钥不同，但服务名相同。

```
服务端 ListPublicKeys 返回 name→pubkey 映射。
当前实现只保留一个公钥（map 覆盖）。
```

如果需要灰度支持，Guard 验签应遍历所有同名的公钥：

```go
// 当前（一个 key）：
pubKey := c.publicKeys[callerName]

// 需要时改成（多个 key，任一通过即可）：
for _, key := range c.publicKeysForName(callerName) {
    if _, err := VerifyTicket(ticket, key); err == nil { goto OK }
}
return 403
```

### 接收方宕机

- 调用方 `client.Post()` 会在 HTTP 层面超时
- 建议在 PostOpts 里配置重试 + 降级（当前 SDK 还不支持，后续加）
- 服务恢复后重新 Register → 其他服务 sync 拉到最新状态

---

## API Key 对比

```
                API Key                          ServiceAuth
认证方式         查数据库（sha256）               本地 Ed25519 验签
身份包含         key_id, role, scopes, owner_id   service_name（只有名字）
归属             创建时指定 owner                  注册时自动推导
调用方式         curl -H "X-API-Key: xxx"         client.Post(url)
接收方式         中间件取 key → 查 db → identity  Guard → CallerFromContext
```

**核心区别：** API Key 是人工创建、指定归属的。ServiceAuth 是服务自己注册、自动推导归属的。其他流程一致。

---

## 术语

| 术语 | 含义 |
|------|------|
| ServiceAuth | 服务间认证系统 |
| 服务注册表 | 所有注册服务的名单 |
| ticket | Ed25519 签名的身份凭证 |
| Guard | 验证 ticket 的 HTTP 中间件 |
| Post/Get/Put/Delete | 调用其他服务的 HTTP 方法，自动签 ticket |
| 调用方 | 发起请求的服务 |
| 接收方 | 处理请求的服务 |
| 封禁 | 将某个服务从注册表中禁用 |
| sync | 客户端每 30s 拉取注册表变更 |
