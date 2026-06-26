# v1.8B-0 — Managed Egress Relay Design

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Sub-phase:** v1.8B-0 — Design only, no code
> **Status:** DESIGN COMPLETE
> **Theme:** Force managed-domain egress through gateway listeners, never direct to remote target port

---

## Table of Contents

1. [定义 Managed Egress Relay](#一定义-managed-egress-relay)
2. [路径规则](#二路径规则)
3. [HTTP Relay 优先](#三http-relay-优先)
4. [Relay 请求模型](#四relay-请求模型)
5. [Target Gateway Dispatch](#五target-gateway-dispatch)
6. [和现有 GatewayLink 的关系](#六和现有-gatewaylink-的关系)
7. [和 v1.8A Safety 的关系](#七和-v18a-safety-的关系)
8. [API / CLI 设计](#八api--cli-设计)
9. [强制语义怎么落地](#九强制语义怎么落地)
10. [安全边界](#十安全边界)
11. [数据缺口](#十一数据缺口)
12. [验收标准](#十二验收标准)

---

## 一、定义 Managed Egress Relay

### 是什么

Managed Egress Relay 是 Aegis 控制面强制路径归一化机制：

> 如果某个 domain / service 已经注册在 Aegis 管理矩阵中，Aegis 知道它运行在哪个 node、IP、port，则请求**必须通过对应节点的 gateway listener**，而非直接访问 `target_host:target_port`。

**核心规则：**

```
调用方 → 源 Gateway (80/443)
       → 目标 Gateway (80/443)
       → 127.0.0.1:<target_port>  ← 只有目标节点本机 gateway 能访问真实端口
```

远端真实端口只在目标节点上由 gateway 本机访问。外部调用方从不直接连接 `remote_ip:target_port`。

### 不是什么

| 不是 | 原因 |
|------|------|
| ❌ 全系统透明代理 | 不劫持所有流量，只接管 Aegis managed domain |
| ❌ iptables 拦截 | 不做 OS 层包过滤 |
| ❌ DNS 劫持 | 不修改系统 DNS 解析 |
| ❌ Service Mesh | 没有 sidecar，没有注入代理 |
| ❌ 任意开放代理 | 只 relay Aegis 管理的路由，不代理任意目标 |
| ❌ 客户端指定 target | 客户端不能传入 target_host:target_port，目标必须从 Aegis DB 获取 |

### 设计原则

1. **HTTP only for v1.8B** — raw TCP / WebSocket / SSH deferred
2. **DB is source of truth** — target 来自 Aegis route/endpoint/node，不来自 header
3. **GatewayLink is authorization** — 跨节点 relay 必须有 GatewayLink
4. **No fallback to direct** — gateway 不可达时不回退到 direct `target_host:target_port`
5. **Layer 1 + 2 first** — Aegis-managed clients + local gateway proxy, 不做透明拦截

---

## 二、路径规则

```
请求 domain
     │
     ▼
┌─────────────────────────────────────────────────────┐
│ Domain 在 Aegis 管理矩阵内？                          │
│ (有 route / managed domain / service)                │
└─────────────────────────────────────────────────────┘
         │                        │
        YES                       NO
         │                        │
         ▼                        ▼
    ┌──────────────┐    ┌──────────────────────────┐
    │ 路径选择      │    │ mode = external_passthrough │
    │ (按拓扑)      │    │ 不接管，客户端自行决定       │
    └──────────────┘    └──────────────────────────┘
         │
    ┌────┴────┬────────┬─────────┐
    │         │        │         │
    ▼         ▼        ▼         ▼
 local    private   public   unreachable
gateway   gateway   gateway
```

### 规则 1: Domain 不在 Aegis 管理矩阵

```
mode = external_passthrough
```

| 条件 | Domain 无 route, 无 managed domain, 无匹配 service |
|------|----------------------------------------------------|
| 行为 | 不接管。调用方可以自由访问 domain（可以是公网 DNS 解析到原始服务器） |
| Safety | 复用 v1.8A `UNKNOWN_DOMAIN` / `PUBLIC_DOMAIN_BOUNCE` 检测，但不强制 |
| 备注 | Aegis 只做 safety 检测，不做路径接管 |

### 规则 2: target_node == from_node

```
mode = local_gateway
```

| 条件 | Route 指向本机节点的 service/endpoint |
|------|---------------------------------------|
| 路径 | `client → 本机 gateway:80/443 → 127.0.0.1:<target_port>` |
| 地址 | 客户端通过本机 gateway listener 访问 domain。Gateway 根据 route → service → endpoint 查本地 DB，转发到 `127.0.0.1` |
| 网络 | 不访问公网。不访问 private/public IP。全程在本机完成 |
| GatewayLink | 不需要（同节点不需要跨节点认证） |
| Safety | `SELF_LOOP` 检测仍然生效：如果 target 端口是 gateway listener 端口 → 报错。如果不是 listener 端口 → 正常 relay |

**关键行为差异（v1.8A vs v1.8B）：**

| | v1.8A | v1.8B |
|--|-------|-------|
| 127.0.0.1:3001 | 安全（non-listener） | 通过 local gateway relay 访问 |
| 127.0.0.1:80 | SELF_LOOP 错误 | SELF_LOOP 错误（仍不允许） |

### 规则 3: target_node != from_node 且 private gateway reachable

```
mode = private_gateway
```

| 条件 | 目标节点有 private IP，且 from_node 可以访问 target_node 的 private IP:80/443 |
|------|-----------------------------------------------------------------------------|
| 路径 | `from_node gateway → target_node_private_ip:80/443 → 127.0.0.1:<target_port>` |
| 地址 | `target_node.private_ip` + 目标节点 gateway listener port（通常 80） |
| 路由 | 客户端把自己的请求发给本机 gateway（或直接用 relay resolver 获得目标 gateway URL） |
| 跨节点 | 源节点 gateway 把请求 forward 到目标节点 private gateway listener |
| 目标 | 目标节点 gateway 查本地 DB → endpoint → `127.0.0.1:<target_port>` |
| GatewayLink | **强制要求** — 所有跨节点 relay 都需要 GatewayLink 认证。private gateway 不是免认证通道。 |
| 安全 | 流量在私有网络内传输，但 GatewayLink 仍然必须，因为 private network 不等于信任边界 |

### 规则 4: private gateway 不可达，但 public gateway reachable

```
mode = public_gateway
```

| 条件 | 目标节点无 private IP，或 private 网络不可达，但 target_node 的 public IP 可达 |
|------|--------------------------------------------------------------------------------|
| 路径 | `from_node gateway → target_node_public_ip:80/443 → 127.0.0.1:<target_port>` |
| 地址 | `target_node.public_ip` + 目标节点 gateway listener port（通常 443） |
| GatewayLink | **强制要求** — 公网 relay 必须有 GatewayLink auth token |
| 安全 | 流量经过公网，GatewayLink 提供身份验证，HMAC signing deferred |
| 备注 | v1.8A public target + GatewayLink 是"安全"状态，v1.8B 将它从"允许直接访问"改为"必须通过 gateway relay" |

### 规则 5: gateway 不可达

```
mode = unavailable
```

| 条件 | `from_node` 无法到达 `target_node` 的任何 IP 上的 gateway listener |
|------|--------------------------------------------------------------------|
| 行为 | **不 fallback 到 direct target_host:target_port**。返回 TARGET_GATEWAY_UNREACHABLE |
| 后果 | 该域名的 relay 不可用。调用方不能通过 Aegis 访问该服务 |
| 理由 | 如果允许 fallback，就绕过了 relay 强制语义。要么走 gateway，要么不走 |
| 恢复 | 修复网络连通性后重试 |

### 规则选择优先级

```
1. external_passthrough   (不在管理矩阵)
2. local_gateway          (同节点)
3. private_gateway        (跨节点, private 可达)
4. public_gateway         (跨节点, 仅 public 可达)
5. unavailable            (不可达)
```

**禁止：** 直接访问 remote target_host:target_port。target_host:target_port 只在 target_node 本机上被 gateway 使用。

---

## 三、HTTP Relay 优先

### v1.8B 范围

| 支持 | 理由 |
|------|------|
| ✅ HTTP service | Caddy reverse_proxy 原生支持 |
| ✅ REST API | 标准 HTTP 请求 |
| ✅ Web app | 同 HTTP |
| ✅ Internal agent HTTP endpoint | HTTP 协议，无需特殊处理 |

### v1.8B 延期

| 延期 | 原因 |
|------|------|
| ❌ raw TCP | 需要 TCP tunnel，复杂度高 |
| ❌ WebSocket tunnel | 需要 WS 升级 + 持久连接处理 |
| ❌ HTTP CONNECT | 需要代理协议支持 |
| ❌ UDP | 无连接协议，无法用 Caddy reverse_proxy 处理 |
| ❌ Database protocol (MySQL, PostgreSQL) | 通常需要 raw TCP，延期 |
| ❌ SSH | raw TCP tunnel，延期 |

**原因：**
HTTP 可以通过 Caddy `reverse_proxy` + headers 实现 relay。
任意 TCP 需要 tunnel（类似 `haproxy_tcp` 的模式），实现和维护复杂度远高于 HTTP。

### 判定条件

Service 的 `kind` 字段决定是否支持：

| Service Kind | v1.8B Relay |
|-------------|-------------|
| `http` | ✅ 支持 |
| `tcp` | ❌ 延期 |
| `file` | N/A（文件服务） |

---

## 四、Relay 请求模型

### 核心约束

请求不能信任客户端传入的 `target_host` / `target_port`。所有目标信息必须从 Aegis DB 获取。

### 允许携带的 Header

当源节点 gateway 转发请求到目标节点 gateway 时，携带以下 header：

| Header | 来源 | 用途 |
|--------|------|------|
| `Host` | 客户端原始请求 | 目标节点识别 domain/route |
| `X-Aegis-Route-ID` | 源节点 DB 查询 | 目标节点直接查找路由 |
| `X-Aegis-Gateway-ID` | 源节点 DB 查询 | 目标节点识别 GatewayLink |
| `X-Aegis-Gateway-Token` | 源节点解密 GatewayLink secret | 目标节点验证 auth |
| `X-Aegis-Source-Node` | 源节点身份 | 目标节点验证 source |
| `X-Aegis-Hop` | 源节点设置 | 防止循环 relay（当前 relay 算 1 hop） |
| `X-Aegis-Request-ID` | 源节点生成 | 全链路追踪 |

### 请求流程

```
Client (Server A 上的应用)
  │
  ├─(A) 通过本机 gateway listener 访问 domain
  │     └─ 或使用 aegis relay resolve 获得目标 gateway URL
  │
  ▼
源节点 Gateway (Server A, 本机 80/443)
  ├─ 解析 Host → 查本地 route
  ├─ 查 route → service → endpoint → target_node
  ├─ 查 target_node 的 gateway URL
  ├─ 解密关联的 GatewayLink token
  ├─ 注入 header (Route-ID, Gateway-ID, Token, Source-Node, Hop=1, Request-ID)
  ├─ 转发请求到 target_node gateway URL
  │
  ▼
目标节点 Gateway (Server B, 80/443)
  ├─ 收到请求
  ├─ 验证 GatewayLink token
  ├─ 验证 route_id 存在
  ├─ 验证 route 指向本机 node
  ├─ 验证 source node 允许
  ├─ 检查 hop ≤ 1
  ├─ 查询 endpoint → local target
  ├─ 转发到 127.0.0.1:<target_port>
  │
  ▼
目标服务 (Server B, 127.0.0.1:2724)
  └─ 响应原路返回
```

### 目标节点不信任的内容

| 内容 | 信任？ | 原因 |
|------|--------|------|
| `X-Aegis-Route-ID` | ⚠️ 验证 | 只用来查 DB，不直接用作转发目标 |
| `Host` | ⚠️ 验证 | 用于 route 匹配，但需要二次确认 |
| `X-Aegis-Target-Port` | ❌ **禁止** | 客户端可能伪造，必须从 DB endpoint 获取 |
| `X-Aegis-Target-Host` | ❌ **禁止** | 同上，必须从 DB 获取 |
| 任意非标准 header | ❌ 忽略 | 不能影响转发逻辑 |

---

## 五、Target Gateway Dispatch

### 识别 Managed Relay 请求

目标节点 Gateway 收到请求后，按以下顺序判断：

```
1. 是否有 X-Aegis-Route-ID header?
   ├─ YES → 进入 Managed Relay Dispatch
   └─ NO  → 按正常 Caddy 路由处理（普通 inbound 请求）
```

### Dispatch 步骤

```
Step 1: 提取 X-Aegis-Route-ID
        └─ 查本地 routes 表是否存在
            ├─ 不存在 → 返回 404 "route not found"
            └─ 存在 → 继续

Step 2: 验证 GatewayLink
        ├─ 取 X-Aegis-Gateway-ID
        ├─ 取 X-Aegis-Gateway-Token
        ├─ 查 trusted_gateways 表
        │   ├─ gateway_id 不存在 → 401
        │   └─ 存在 → 验证 token (CheckAuth)
        │       ├─ 失败 → 401 "invalid gateway token"
        │       └─ 成功 → 继续
        └─ public_gateway 模式且 GatewayLink 无效 → 403

Step 3: 验证 route 归属
        ├─ 查 route → service → endpoints
        ├─ endpoints 是否指向本机 node?
        │   ├─ 否 → 403 "route not local"（防止跨节点劫持）
        │   └─ 是 → 继续
        └─ 注意：endpoint.type 应该是 local 才能确认是本机

Step 4: 验证 source node
        ├─ 取 X-Aegis-Source-Node
        ├─ 查 nodes 表
        │   ├─ source node 不存在 → 403 "unknown source node"
        │   └─ 存在 → 继续
        └─ (未来: source node allowlist)

Step 5: 验证 hop count
        ├─ 取 X-Aegis-Hop
        ├─ hop ≥ 1 → 403 "max hops exceeded"
        │   (防止 A → B → A → B 循环)
        └─ hop == 0 或 1 → 继续
            (源节点设 1, 目标节点验证 ≤ 1)

Step 6: 查询 final local target
        ├─ 从 endpoint 解析 127.0.0.1:<target_port>
        │   ├─ endpoint.type == "local" → 使用 endpoint.address
        │   └─ 否则 → 403 "endpoint not local"
        ├─ 禁止转发到非 127.0.0.1 的目标
        └─ 最终 target 只来自 DB，不来自 header

Step 7: 转发
        ├─ Caddy reverse_proxy 127.0.0.1:<port>
        ├─ 写 relay event log
        └─ 返回响应
```

### 禁止行为

| 禁止 | 后果 |
|------|------|
| 根据 header 中的 `target_port` 直接转发 | 开放代理风险 |
| 转发到非本机 IP (`10.x.x.x`, `192.168.x.x`, 公网 IP) | 被用作跳板 |
| 无 GatewayLink 的公网 relay | 无认证转发 |
| hop > 1 的请求 | 循环 relay |
| 回落 fallback 到 `remote_host:remote_port` | 绕过强制语义 |

### 返回码

| 场景 | Status | Body |
|------|--------|------|
| 成功 | 200 | 原始响应 |
| Route not found | 404 | `{"error":"route_not_found"}` |
| Invalid token | 401 | `{"error":"invalid_gateway_token"}` |
| Route not local | 403 | `{"error":"route_not_local"}` |
| Unknown source node | 403 | `{"error":"unknown_source_node"}` |
| Max hops | 403 | `{"error":"max_hops_exceeded"}` |
| Endpoint not local | 403 | `{"error":"endpoint_not_local"}` |
| Gateway unreachable | 503 | `{"error":"target_gateway_unreachable"}` |

---

## 六、和现有 GatewayLink 的关系

### 现状

GatewayLink（`trusted_gateways` 表）当前用于：

```
Route A → Server B
  ├── route.gateway_link_id → trusted_gateway.id
  └── Apply 时:
        ├── 匹配 gateway type = "upstream"
        ├── 注入 header_up X-Aegis-Gateway-Token
        └── Server B verifier 验证 token
```

### v1.8B 扩展

GatewayLink 在 v1.8B 中作用扩大：

| 维度 | v1.7/v1.8A | v1.8B |
|------|-------------|-------|
| 用途 | Verifier 示例 / 可选认证 | **Relay authorization metadata** |
| 是否必需 | 可选（public target 建议使用） | public_gateway 模式**强制** |
| 关联数据 | `host`, `private_ip`, `port`, `auto_route` | 同上 + relay 路由决策 |
| 路由决策 | 不参与 | 决定 private_gateway vs public_gateway |
| 生命周期 | 创建 → 静态使用 | 创建 → 路由 → relay → 验证 → rotate |

### 没有变化的

- GatewayLink 仍然**不是** route source of truth
- Route + Endpoint 仍然决定最终 target
- GatewayLink 不存储 target info，只存储网关节点连接信息
- `auth_value` 仍然 HMAC hashed（v1.7 就是 hash，不是 plaintext）

### GatewayLink 在 Relay 中的角色

```
跨节点 relay 流程:
  1. 源节点查 route → 取 gateway_link_id
  2. 查 trusted_gateways → 取 host/private_ip/port + auth_value
  3. 用 auth_value 生成 X-Aegis-Gateway-Token
  4. 路由请求到 gateway URL
  5. 目标节点验证 token (CheckAuth)
```

`trusted_gateway` 的 `auto_route` 字段决定 IP 选择：

| auto_route | 行为 |
|-----------|------|
| `true` | 优先 `private_ip`，回退 `host` (public) |
| `false` | 只使用 `host`（不自动切换） |

### `gateway_type` 在 Relay 中的意义

| Gateway Type | Relay 行为 |
|-------------|-----------|
| `upstream` | 本节点 forward 到它 |
| `downstream` | 它 forward 到本节点 |
| `peer` | 双向 relay |

*目前仅有 `upstream` 类型用于 relay。*

---

## 七、和 v1.8A Safety 的关系

### 复用

| v1.8A 组件 | v1.8B 复用方式 |
|-----------|----------------|
| IP Classification (ClassifyIP) | 判断 target_ip 类型：loopback/private/public |
| Current Node Address (IsCurrentNodeAddress) | 判断 target_node == from_node |
| Self-Loop Detection (isGatewayListenerTarget) | local_gateway 模式下防止转发到 listener 端口 |
| GatewayLink Bypass Detection | public_gateway 模式强制要求 GatewayLink |
| Trace Egress Model | 扩展用于 relay route resolution |
| Risk Codes | 用于 relay decision 的辅助判断 |

### 变化

| 场景 | v1.8A 检测 | v1.8B 行动 |
|------|-----------|-----------|
| Public target, no GatewayLink | `GATEWAY_LINK_BYPASS_RISK` warning | → `unavailable` 或 `public_gateway` (如果 GatewayLink 存在) |
| Public target, has GatewayLink | ✅ Safe (允许直接访问) | → `public_gateway` (强制走 gateway) |
| Private target, cross-node | 不检测 | → `private_gateway` (需要 GatewayLink) |
| Loopback, non-listener | ✅ Safe | → `local_gateway` (仍然安全) |

### Safety → Relay 映射

```
CheckRouteSafety() / GetPlannerWarnings() 的结果
  ↓
RelayResolver.Resolve(domain, from_node)
  ├── 如果风险包括 SELF_LOOP → 拒绝 relay
  ├── 如果风险包括 GATEWAY_LINK_BYPASS_RISK → 要求创建 GatewayLink
  └── 如果无风险 → 正常路径选择 (local/private/public_gateway)
```

**v1.8A safety 检测不阻止 relay。** v1.8B 只在 relay 决策中使用 safety 信息做提示，不强制执行。强制执行通过 relay resolver 的路径规则实现。

---

## 八、API / CLI 设计

### API: Egress Relay 查询

```
GET /api/admin/v1/resolve/egress-relay?domain=<domain>&from_node=<node_id>
```

**Response (managed domain, private_gateway 模式):**

```json
{
  "domain": "app.example.com",
  "managed": true,
  "mode": "private_gateway",
  "from_node": {
    "id": "nd_a",
    "hostname": "server-a"
  },
  "target_node": {
    "id": "nd_b",
    "hostname": "server-b",
    "private_ip": "10.0.0.8",
    "public_ip": "43.159.34.11"
  },
  "gateway_url": "http://10.0.0.8:80",
  "gateway_port": 80,
  "protocol": "http",
  "route_id": "rt_abc123",
  "gateway_link_id": "gwlink_xyz456",
  "direct_target_suppressed": true,
  "final_local_target": "127.0.0.1:2724",
  "hop_limit": 1,
  "risks": [],
  "recommendation": "send request to target gateway, not remote target port"
}
```

**Response (local_gateway 模式):**

```json
{
  "domain": "api.internal",
  "managed": true,
  "mode": "local_gateway",
  "from_node": {
    "id": "nd_a",
    "hostname": "server-a"
  },
  "target_node": {
    "id": "nd_a",
    "hostname": "server-a"
  },
  "gateway_url": "http://127.0.0.1:80",
  "gateway_port": 80,
  "protocol": "http",
  "route_id": "rt_def456",
  "di
rect_target_suppressed": true,
  "final_local_target": "127.0.0.1:3001",
  "hop_limit": 0,
  "risks": [],
  "recommendation": "request routed through local gateway"
}
```

**Response (public_gateway 模式):**

```json
{
  "domain": "app.example.com",
  "managed": true,
  "mode": "public_gateway",
  "from_node": {
    "id": "nd_a",
    "hostname": "server-a"
  },
  "target_node": {
    "id": "nd_b",
    "hostname": "server-b",
    "public_ip": "43.159.34.11"
  },
  "gateway_url": "http://43.159.34.11:80",
  "gateway_port": 80,
  "protocol": "http",
  "route_id": "rt_abc123",
  "gateway_link_id": "gwlink_xyz456",
  "gateway_link_required": true,
  "direct_target_suppressed": true,
  "final_local_target": "127.0.0.1:2724",
  "hop_limit": 1,
  "risks": [
    {
      "code": "PUBLIC_TARGET_EGRESS",
      "severity": "info",
      "message": "egress traverses public network — ensure GatewayLink is configured"
    }
  ],
  "recommendation": "send request to target public gateway with GatewayLink auth"
}
```

**Response (external_passthrough):**

```json
{
  "domain": "external-service.com",
  "managed": false,
  "mode": "external_passthrough",
  "from_node": {
    "id": "nd_a",
    "hostname": "server-a"
  },
  "target_node": null,
  "gateway_url": null,
  "route_id": null,
  "direct_target_suppressed": false,
  "risks": [
    {
      "code": "UNKNOWN_DOMAIN",
      "severity": "info",
      "message": "domain is not managed by Aegis"
    }
  ],
  "recommendation": "domain is external — relay not available"
}
```

**Response (unavailable):**

```json
{
  "domain": "app.example.com",
  "managed": true,
  "mode": "unavailable",
  "from_node": {
    "id": "nd_a",
    "hostname": "server-a"
  },
  "target_node": {
    "id": "nd_b",
    "hostname": "server-b"
  },
  "route_id": "rt_abc123",
  "direct_target_suppressed": true,
  "error": "TARGET_GATEWAY_UNREACHABLE",
  "error_detail": "target node nd_b has no reachable gateway listener from nd_a",
  "risks": [],
  "recommendation": "check network connectivity between nodes or add a trusted gateway"
}
```

### CLI

| 命令 | 描述 |
|------|------|
| `aegis relay resolve <domain>` | 解析 domain 的 egress relay 路径 |
| `aegis relay resolve <domain> --from-node <node_id>` | 指定源节点 |
| `aegis relay resolve <domain> --json` | JSON 输出 |

### API 安全

| 端点 | 认证 | Scope |
|------|------|-------|
| `GET /api/admin/v1/resolve/egress-relay` | Admin session | 仅 admin |
| `GET /api/service/v1/resolve/egress-relay` | Bearer token | 受限 scope |

**Service API 限制：**
- Admin API 可以显示 `final_local_target`（完整的内部地址）
- Service API 默认**不显示** `final_local_target`（只返回 `gateway_url`，不暴露其他 scope 的 internal target）
- Service API 只返回和 token scope 匹配的 route 的 relay 信息

**Admin API 可以显示的内容：**
- `final_local_target`（`127.0.0.1:<port>`）
- `target_node.private_ip`
- 完整的 relay path

---

## 九、强制语义怎么落地

v1.8B 不强制执行（不做拦截），但必须设计强制路径为将来实现定义清晰的分层。

### Layer 1: Aegis-Managed Clients

| 范围 | 说明 |
|------|------|
| Aegis CLI | `aegis relay resolve` 返回 gateway URL，CLI 工具内部使用 |
| Aegis SDK (未来) | 调用方集成 SDK，自动解析 relay 路径 |
| 用户自定义脚本 | 文档指导用户调用 `relay resolve` 代替直接访问 |

**规则：** Aegis 自身组件必须使用 `RelayResolver`。不允许直接使用 `target_host:target_port`。

**v1.8B 实现：** `RelayResolver` service + CLI + API。Aegis 产生的请求（如果有）走 relay。

### Layer 2: Local Gateway Proxy

```
本机 localhost 提供 managed domain relay endpoint:
  http://localhost:8080/<domain>/<path>
  或
  curl --resolve <domain>:80:127.0.0.1 http://<domain>/<path>
```

| 功能 | 说明 |
|------|------|
| 本机 gateway 代理 | 提供 localhost 端点，managed domain 请求可发给本机 gateway |
| 自动转发 | gateway 根据 relay resolver 自动转发到目标 gateway |
| GatewayLink 管理 | local gateway proxy 自动附加 GatewayLink token |
| scope 控制 | Service API key 只能 relay 到自己 scope 内的 route |

**v1.8B 实现：** 在 gateway 配置中为 managed domain 注入 `reverse_proxy` 到目标 gateway。不实现独立的 local proxy 进程。

### Layer 3: Transparent Interception (v2)

```
DNS / iptables / TProxy
  ├── Aegis managed domain → gateway (而非原始 DNS)
  └── 非 managed domain → 原始 DNS
```

| 技术 | 说明 |
|------|------|
| DNS 劫持 | Aegis managed domain 解析到本机 gateway |
| iptables REDIRECT | 将 managed domain 的出口流量重定向到 gateway |
| TProxy | 透明代理模式 |

**v2 才实现。** v1.8B 不做透明拦截。

### v1.8B 范围

```
Layer 1: Aegis-managed clients  ✅ 实现
Layer 2: Local gateway proxy     ⚠️ 部分实现（gateway 间 relay，非独立 proxy）
Layer 3: Transparent interception ❌ 延期
```

---

## 十、安全边界

### 10 条安全规则

| # | 规则 | 违反后果 | 验证方式 |
|---|------|---------|---------|
| 1 | **Relay 不能成为开放代理** | 攻击者利用 Aegis 转发任意流量 | 目标必须来自 DB route/endpoint，不来自 header |
| 2 | **Header 不能指定任意 target_port** | 开放代理 / SSRF | target 只从 endpoint.address 获取 |
| 3 | **目标必须来自 Aegis DB** | 绕过控制面 | Dispatch Step 6 强制 DB lookup |
| 4 | **跨节点 relay 必须有 GatewayLink** | 未授权访问 | Dispatch Step 2 强制 GatewayLink 验证 |
| 5 | **public_gateway 模式必须有 GatewayLink** | 公网未认证 relay | 路径规则 4 强制 |
| 6 | **source node 必须可验证** | 伪造来源 | Dispatch Step 4 检查 nodes 表 |
| 7 | **hop 必须限制 (≤1)** | 无限循环 relay | Dispatch Step 5 检查 hop count |
| 8 | **Service API 不能泄漏其他 scope 的 internal target** | scope 越权 | `final_local_target` 仅 admin API 返回 |
| 9 | **local_gateway 不应打成 gateway listener 自身循环** | SELF_LOOP | 复用 `isGatewayListenerTarget()` 检测 |
| 10 | **relay failure 不应 fallback 到 remote target port** | 绕过强制语义 | 路径规则 5: `unavailable` 不 fallback |

### 附加风险

| 风险 | 说明 | 缓解 |
|------|------|------|
| GatewayLink token 泄露 | token 在 relay header 中传输 | token 到期 rotate, 未来 HMAC signing |
| Caddy 配置含 relay 目标 | Caddyfile 包含 127.0.0.1:port | Caddyfile 权限 0600 |
| 目标节点 gateway 被攻破 | 攻击者可获取 route/endpoint 信息 | 最小权限原则, audit log |
| 中间人攻击 (public_gateway) | 公网传输被拦截 | GatewayLink 提供身份验证（未来 TLS + HMAC） |
| 循环 relay 检测绕过 | 攻击者伪造 hop=0 header | 源节点设置 hop, 目标节点验证, 加密 hop header |

---

## 十一、数据缺口

**详细数据缺口分析见独立文档：** `docs/v1.8/managed-egress-relay-data-gap.md`

### 摘要

| # | 需要的字段 | 是否已有 | 备注 |
|---|-----------|---------|------|
| 1 | `route.node_id` | ❌ 没有 | Route 通过 service → endpoints 间接关联节点，无直接 node_id |
| 2 | `endpoint.node_id` | ❌ 没有 | Endpoint 只有 service_id，无法直接知道属于哪个 node |
| 3 | `endpoint.local_host` / `endpoint.local_port` | ⚠️ 部分 | Endpoint 有 address (host:port) 但 type=local 时 address 可能是 `127.0.0.1:port` |
| 4 | `node.private_ip` | ✅ 有 | `NodeRecord.PrivateIP` |
| 5 | `node.public_ip` | ✅ 有 | `NodeRecord.PublicIP` |
| 6 | `node.gateway_private_url` | ❌ 没有 | 可通过 node.private_ip + listener port 推导 |
| 7 | `node.gateway_public_url` | ❌ 没有 | 同上 |
| 8 | `node.network_id` | ❌ 没有 | 用于判断节点是否在同一个私有网络 |
| 9 | `trusted_gateway.source_node_id` | ❌ 没有 | 当前无源节点字段 |
| 10 | `trusted_gateway.target_node_id` | ❌ 没有 | 当前无目标节点字段 |
| 11 | Listener ports | ✅ 有 | `Listener.Port` 可查询 |
| 12 | Source node identity | ✅ 有 | `NodeRecord.NodeID` 可作为身份 |

---

## 十二、验收标准

### 设计问题回答

| # | 问题 | 答案 |
|---|------|------|
| 1 | 为什么不能直接访问 remote target port？ | 直接访问绕过 Aegis 控制面，无法实施路径归一化、GatewayLink 验证、scope 控制。真实端口只允许在目标节点本机被 gateway 访问。 |
| 2 | 什么情况下 `local_gateway`？ | `target_node == from_node`，服务在本机。通过本机 gateway 转发到 `127.0.0.1:<port>`。 |
| 3 | 什么情况下 `private_gateway`？ | 跨节点，目标节点 private IP 可达。请求经 source gateway → target private gateway → `127.0.0.1:<port>`。 |
| 4 | 什么情况下 `public_gateway`？ | 跨节点，仅 public IP 可达。请求经 source gateway → target public gateway → `127.0.0.1:<port>`。**必须**有 GatewayLink。 |
| 5 | 什么情况下 `external_passthrough`？ | Domain 不在 Aegis 管理矩阵（无 route、无 managed domain、无匹配 service）。Aegis 不接管。 |
| 6 | 什么情况下 `unavailable`？ | Domain 在管理矩阵内，但 target node gateway 不可达。**不 fallback** 到 direct target。 |
| 7 | Server B gateway 如何安全 dispatch 到 127.0.0.1:2724？ | 通过 6 步验证：route 存在、GatewayLink 有效、route 归属本机、source node 合法、hop ≤1、endpoint 是 local 类型。最终 target 只从 DB 读取。 |
| 8 | 如何防止开放代理？ | 目标不从 header 读取。转发 target 固定为 `127.0.0.1:<port>`，从 endpoint DB 记录获取。 |
| 9 | 如何防止循环 relay？ | `X-Aegis-Hop` 计数器 + 目标节点验证 hop ≤ 1。不递增 hop 值（只允许单跳）。 |
| 10 | 如何避免 scope 泄漏？ | `final_local_target` 仅 admin API 返回。Service API 不显示内部地址。 |
| 11 | v1.8B 实现哪些层，v2 才实现哪些层？ | Layer 1 (managed clients) + 部分 Layer 2 (gateway 间 relay) 在 v1.8B。Layer 2 local proxy + Layer 3 transparent interception 在 v2。 |

### 实现前条件（v1.8B-1）

- [ ] 数据缺口评估完成并决定是否填补
- [ ] `RelayResolver` service 接口设计完成
- [ ] Gateway 间 relay header 规范定稿
- [ ] Target Gateway Dispatch 逻辑设计完成
- [ ] API 端点和 CLI 命令设计完成
- [ ] 安全规则实现路径明确

---

**v1.8B-0 Managed Egress Relay Design: COMPLETE**

Next: v1.8B-1 — 数据缺口填补 + RelayResolver service + API/CLI + Gateway Dispatch (HTTP).
