# Aegis 代码分叉 & 历史遗留 & 冗余记录

> 审计日期：2026-07-08
> 范围：`internal/`、`pkg/`、`cmd/`（不含 UI）

---

## 一、严重 — 节点 API 认证断裂

**5 个 `/api/node/v1/*` 端点无法被正常访问**，因为 `token.AuthMiddleware` 不识别节点凭据 Bearer token。

| 端点 | 文件 |
|------|------|
| `POST /api/node/v1/heartbeat` | `internal/httpapi/routes.go:237` |
| `GET /api/node/v1/binary` | `internal/httpapi/routes.go:238` |
| `GET /api/node/v1/gateway-link-token/{id}` | `internal/httpapi/routes.go:239` |
| `GET /api/node/v1/desired-state` | `internal/httpapi/routes.go:258` |
| `POST /api/node/v1/actual-state` | `internal/httpapi/routes.go:259` |

**根因**：`isPublicPath()`（`internal/token/middleware.go:56`）不包含 `/api/node/v1/` 前缀。中间件在检查 Bearer token 时不匹配节点 token → 返回 401。处理程序级的 `authenticateNodeRequest()` 永远不会到达。

**为什么测试能过**：测试直接注册路由到 mux，不走中间件链。

---

## 二、高优先级 — 代码分叉

### 2.1 `isPrivateIP` / `IsPrivateAddress` — 5 个分叉

| 位置 | 签名 | 类型 |
|------|--------|------|
| `internal/safety/ipclass.go:58` | `IsPrivateIP(host string) bool` | ✅ 事实来源（已导出） |
| `internal/tcp/manager.go:180` | `IsPrivateAddress(host string) bool` | ❌ 分叉 |
| `internal/udp/manager.go:166` | `IsPrivateAddress(host string) bool` | ❌ 分叉（注释说"与 tcp 相同逻辑"） |
| `internal/serviceauth/admin.go:147` | `isPrivateIP(ip net.IP) bool` | ❌ 分叉（未导出版） |
| `pkg/serviceauth/ipcheck.go:98` | `isPrivateIP(ip net.IP) bool` | ❌ 分叉（SDK 自己维护一份） |

**标准做法**：统一调 `internal/safety.IsPrivateIP()`。

### 2.2 硬编码 Provider 名

| 文件:行 | 代码 | 违反规则 |
|----------|------|-------------|
| `internal/apply/workflow.go:102` | `caddy := w.registry.Get("caddy")` | provider-architecture.md 红线规则 #1 |
| `internal/apply/workflow.go:287` | `caddy := w.registry.Get("caddy")` | 同上 |

**标准做法**：用 `p.HasCapability(provider.CapRouteHost)` 按能力找，不按名字找。

### 2.3 `fmt.Sprintf("%s:%d")` 代替 `net.JoinHostPort` — ~15 处

IPv6 地址包含 `:`，`%s:%d` 会拼出错误格式。应使用 `net.JoinHostPort(host, port)`。

关键位置：
- `internal/action/bind_http_domain.go:92`
- `internal/action/update_target.go:60`
- `internal/addr/addr.go:93`
- `internal/gateway/diagnostics.go:204`
- `internal/gateway/local_dispatch.go:31`
- `internal/gateway/relay_resolver.go:239`

### 2.4 `contains` 分叉

| 位置 | 函数 | 问题 |
|------|----------|----------|
| `internal/gateway/localgw_handler.go:295` | `containsSubstring(s, substr)` | 手写，等同于 `strings.Contains` |
| `internal/gateway/localgw_handler.go:289` | `stringsContains(s, substr)` | 包装上面那个 |

---

## 三、中优先级 — 历史遗留

### 3.1 Slog 迁移未完成

CLAUDE.md 标注 3 个包未迁移，**实际共 21 个文件，~80 处 `log.Printf` 调用**：

| 包 | 文件 | `log.Printf` 数 |
|-------|------|----------|
| `internal/dns/` | `manager.go`, `server.go`, `reachability.go`, `resolver.go` | 22 |
| `internal/nodeagent/` | `agent.go` | 7 |
| `internal/transparent/` | `manager.go`, `proxy.go`, `redirect_linux.go` | 17 |
| `internal/httpapi/handlers/` | `admin.go`, `system.go`, `dns.go`, `egress.go` | 15 |
| `internal/provider/` | `caddy.go`, `haproxy.go` | 6 |
| `internal/store/` | `backup.go` | 3 |
| `internal/infra/` | `acme_obtain.go` | 2 |
| `internal/core/` | `recovery.go` | 2 |
| `internal/nodestate/` | `datasource_db.go`, `generator.go` | 2 |
| `internal/distnode/` | `distnode.go` | 1 |

### 3.2 `ServiceRecord` 废弃字段

`internal/serviceauth/model.go:19-23`：
```go
Host     string `json:"-"`    // 废弃
Port     int    `json:"-"`    // 废弃
NodeHost string `json:"-"`    // 废弃
APIsJSON string `json:"-"`    // 废弃
```

保留以兼容老数据扫描，但新注册不再填充。

### 3.3 `TrustedGateway.AuthValue` HMAC 回退

`internal/gateway/gwlink_model.go:40` — 旧 HMAC 哈希密钥，与新的加密存储并存作为回退。

---

## 四、低优先级 — 冗余

### 4.1 ServiceAuth Groups + Policies：存了但从不评估

| 证据 | 位置 |
|------|------|
| Policy 结构体定义 | `internal/serviceauth/model.go:84-92` |
| CRUD 仓库方法（增删改查） | `internal/serviceauth/repository.go:392-423` |
| 加载到 Register/Sync 响应 | `internal/serviceauth/service.go:101,142` |
| 管理 API 路由 | `internal/httpapi/routes.go:365-370` |
| **但 Never Evaluated（无评估引擎）** | — |

EvaluatePolicy 函数已删除（上一轮清理），但 Groups + Policies 的 API、Repository、Sync 通路还在。SDK 同步后拿到数据但没用。此前的"策略引擎"设计已废弃。

### 4.2 `Repository.JoinStrings()` — 死导出

`internal/serviceauth/repository.go:335`：
```go
func (r *Repository) JoinStrings(ss []string, sep string) string {
    return strings.Join(ss, sep)
}
```

字面量等于 `strings.Join`。零调用者。

### 4.3 `routingpolicy.ListServicePolicies()` / `ListRoutePolicies()` — 死导出

`internal/routingpolicy/service.go:164-176` — 两个方法已定义且导出，包外无调用者，无 HTTP handler。

### 4.4 CLI 死代码存根

`internal/cli/node_run.go:81-105`：
- 空 `init()` 函数（仅注释）
- 丢弃结果的 IIFE：`var _ = func() bool { return true }()`
- 导入静音器：`var _ = os.Stderr`

---

## 五、UI 冗余（仅记录，本次不改）

### 5.1 未路由页面（8 个文件）

`ui/src/pages/fabric/` 下仍有 8 个文件没有被 `App.tsx` 路由：
`Gateways.tsx`, `GatewayDetail.tsx`, `GatewayLinks.tsx`, `Listeners.tsx`, `RoutingTable.tsx`, `Topology.tsx`, `ProvidersDetail.tsx`, `ImportConfig.tsx`

### 5.2 工作区导航与实际页面不匹配

`constants.ts` 中 fabric 导航项只列了 4 项，但 pages/fabric/ 下有 12 个文件。AuthServices/AuthCallGraph 在 fabric 目录但路由在 `/auth`。

### 5.3 Groups 标签在 UI 中

`AuthServices.tsx` 有 "服务组" 标签，对应的是已废弃的 Groups 概念（原计划做防火墙，实际只是标签）。

---

## 优先级建议

| 优先级 | 项目 | 估计 |
|----------|------|--------|
| **P0** | 修 Node API 认证断裂 | 小（中间件加几行路由白名单） |
| **P1** | 统一 `isPrivateIP` → `safety.IsPrivateIP` | 中（5 处替换） |
| **P1** | 修 `fmt.Sprintf("%s:%d")` → `net.JoinHostPort` | 中（~15 处） |
| **P2** | 删死代码（JoinStrings, ListServicePolicies, CLI 存根） | 小 |
| **P2** | 清理 Groups/Policies 通路（或确认废弃） | 中 |
| **P3** | Slog 迁移 | 大（80 处） |
| **P3** | Provider 名硬编码 | 小（2 处） |
| **P3** | UI 冗余 | 中（8 文件） |
