# DistNode 规范 — 分布式节点运行时复用守则

> **自动加载范围：** `internal/distnode/**`
>
> **目的：** 确保 distnode 保持可复用、零外部依赖、无 Aegis 业务泄露。

---

## 红线规则（永远不能做的事）

### ❌ 引入 Aegis 内部包的依赖

```go
// ❌ 永远不要
import "aegis/internal/route"
import "aegis/internal/node"

// ✅ distnode 只能导入标准库
import "net/http"
import "crypto/hmac"
```

distnode 是通用底座。**任何 Aegis 业务代码都不能出现在 `distnode/` 里。** 业务逻辑通过 `Transport.Register("Aegis.XXX", handler)` 注册，写在 `handlers/distnode_admin.go` 里。

### ❌ 在 distnode 中加存储实现

```go
// ❌ 永远不要
type SQLiteStorage struct { db *sql.DB }  // SQLite 不是跨项目通用的

// ✅ StorageDriver 接口已经定义，实现放在使用方项目里
// Aegis 用 SQLite，其他项目可能用 etcd 或 PostgreSQL
```

### ❌ 硬编码端口

```go
// ❌ 永远不要
addr := "127.0.0.1:7380"
port := 443

// ✅ 从 Config 读
cfg.Addr  // 调用方决定监听什么地址
```

### ❌ 在 distnode 中放业务 handler

```go
// ❌ 永远不要 — 把业务逻辑写进 distnode 包
dn.Transport.Register("ListRoutes", ...)

// ✅ 在 handlers/distnode_admin.go 里注册
handlers.RegisterAegisTransportHandlers(dn, h)
```

---

## 复用规则

### 1. 引用现有 distnode 的唯一方式

```go
// 新项目复制整个 internal/distnode/ 目录
// 然后：
type MyNode struct {
    *distnode.DistNode          // 嵌入
    // 我的业务字段
}

// 注册业务 handler
dn.Transport.Register("MyApp.DoSomething", myHandler)

// 跨节点调用
dn.Transport.Call(ctx, "other_node", "MyApp.DoSomething", args, &reply)
```

### 2. 保持零外部依赖

```
distnode 只能使用:
  - 标准库 (net/http, crypto/hmac, encoding/json, sync, time, context, io, log, errors, fmt, bytes)
  - 不能有 go.mod replace 或 vendor 依赖
```

验证方法：

```bash
go list -m all  # distnode 不应该引入任何第三方包
```

### 3. 存储由调用方决定

```go
// distnode 定义 StorageDriver 接口（可选使用）
// 但 distnode 本身不实例化任何存储

// 调用方实现：
type SQLiteStorage struct { *sql.DB }
func (s *SQLiteStorage) Get(key string) ([]byte, error) { ... }
func (s *SQLiteStorage) Set(key string, val []byte) error { ... }
```

---

## 注释规范

所有跨包可访问的标识符必须写 `// 注释`：

| 元素 | 必须注释 |
|------|---------|
| `type Config` | 用法示例 + 字段说明 |
| `type Membership` | 线程安全说明 + 当前实现方式 |
| `Transport.Call` | 参数说明 + 错误返回类型 |
| `StorageDriver` | 可用实现列表 |
| `Role.Declare` | 自声明 vs 外部赋值的区别 |

---

## 防分叉检查清单（改 distnode 前必查）

- [ ] 新逻辑是否已经在 `distnode` 包里有类似实现？
- [ ] 是否引入了 `aegis/internal/*` 的依赖？
- [ ] 是否硬编码了端口/IP/地址？
- [ ] 是否在 distnode 里加了业务逻辑 (route/service/endpoint)？
- [ ] `go list -m all` 是否有新依赖？
