# Aegis 功能清单 v1.8L

> 每个功能标注：**状态**（✅ 可用 / ⚠️ 部分 / 🧪 原型 / ❌ 未接入）、**类型**（📖 查看 / ✏️ 操作）、**页面**、**API 端点**

---

## 一、流量管理

### 1.1 路由 (Route)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 1 | 查看路由列表 | ✅ | 📖 | Routes | `GET /api/admin/v1/routes` |
| 2 | 创建路由 | ❌ | ✏️ | — | `POST /api/routes` |
| 3 | 查看路由详情 | ✅ | 📖 | RouteDetail | `GET /api/routes/{id}` |
| 4 | 更新路由 | ❌ | ✏️ | — | `PATCH /api/routes/{id}` |
| 5 | 启用路由 | ❌ | ✏️ | — | `POST /api/routes/{id}/enable` |
| 6 | 禁用路由 | ❌ | ✏️ | — | `POST /api/routes/{id}/disable` |
| 7 | 切换路由目标服务 | ❌ | ✏️ | — | `POST /api/routes/{id}/switch-service` |
| 8 | 开启维护模式 | ❌ | ✏️ | — | `POST /api/routes/{id}/maintenance-on` |
| 9 | 关闭维护模式 | ❌ | ✏️ | — | `POST /api/routes/{id}/maintenance-off` |
| 10 | 删除路由 | ❌ | ✏️ | — | 无直接端点（通过 disable + delete domain action） |
| 11 | 路由安全检测 | ✅ | 📖 | Safety | `GET /api/admin/v1/routes/{id}/safety` |
| 12 | 全部路由安全检测 | ✅ | 📖 | Safety | `GET /api/admin/v1/routes/safety` |
| 13 | 路由网关策略 | ✅ | 📖 | GatewayPolicies | `GET /api/admin/v1/routes/{id}/gateway-policy` |
| 14 | 设置路由网关策略 | ❌ | ✏️ | — | `PUT /api/admin/v1/routes/{id}/gateway-policy` |
| 15 | 查看"我的路由" | ❌ | 📖 | — | `GET /api/v1/my/routes` |

### 1.2 服务 (Service)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 16 | 查看服务列表 | ✅ | 📖 | Services | `GET /api/admin/v1/services` |
| 17 | 创建服务 | ❌ | ✏️ | — | `POST /api/services` |
| 18 | 查看服务详情 | ✅ | 📖 | ServiceDetail | `GET /api/services/{id}` |
| 19 | 更新服务 | ❌ | ✏️ | — | `PATCH /api/services/{id}` |
| 20 | 启用服务 | ❌ | ✏️ | — | `POST /api/services/{id}/enable` |
| 21 | 禁用服务 | ❌ | ✏️ | — | `POST /api/services/{id}/disable` |
| 22 | 查看服务端点 | ✅ | 📖 | ServiceDetail | `GET /api/services/{id}/endpoints` |
| 23 | 服务网关策略 | ✅ | 📖 | GatewayPolicies | `GET /api/admin/v1/services/{id}/gateway-policy` |
| 24 | 设置服务网关策略 | ❌ | ✏️ | — | `PUT /api/admin/v1/services/{id}/gateway-policy` |
| 25 | 查看"我的服务" | ❌ | 📖 | — | `GET /api/v1/my/services` |

### 1.3 端点 (Endpoint)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 26 | 查看端点列表 | ✅ | 📖 | Endpoints | `GET /api/services/{id}/endpoints` (循环) |
| 27 | 创建端点 | ❌ | ✏️ | — | `POST /api/services/{id}/endpoints` |
| 28 | 更新端点 | ❌ | ✏️ | — | `PATCH /api/endpoints/{id}` |
| 29 | 启用端点 | ❌ | ✏️ | — | `POST /api/endpoints/{id}/enable` |
| 30 | 禁用端点 | ❌ | ✏️ | — | `POST /api/endpoints/{id}/disable` |
| 31 | 删除端点 | ❌ | ✏️ | — | `DELETE /api/endpoints/{id}` |

### 1.4 配置下发 (Apply)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 32 | 推送配置 | ✅ | ✏️ | ApplyConfig | `POST /api/apply` |
| 33 | Dry-run 预览 | ✅ | ✏️ | ApplyConfig | `POST /api/apply/dry-run` |
| 34 | 回滚配置 | ✅ | ✏️ | ApplyConfig | `POST /api/rollback` |
| 35 | 查看当前配置 | ✅ | 📖 | Config | `GET /api/config/current` |
| 36 | 查看预览配置 | ✅ | 📖 | Config | `GET /api/config/preview` |
| 37 | 查看配置 Diff | ✅ | 📖 | — | `GET /api/config/diff` |
| 38 | 查看 Apply 历史 | ✅ | 📖 | ApplyConfig | `GET /api/apply/history` |
| 39 | 系统 Apply（管理员） | ✅ | ✏️ | — | `POST /api/admin/v1/system/apply` |

### 1.5 端口暴露 (Exposure)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 40 | 查看暴露列表 | ✅ | 📖 | Exposures | `GET /api/exposures` |
| 41 | 创建暴露 | ✅ | ✏️ | Exposures | `POST /api/exposures` |
| 42 | 查看暴露详情 | ✅ | 📖 | — | `GET /api/exposures/{id}` |
| 43 | 更新暴露 | ✅ | ✏️ | — | `PATCH /api/exposures/{id}` |
| 44 | 激活暴露 | ✅ | ✏️ | Exposures | `POST /api/exposures/{id}/activate` |
| 45 | 停用暴露 | ✅ | ✏️ | Exposures | `POST /api/exposures/{id}/disable` |

### 1.6 快速创建 (Quick Create)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 46 | 一键创建域名映射 | ✅ | ✏️ | QuickCreate | `POST /api/v1/actions/bind-http-domain` |
| 47 | 一键创建 TLS 后端 | ✅ | ✏️ | — | `POST /api/v1/actions/bind-tls-backend` |
| 48 | 更新目标 | ✅ | ✏️ | — | `PATCH /api/v1/actions/update-target` |
| 49 | 禁用域名 | ✅ | ✏️ | — | `POST /api/v1/actions/disable-domain` |
| 50 | 删除域名 | ✅ | ✏️ | — | `DELETE /api/v1/actions/domain` |

### 1.7 Caddyfile 导入

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 51 | 预览 Caddyfile 导入 | ✅ | 📖 | Import | `GET /api/admin/v1/import/caddy/preview` |
| 52 | 确认 Caddyfile 导入 | ✅ | ✏️ | Import | `POST /api/admin/v1/import/caddy/confirm` |

---

## 二、基础设施

### 2.1 节点 (Node)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 53 | 查看节点列表 | ✅ | 📖 | Nodes | `GET /api/admin/v1/nodes` |
| 54 | 查看节点详情 | ✅ | 📖 | NodeDetail | `GET /api/admin/v1/nodes/{id}` |
| 55 | 查看节点健康 | ✅ | 📖 | NodeDetail | `GET /api/admin/v1/nodes/{id}/health` |
| 56 | 查看节点能力 | ✅ | 📖 | NodeDetail | `GET /api/admin/v1/nodes/{id}/capabilities` |
| 57 | 刷新节点能力 | ✅ | ✏️ | — | `POST /api/admin/v1/nodes/{id}/refresh-capabilities` |
| 58 | 查看节点网关 | ✅ | 📖 | NodeDetail | `GET /api/admin/v1/nodes/{id}/gateways` |
| 59 | 触发节点更新 | ✅ | ✏️ | Nodes, NodeDetail | `POST /api/admin/v1/nodes/{id}/update` |
| 60 | 远程部署节点 | ✅ | ✏️ | Nodes | `POST /api/admin/v1/nodes/deploy` |
| 61 | 上传更新二进制 | ✅ | ✏️ | — | `POST /api/admin/v1/system/upload-binary` |
| 62 | 查看二进制信息 | ✅ | 📖 | — | `GET /api/admin/v1/system/binary-info` |
| 63 | 查看待更新列表 | ✅ | 📖 | — | `GET /api/admin/v1/system/pending-updates` |
| 64 | 查看节点期望状态 | ❌ | 📖 | — | `GET /api/admin/v1/nodes/{id}/desired-state` |
| 65 | 创建节点期望状态 | ❌ | ✏️ | — | `POST /api/admin/v1/nodes/{id}/desired-state` |
| 66 | 查看节点实际状态 | ❌ | 📖 | — | `GET /api/admin/v1/nodes/{id}/actual-state` |
| 67 | 查看节点同步状态 | ✅ | 📖 | SyncStatus | `GET /api/admin/v1/nodes/{id}/sync-status` |

### 2.2 加入令牌 (Join Token)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 68 | 查看令牌列表 | ✅ | 📖 | JoinTokens | `GET /api/admin/v1/node-join-tokens` |
| 69 | 创建加入令牌 | ✅ | ✏️ | JoinTokens | `POST /api/admin/v1/node-join-tokens` |
| 70 | 撤销加入令牌 | ✅ | ✏️ | JoinTokens | `POST /api/admin/v1/node-join-tokens/{id}/revoke` |

### 2.3 网关 (Gateway)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 71 | 查看网关列表 | ✅ | 📖 | Gateways | `GET /api/admin/v1/gateways` |
| 72 | 创建网关 | ✅ | ✏️ | — | `POST /api/admin/v1/gateways` |
| 73 | 查看网关详情 | ✅ | 📖 | GatewayDetail | `GET /api/admin/v1/gateways/{id}` |
| 74 | 更新网关 | ✅ | ✏️ | Gateways | `PATCH /api/admin/v1/gateways/{id}` |
| 75 | 查看网关域名 | ✅ | 📖 | — | `GET /api/admin/v1/gateway/domains` |
| 76 | 创建网关域名 | ❌ | ✏️ | — | `POST /api/admin/v1/gateway/domains` |
| 77 | 附加网关路由 | ❌ | ✏️ | — | `POST /api/admin/v1/gateway/routes` |
| 78 | 分离网关路由 | ❌ | ✏️ | — | `DELETE /api/admin/v1/gateway/routes/{id}` |
| 79 | 查看网关监听器 | ✅ | 📖 | — | `GET /api/admin/v1/gateway/listeners` |
| 80 | 更新 TLS 策略 | ❌ | ✏️ | — | `PUT /api/admin/v1/gateway/domains/{id}/tls` |

### 2.4 网关链路 (Gateway Link)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 81 | 查看链路列表 | ✅ | 📖 | GatewayLinks | `GET /api/admin/v1/gateway-links` |
| 82 | 创建链路 | ✅ | ✏️ | GatewayLinks | `POST /api/admin/v1/gateway-links` |
| 83 | 查看链路详情 | ✅ | 📖 | GatewayLinkDetail | `GET /api/admin/v1/gateway-links/{id}` |
| 84 | 删除链路 | ✅ | ✏️ | GatewayLinks | `DELETE /api/admin/v1/gateway-links/{id}` |
| 85 | 轮换链路密钥 | ✅ | ✏️ | GatewayLinks, Detail | `POST /api/admin/v1/gateway-links/{id}/rotate` |

### 2.5 拓扑 (Topology)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 86 | 查看拓扑矩阵 | ✅ | 📖 | Topology | `GET /api/admin/v1/topology/matrix` |
| 87 | 查看拓扑路径 | ✅ | 📖 | TopologyPath | `GET /api/admin/v1/topology/path` |
| 88 | 创建拓扑边 | ✅ | ✏️ | — | `POST /api/admin/v1/topology/edges` |
| 89 | 更新拓扑边 | ✅ | ✏️ | — | `PATCH /api/admin/v1/topology/edges/{id}` |

### 2.6 路由表 (Routing Table)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 90 | 查看节点路由表 | ✅ | 📖 | RoutingTable | `GET /api/admin/v1/nodes/{id}/routing-table` |
| 91 | 生成路由表 | ✅ | ✏️ | RoutingTable | `POST /api/admin/v1/nodes/{id}/routing-table/generate` |
| 92 | 预览路由 | ✅ | 📖 | RoutingTable | `GET /api/admin/v1/routing/preview` |
| 93 | 验证路由 | ✅ | 📖 | RoutingTable | `GET /api/admin/v1/routing/validate` |

### 2.7 本地网关 (Local Gateway)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 94 | 查看本地网关运行时 | ✅ | 📖 | LocalGatewayRuntime | 无专用 API（从 nodes + gateways 推导） |

### 2.8 监听器 (Listener)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 95 | 查看监听器列表 | ✅ | 📖 | Listeners | 无专用 API（从 gateways 推导） |

### 2.9 中间件 (Middleware/Provider)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 96 | 查看提供者列表 | ✅ | 📖 | Providers | `GET /api/admin/v1/providers` |
| 97 | 诊断所有提供者 | ✅ | ✏️ | Providers, Middleware | `POST /api/admin/v1/providers/diagnose` |
| 98 | 安装提供者 | ✅ | ✏️ | Middleware | `POST /api/admin/v1/providers/{provider}/install` |
| 99 | 查看提供者配置 | ✅ | 📖 | Middleware | `GET /api/admin/v1/providers/{provider}/config` |
| 100 | 保存提供者配置 | ✅ | ✏️ | Middleware | `PUT /api/admin/v1/providers/{provider}/config` |
| 101 | 重载提供者 | ✅ | ✏️ | Middleware | `POST /api/admin/v1/providers/{provider}/reload` |
| 102 | 提供者服务控制 | ✅ | ✏️ | Middleware | `POST /api/admin/v1/providers/{provider}/service` |
| 103 | 卸载提供者 | ✅ | ✏️ | Middleware | `DELETE /api/admin/v1/providers/{provider}` |

---

## 三、可观测性

### 3.1 仪表盘 (Dashboard)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 104 | 系统总览 | ✅ | 📖 | Dashboard | `GET /api/admin/v1/system/overview` |
| 105 | 系统状态 | ✅ | 📖 | (多处使用) | `GET /api/system/status` |
| 106 | 部署流水线 | ✅ | 📖 | Dashboard | (前端组件，纯展示) |

### 3.2 健康检查 (Health)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 107 | 系统健康指标 | ✅ | 📖 | HealthCheck | `GET /api/admin/v1/system/health` |
| 108 | 存活探针 | ✅ | 📖 | — | `GET /api/healthz` |
| 109 | 就绪探针 | ✅ | 📖 | — | `GET /api/readyz` |
| 110 | 查看健康结果 | ✅ | 📖 | HealthCheck | `GET /api/health` |
| 111 | 执行全部健康检查 | ✅ | ✏️ | HealthCheck | `POST /api/health/check-all` |
| 112 | 查看服务健康 | ✅ | 📖 | — | `GET /api/health/services/{id}` |
| 113 | 端口冲突扫描 | ✅ | 📖 | HealthCheck | `GET /api/admin/v1/ports/scan` |
| 114 | 集群健康 | ✅ | 📖 | — | `GET /api/admin/v1/cluster/health` |

### 3.3 追踪 (Trace)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 115 | 域名追踪 (Ingress) | ✅ | 📖 | Trace | `GET /api/admin/v1/trace/domain/{domain}` |
| 116 | SNI 追踪 | ✅ | 📖 | — | `GET /api/admin/v1/trace/sni/{sni_host}` |
| 117 | 路由追踪 | ✅ | 📖 | — | `GET /api/admin/v1/trace/route/{route_id}` |
| 118 | 出站追踪 (Egress) | ✅ | 📖 | Trace | `GET /api/admin/v1/trace/egress` |

### 3.4 安全检测 (Safety)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 119 | 全部路由安全检测 | ✅ | 📖 | Safety | `GET /api/admin/v1/routes/safety` |
| 120 | 单条路由安全检测 | ✅ | 📖 | — | `GET /api/admin/v1/routes/{id}/safety` |

### 3.5 中继 (Relay)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 121 | 中继解析 | ✅ | 📖 | Relay | `GET /api/admin/v1/relay/resolve` |
| 122 | 中继追踪 | ✅ | 📖 | Relay | `GET /api/admin/v1/trace/domain/{domain}` (复用) |

### 3.6 日志 (Logs)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 123 | 查看操作日志 | ✅ | 📖 | Logs | `GET /api/admin/v1/operations` |
| 124 | 查看审计日志 | ✅ | 📖 | Logs | `GET /api/admin/v1/audit-logs` |
| 125 | 查看 Apply 日志 | ✅ | 📖 | — | `GET /api/admin/v1/apply-logs` |
| 126 | 查看节点事件 | ✅ | 📖 | — | `GET /api/admin/v1/node-events` |
| 127 | 原始日志 | ✅ | 📖 | — | `GET /api/logs` |

### 3.7 诊断 & 验收

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 128 | 运行系统诊断 | ✅ | ✏️ | Doctor | `POST /api/admin/v1/system/doctor` |
| 129 | 系统验证 | ✅ | ✏️ | — | `POST /api/admin/v1/system/verify` |
| 130 | 冒烟测试 | ✅ | ✏️ | Smoke | `POST /api/admin/v1/system/doctor` (复用) |
| 131 | 验收状态 | ✅ | 📖 | Acceptance | `GET /api/system/status` + `GET /api/admin/v1/routes/safety` |
| 132 | 导出诊断数据 | ✅ | 📖 | — | `GET /api/admin/v1/diagnostics/export` |

### 3.8 同步状态 (Sync)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 133 | 查看节点同步状态 | ✅ | 📖 | SyncStatus | `GET /api/admin/v1/nodes/{id}/sync-status` (循环) |

---

## 四、安全 & 访问控制

### 4.1 认证 (Auth)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 134 | 管理员登录 | ✅ | ✏️ | Login | `POST /api/admin/v1/auth/login` |
| 135 | 管理员登出 | ✅ | ✏️ | (Sidebar) | `POST /api/admin/v1/auth/logout` |
| 136 | 查看当前用户 | ✅ | 📖 | (AppLayout) | `GET /api/admin/v1/auth/me` |
| 137 | 修改密码 | ✅ | ✏️ | Settings | `POST /api/admin/v1/auth/change-password` |

### 4.2 Scope & API 密钥

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 138 | 查看 Scope 列表 | ✅ | 📖 | Scopes | `GET /api/admin/v1/scopes` |
| 139 | 创建 Scope | ✅ | ✏️ | Scopes | `POST /api/admin/v1/scopes` |
| 140 | 查看 API 密钥列表 | ✅ | 📖 | ApiKeys | `GET /api/admin/v1/api-keys` |
| 141 | 创建 API 密钥 | ✅ | ✏️ | ApiKeys | `POST /api/admin/v1/scopes/{id}/api-keys` |
| 142 | 撤销 API 密钥 | ✅ | ✏️ | ApiKeys | `POST /api/admin/v1/api-keys/{id}/revoke` |
| 143 | 轮换 API 密钥 | ✅ | ✏️ | — | `POST /api/admin/v1/api-keys/{id}/rotate` |

### 4.3 凭据 (Credential)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 144 | 查看凭据列表 | ✅ | 📖 | Credentials | `GET /api/admin/v1/credentials` |
| 145 | 创建凭据 | ✅ | ✏️ | Credentials | `POST /api/admin/v1/credentials` |
| 146 | 查看凭据详情 | ✅ | 📖 | — | `GET /api/admin/v1/credentials/{id}` |
| 147 | 删除凭据 | ✅ | ✏️ | Credentials | `DELETE /api/admin/v1/credentials/{id}` |
| 148 | 轮换凭据 | ✅ | ✏️ | — | `POST /api/admin/v1/credentials/{id}/rotate` |
| 149 | 解析凭据别名 | ✅ | 📖 | — | `GET /api/admin/v1/credentials/resolve` |
| 150 | 揭示凭据（仅一次） | ✅ | ✏️ | — | `POST /api/admin/v1/credentials/{id}/reveal` |

---

## 五、系统设置

### 5.1 面板配置

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 151 | 查看系统设置 | ✅ | 📖 | Settings | `GET /api/settings` |
| 152 | 修改面板域名 | ✅ | ✏️ | Settings | `PATCH /api/admin/v1/settings` |
| 153 | 修改 Let's Encrypt 邮箱 | ✅ | ✏️ | Settings | `PATCH /api/admin/v1/settings` |
| 154 | 上传自定义 TLS 证书 | ✅ | ✏️ | Settings | `PATCH /api/admin/v1/settings` |
| 155 | 修改密码 | ✅ | ✏️ | Settings | 同 #137 |

### 5.2 DNS 解析器

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 156 | 查看 DNS 状态 | ✅ | 📖 | Settings | `GET /api/admin/v1/dns/status` |
| 157 | 启用 DNS | ✅ | ✏️ | Settings | `POST /api/admin/v1/dns/enable` |
| 158 | 停用 DNS | ✅ | ✏️ | Settings | `POST /api/admin/v1/dns/disable` |
| 159 | 刷新 DNS 记录 | ✅ | ✏️ | Settings | `POST /api/admin/v1/dns/refresh` |

### 5.3 透明代理 (Transparent Proxy)

| # | 功能 | 状态 | 类型 | 页面 | API |
|---|------|:--:|:--:|------|-----|
| 160 | 查看透明代理规则 | ✅ | 📖 | TransparentProxy | `GET /api/admin/v1/transparent/rules` |
| 161 | 删除透明代理规则 | ✅ | ✏️ | TransparentProxy | `DELETE /api/admin/v1/transparent/rules/{id}` |

---

## 六、原型/占位（未实际工作）

| # | 页面 | 状态 | 说明 |
|---|------|:--:|------|
| 162 | SecurityPage | 🧪 | 静态信息页，无 API 调用 |
| 163 | MaintenancePage | 🧪 | 操作项全部 disabled，无 API |
| 164 | ActionsPage | 🧪 | 纯静态卡片，mock 数据 |

---

## 七、后端有 API 但前端未接入

| # | 端点 | 说明 |
|---|------|------|
| 165 | `GET/POST /api/projects` | 项目 CRUD（CLI 时代遗留） |
| 166 | `GET/POST /api/managed-domains` | 受管域名（DNS TXT 验证流程） |
| 167 | `POST /api/admin/v1/deployments` | 部署版本管理 |
| 168 | `GET /api/admin/v1/node-events` | 节点事件流 |
| 169 | `GET /api/admin/v1/edge-rules` | SNI 路由规则 |
| 170 | `GET /api/v1/my/edge-rules` | 我的 SNI 规则 |
| 171 | `GET /api/v1/my/operations` | 我的操作记录 |
| 172 | `GET /api/admin/v1/nodes/{id}/desired-state` | 节点期望状态 |
| 173 | `POST /api/admin/v1/nodes/{id}/desired-state` | 创建期望状态 |
| 174 | `GET /api/admin/v1/nodes/{id}/actual-state` | 节点实际状态 |

---

## 统计

| 类别 | 总数 | ✅ 可用 | ⚠️ 部分 | 🧪 原型 | ❌ 未接入 |
|------|:--:|:--:|:--:|:--:|:--:|
| 流量管理 | 52 | 38 | 0 | 0 | 14 |
| 基础设施 | 52 | 44 | 0 | 0 | 8 |
| 可观测性 | 30 | 30 | 0 | 0 | 0 |
| 安全 & 访问 | 17 | 17 | 0 | 0 | 0 |
| 系统设置 | 11 | 11 | 0 | 0 | 0 |
| 原型占位 | 3 | 0 | 0 | 3 | 0 |
| 后端未接入 | 10 | 0 | 0 | 0 | 10 |
| **合计** | **175** | **140** | **0** | **3** | **32** |
