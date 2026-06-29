# Aegis 运维场景手册

本文档覆盖 Aegis 控制平面的 6 个规范运维场景，每一步均提供可复制粘贴的 curl 命令。所有路径和方法均与 `internal/httpapi/routes.go` 中注册的路由一致。

**全局约定：**
- `{AEGIS_BASE}` — Aegis 控制平面地址，例如 `http://127.0.0.1:9527`（开发）或 `https://<SERVER_A_IP>`（生产）
- `{ADMIN_TOKEN}` — 管理员令牌，在 Aegis 首次启动时生成并打印在控制台，也可通过 `GET /api/settings` 查看（已脱敏）
- `{PROJECT_ID}`, `{SERVICE_ID}`, `{ROUTE_ID}` 等 — 由前置步骤返回的动态 ID
- `-b cookies.txt -c cookies.txt` — 使用 curl cookie jar 保存会话，确保登录态跨命令传递
- 所有 Admin API（`/api/admin/v1/*`）需要有效 session cookie，在登录步骤获取
- 所有请求 Content-Type 为 `application/json`，除非特别说明

---

## 场景 A：单节点全流程（Single Node Full Flow）

### 前置条件

- 一台 Linux 服务器（本地开发机或 VPS），已安装 Caddy
- Aegis 二进制文件已编译或下载
- Caddy 配置路径（默认 `/etc/caddy/Caddyfile`）可写
- 端口 80 / 443 在防火墙已开放（如需外部访问）

### A.1 初始化 Aegis

首次启动 Aegis 时，系统会自动创建 SQLite 数据库、生成管理员密码和令牌。

```bash
# 启动 Aegis 服务（前台运行，日志输出到控制台）
./aegis serve --config aegis.yaml

# 首次启动输出示例：
# [INIT] database initialized at /var/lib/aegis/aegis.db
# [INIT] admin password:  xxxxxxxxxxxx  <-- 记录此密码
# [INIT] admin token:    xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
# [INFO] server listening on :9527
```

> **说明：** 管理员密码和令牌仅在首次启动时打印一次。请妥善保存。丢失密码需手动操作 SQLite 数据库重置。

### A.2 查看系统状态

检查 Aegis 是否正常运行，数据库状态、代理配置及资源计数。

```bash
# 系统状态（无需认证）
curl -s ${AEGIS_BASE}/api/system/status | jq .
```

**预期响应：**
```json
{
  "name": "aegis",
  "version": "0.x",
  "server_time": "2026-01-15T10:30:00+08:00",
  "proxy": {
    "provider": "caddy",
    "config_path": "/etc/caddy/Caddyfile",
    "validate_available": true,
    "reload_command_configured": true
  },
  "store": {
    "sqlite_path": "/var/lib/aegis/aegis.db",
    "schema_version": "v1.8E"
  },
  "counts": {
    "projects": 0,
    "services": 0,
    "routes": 0,
    "managed_domains": 0
  },
  "health": {
    "healthy_endpoints": 0,
    "unhealthy_endpoints": 0,
    "unknown_endpoints": 0
  }
}
```

### A.3 管理员登录

所有管理操作需要有效的 session cookie。

```bash
# 管理员登录
curl -s -X POST ${AEGIS_BASE}/api/admin/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"{ADMIN_PASSWORD}"}' \
  -b cookies.txt -c cookies.txt | jq .
```

**预期响应：**
```json
{
  "user": {
    "id": "usr_xxxxxxxx",
    "username": "admin"
  },
  "expires_at": "2026-01-15T22:30:00+08:00"
}
```

> **说明：** 登录成功后，session cookie `aegis_admin_session` 保存在 `cookies.txt` 中，后续命令通过 `-b cookies.txt` 携带。登录有频率限制：5 次/分钟/IP。

验证登录状态：

```bash
curl -s ${AEGIS_BASE}/api/admin/v1/auth/me -b cookies.txt | jq .
```

### A.4 创建 Project

Project 是资源的顶层组织单位（v1.7 起强制要求 service 关联 project）。

```bash
curl -s -X POST ${AEGIS_BASE}/api/projects \
  -H "Content-Type: application/json" \
  -d '{"name":"demo-project","description":"演示项目"}' \
  -b cookies.txt | jq .
```

**预期响应（201 Created）：**
```json
{
  "id": "prj_xxxxxxxx",
  "name": "demo-project",
  "description": "演示项目",
  "status": "active",
  "created_at": "2026-01-15T10:30:01+08:00",
  "updated_at": "2026-01-15T10:30:01+08:00"
}
```
记录返回的 `id` 为 `{PROJECT_ID}`。

### A.5 创建 Service

Service 代表一个后端应用，是 endpoint 和 route 的聚合点。

```bash
curl -s -X POST ${AEGIS_BASE}/api/services \
  -H "Content-Type: application/json" \
  -d "{\"project_id\":\"{PROJECT_ID}\",\"name\":\"app-nginx\",\"kind\":\"web\",\"env\":\"production\"}" \
  -b cookies.txt | jq .
```

**预期响应（201 Created）：**
```json
{
  "id": "svc_xxxxxxxx",
  "project_id": "prj_xxxxxxxx",
  "name": "app-nginx",
  "kind": "web",
  "env": "production",
  "status": "active",
  "note": "",
  "created_at": "2026-01-15T10:30:02+08:00",
  "updated_at": "2026-01-15T10:30:02+08:00"
}
```
记录返回的 `id` 为 `{SERVICE_ID}`。

### A.6 创建 Endpoint

Endpoint 是 Service 的具体后端实例（IP:Port 或 unix socket）。

```bash
curl -s -X POST ${AEGIS_BASE}/api/services/{SERVICE_ID}/endpoints \
  -H "Content-Type: application/json" \
  -d '{"type":"http","address":"127.0.0.1:8080","node_id":"self"}' \
  -b cookies.txt | jq .
```

**预期响应（201 Created）：**
```json
{
  "id": "ep_xxxxxxxx",
  "service_id": "svc_xxxxxxxx",
  "type": "http",
  "address": "127.0.0.1:8080",
  "enabled": true,
  "node_id": "self",
  "created_at": "2026-01-15T10:30:03+08:00",
  "updated_at": "2026-01-15T10:30:03+08:00"
}
```

> **参数说明：**
> - `type`: `http`, `https`, `h2c`, `fastcgi`, `unix` 等
> - `address`: 后端地址，格式取决于 type
> - `node_id`: 对于单节点部署填 `"self"` 即可

### A.7 创建 Route

Route 将域名绑定到 Service，实现 "域名 → Service → Endpoints" 的映射链。

```bash
curl -s -X POST ${AEGIS_BASE}/api/routes \
  -H "Content-Type: application/json" \
  -d "{\"domain\":\"demo.example.com\",\"service_id\":\"{SERVICE_ID}\"}" \
  -b cookies.txt | jq .
```

**预期响应（201 Created）：**
```json
{
  "id": "rte_xxxxxxxx",
  "domain": "demo.example.com",
  "service_id": "svc_xxxxxxxx",
  "tls_enabled": false,
  "status": "active",
  "maintenance_enabled": false,
  "maintenance_message": "",
  "created_at": "2026-01-15T10:30:04+08:00",
  "updated_at": "2026-01-15T10:30:04+08:00"
}
```
记录返回的 `id` 为 `{ROUTE_ID}`。

> **说明：** `tls_enabled` 默认为 `false`。如需 HTTPS，需要先创建 Managed Domain 并进行域名验证，或在后续通过 PATCH 开启。

### A.8 启用 Route

新创建的 route 默认为启用状态；如之前被禁用，可通过此步骤启用。

```bash
curl -s -X POST ${AEGIS_BASE}/api/routes/{ROUTE_ID}/enable \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{"status": "enabled"}
```

> **说明：** 所有变更操作（创建/启用/禁用/修改）均会调用 `MarkPending`，标记配置已变更但尚未 Apply。这是设计意图 —— Aegis 不自动应用变更，需显式执行 Apply。

### A.9 预览配置

在 Apply 之前，先预览即将生成的配置文件（Dry Run），检查警告信息。

```bash
curl -s ${AEGIS_BASE}/api/config/preview -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "rendered_config": "demo.example.com {\n  reverse_proxy 127.0.0.1:8080\n}\n",
  "warnings": [],
  "route_count": 1,
  "managed_domain_count": 0,
  "skipped_count": 0
}
```

> **检查要点：**
> - `warnings` 为空表示无警告
> - `rendered_config` 包含生成的 Caddy 配置片段
> - 如果存在警告，先解决问题再 Apply

### A.10 执行 Apply

将变更应用到 Caddy（写入 Caddyfile 并 reload Caddy）。

```bash
curl -s -X POST ${AEGIS_BASE}/api/apply \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "version": "demo.example.com {\n",
  "warnings": [],
  "route_count": 1,
  "managed_domain_count": 0
}
```

> **说明：** Apply 执行以下流程：
> 1. 生成完整的 Caddyfile
> 2. 备份当前 Caddyfile（如已存在）
> 3. 写入新 Caddyfile
> 4. 执行 `caddy validate` 验证配置
> 5. 验证通过后执行 `caddy reload`
> 6. 记录 Apply 历史
>
> 如果第 4 步验证失败，Apply 会**自动回滚**到旧配置，详见场景 D。

### A.11 验证 Apply 历史

查看 Apply 记录，确认最近的 Apply 状态。

```bash
curl -s ${AEGIS_BASE}/api/apply/history -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
[
  {
    "id": "apply_001",
    "version": "v1-20260115T103010",
    "config_path": "/etc/caddy/Caddyfile",
    "backup_path": "/var/lib/aegis/backups/config/Caddyfile.20260115T103010.bak",
    "status": "success",
    "message": "applied 1 routes, 0 managed domains",
    "created_at": "2026-01-15T10:30:10+08:00"
  }
]
```

### A.12 检查健康状态

验证所有 endpoint 的健康检查结果。

```bash
curl -s ${AEGIS_BASE}/api/health -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
[
  {
    "id": "hck_xxxxxxxx",
    "service_id": "svc_xxxxxxxx",
    "endpoint_id": "ep_xxxxxxxx",
    "status": "healthy",
    "latency_ms": 1250,
    "message": "HTTP 200 OK",
    "checked_at": "2026-01-15T10:30:15+08:00"
  }
]
```

> **说明：** Aegis 定期对所有启用的 endpoint 执行健康检查。`status` 可能为 `healthy`、`unhealthy` 或 `unknown`。

### 场景 A 验证清单

- [ ] 系统状态返回 `counts.projects >= 1`
- [ ] 登录成功，`/api/admin/v1/auth/me` 返回用户信息
- [ ] Project、Service、Endpoint、Route 均创建成功并返回 `id`
- [ ] 配置预览无 `warnings`
- [ ] Apply 成功且 Apply 历史中有 `status: "success"` 记录
- [ ] 健康检查返回 endpoint 状态
- [ ] 访问 `http://demo.example.com`（或配置的域名）能到达后端 `127.0.0.1:8080`

---

## 场景 B：双节点跨机器路由（Two-Node Cross-Machine Routing）

### 前置条件

- **Server A**（控制平面 + 节点代理，IP: `{SERVER_A_IP}`）：已启动 Aegis `serve`，Caddy 已安装
- **Server B**（远程节点，IP: `{SERVER_B_IP}`）：Caddy 已安装，端口 80 TCP 在安全组开放
- 安全组规则：两台服务器之间端口 80 TCP 双向放通
- SSH 从开发机到两台 VPS 均已配置

> **前置验证（网络连通性）：**
> ```bash
> # 从 Server A 验证能到达 Server B 的 80 端口
> ssh ubuntu@${SERVER_A_IP} "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://${SERVER_B_IP}:80/"
> ```

### B.1 确认 Server A 上 Aegis 运行

```bash
ssh ubuntu@${SERVER_A_IP} "curl -s http://127.0.0.1:9527/api/system/status | jq .name"
# 预期: "aegis"
```

### B.2 创建 Join Token（Server A 控制平面）

Join Token 用于 Server B 上的节点代理向控制平面注册。

```bash
curl -s -X POST ${AEGIS_BASE}/api/admin/v1/node-join-tokens \
  -H "Content-Type: application/json" \
  -d '{"name":"server-b-token","allowed_roles":["edge","gateway"],"expected_node_name":"node-b","expires_in_seconds":7200}' \
  -b cookies.txt | jq .
```

**预期响应（201 Created）：**
```json
{
  "id": "jnt_xxxxxxxx",
  "name": "server-b-token",
  "raw_join_token": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "token_redacted": false,
  "expires_at": "2026-01-15T12:30:00+08:00",
  "allowed_roles": ["edge", "gateway"],
  "warning": "store this token securely — it will not be shown again"
}
```
记录 `raw_join_token` 为 `{JOIN_TOKEN}`。

> **说明：** `raw_join_token` 仅在此次响应中返回，之后不可恢复。请立即复制保存。

### B.3 在 Server B 上部署节点代理

在 Server B 上执行，使用 Aegis 的 `node run` 模式或一键部署。

```bash
# 方案 1：直接在 Server B 上使用 join token 启动
ssh ubuntu@${SERVER_B_IP} "nohup ./aegis node run \
  --control-plane http://${SERVER_A_IP}:9527 \
  --join-token ${JOIN_TOKEN} \
  --node-name node-b \
  --role edge,gateway \
  > /var/log/aegis-node.log 2>&1 &"

# 方案 2：使用一键部署 API（在 Server A 上执行）
curl -s -X POST ${AEGIS_BASE}/api/admin/v1/nodes/deploy \
  -H "Content-Type: application/json" \
  -d '{"host":"{SERVER_B_IP}","ssh_port":22,"username":"ubuntu","join_token":"{JOIN_TOKEN}","node_name":"node-b","role":"edge,gateway"}' \
  -b cookies.txt | jq .
```

> **说明：** 节点代理启动后会自动调用 `POST /api/node/v1/join` 注册，之后定期发送心跳 `POST /api/node/v1/heartbeat`。

### B.4 验证节点上线

```bash
# 查看节点列表
curl -s ${AEGIS_BASE}/api/admin/v1/nodes -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "nodes": [
    {
      "id": "nod_xxxxxxxx",
      "node_id": "node-a",
      "name": "server-a",
      "role": "leader,gateway",
      "status": "online",
      "hostname": "server-a",
      "public_ip": "{SERVER_A_IP}",
      "is_leader": true,
      "last_heartbeat_at": "2026-01-15T10:35:00+08:00"
    },
    {
      "id": "nod_yyyyyyyy",
      "node_id": "node-b",
      "name": "server-b",
      "role": "edge,gateway",
      "status": "online",
      "hostname": "server-b",
      "public_ip": "{SERVER_B_IP}",
      "is_leader": false,
      "last_heartbeat_at": "2026-01-15T10:35:00+08:00"
    }
  ],
  "count": 2
}
```

查看详细节点信息：

```bash
curl -s ${AEGIS_BASE}/api/admin/v1/nodes/node-b -b cookies.txt | jq .
curl -s ${AEGIS_BASE}/api/admin/v1/nodes/node-b/health -b cookies.txt | jq .
```

### B.5 为 Server B 创建 Gateway 清单

Gateway 代表一个节点上的流量入口（Caddy listener），用于路由表生成。

```bash
# 为 Server B 创建 public gateway
curl -s -X POST ${AEGIS_BASE}/api/admin/v1/gateways \
  -H "Content-Type: application/json" \
  -d '{"node_id":"node-b","name":"server-b-public","type":"public","provider":"caddy","bind_addr":":80","host":"{SERVER_B_IP}","port":80,"scheme":"http","public_accessible":true,"private_accessible":false,"priority":10}' \
  -b cookies.txt | jq .
```

**预期响应（201 Created）：**
```json
{
  "gateway_id": "gw_xxxxxxxx",
  "node_id": "node-b",
  "name": "server-b-public",
  "type": "public",
  "provider": "caddy",
  "host": "{SERVER_B_IP}",
  "port": 80,
  "enabled": true,
  "status": "active"
}
```

### B.6 创建 Gateway Link（A 到 B 的跨节点链路）

Gateway Link 是 Aegis 控制平面到远程网关的认证通信链路。

```bash
curl -s -X POST ${AEGIS_BASE}/api/admin/v1/gateway-links \
  -H "Content-Type: application/json" \
  -d '{"name":"link-a-to-b","host":"{SERVER_B_IP}","port":80,"gateway_type":"upstream","auto_route":true}' \
  -b cookies.txt | jq .
```

**预期响应（201 Created）：**
```json
{
  "id": "glk_xxxxxxxx",
  "name": "link-a-to-b",
  "host": "{SERVER_B_IP}",
  "port": 80,
  "gateway_type": "upstream",
  "auto_route": true,
  "status": "active",
  "auth_type": "hmac-sha256",
  "secret": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "warning": "store this secret securely — it will not be shown again"
}
```
记录 `id` 为 `{GATEWAY_LINK_ID}`，`secret` 为 `{GATEWAY_LINK_SECRET}`。

### B.7 创建 Topology Edge

Topology Edge 描述两个节点之间的网络可达关系。

```bash
curl -s -X POST ${AEGIS_BASE}/api/admin/v1/topology/edges \
  -H "Content-Type: application/json" \
  -d '{"from_node_id":"node-a","to_node_id":"node-b","public_reachable":true,"gateway_link_id":"{GATEWAY_LINK_ID}"}' \
  -b cookies.txt | jq .
```

**预期响应（201 Created）：**
```json
{
  "id": "tpe_xxxxxxxx",
  "from_node_id": "node-a",
  "to_node_id": "node-b",
  "public_reachable": true,
  "private_reachable": false,
  "gateway_link_id": "glk_xxxxxxxx",
  "status": "active"
}
```

### B.8 在 Server B 上创建 Service + Endpoint

这些 Service 和 Endpoint 在 Server B 上运行，但由 Server A 控制平面管理。

```bash
# 创建 Service
curl -s -X POST ${AEGIS_BASE}/api/services \
  -H "Content-Type: application/json" \
  -d '{"project_id":"{PROJECT_ID}","name":"app-on-b","kind":"web","env":"production"}' \
  -b cookies.txt | jq .

# 记录返回的 id 为 {SERVICE_B_ID}

# 创建 Endpoint（绑定到 node-b）
curl -s -X POST ${AEGIS_BASE}/api/services/{SERVICE_B_ID}/endpoints \
  -H "Content-Type: application/json" \
  -d '{"type":"http","address":"127.0.0.1:3000","node_id":"node-b"}' \
  -b cookies.txt | jq .
```

### B.9 设置 Gateway Policy

为 Service 设置路由策略，指定它应由哪个 gateway 承载。

```bash
curl -s -X PUT ${AEGIS_BASE}/api/admin/v1/services/{SERVICE_B_ID}/gateway-policy \
  -H "Content-Type: application/json" \
  -d '{"mode":"manual","primary_gateway_id":"{SERVER_B_GATEWAY_ID}","allow_local":true,"allow_public":true}' \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "service_id": "svc_xxxxxxxx",
  "mode": "manual",
  "primary_gateway_id": "gw_xxxxxxxx",
  "allow_local": true,
  "allow_public": true,
  "source": "service"
}
```

### B.10 为 Server B 生成路由表

路由表决定了该节点上 Caddy 如何路由流量。

```bash
curl -s -X POST ${AEGIS_BASE}/api/admin/v1/nodes/node-b/routing-table/generate \
  -H "Content-Type: application/json" \
  -d '{"persist":true,"reason":"initial routing table for node-b"}' \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "node_id": "node-b",
  "revision": 1,
  "entries": [
    {
      "domain": "*.example.com",
      "target": "127.0.0.1:3000",
      "gateway": "server-b-public",
      "type": "local"
    }
  ],
  "warnings": [],
  "persisted": true,
  "validation": {
    "is_valid": true,
    "errors": []
  }
}
```

### B.11 预演路由

查看特定域名从指定节点的路由解析结果。

```bash
curl -s "${AEGIS_BASE}/api/admin/v1/routing/preview?from_node_id=node-a&domain=app-on-b.example.com" \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "from_node_id": "node-a",
  "domain": "app-on-b.example.com",
  "entries": [
    {
      "domain": "app-on-b.example.com",
      "target": "{SERVER_B_IP}:80",
      "gateway": "link-a-to-b",
      "type": "relay"
    }
  ],
  "count": 1
}
```

### B.12 验证跨节点 HTTP 请求

从 Server A 向 Server B 发起 HTTP 请求，验证流量能跨越节点。

```bash
# 在 Server A 上测试
ssh ubuntu@${SERVER_A_IP} "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 5 http://${SERVER_B_IP}:80/"

# 或通过 Aegis relay 端点（如果配置了 relay dispatch）
ssh ubuntu@${SERVER_A_IP} "curl -s -o /dev/null -w '%{http_code}' -H 'Host: app-on-b.example.com' http://127.0.0.1:80/"
```

### 场景 B 验证清单

- [ ] Server A 和 Server B 的节点状态均为 `online`
- [ ] Join Token 创建成功，Server B 节点代理启动并连接
- [ ] Gateway Link 创建成功，`auth_type` 为 `hmac-sha256`
- [ ] Topology Edge 创建成功，`public_reachable` 为 `true`
- [ ] Gateway Policy 设置成功
- [ ] 路由表生成无错误，validation.is_valid 为 true
- [ ] 路由预演能找到跨节点路由条目
- [ ] 跨节点 HTTP 请求返回预期响应码（2xx/4xx 均为成功连通）

---

## 场景 C：多服务策略路由（Multi-Service Policy Routing）

### 前置条件

- 单节点或多节点部署均可，以下以双节点为例
- 至少创建 2 个节点（`node-a` 作为控制平面，`node-b` 作为边缘节点）
- 3 个 Gateway 清单已创建：`local-gw`（本地）、`private-gw`（内网）、`public-gw`（公网）
- 已理解 Gateway Policy 模式：`auto`（自动选择）、`manual`（指定 primary + fallback）、`local_only`（仅本地）

### C.1 创建 3 个 Services

```bash
# App-A: 仅本地可访问的内部服务
curl -s -X POST ${AEGIS_BASE}/api/services \
  -H "Content-Type: application/json" \
  -d '{"project_id":"{PROJECT_ID}","name":"app-a","kind":"web","env":"production"}' \
  -b cookies.txt | jq '.id'

# App-B: 优先内网网关，可回落本地
curl -s -X POST ${AEGIS_BASE}/api/services \
  -H "Content-Type: application/json" \
  -d '{"project_id":"{PROJECT_ID}","name":"app-b","kind":"web","env":"production"}' \
  -b cookies.txt | jq '.id'

# App-C: 优先本地，可回落公网
curl -s -X POST ${AEGIS_BASE}/api/services \
  -H "Content-Type: application/json" \
  -d '{"project_id":"{PROJECT_ID}","name":"app-c","kind":"web","env":"production"}' \
  -b cookies.txt | jq '.id'
```
记录返回的三个 `id` 分别为 `{SVC_A_ID}`、`{SVC_B_ID}`、`{SVC_C_ID}`。

### C.2 创建 3 个 Gateways

```bash
# local-gw: 本地 loopback gateway，仅节点自身可访问
curl -s -X POST ${AEGIS_BASE}/api/admin/v1/gateways \
  -H "Content-Type: application/json" \
  -d '{"node_id":"node-a","name":"local-gw","type":"local","provider":"caddy","bind_addr":"127.0.0.1:80","host":"127.0.0.1","port":80,"scheme":"http","public_accessible":false,"private_accessible":false,"priority":10}' \
  -b cookies.txt | jq .

# private-gw: 内网 gateway，仅 VPC 内可达
curl -s -X POST ${AEGIS_BASE}/api/admin/v1/gateways \
  -H "Content-Type: application/json" \
  -d '{"node_id":"node-a","name":"private-gw","type":"private","provider":"caddy","bind_addr":"10.0.0.1:80","host":"10.0.0.1","port":80,"scheme":"http","public_accessible":false,"private_accessible":true,"priority":5}' \
  -b cookies.txt | jq .

# public-gw: 公网 gateway，对外暴露
curl -s -X POST ${AEGIS_BASE}/api/admin/v1/gateways \
  -H "Content-Type: application/json" \
  -d '{"node_id":"node-a","name":"public-gw","type":"public","provider":"caddy","bind_addr":":443","host":"{SERVER_A_IP}","port":443,"scheme":"https","public_accessible":true,"private_accessible":false,"priority":1}' \
  -b cookies.txt | jq .
```
记录返回的 gateway_id 分别为 `{LOCAL_GW_ID}`、`{PRIVATE_GW_ID}`、`{PUBLIC_GW_ID}`。

### C.3 设置差异化 Gateway Policy

```bash
# App-A: local-only — 仅通过本地 gateway 访问
curl -s -X PUT ${AEGIS_BASE}/api/admin/v1/services/{SVC_A_ID}/gateway-policy \
  -H "Content-Type: application/json" \
  -d '{"mode":"manual","primary_gateway_id":"{LOCAL_GW_ID}","allow_local":true,"allow_private":false,"allow_public":false}' \
  -b cookies.txt | jq .

# App-B: private-preferred — 优先内网，回落本地
curl -s -X PUT ${AEGIS_BASE}/api/admin/v1/services/{SVC_B_ID}/gateway-policy \
  -H "Content-Type: application/json" \
  -d '{"mode":"manual","primary_gateway_id":"{PRIVATE_GW_ID}","fallback_gateway_ids":["{LOCAL_GW_ID}"],"allow_local":true,"allow_private":true,"allow_public":false}' \
  -b cookies.txt | jq .

# App-C: public-fallback — 优先本地，可回落公网
curl -s -X PUT ${AEGIS_BASE}/api/admin/v1/services/{SVC_C_ID}/gateway-policy \
  -H "Content-Type: application/json" \
  -d '{"mode":"manual","primary_gateway_id":"{LOCAL_GW_ID}","fallback_gateway_ids":["{PUBLIC_GW_ID}"],"allow_local":true,"allow_public":true}' \
  -b cookies.txt | jq .
```

验证策略已生效：

```bash
curl -s ${AEGIS_BASE}/api/admin/v1/services/{SVC_A_ID}/gateway-policy -b cookies.txt | jq '.mode'
# 预期: "manual"

curl -s ${AEGIS_BASE}/api/admin/v1/services/{SVC_B_ID}/gateway-policy -b cookies.txt | jq '.fallback_gateway_ids'
# 预期: ["{LOCAL_GW_ID}"]
```

### C.4 为每个 Service 创建 Endpoints

```bash
# App-A endpoint (在 node-a 本地)
curl -s -X POST ${AEGIS_BASE}/api/services/{SVC_A_ID}/endpoints \
  -H "Content-Type: application/json" \
  -d '{"type":"http","address":"127.0.0.1:8081","node_id":"node-a"}' \
  -b cookies.txt | jq .

# App-B endpoint (在 node-b)
curl -s -X POST ${AEGIS_BASE}/api/services/{SVC_B_ID}/endpoints \
  -H "Content-Type: application/json" \
  -d '{"type":"http","address":"127.0.0.1:8082","node_id":"node-b"}' \
  -b cookies.txt | jq .

# App-C endpoints (本地 + 远程)
curl -s -X POST ${AEGIS_BASE}/api/services/{SVC_C_ID}/endpoints \
  -H "Content-Type: application/json" \
  -d '{"type":"http","address":"127.0.0.1:8083","node_id":"node-a"}' \
  -b cookies.txt | jq .
curl -s -X POST ${AEGIS_BASE}/api/services/{SVC_C_ID}/endpoints \
  -H "Content-Type: application/json" \
  -d '{"type":"http","address":"127.0.0.1:8083","node_id":"node-b"}' \
  -b cookies.txt | jq .
```

### C.5 生成路由表

```bash
# 为 node-a 生成路由表
curl -s -X POST ${AEGIS_BASE}/api/admin/v1/nodes/node-a/routing-table/generate \
  -H "Content-Type: application/json" \
  -d '{"persist":true,"reason":"multi-service policy routing"}' \
  -b cookies.txt | jq '{node_id, entries: [.entries[] | {domain, target, type, gateway}], warnings}'
```

**预期观察点：**
- `app-a` 的 entry.type 应为 `local`，仅出现在本地
- `app-b` 的 entry 应经过 private gateway relay
- `app-c` 的 entry 优先 local，带 fallback 到 public

### C.6 查看路由表

```bash
curl -s ${AEGIS_BASE}/api/admin/v1/nodes/node-a/routing-table -b cookies.txt | jq .
```

### C.7 预演路由

```bash
# 查询 app-b 从 node-a 的路由
curl -s "${AEGIS_BASE}/api/admin/v1/routing/preview?from_node_id=node-a&domain=app-b.example.com" \
  -b cookies.txt | jq .

# 查询 app-c 从 node-b 的路由
curl -s "${AEGIS_BASE}/api/admin/v1/routing/preview?from_node_id=node-b&domain=app-c.example.com" \
  -b cookies.txt | jq .
```

### C.8 验证路由

```bash
curl -s "${AEGIS_BASE}/api/admin/v1/routing/validate?from_node_id=node-a" \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "node_id": "node-a",
  "is_valid": true,
  "errors": [],
  "warnings": [],
  "entry_count": 3
}
```

### C.9 检查集群健康

```bash
curl -s ${AEGIS_BASE}/api/admin/v1/cluster/health -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "node_count": 2,
  "leader_node_id": "node-a",
  "split_brain": false,
  "nodes": [
    {
      "node_id": "node-a",
      "hostname": "server-a",
      "role": "leader,gateway",
      "status": "online",
      "is_leader": true,
      "sync_status": "in_sync",
      "desired_revision": 1,
      "applied_revision": 1
    },
    {
      "node_id": "node-b",
      "hostname": "server-b",
      "role": "edge,gateway",
      "status": "online",
      "is_leader": false,
      "sync_status": "in_sync",
      "desired_revision": 1,
      "applied_revision": 1
    }
  ],
  "overall_healthy": true,
  "issues": null
}
```

### C.10 查看拓扑矩阵

```bash
curl -s ${AEGIS_BASE}/api/admin/v1/topology/matrix -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "matrix": [
    {
      "id": "tpe_xxxxxxxx",
      "from_node_id": "node-a",
      "to_node_id": "node-b",
      "public_reachable": true,
      "gateway_link_id": "glk_xxxxxxxx",
      "status": "active"
    }
  ],
  "count": 1
}
```

### 场景 C 验证清单

- [ ] 3 个 Service 创建成功，3 个 Gateway 创建成功
- [ ] 3 种策略（local-only / private-preferred / public-fallback）各设置正确
- [ ] 路由表生成成功，无 warning
- [ ] 路由预演能找到对应 domain 的正确路由条目
- [ ] 路由验证 `is_valid` 为 `true`
- [ ] 集群健康 `overall_healthy` 为 `true`，`split_brain` 为 `false`
- [ ] 拓扑矩阵显示正确的节点连通关系

---

## 场景 D：Apply 故障自动回滚（Apply Failure + Auto-Rollback）

### 前置条件

- Aegis 已完成初始部署，至少有一次成功的 Apply（参见场景 A）
- Caddy `validate` 命令可用（`caddy validate --config /path/to/Caddyfile`）
- 理解 Aegis 的 Apply 流程：生成配置 -> 备份 -> 写入 -> 验证 -> reload -> 记录

### D.1 正常 Apply（建立 Baseline）

```bash
# 确保当前处于干净状态
curl -s -X POST ${AEGIS_BASE}/api/apply -b cookies.txt | jq '{route_count, warnings}'
```

**预期：** `route_count >= 1`，`warnings: []`

### D.2 记录当前配置

```bash
curl -s ${AEGIS_BASE}/api/config/current -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "config": "demo.example.com {\n  reverse_proxy 127.0.0.1:8080\n}\n"
}
```

### D.3 创建会导致验证失败的配置变更

制造一个故意错误的配置变更。例如：创建一个指向不存在后端的 route，或删除所有 enabled endpoint。

```bash
# 方法 1：创建一个指向不存在服务的 route（会导致 Caddy 配置引用未解析的上游）
curl -s -X POST ${AEGIS_BASE}/api/routes \
  -H "Content-Type: application/json" \
  -d '{"domain":"invalid.example.com","service_id":"svc_nonexistent"}' \
  -b cookies.txt | jq .

# 方法 2：禁用所有现有 endpoint（会导致 Caddy 生成空的 upstream）
# 先查看当前 endpoints
curl -s ${AEGIS_BASE}/api/services/{SERVICE_ID}/endpoints -b cookies.txt | jq '.[].id'
# 禁用每个 endpoint
curl -s -X POST ${AEGIS_BASE}/api/endpoints/{ENDPOINT_ID}/disable -b cookies.txt | jq .
```

### D.4 Dry-Run 预览（检查警告）

```bash
curl -s ${AEGIS_BASE}/api/config/preview -b cookies.txt | jq '{warnings, route_count}'
```

**预期：** 如果有警告，`warnings` 数组不为空。

也可以查看配置差异：

```bash
curl -s ${AEGIS_BASE}/api/config/diff -b cookies.txt | jq .
```

### D.5 执行 Apply（预期失败）

```bash
curl -s -X POST ${AEGIS_BASE}/api/apply \
  -b cookies.txt | jq .
```

**预期：** 如果 Caddy 验证失败，Apply 返回错误响应（HTTP 500）。

> **说明：** Aegis Apply 流程在 Caddy 验证失败时：
> 1. 记录验证错误信息
> 2. 自动将 Caddyfile 恢复为 Apply 前的备份版本
> 3. 执行 `caddy reload` 恢复旧配置
> 4. 在 Apply 历史中记录失败条目

### D.6 验证服务仍正常运行

```bash
# 确认当前配置与 Apply 前一致
curl -s ${AEGIS_BASE}/api/config/current -b cookies.txt | jq .
```

**预期：** `config` 内容与步骤 D.2 中记录的一致（自动回滚成功）。

### D.7 查看 Apply 历史（应有失败记录）

```bash
curl -s ${AEGIS_BASE}/api/apply/history -b cookies.txt | jq .
```

**预期响应：** 最近一条记录的 `status` 应为 `"failed"`：
```json
[
  {
    "id": "apply_003",
    "version": "v1-20260115T104000",
    "config_path": "/etc/caddy/Caddyfile",
    "backup_path": "/var/lib/aegis/backups/config/Caddyfile.20260115T104000.bak",
    "status": "failed",
    "message": "caddy validate failed: ...",
    "created_at": "2026-01-15T10:40:00+08:00"
  },
  ...
]
```

### D.8 手动回滚到指定版本

如果自动回滚未生效，或需要恢复到更早的版本：

```bash
# 查看历史，找到成功版本的 version 字段
curl -s ${AEGIS_BASE}/api/apply/history -b cookies.txt | jq '.[] | select(.status == "success") | .version'

# 手动回滚到指定版本
curl -s -X POST ${AEGIS_BASE}/api/rollback \
  -H "Content-Type: application/json" \
  -d '{"version":"v1-20260115T103010"}' \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{"status": "rolled_back"}
```

### 场景 D 验证清单

- [ ] 正常 Apply 成功（baseline 建立）
- [ ] 配置预览能显示警告信息
- [ ] 错误配置导致 Apply 返回错误（非 200）
- [ ] 当前配置自动恢复为 Apply 前的版本
- [ ] Apply 历史中有 `status: "failed"` 记录
- [ ] 手动回滚到历史版本成功

---

## 场景 E：数据库备份与恢复（DB Backup + Recovery）

### 前置条件

- Aegis 服务器正常运行
- SQLite 数据库路径已知（默认 `/var/lib/aegis/aegis.db`）
- 具有服务器文件系统访问权限（SSH 或本地终端）
- 备份目录已配置（默认 `/var/lib/aegis/backups/db/`）

### E.1 确认备份机制

Aegis 的 BackManager 定期自动备份 SQLite 数据库。

```bash
# 检查备份目录
ssh ubuntu@${AEGIS_HOST} "ls -la /var/lib/aegis/backups/db/ 2>/dev/null || echo 'Backup directory not found'"

# 查看 Aegis 日志中的备份记录
ssh ubuntu@${AEGIS_HOST} "grep -i backup /var/log/aegis*.log 2>/dev/null | tail -20"
```

**预期输出：** 备份目录下有 `.db` 文件，文件名包含时间戳，例如 `aegis-20260115-103000.db`。

### E.2 查看备份文件

```bash
# 列出备份文件，按时间排序
ssh ubuntu@${AEGIS_HOST} "ls -lht /var/lib/aegis/backups/db/ | head -20"

# 检查最新备份的完整性
ssh ubuntu@${AEGIS_HOST} "sqlite3 /var/lib/aegis/backups/db/aegis-20260115-103000.db 'PRAGMA integrity_check;'"
```

**预期：** `integrity_check` 返回 `ok`。

### E.3 创建手动备份（安全操作）

```bash
# 复制当前数据库文件
ssh ubuntu@${AEGIS_HOST} "cp /var/lib/aegis/aegis.db /var/lib/aegis/backups/db/manual-backup-$(date +%Y%m%d-%H%M%S).db"

# 验证手动备份
ssh ubuntu@${AEGIS_HOST} "ls -la /var/lib/aegis/backups/db/manual-backup-*.db"
```

### E.4 模拟灾难场景

```bash
# 停止 Aegis 服务
ssh ubuntu@${AEGIS_HOST} "sudo systemctl stop aegis || pkill -f 'aegis serve'"

# 确认服务已停止
ssh ubuntu@${AEGIS_HOST} "pgrep -f 'aegis serve' || echo 'Aegis stopped'"

# 移动（重命名）当前数据库文件模拟数据丢失
ssh ubuntu@${AEGIS_HOST} "mv /var/lib/aegis/aegis.db /var/lib/aegis/aegis.db.disaster-$(date +%Y%m%d-%H%M%S)"
```

### E.5 从最新备份恢复

```bash
# 查找最新备份
LATEST_BACKUP=$(ssh ubuntu@${AEGIS_HOST} "ls -t /var/lib/aegis/backups/db/*.db 2>/dev/null | head -1")
echo "Latest backup: ${LATEST_BACKUP}"

# 从备份恢复
ssh ubuntu@${AEGIS_HOST} "cp ${LATEST_BACKUP} /var/lib/aegis/aegis.db"

# 验证恢复后的数据库完整性
ssh ubuntu@${AEGIS_HOST} "sqlite3 /var/lib/aegis/aegis.db 'PRAGMA integrity_check;'"
```

**预期：** `integrity_check` 返回 `ok`。

### E.6 重启 Aegis 并验证数据完整

```bash
# 启动 Aegis
ssh ubuntu@${AEGIS_HOST} "sudo systemctl start aegis || nohup ./aegis serve --config /etc/aegis/aegis.yaml > /var/log/aegis.log 2>&1 &"

# 等待服务启动
sleep 3

# 检查系统状态
curl -s ${AEGIS_BASE}/api/system/status | jq '.counts'
```

### E.7 验证数据完整性

```bash
# 验证所有资源可正常访问
curl -s ${AEGIS_BASE}/api/services -b cookies.txt | jq '.[] | {id, name}'
curl -s ${AEGIS_BASE}/api/routes -b cookies.txt | jq '.[] | {id, domain}'

# 登录验证
curl -s -X POST ${AEGIS_BASE}/api/admin/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"{ADMIN_PASSWORD}"}' \
  -c cookies_restore.txt | jq .
```

### 场景 E 验证清单

- [ ] 备份目录存在且包含定期备份文件
- [ ] 备份文件 `integrity_check` 通过
- [ ] 手动备份成功创建
- [ ] Aegis 停止后数据库文件可被移动
- [ ] 从备份恢复后 `integrity_check` 通过
- [ ] Aegis 重启后服务正常
- [ ] 所有 API 可正常访问，数据无丢失（对比恢复前后的资源计数）

---

## 场景 F：端到端请求追踪（End-to-End Request Tracing）

### 前置条件

- 已完成场景 A（单节点基础部署）或场景 B（双节点部署）
- 至少有一条完整的路由链：domain -> route -> service -> endpoints
- 如果验证跨节点追踪，需要至少 2 个节点并配置 Gateway Link 和 Topology Edge
- 已登录管理后台（有效 session cookie）

### F.1 准备测试数据（创建完整路由链）

如果尚未建立，先创建一个完整的路由链。

```bash
# 创建 Project
PROJECT_ID=$(curl -s -X POST ${AEGIS_BASE}/api/projects \
  -H "Content-Type: application/json" \
  -d '{"name":"trace-demo","description":"追踪演示"}' \
  -b cookies.txt | jq -r '.id')

# 创建 Service（本地 + 远程 endpoint）
SERVICE_ID=$(curl -s -X POST ${AEGIS_BASE}/api/services \
  -H "Content-Type: application/json" \
  -d "{\"project_id\":\"${PROJECT_ID}\",\"name\":\"trace-svc\",\"kind\":\"web\",\"env\":\"staging\"}" \
  -b cookies.txt | jq -r '.id')

# 创建本地 Endpoint
curl -s -X POST ${AEGIS_BASE}/api/services/${SERVICE_ID}/endpoints \
  -H "Content-Type: application/json" \
  -d '{"type":"http","address":"127.0.0.1:9090","node_id":"node-a"}' \
  -b cookies.txt | jq -r '.id'

# 如有远程节点，创建远程 Endpoint
curl -s -X POST ${AEGIS_BASE}/api/services/${SERVICE_ID}/endpoints \
  -H "Content-Type: application/json" \
  -d '{"type":"http","address":"127.0.0.1:9090","node_id":"node-b"}' \
  -b cookies.txt | jq -r '.id'

# 创建 Route
ROUTE_ID=$(curl -s -X POST ${AEGIS_BASE}/api/routes \
  -H "Content-Type: application/json" \
  -d "{\"domain\":\"trace.example.com\",\"service_id\":\"${SERVICE_ID}\"}" \
  -b cookies.txt | jq -r '.id')

echo "SERVICE_ID=${SERVICE_ID} ROUTE_ID=${ROUTE_ID}"
```

### F.2 按域名追踪

从域名出发，追踪完整的路由解析链：域名 -> route -> service -> endpoints -> gateway -> 出口。

```bash
curl -s ${AEGIS_BASE}/api/admin/v1/trace/domain/trace.example.com \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "domain": "trace.example.com",
  "trace_status": "found",
  "route": {
    "id": "rte_xxxxxxxx",
    "domain": "trace.example.com",
    "service_id": "svc_xxxxxxxx",
    "status": "active"
  },
  "service": {
    "id": "svc_xxxxxxxx",
    "name": "trace-svc",
    "kind": "web"
  },
  "endpoints": [
    {
      "id": "ep_xxxxxxxx",
      "address": "127.0.0.1:9090",
      "node_id": "node-a",
      "enabled": true
    },
    {
      "id": "ep_yyyyyyyy",
      "address": "127.0.0.1:9090",
      "node_id": "node-b",
      "enabled": true
    }
  ],
  "hops": [...]
}
```

> **说明：** `trace_status` 为 `found` 表示域名已注册路由；`not_found` 表示该域名未在任何路由中注册（HTTP 404）。

### F.3 按路由 ID 追踪

```bash
curl -s ${AEGIS_BASE}/api/admin/v1/trace/route/{ROUTE_ID} \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：** 与 F.2 结构类似，但从 route ID 开始解析，不需要域名。

### F.4 按 SNI 追踪

SNI（Server Name Indication）追踪用于 TLS 场景，模拟 Caddy 收到 TLS ClientHello 时的路由查询。

```bash
curl -s ${AEGIS_BASE}/api/admin/v1/trace/sni/trace.example.com \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：** 结构与域名追踪类似，额外的 TLS 相关信息（如证书策略、TLS mode）。

### F.5 Egress 追踪

查询特定域名从指定节点的出口路径。

```bash
curl -s "${AEGIS_BASE}/api/admin/v1/trace/egress?domain=trace.example.com&from_node=node-a" \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "domain": "trace.example.com",
  "from_node": "node-a",
  "egress_path": [
    {
      "type": "gateway",
      "name": "local-gw",
      "host": "127.0.0.1:80"
    },
    {
      "type": "endpoint",
      "address": "127.0.0.1:9090"
    }
  ]
}
```

### F.6 安全检查（单路由）

对特定路由进行安全风险评估。

```bash
curl -s ${AEGIS_BASE}/api/admin/v1/routes/{ROUTE_ID}/safety \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "route_id": "rte_xxxxxxxx",
  "domain": "trace.example.com",
  "checks": [
    {"check": "endpoint_count", "status": "ok", "message": "has 2 endpoints"},
    {"check": "service_active", "status": "ok", "message": "service is active"},
    {"check": "tls_configured", "status": "warning", "message": "TLS not enabled for this route"},
    {"check": "exposed_on_public_gateway", "status": "warning", "message": "route may be publicly accessible"}
  ],
  "overall": "warning"
}
```

### F.7 检查所有路由安全

```bash
curl -s ${AEGIS_BASE}/api/admin/v1/routes/safety \
  -b cookies.txt | jq '.[] | {route_id, domain, overall}'
```

**预期响应（200 OK）：** 返回所有路由的安全检查结果数组。

### F.8 Relay 解析

查询中继路由解析，用于诊断跨节点流量路径。

```bash
curl -s "${AEGIS_BASE}/api/admin/v1/relay/resolve?domain=trace.example.com&from_node=node-a" \
  -b cookies.txt | jq .
```

**预期响应（200 OK）：** 返回从指定节点到目标域名的 relay 路由信息。

### F.9 查看审计日志

审计日志记录所有管理操作，用于安全审计和问题排查。

```bash
curl -s ${AEGIS_BASE}/api/admin/v1/audit-logs -b cookies.txt | jq .
```

**预期响应（200 OK）：**
```json
{
  "audit_logs": [
    {
      "id": "aud_xxxxxxxx",
      "action": "route.create",
      "resource_type": "route",
      "resource_id": "rte_xxxxxxxx",
      "status": "success",
      "detail": "route created via admin CRUD",
      "created_at": "2026-01-15T11:00:00+08:00"
    },
    ...
  ],
  "count": 25
}
```

### F.10 验证 X-Request-ID 贯穿全链路

Aegis 在每个请求中注入 `X-Request-ID` header，用于端到端追踪。检查方式：

```bash
# 登录请求，查看响应 header
curl -s -D - -X POST ${AEGIS_BASE}/api/admin/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"{ADMIN_PASSWORD}"}' \
  -o /dev/null 2>&1 | grep -i x-request-id

# 或者检查 Aegis 日志中的 request ID
ssh ubuntu@${AEGIS_HOST} "grep 'X-Request-ID\|request_id' /var/log/aegis*.log 2>/dev/null | tail -10"
```

### 场景 F 验证清单

- [ ] 域名追踪返回完整链路（domain -> route -> service -> endpoints）
- [ ] 路由追踪与域名追踪结果一致
- [ ] SNI 追踪正常返回
- [ ] Egress 追踪显示正确的出口路径
- [ ] 路由安全检查覆盖全部检查项
- [ ] 所有路由安全检查无 Critical 级别问题
- [ ] Relay 解析返回有效的 relay 路由（跨节点场景）
- [ ] 审计日志包含最近的操作记录
- [ ] 请求中携带 X-Request-ID 用于全链路追踪

---

## 附录 A：常用诊断命令速查

| 操作 | 命令 |
|------|------|
| 系统状态 | `GET /api/system/status` |
| 管理系统概览 | `GET /api/admin/v1/system/overview` |
| 配置预览 | `GET /api/config/preview` |
| 配置差异 | `GET /api/config/diff` |
| Dry-run Apply | `POST /api/apply/dry-run` |
| Apply | `POST /api/apply` |
| 回滚 | `POST /api/rollback` `{"version": "..."}` |
| Apply 历史 | `GET /api/apply/history` |
| 健康检查 | `GET /api/health` |
| 执行全部健康检查 | `POST /api/health/check-all` |
| 单个 Service 健康 | `GET /api/health/services/{id}` |
| 集群健康 | `GET /api/admin/v1/cluster/health` |
| 节点列表 | `GET /api/admin/v1/nodes` |
| 节点详情 | `GET /api/admin/v1/nodes/{id}` |
| 节点健康 | `GET /api/admin/v1/nodes/{id}/health` |
| Gateway 列表 | `GET /api/admin/v1/gateways` |
| Gateway Link 列表 | `GET /api/admin/v1/gateway-links` |
| 拓扑矩阵 | `GET /api/admin/v1/topology/matrix` |
| 拓扑路径 | `GET /api/admin/v1/topology/path?from=X&to=Y` |
| 路由验证 | `GET /api/admin/v1/routing/validate?from_node_id=X` |
| 审计日志 | `GET /api/admin/v1/audit-logs` |
| 操作日志 | `GET /api/admin/v1/operations` |
| Apply 日志 | `GET /api/admin/v1/apply-logs` |
| 节点事件 | `GET /api/admin/v1/node-events` |
| Doctor 诊断 | `POST /api/admin/v1/system/doctor` |
| System Verify | `POST /api/admin/v1/system/verify` |
| 诊断导出 | `GET /api/diagnostics/export` |
| Provider 诊断 | `POST /api/admin/v1/providers/diagnose` |

## 附录 B：常见 HTTP 状态码

| 状态码 | 含义 | 常见原因 |
|--------|------|----------|
| 200 | 成功 | 正常响应 |
| 201 | 已创建 | POST 创建资源成功 |
| 400 | 请求错误 | 参数缺失或格式错误 |
| 401 | 未认证 | session 过期或无效，重新登录 |
| 403 | 禁止 | Service API Key 访问 Admin API |
| 404 | 未找到 | 资源不存在或域名未注册路由 |
| 409 | 冲突 | 资源名称重复 |
| 422 | 无法处理 | 业务逻辑拒绝（如 archive 已有 route 的 project）|
| 423 | 锁定 | 有另一个 Apply 正在执行 |
| 429 | 频率限制 | 登录尝试过多（5次/分钟/IP）|
| 500 | 服务器错误 | 内部错误，检查 Aegis 日志 |

## 附录 C：Session 与认证说明

- Admin 登录使用 `POST /api/admin/v1/auth/login`，成功后设置 cookie `aegis_admin_session`
- Session 有效期默认 12 小时
- `Secure` flag 在开发环境默认为 `false`，生产环境设为 `true`（仅 HTTPS 传输 cookie）
- 登出使用 `POST /api/admin/v1/auth/logout`
- Service API Key 通过 `POST /api/admin/v1/scopes/{id}/api-keys` 创建
- Service API Key **不能**访问 `/api/admin/v1/*` 路径（由 `isSystemRoute()` 拦截）
- Gateway Link 使用 HMAC-SHA256 认证，带 5 分钟时间戳重放保护
