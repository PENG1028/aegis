# DistNode Architecture — 分布式节点唯一抽象层

> **自动加载范围:** `internal/distnode/**`, `cmd/aegis/main.go`
>
> **目的:** 防止再次出现多套分布式系统并存。distnode 是唯一的跨节点通信层。

---

## ⚠️ 历史遗留（待删除）

以下三个包是 distnode 之前的旧分布式实现，**已废弃，不要使用、不要修改、不要新增引用**：

| 包 | 状态 | 原因 |
|----|------|------|
| `internal/nodeagent/` | ❌ 废弃 | distnode.Membership 已替代心跳/发现 |
| `internal/noderuntime/` | ❌ 废弃 | distnode.Transport 已替代同步 |
| `internal/nodestate/` | ❌ 废弃 | distnode.Transport.Call 可直接读远程状态 |
| `internal/cli/node_run.go` | ❌ 废弃 | `aegis serve` + distnode goroutine 替代 |
| `POST /api/node/v1/*` | ❌ 废弃 | distnode 用静态 peer + Transport.Call |

**等待合适的重构窗口后删除。在此之前，这 5 个组件是冻结代码。**

---

## 一、唯一定理

**Aegis 的跨节点通信只有一层：`internal/distnode/`**

```
distnode.DistNode
  ├─ Identity      — HMAC token 认证
  ├─ Membership    — 静态 peer 列表 + 健康检查循环
  ├─ Transport     — Register/Call 跨节点 RPC
  ├─ Role          — 节点自声明角色 (panel/agent/gateway/leader)
  └─ StorageDriver — 可选持久化接口
```

## 二、如何接入

### aegis serve 启动时初始化 distnode

```go
// cmd/aegis/main.go (已实现)
if cfg.DistNode.Enabled {
    dn = distnode.New(distnode.Config{
        ID:     cfg.DistNode.ID,
        Name:   cfg.DistNode.Name,
        Addr:   cfg.DistNode.Addr,
        Secret: cfg.DistNode.Secret,
        Peers:  toPeerConfigs(cfg.DistNode.Peers),
    })
    RegisterAegisTransportHandlers(dn, h, mux)
    go dn.Start(ctx)
}
```

### 加入新节点

**不需要 SSH 部署、不需要 join token。** 只需要：

1. 在控制平面配置里加一条 peer 记录
2. 新节点上 enable distnode + 配置 peer 指向控制平面
3. 重启 `aegis serve`
4. Membership 自动发现（HTTP health check 每 15 秒）

### 跨节点调用

```go
// A 面板查 B 面板的路由列表
var routes []Route
dn.Transport.Call(ctx, "node_b", "Aegis.ListRoutes", nil, &routes)

// 通用代理：调用 B 上任意 API
dn.Transport.Call(ctx, "node_b", "Aegis.ProxyRequest",
    ProxyRequest{Method: "GET", Path: "/api/admin/v1/routes"}, &resp)
```

### 前端多节点聚合

```ts
// 一条 API 调用，聚合所有节点的数据
GET /api/admin/v1/distnode/aggregate?path=/api/admin/v1/routes

// 返回
[
  {node_id: "node_a", status: 200, body: [...]},
  {node_id: "node_b", status: 200, body: [...]},
]
```

## 三、红线规则

### ❌ 永远不要

```
- 新增 POST /api/node/v1/* 端点        → 用 distnode.Transport.Register
- 使用 nodeagent/noderuntime/nodestate  → 已废弃
- 在 serve.go 外创建独立的 agent 进程   → distnode 在 serve 内作为 goroutine
- 通过 HTTP API 实现心跳/发现           → distnode.Membership 已实现
- 硬编码节点间通信格式                  → distnode.Transport 统一 JSON-RPC
```

### ✅ 应该做的

```
- 新增跨节点功能 → dn.Transport.Register("Aegis.XXX", handler)
- 查询远程节点   → dn.Transport.Call(ctx, nodeID, method, args, &reply)
- 前端聚合数据   → GET /api/admin/v1/distnode/aggregate?path=...
- 节点发现       → 配置 peer list，Membership 自动管理
```

## 四、与 cluster/ 的关系

| distnode | cluster |
|----------|---------|
| 成员发现 + 通信 | Leader 选举 |
| Transport.Call | 集群健康聚合 |
| Membership.AlivePeers | 冲突/脑裂检测 |

**两者互补，不冲突。** cluster 读 distnode 的 Membership 来做健康聚合。
