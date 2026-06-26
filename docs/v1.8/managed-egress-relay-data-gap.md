# v1.8B-0 — Managed Egress Relay Data Gap Analysis

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Sub-phase:** v1.8B-0 — Design only, no code
> **Status:** DATA GAP ANALYSIS COMPLETE
> **Purpose:** Identify missing data fields required for Managed Egress Relay implementation

---

## Table of Contents

1. [Data Requirement Map](#1-data-requirement-map)
2. [Current Schema Inventory](#2-current-schema-inventory)
3. [Gap Details](#3-gap-details)
4. [Derived Data](#4-derived-data)
5. [Recommendations](#5-recommendations)

---

## 1. Data Requirement Map

```
Relay Decision Flow
  │
  ├── domain → 查 route → route.gateway_link_id?
  │     │
  │     ├── Route 需要: domain, service_id, gateway_link_id
  │     │     ├── domain:                    ✅ Route.Domain
  │     │     ├── service_id:                ✅ Route.ServiceID
  │     │     └── gateway_link_id:           ✅ Route.GatewayLinkID
  │     │
  │     ├── Service 需要: kind, status
  │     │     ├── kind:                      ✅ Service.Kind
  │     │     └── status:                    ✅ Service.Status
  │     │
  │     ├── Endpoint 需要: type, address, node_id, local_host, local_port
  │     │     ├── type:                      ✅ Endpoint.Type
  │     │     ├── address:                   ✅ Endpoint.Address
  │     │     ├── node_id:                   ❌ GAP
  │     │     └── local_host/local_port:     ⚠️ 部分 (address 可解析)
  │     │
  │     ├── target_node 需要: id, private_ip, public_ip, gateway_urls
  │     │     ├── id:                        ✅ Node.NodeID
  │     │     ├── private_ip:                ✅ Node.PrivateIP
  │     │     ├── public_ip:                 ✅ Node.PublicIP
  │     │     ├── gateway_private_url:       ❌ GAP (可推导)
  │     │     └── gateway_public_url:        ❌ GAP (可推导)
  │     │
  │     └── GatewayLink 需要: 验证 + 路由
  │           ├── source_node_id:            ❌ GAP
  │           ├── target_node_id:            ❌ GAP
  │           ├── host (public):             ✅ TrustedGateway.Host
  │           ├── private_ip:                ✅ TrustedGateway.PrivateIP
  │           ├── port:                      ✅ TrustedGateway.Port
  │           ├── auth_value (hash):         ✅ TrustedGateway.AuthValue
  │           └── auto_route:                ✅ TrustedGateway.AutoRoute
  │
  └── mode decision (local/private/public/unavailable)
        ├── target_node == from_node?:       ✅ (比较 node_id)
        ├── private IP reachable?:           ⚠️ (假设可达，无明确 network_id)
        └── public IP reachable?:            ⚠️ (假设可达)
```

---

## 2. Current Schema Inventory

### Table: `nodes` (migration 010 + 011 + 012)

```sql
id            TEXT PRIMARY KEY,       -- DB ID
node_id       TEXT NOT NULL,           -- nd_a, nd_b...
hostname      TEXT NOT NULL,
local_ip      TEXT DEFAULT '127.0.0.1',
private_ip    TEXT DEFAULT '',
public_ip     TEXT DEFAULT '',
is_current    INTEGER DEFAULT 0,
ip_migrated   INTEGER DEFAULT 0,
last_seen     TEXT NOT NULL,
created_at    TEXT NOT NULL,
updated_at    TEXT NOT NULL
```

**Post-v1.7A additions (not shown in DDL but in model):**
- `is_leader` INTEGER (migration 011)
- `state_version` INTEGER (migration 012)
- `capabilities` TEXT (migration 021)

### Table: `routes` (migration 001 + 025)

```sql
id                 TEXT PRIMARY KEY,
domain             TEXT NOT NULL,
path_prefix        TEXT DEFAULT '',
strip_prefix       INTEGER DEFAULT 0,
service_id         TEXT NOT NULL,
tls_enabled        INTEGER DEFAULT 0,
status             TEXT DEFAULT 'active',
maintenance_enabled    INTEGER DEFAULT 0,
maintenance_message    TEXT DEFAULT '',
space_id           TEXT DEFAULT '',
owner_type         TEXT DEFAULT 'admin',
owner_id           TEXT DEFAULT '',
created_by_token_id    TEXT DEFAULT '',
gateway_link_id    TEXT DEFAULT '',    -- migration 025
created_at         TEXT NOT NULL,
updated_at         TEXT NOT NULL
```

### Table: `services` (migration 001)

```sql
id                TEXT PRIMARY KEY,
project_id        TEXT NOT NULL,
name              TEXT NOT NULL,
kind              TEXT NOT NULL,       -- http|tcp|file
env               TEXT DEFAULT 'dev',
status            TEXT DEFAULT 'active',
note              TEXT DEFAULT '',
space_id          TEXT DEFAULT '',
owner_type        TEXT DEFAULT 'admin',
owner_id          TEXT DEFAULT '',
created_by_token_id   TEXT DEFAULT '',
created_at        TEXT NOT NULL,
updated_at        TEXT NOT NULL
```

### Table: `endpoints` (migration 001)

```sql
id                TEXT PRIMARY KEY,
service_id        TEXT NOT NULL,
type              TEXT NOT NULL,       -- local|private|public
address           TEXT NOT NULL,       -- host:port (e.g. "127.0.0.1:3001")
enabled           INTEGER DEFAULT 1,
created_at        TEXT NOT NULL,
updated_at        TEXT NOT NULL
```

### Table: `trusted_gateways` (migration 024)

```sql
id                TEXT PRIMARY KEY,
name              TEXT DEFAULT '',
host              TEXT DEFAULT '',     -- public IP or hostname
private_ip        TEXT DEFAULT '',
port              INTEGER DEFAULT 443, -- target port (usually 443)
auth_type         TEXT DEFAULT 'shared_secret',
auth_value        TEXT DEFAULT '',     -- hashed
gateway_type      TEXT DEFAULT 'upstream', -- upstream|downstream|peer
auto_route        INTEGER DEFAULT 1,   -- prefer private IP
status            TEXT DEFAULT 'active',
created_at        TEXT NOT NULL,
updated_at        TEXT NOT NULL
```

### Table: `listeners` (migration 006)

```sql
id                TEXT PRIMARY KEY,
node_id           TEXT NOT NULL,
provider          TEXT NOT NULL,
protocol          TEXT NOT NULL,       -- http|https|tcp|tls_mux
bind_ip           TEXT NOT NULL,
port              INTEGER NOT NULL,
purpose           TEXT NOT NULL,
status            TEXT NOT NULL,
created_at        TEXT NOT NULL,
updated_at        TEXT NOT NULL
```

---

## 3. Gap Details

### Gap 1: Endpoint → Node 关联

| 字段 | `endpoint.node_id` |
|------|-------------------|
| **现状** | Endpoint 只有 `service_id`，Service 没有直接的 `node_id` 字段 |
| **推理链** | `Route → Service → Endpoint`，但没有任何字段标记 endpoint 属于哪个 node |
| **为什么需要** | Target Gateway Dispatch Step 3 需要确认 "endpoint belongs to local node" |
| **变通方案** | 通过 `endpoint.type == 'local'` 推断 endpoint 在目标节点本机。但无法确认 "这是否是指定 target_node 的 endpoint" |
| **填补建议** | 在 migration 中添加 `endpoints.node_id`，或通过 `service_id` 和节点的 `is_current` 隐式关联 |

### Gap 2: Route → Node 直接关联

| 字段 | `route.node_id` |
|------|----------------|
| **现状** | Route 通过 `service_id → endpoints` 间接关联节点，无直接字段 |
| **为什么需要** | 快速判断 `route 指向哪个 node`，不需要联表查询 endpoints |
| **变通方案** | 联表查询: `routes → services → endpoints` 取 local endpoint 的 node 信息 |
| **填补建议** | 不直接添加。保持 route 通过 service 间接关联节点的模式。relay resolver 做联表查询。 |

### Gap 3: Endpoint 解析地址

| 字段 | `endpoint.address` 可解析性 |
|------|---------------------------|
| **现状** | Address 是自由格式字符串（如 `"127.0.0.1:3001"`），需要 `net.SplitHostPort` 解析 |
| **风险** | 有些 address 可能是 hostname、路径、或不含端口 |
| **为什么需要** | Relay dispatch 需要明确的 `host` + `port` |
| **变通方案** | 在 runtime 解析 address。对于 `type=local` 的 endpoint，预期格式为 `127.0.0.1:<port>` |
| **填补建议** | 在 endpoint model 中添加 `Host` + `Port` 独立字段，或通过 validator 保证 `local` 类型的 address 格式规范 |

### Gap 4: Node Gateway URLs

| 字段 | `node.gateway_private_url` / `node.gateway_public_url` |
|------|--------------------------------------------------------|
| **现状** | Nodes 表有 `private_ip` 和 `public_ip`，但没有 gateway URL 概念 |
| **推导** | 可以运行时推导：`http://<private_ip>:<listener_port>` |
| **为什么需要** | Relay 决策需要知道目标节点 gateway 的监听地址 |
| **变通方案** | 查询 `listeners` 表获取该节点的所有 listener，按 `purpose` 筛选出 `public_http` / `public_tls_mux` 端口，结合 node IP 构造 URL |
| **填补建议** | 不在 DB 中添加冗余字段。在 relay service 中运行时构造。 |

**Gateway URL 推导逻辑：**

```
func ResolveGatewayURL(node *NodeRecord, listeners []Listener, preferPrivate bool) string {
    // 1. 找第一个 active 的 public_http 或 public_tls_mux listener
    for _, l := range listeners {
        if l.NodeID == node.NodeID && l.Status == "active" {
            if l.Purpose == "public_http" || l.Purpose == "public_tls_mux" {
                ip := node.PublicIP
                if preferPrivate && node.PrivateIP != "" {
                    ip = node.PrivateIP
                }
                return fmt.Sprintf("http://%s:%d", ip, l.Port)
            }
        }
    }
    // 2. Fallback: 80
    ip := node.PublicIP
    if preferPrivate && node.PrivateIP != "" {
        ip = node.PrivateIP
    }
    return fmt.Sprintf("http://%s:80", ip)
}
```

### Gap 5: Network/Reachability Awareness

| 字段 | `node.network_id` 或可达性信息 |
|------|-------------------------------|
| **现状** | 没有关于节点间网络拓扑的信息 |
| **为什么需要** | 判断 `private_gateway` vs `public_gateway` 需要知道 private IP 是否可达 |
| **当前假设** | v1.8B 假设: 如果 target_node 有 PrivateIP 且 from_node 在同一网络，private 可达。具体由 operator 配置 `trusted_gateway.auto_route` 控制 |
| **变通方案** | 不添加 `network_id`。使用 `trusted_gateway.auto_route` 字段控制 IP 选择。如果 `auto_route = true` 且 `private_ip` 存在 → 尝试 private。如果失败 → operator 手动设置为 `false` |
| **填补建议** | 不在 v1.8B 中添加。将来可通过 connectivity check 实现自动侦测 |

### Gap 6: TrustedGateway 节点关联

| 字段 | `trusted_gateway.source_node_id` / `trusted_gateway.target_node_id` |
|------|-------------------------------------------------------------------|
| **现状** | `TrustedGateway` 有 `Host`, `PrivateIP`, `Port` 等连接信息，但没有关联到 Aegis node 记录 |
| **为什么需要** | Relay 决策需要知道 "这个 GatewayLink 连接哪两个节点"。当前只能通过 host/private_ip 模糊匹配到节点 |
| **变通方案** | 通过 `host` 或 `private_ip` 匹配到 `nodes` 表的节点记录 |
| **填补建议** | 添加 `target_node_id` 字段到 `trusted_gateways`。这是 relay 决策中比较重要的 gap |

### Gap 7: Route GatewayLink 反向查询

| 字段 | `route → trusted_gateway` 反向查询 |
|------|------------------------------------|
| **现状** | `route.gateway_link_id` → `trusted_gateway.id` (正向) |
| **反向** | 知道 trusted_gateway，想查所有使用它的 route（relay 配置管理需要） |
| **填补建议** | 不需要新字段。SQL 查询: `SELECT * FROM routes WHERE gateway_link_id = ?` |

### Gap 8: Relay Session/Event Log

| 字段 | Relay Event Log |
|------|----------------|
| **现状** | 没有 relay 专用 event log |
| **为什么需要** | 追踪 relay 请求的来源、目标、成功/失败 |
| **变通方案** | 复用 `node_events` 表记录 relay 事件 |
| **填补建议** | 在 `node_events` 中添加 `event_type = 'relay'`，payload 包含 route_id, source_node, target_node, success/fail |

---

## 4. Derived Data

以下数据不需要新字段，可以通过现有数据推导：

| 派生数据 | 推导来源 | 推导方法 |
|---------|---------|---------|
| Route → Node 映射 | Route → Service → Endpoints → Endpoint address + node IP | 查询 service 的所有 endpoints，解析 address，匹配 node IP |
| Node gateway URL | Node IP + Listener port | 见上文 `ResolveGatewayURL` |
| Self-node check | from_node.id 和 target_node.id 比较 | 直接比较 |
| Managed domain check | Route 表查询 | `SELECT * FROM routes WHERE domain = ?` |
| Hop validation | Header `X-Aegis-Hop` | 解析整数值，检查 ≤ 1 |
| GatewayLink token validation | `TrustedGateway.CheckAuth()` | 已经实现 (HMAC hash) |

---

## 5. Recommendations

### 实现前必须填补的 Gap (高优先级)

| # | Gap | 迁移 | 影响 |
|---|-----|------|------|
| 1 | `trusted_gateway.target_node_id` | Migration 026 | **必须** — relay 决策需要知道 GatewayLink 连接的目标节点。没有这个字段，relay resolver 无法确定跨节点路由。 |
| 2 | `endpoint.node_id` 或等价关联 | Migration 026 | **必须** — Target Gateway Dispatch 需要确认 endpoint 属于本机节点 |

### 建议填补但不阻塞 (中优先级)

| # | Gap | 理由 |
|---|------|------|
| 3 | Endpoint Host + Port 独立字段 | 降低 address 解析风险。但解析现有 address 字段可作为 v1.8B-1 快速实现 |
| 4 | Relay event log type | 可观察性需求，不阻塞 relay 功能 |

### 不需要填补 (低优先级)

| # | Gap | 理由 |
|---|------|------|
| 5 | Node gateway URLs | 运行时推导，不需要 DB 存储 |
| 6 | Network ID | v1.8B 使用 `auto_route` + operator 判断，不自动侦测 |
| 7 | Route.node_id | 通过 service → endpoints 联表查询，不需要冗余字段 |

### Migration 026 草案

```sql
-- 1. 添加 trusted_gateway.target_node_id
ALTER TABLE trusted_gateways ADD COLUMN target_node_id TEXT NOT NULL DEFAULT '';

-- 2. 添加 endpoints.node_id (可选，如果选择填补)
ALTER TABLE endpoints ADD COLUMN node_id TEXT NOT NULL DEFAULT '';

-- 3. 添加 endpoints 的 host+port 独立字段 (可选)
ALTER TABLE endpoints ADD COLUMN host TEXT NOT NULL DEFAULT '';
ALTER TABLE endpoints ADD COLUMN port INTEGER NOT NULL DEFAULT 0;

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_trusted_gateways_target_node ON trusted_gateways(target_node_id);
CREATE INDEX IF NOT EXISTS idx_endpoints_node_id ON endpoints(node_id);
```

**注意：** 以上 migration 草案仅用于设计阶段参考。实际 migration 名称和 SQL 在 v1.8B-1 实现时确认。

### Implementation Guidance

```
v1.8B-1 的数据准备工作:
  1. 创建 migration 026
  2. 添加 trusted_gateway.target_node_id
  3. (可选) 添加 endpoint.node_id
  4. 更新 gateway_link repository 扫描新字段
  5. 更新 endpoint repository 扫描新字段
  6. 更新 model structs
  7. 数据填充脚本: 通过 IP 匹配自动填充现有记录

v1.8B-1 的 RelayResolver:
  1. 创建 internal/relay/ 包
  2. 实现 Resolver struct
  3. 实现 Resolve(domain, fromNodeID) → RelayPath
  4. 实现 gateway URL 推导逻辑
  5. 集成 gateway_link 验证
  6. 试验性 CLI 命令
  7. 试验性 API 端点
```

---

## Summary

| 数据项 | 状态 | 优先级 | 行动 |
|--------|------|--------|------|
| `trusted_gateway.target_node_id` | ❌ 缺失 | 🔴 High | Migration 026 |
| `endpoint.node_id` | ❌ 缺失 | 🔴 High | Migration 026 |
| Endpoint address 解析 | ⚠️ 脆弱 | 🟡 Medium | address validator |
| Gateway URL 推导 | ✅ 可推导 | — | Service 层实现 |
| Network reachability | ⚠️ 手动 | 🟢 Low | Operator 配置 |
| Node gateway URLs | ✅ 可推导 | — | Service 层实现 |
| Relay event log | ❌ 缺失 | 🟢 Low | 未来增强 |
| Route → Node 查询 | ⚠️ 联表 | 🟢 Low | Service 层联表查询 |

---

**v1.8B-0 Data Gap Analysis: COMPLETE**

Next: v1.8B-1 — 填补数据缺口 + 实现 RelayResolver + API/CLI + Gateway Dispatch.
