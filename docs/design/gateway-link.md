# Gateway Link

跨节点（Server A → Server B）的认证与流量转发机制。

## 1. 认证方式

当前使用 **HMAC-SHA256 动态签名** 认证（v1.7AD 升级），带时间戳防重放（5 分钟窗口）。

### 请求头

```
X-Aegis-Gateway-Link:  <link_id>        # 标识 gateway link
X-Aegis-Gateway-Token: <hmac_signature> # HMAC-SHA256 签名
X-Aegis-Timestamp:     <unix_timestamp> # 防重放
```

### 验证流程

```
Server A (发起端)
  Caddy reverse_proxy {
      header_up X-Aegis-Gateway-Link  "gw_abc123"
      header_up X-Aegis-Gateway-Token "<hmac>"
      header_up X-Aegis-Timestamp     "1719600000"
  }
        │
        │ HTTP/HTTPS :80
        ▼
Server B (接收端)
  验证器检查:
  1. X-Aegis-Gateway-Link 存在且匹配
  2. X-Aegis-Timestamp 在 5 分钟窗口内
  3. HMAC-SHA256 签名匹配
  4. 通过 → 200 OK | 缺失 → 401 | 错误 → 403
```

## 2. 路由绑定

```
Route ──optional──→ GatewayLinkID (varchar, nullable)
                       │
                  Planner 读取 GatewayLink 记录
                       │
                  Caddy render: header_up X-Aegis-Gateway-*
```

| Action | Method | Path | Field |
|--------|--------|------|-------|
| 创建路由时绑定 | POST /api/routes | body.gateway_link_id |
| 更新路由绑��� | PATCH /api/routes/{id} | body.gateway_link_id |
| 解除绑定 | PATCH /api/routes/{id} | body.gateway_link_id="" |

### Planner 行为

当 `route.GatewayLinkID` 非空时：
1. 查询 GatewayLink 记录
2. 网关存在且活跃 → 注入 ExtraHeaders（X-Aegis-Gateway-Link、X-Aegis-Gateway-Token、X-Aegis-Timestamp）
3. 网关不存在或不活跃 → apply plan 中产生警告

## 3. 密钥管理

### Token 生命周期

```
Create GatewayLink
  → 生成随机 token
  → DB 存储 HMAC-SHA256 哈希（非明文）
  → 返回原始 token 一次（此时仅此一次）
  → 后续 list/get 不再返回 token

Planner/Caddy render
  → 从 DB 读取 token
  → 注入 Caddyfile 作为 header_up 值
  → token 存在于渲染后的配置和备份中

不会暴露 token 的位置:
  ❌ apply logs
  ❌ operation logs
  ❌ audit logs
  ❌ trace 输出
  ❌ list/get API
```

### 风险评估

| 位置 | Token 存在 | 风险 | 缓解措施 |
|------|:---:|------|------|
| SQLite DB | HMAC 哈希 | 低 | DB 文件权限 0600 |
| Caddyfile (渲染后) | 临时签名 | 中 | Caddyfile 权限 0600 |
| 配置备份 | 临时签名 | 中 | 备份目录限制 |
| API create 响应 | 原始 token 一次 | 低 | 单次返回 + 警告 |
| API list/get | ❌ | — | — |

## 4. 密钥轮换

```
1. POST /api/admin/v1/gateway-links/{id}/rotate
2. Aegis 生成新随机 token
3. 更新 trusted_gateways.auth_value
4. token_version 递增
5. MarkPending("gateway link secret rotated")
6. 返回新 token 一次

手动步骤:
7. 复制新 token 到 Server B 验证器配置
8. Server B 重启或热加载新 token
9. POST /api/admin/v1/system/apply
10. Caddy 热加载新 header_up token
11. 旧 token 被 Server B 拒绝，新 token 接受
```

### 轮换窗口

| 状态 | Server A | Server B | 结果 |
|------|----------|----------|------|
| 轮换前 | Token v1 在 Caddy | 验证器期望 v1 | ✅ 200 |
| 轮换后→apply前 | Token v2 在 DB，v1 在 Caddy | 验证器期望 v1 | ✅ 200（旧配置） |
| Apply | Token v2 在 Caddy | 验证器期望 v1 | ❌ 403 |
| 更新验证器 | Token v2 在 Caddy | 验证器期望 v2 | ✅ 200 |

## API 参考

```
POST   /api/admin/v1/gateway-links            # 创建
GET    /api/admin/v1/gateway-links            # 列表（不含 token）
GET    /api/admin/v1/gateway-links/{id}       # 详情（不含 token）
DELETE /api/admin/v1/gateway-links/{id}       # 删除
POST   /api/admin/v1/gateway-links/{id}/rotate # 轮换（返回新 token 一次）
```

## 实现位置

| 模块 | 文件 |
|------|------|
| HMAC 签名/验证 | `internal/gateway_link/crypto.go` |
| 路由绑定 | `internal/apply/planner.go` |
| Caddy 渲染 | `internal/provider/caddy_http.go` |
| 跨节���路由解析 | `internal/noderuntime/caddy_applier.go` |
