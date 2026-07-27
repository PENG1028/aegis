# Aegis

**基础设施入口控制工具 — v1.7AD**

Aegis 管理多项目的基础设施入口：服务注册、域名路由、反向代理配置生成、健康检查、Apply / Rollback、受管域名、Endpoint 解析、Gateway Link（跨网关认证）、EdgeMux（SNI 透传）、TCP 代理、Provider 诊断。

## Aegis 是什么

- 一个管理跨项目服务入口的 CLI / API 工具
- 一个安全的配置下发器（校验 → 备份 → 替换 → 重载 → 审计）
- 一个后端服务健康检查器
- 一个受管域名 DNS 验证与生命周期管理工具

## Aegis 不是什么（暂不实现）

- 完整 PaaS 平台
- 认证 / 授权系统
- 灰度 / 金丝雀发布平台
- Service Mesh
- 多节点分布式网关
- Docker / 数据库部署管理
- 开放代理平台

## 架构

```
CLI (Cobra) ────────────┐
                         ├── Application Services ── Domain Logic ── Repository ── SQLite
HTTP API (130+ routes) ─┘        │
                                 ├── EndpointResolver (local → private → public → fail)
                                 ├── ProxyAdapter (Caddy, HAProxy, 预留 Nginx)
                                 ├── EdgeMux (HAProxy SNI passthrough → Caddy TLS)
                                 ├── Gateway Link (HMAC-SHA256 cross-gateway auth)
                                 ├── AuthMiddleware (Bearer Token + Admin Session)
                                 └── Rate Limiter (login brute-force protection)
```

- **CLI** 只解析参数、调用 AppService、格式化输出——不含业务逻辑
- **HTTP API** 复用同一批 Application Services
- **ProxyAdapter** 抽象代理后端——当前 Caddy，未来可接入 Nginx
- **EndpointResolver** 固定解析规则：local → private → public → fail

## 环境要求

- Go 1.22+
- Caddy（可选——`apply --dry-run` 和 `validate` 无 Caddy 也能工作）
- SQLite（嵌入式，无需外部数据库）

## 安装

```bash
git clone <repo-url> aegis
cd aegis
go build -o aegis ./cmd/aegis/
```

## 快速开始

### 1. 初始化

```bash
./aegis init
```

创建：
- `.aegis/config.yaml` — 配置文件
- `.aegis/aegis.db` — SQLite 数据库（20+ 张表）
- `.aegis/backups/` — 配置备份目录
- `.aegis/Caddyfile` — 被管理的 Caddy 配置

### 2. 创建项目

```bash
./aegis project create policy-page --description "策略管理应用"
```

### 3. 添加服务

```bash
./aegis service add policy-web \
  --project policy-page \
  --env prod \
  --kind http
```

注意：Service 不直接绑定 upstream 地址，地址通过 Endpoint 管理。

### 4. 添加 Endpoint

```bash
# 本地端点
./aegis endpoint add policy-web --type local --address http://127.0.0.1:3001

# 内网端点
./aegis endpoint add policy-web --type private --address http://10.0.0.5:3001

# 公网端点
./aegis endpoint add policy-web --type public --address http://1.2.3.4:3001

# 查看端点列表
./aegis endpoint list policy-web
```

### 5. 添加路由

```bash
./aegis route add policy.example.com --service policy-web
```

### 6. 预览配置（Dry-run）

```bash
./aegis apply --dry-run
```

输出：
```caddyfile
policy.example.com {
    encode gzip
    reverse_proxy http://127.0.0.1:3001
}
```

### 7. 应用配置

```bash
./aegis apply
```

完整流程：
1. 读取 active 路由和受管域名
2. 通过 EndpointResolver 解析可用端点（local → private → public）
3. 生成 GatewayConfig
4. 渲染 Caddyfile
5. 写入临时文件
6. 校验（`caddy validate`）
7. 备份当前配置
8. 替换正式配置
9. 重载 Caddy
10. 记录 apply_versions 和 operation_logs

### 8. 健康检查

```bash
# 查看最新健康状态
./aegis health

# 实时检查所有服务
./aegis health --all

# 检查指定服务
./aegis health policy-web
```

### 9. 维护模式

```bash
# 开启维护模式
./aegis maintenance on policy.example.com --message "系统维护中"
./aegis apply --dry-run

# 关闭维护模式
./aegis maintenance off policy.example.com
./aegis apply
```

### 10. 路由切换

```bash
# 创建新版本服务
./aegis service add policy-web-v2 \
  --project policy-page \
  --env preview \
  --kind http

# 为新服务添加端点
./aegis endpoint add policy-web-v2 --type local --address http://127.0.0.1:3002

# 切换路由
./aegis route switch policy.example.com --service policy-web-v2
./aegis apply --dry-run
```

### 11. 受管域名

```bash
# 添加受管域名
./aegis managed-domain add login.customer.com \
  --service policy-web \
  --owner tenant_123 \
  --target-type auth_page \
  --target-ref auth_page_456

# 输出 DNS 验证信息：
#   Type:  dns_txt
#   Name:  _aegis.login.customer.com
#   Value: aegis-verify-xxx
#
# 在 DNS 中添加 TXT 记录后执行验证：
./aegis managed-domain verify login.customer.com

# 验证通过后启用：
./aegis managed-domain enable login.customer.com

# 应用配置：
./aegis apply
```

受管域名规则：
- 只能绑定到 Aegis 已注册的 Service
- 不允许传入任意 upstream
- 必须 DNS 验证后才能 active
- pending_verification / failed / disabled 状态不生成 Caddy 配置

### 12. 回滚

```bash
./aegis rollback
```

恢复到最近一次成功的配置备份。

### 13. 启动 HTTP API

```bash
./aegis serve --addr 127.0.0.1:7380
```

API 默认监听 `127.0.0.1`，需要 Bearer Token 认证。

```bash
# 无 Token → 401
curl http://127.0.0.1:7380/api/system/status

# 带 Token → 200
curl http://127.0.0.1:7380/api/system/status \
  -H "Authorization: Bearer change-me"
```

## CLI 命令参考

### 系统
```bash
aegis init                              # 初始化
aegis serve --addr 127.0.0.1:7380       # 启动 HTTP API
aegis settings show                     # 查看配置
```

### 项目
```bash
aegis project create <name> [--description "..."]
aegis project list
aegis project show <name-or-id>
aegis project archive <name-or-id>
```

### 服务
```bash
aegis service add <name> --project <project> [--env prod] [--kind http]
aegis service list
aegis service show <name-or-id>
aegis service enable <name-or-id>
aegis service disable <name-or-id>
aegis service update <name-or-id> [--kind <kind>] [--env <env>] [--note <note>]
```

### 端点
```bash
aegis endpoint add <service> --type local --address http://127.0.0.1:3001
aegis endpoint add <service> --type private --address http://10.0.0.5:3001
aegis endpoint add <service> --type public --address http://1.2.3.4:3001
aegis endpoint list <service>
aegis endpoint enable <endpoint-id>
aegis endpoint disable <endpoint-id>
```

### 路由
```bash
aegis route add <domain> --service <service>
aegis route list
aegis route show <domain-or-id>
aegis route enable <domain-or-id>
aegis route disable <domain-or-id>
aegis route switch <domain-or-id> --service <service>
```

### 受管域名
```bash
aegis managed-domain add <domain> --service <service> --owner <ref> --target-type <type> --target-ref <ref>
aegis managed-domain verify <domain-or-id>
aegis managed-domain enable <domain-or-id>
aegis managed-domain disable <domain-or-id>
aegis managed-domain list
```

### 配置下发
```bash
aegis apply                  # 完整下发
aegis apply --dry-run        # 仅预览
aegis validate               # 生成并校验
aegis rollback               # 回滚
aegis apply history          # 下发历史
```

### 维护模式
```bash
aegis maintenance on <domain> [--message "..."]
aegis maintenance off <domain>
aegis maintenance status
```

### 健康检查
```bash
aegis health                  # 查看最新结果
aegis health <service>        # 检查指定服务
aegis health --all            # 检查所有服务
```

### 日志
```bash
aegis logs                    # 查看操作日志
aegis logs --action apply     # 按操作过滤
aegis logs --target <id>      # 按目标过滤
```

## HTTP API 参考

所有 API 需要 `Authorization: Bearer <token>` 头。

### 系统
```
GET  /api/system/status
```

### 项目
```
GET    /api/projects
POST   /api/projects
GET    /api/projects/:id
PATCH  /api/projects/:id
POST   /api/projects/:id/archive
```

### 服务
```
GET    /api/services
POST   /api/services
GET    /api/services/:id
PATCH  /api/services/:id
POST   /api/services/:id/enable
POST   /api/services/:id/disable
```

### 端点
```
GET    /api/services/:id/endpoints
POST   /api/services/:id/endpoints
PATCH  /api/endpoints/:id
POST   /api/endpoints/:id/enable
POST   /api/endpoints/:id/disable
DELETE /api/endpoints/:id
```

### 路由
```
GET    /api/routes
POST   /api/routes
GET    /api/routes/:id
PATCH  /api/routes/:id
POST   /api/routes/:id/enable
POST   /api/routes/:id/disable
POST   /api/routes/:id/switch-service
POST   /api/routes/:id/maintenance-on
POST   /api/routes/:id/maintenance-off
```

### 受管域名
```
GET    /api/managed-domains
POST   /api/managed-domains
GET    /api/managed-domains/:id
POST   /api/managed-domains/:id/verify
POST   /api/managed-domains/:id/enable
POST   /api/managed-domains/:id/disable
DELETE /api/managed-domains/:id
```

### 配置 / 下发
```
GET  /api/config/preview
GET  /api/config/diff
POST /api/apply
POST /api/apply/dry-run
POST /api/rollback
GET  /api/apply/history
```

### 健康 / 日志 / 设置
```
GET  /api/health
POST /api/health/check-all
GET  /api/health/services/:id
GET  /api/logs
GET  /api/settings
PATCH /api/settings
```

## 配置文件

开发环境默认配置（`.aegis/config.yaml`）：

```yaml
proxy:
  provider: caddy
  caddyfile_path: ./.aegis/Caddyfile
  caddy_binary: caddy
  reload_command: ""
  validate_command: ""
  backup_dir: ./.aegis/backups
  email: ""

store:
  sqlite_path: ./.aegis/aegis.db

server:
  addr: 127.0.0.1:7380
  admin_token: change-me

managed_domain:
  gateway_domain: ""

runtime:
  config_dir: ./.aegis/config
  data_dir: ./.aegis
```

### 生产环境配置

```yaml
proxy:
  provider: caddy
  caddyfile_path: /etc/caddy/Caddyfile
  caddy_binary: caddy
  reload_command: systemctl reload caddy
  validate_command: caddy validate --config {{config_path}}
  backup_dir: /var/lib/aegis/backups
  email: admin@example.com

store:
  sqlite_path: /var/lib/aegis/aegis.db

server:
  addr: 127.0.0.1:7380
  admin_token: "<强随机令牌>"

managed_domain:
  gateway_domain: gateway.example.com

runtime:
  config_dir: /etc/aegis
  data_dir: /var/lib/aegis
```

使用：
```bash
aegis --config /etc/aegis/config.yaml apply
```

### 开发模式

- `reload_command: ""` 和 `validate_command: ""` 为空时跳过对应步骤
- 使用当前目录下的 `.aegis/` 目录
- 无需 root 权限

## 项目结构

```
aegis/
  go.mod
  README.md
  cmd/aegis/main.go                    # 入口 & 全量装配
  internal/
    app/                               # 服务容器与接口定义
    config/                            # YAML 配置加载
    store/                             # SQLite 数据库 & 迁移
    id/                                # ID 生成器
    project/                           # 项目：模型 / 仓库 / 应用服务
    service/                           # 服务：模型 / 仓库 / 应用服务
    endpoint/                          # 端点：模型 / 仓库 / 解析器
    route/                             # 路由：模型 / 仓库 / 应用服务
    manageddomain/                     # 受管域名：模型 / 仓库 / 应用服务 / DNS检查
    proxy/                             # 代理抽象层
      adapter.go                       #   ProxyAdapter 接口 + GatewayConfig
      caddy/                           #   Caddy 适配器（渲染 / 校验 / 重载）
      nginx/                           #   Nginx 桩（未实现）
    apply/                             # 配置下发：计划 / 执行 / 回滚 / diff
    health/                            # 健康检查：TCP 连接检测
    logs/                              # 操作日志：审计追踪
    token/                             # API Token：模型 / 中间件 / 仓库
    httpapi/                           # HTTP API 服务
      handlers/                        #   21 个 REST 端点处理器
    cli/                               # Cobra CLI 命令
  templates/caddy/Caddyfile.tmpl
  examples/simple/README.md
```

## 核心领域模型

### Project
项目是服务和路由的顶层分组单位。

### Service
代表一个被 Aegis 管理的后端服务。`Kind` 支持 `http` / `tcp` / `file`。不直接存储 upstream 地址——地址由 Endpoint 管理。

### Endpoint
服务的网络端点。按优先级 `local > private > public` 解析。每个 Endpoint 做 TCP 连接检查（2s 超时）。

### Route
管理员维护的内部路由。将域名映射到服务。

### ManagedDomain
外部业务方接入的受控域名。必须通过 DNS TXT 验证后才能激活。只能绑定 Aegis 已注册的 Service。

### ApplyVersion
每次配置下发的记录：版本号、备份路径、渲染配置、状态。

### HealthCheck
每端点健康检查结果：状态、延迟、消息。

### OperationLog
所有操作的审计日志：操作类型、目标、结果、执行者（cli / api / system）。

### APIToken
HTTP API 的 Bearer Token。第一版从 config 加载 admin token，数据库模型已预留。

## 数据库表

共 20+ 张表：`projects`, `services`, `endpoints`, `routes`, `managed_domains`, `health_checks`, `apply_versions`, `operation_logs`, `api_tokens`, `exposures`, `listeners`, `edge_mux_rules`, `nodes`, `upgrade_sessions`, `spaces`, `admin_users`, `admin_sessions`, `apply_logs`, `audit_logs`, `node_events`, `trusted_gateways`, `deployments`

## 关键设计决策

- **Endpoint Resolver** 不做智能调度——严格按 `local → private → public` 顺序尝试，第一个 TCP 可达即返回
- **ManagedDomain** 不允许传入自定义 upstream——只能绑定已注册的 Service
- **HTTP API** 默认只监听 127.0.0.1——不暴露公网
- **所有危险操作可审计**——OperationLog 含 actor 追踪
- **所有配置变更可回滚**——Apply 前自动备份，Rollback 恢复最近成功版本
- **Caddyfile 语法不污染 domain model**——通过 GatewayConfig / RouteConfig 隔离

## 后续路线图

- [ ] Nginx Adapter 真实实现
- [ ] Caddy Admin API 适配器
- [ ] Web UI
- [ ] 多 Gateway Node 管理
- [ ] 流量灰度 / 百分比分流
- [ ] Waiting Room
- [ ] 完整 RBAC / 多租户
- [ ] Workspace / Client 管理

## License

MIT
