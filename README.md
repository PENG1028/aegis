# Aegis

**个人基础设施网关控制平面 — v1.9C-3**

Aegis 是一个管理跨项目服务入口的控制面板：通过 Web UI / CLI / HTTP API 管理域名路由、TCP/UDP 端口转发、TLS 证书、分布式节点和服务间认证，并把配置安全地下发到 Caddy / HAProxy。

Go + SQLite（嵌入式，无外部数据库）+ React 前端，单二进制部署。

## 功能总览

| 版本 | 功能 |
|------|------|
| v1.9A | **ServiceAuth** — 服务间认证（Ed25519 ticket）+ Egress 出站网关 |
| v1.9B | **DistNode** — 分布式节点运行时（静态 peer + HMAC + Transport RPC），默认启用 |
| v1.9C | **CertStore + ACME** — 证书存储 + 内嵌 lego ACME 客户端（替代 certbot） |
| v1.9C-2 | **FlowBridge** — 数据面实例管理 + 域名绑定实例（Aegis 只做 TLS 终止 + 转发） |
| v1.9C-3 | **大修批次** — 回滚链路真实备份、`/call` 强制 ticket、unix endpoint、服务名抢占/封禁自愈拦截等 38 项修复 |

完整能力清单见 `docs/design/feature-catalog.md`。

## Aegis 是什么

- 一个管理跨项目服务入口的 **CLI / API / Web UI** 工具
- 一个安全的**配置下发器**（校验 → 备份 → 替换 → 重载 → 审计，支持回滚）
- Caddy（80 HTTP）+ HAProxy（443 TLS SNI）的**配置生成器**
- 一个分布式节点运行时（静态 peer + HMAC 认证 + Transport RPC）
- 一个服务间认证网关（Ed25519 ticket + 拓扑发现）
- 内置 ACME 客户端（自动申请 / 续期证书）

**Aegis 不在数据路径中** — Caddy/HAProxy 独立承载流量，Aegis 只负责配置管理与生命周期。

## 环境要求

| 组件 | 要求 |
|------|------|
| Go | 1.25+（go.mod 声明 `go 1.25.0`；旧版本会自动下载 toolchain） |
| Node.js | 20+（仅构建前端需要） |
| 运行时 | 无需外部数据库；SQLite 嵌入式 |
| 服务器 | 见 `docs/runbooks/install-runbook.md`（Ubuntu 22.04/24.04 + Caddy 2.x + HAProxy 2.x） |

> **国内网络提示**：Go 会自动下载 toolchain 和依赖，若失败请配置镜像：
> ```bash
> go env -w GOPROXY=https://goproxy.cn,direct
> go env -w GOSUMDB=sum.golang.org
> ```

## 快速开始（本地开发）

```bash
# 1. 构建（注意：必须走 make，前端产物会自动嵌入二进制）
make build          # Windows 无 make 时：
                    #   cd ui && npm install && npm run build
                    #   xcopy /E /I /Y ui\dist internal\uiassets\dist
                    #   go build -o aegis ./cmd/aegis/

# 2. 初始化（创建 .aegis/ 目录、SQLite 数据库、备份目录）
./aegis init

# 3. 启动 API（默认监听 127.0.0.1:7380）
./aegis serve --addr 127.0.0.1:7380

# 4. 首次启动会向 stderr 打印管理员凭据：
#    === AEGIS FIRST-RUN ADMIN CREDENTIALS ===
#      Username: admin
#      Password: <随机生成>
#    请立即保存并登录修改。

# 5. 打开 Web UI
#    二进制自带前端：浏览器访问 http://127.0.0.1:7380
#    开发模式热更新：cd ui && npm run dev   → http://localhost:3000
```

> 开发模式（`reload_command` / `validate_command` 为空）会跳过 Caddy 校验和重载，
> 无需 root 权限即可试用完整 CRUD 和 Apply 流程。

## 生产部署

### 端口规则 ⚠️ 极其重要

云安全组**只开放 80 (TCP) 和 443 (TCP+UDP)**。跨机测试只能用这两个端口。

| 端口 | 服务 | 用途 |
|------|------|------|
| 80 | Caddy | HTTP 反向代理 + Let's Encrypt |
| 443 | HAProxy | TLS SNI 直通 → Caddy TLS |
| 7380 | Aegis | 内部 API（默认仅 localhost，从不暴露公网） |

### 一键部署 / 更新

```bash
# 构建 Linux 版并部署到 Server A / Server B
make deploy-server-a          # 参数: SERVER_A=<IP> SSH_USER=ubuntu
make deploy-server-b

# 安全更新（健康检查 → 备份 → 优雅停服 → 原子替换 → 启动 → 验证，失败自动给回滚命令）
make update-server-a
make update-server-b
make update-all               # 依次更新 B → A

# 输出包含面板 URL 和管理员密码
```

手动安装流程（Caddy/HAProxy 安装、systemd 单元、生产配置）见 **`docs/runbooks/install-runbook.md`**。

### 运维

| 场景 | 文档 |
|------|------|
| 回滚 | `docs/runbooks/rollback-runbook.md` |
| 重启安全 | `docs/runbooks/restart-safety-runbook.md` |
| 部署模型 | `docs/runbooks/deployment-model.md` |
| 接受验证 | `docs/runbooks/runtime-acceptance-runbook.md` |

## Web UI 概览

登录后按模块组织：

| 页面分组 | 内容 |
|----------|------|
| Command Center | 系统状态、Apply/Rollback/变更历史 |
| Access | 服务间认证拓扑、节点、凭证（credential）、证书（CertStore） |
| Exposure | TCP/UDP 端口暴露、FlowBridge 实例、透明代理规则 |
| Fabric | 项目 / 服务 / 端点 / 路由（含 flowbridge upstream）管理 |
| Observe | 健康检查、日志、追踪（trace）、DNS 状态 |
| Release | 部署记录、更新 |
| Runtime | distnode 成员、网关策略路由、路由表、Provider 诊断 |
| Settings | 管理配置、API Token、出站规则（egress） |

## CLI 使用

```bash
aegis --help                 # 全部命令
aegis init                   # 初始化
aegis serve --addr 127.0.0.1:7380
aegis doctor                 # 环境自检
aegis diagnostics export     # 导出诊断包
```

常用命令族：

```bash
# 项目 / 服务 / 端点 / 路由
aegis project create <name>
aegis service add <name> --project <project>
aegis endpoint add <service> --type local --address http://127.0.0.1:3001
aegis route add <domain> --service <service>

# 配置下发（校验 → 备份 → 替换 → 重载，可回滚）
aegis apply --dry-run        # 仅预览
aegis apply                  # 执行
aegis rollback               # 回滚最近一次成功版本
aegis apply history

# 其他
aegis managed-domain add/verify/enable <domain>
aegis exposure activate <exposure-id>
aegis health [service]
aegis maintenance on/off <domain>
aegis logs
```

## HTTP API

**认证方式（4 种）：**

| 认证 | 覆盖范围 |
|------|----------|
| Admin Session Cookie | `/api/` 全部（登录后），HttpOnly + SameSite=Strict |
| Bearer Token | `/api/`（单一静态 admin token） |
| DistNode HMAC | `/api/distnode/v1/call`（节点间 RPC） |
| ServiceAuth Ed25519 ticket | `/api/service-auth/v1/`（服务间调用，白名单制） |

**核心端点：**

```
GET  /api/system/status            # 系统状态 + 版本
GET  /api/healthz                  # 存活探针
POST /api/apply                    # 执行配置下发
GET  /api/config/preview           # 预览 Caddyfile/HAProxy 配置

# 业务 CRUD（projects / services / endpoints / routes / managed-domains）
GET|POST|PATCH /api/{resource}

# 管理后台（/api/admin/v1/）
POST /api/admin/v1/auth/login
GET  /api/admin/v1/distnode/status            # 分布式节点状态
GET  /api/admin/v1/nodes                      # 节点列表
GET  /api/admin/v1/certificates               # 证书管理
POST /api/admin/v1/flowbridge                 # FlowBridge 实例
GET  /api/admin/v1/service-auth/topology      # 服务调用拓扑
```

对外 API 使用指南见 `docs/external-api-guide.md`（以 `internal/httpapi/routes.go` 为准）。

## 配置文件

默认路径 `.aegis/config.yaml`（0600 权限），生产环境 `/etc/aegis/config.yaml`：

```yaml
proxy:
  provider: caddy
  caddyfile_path: /etc/caddy/Caddyfile
  reload_command: systemctl reload caddy
  validate_command: caddy validate --config {{config_path}}
  backup_dir: /var/lib/aegis/backups
  email: admin@example.com        # ACME 邮箱
  acme_server: ""                 # 生产 CA；测试用 Let's Encrypt staging

store:
  sqlite_path: /var/lib/aegis/aegis.db

server:
  addr: 127.0.0.1:7380            # API 端口，代码中从不硬编码
  admin_token: "<强随机令牌>"

managed_domain:
  gateway_domain: gateway.example.com

runtime:
  config_dir: /etc/aegis
  data_dir: /var/lib/aegis
```

```bash
aegis --config /etc/aegis/config.yaml apply
```

## 核心概念

- **Service** — 被管理的后端服务（http/tcp/file），不直接存 upstream，地址由 Endpoint 管理
- **Endpoint** — 服务端点，解析顺序固定 `local → private → public → fail`（不做智能调度）
- **Route** — 域名 → 服务映射；可绑定 FlowBridge 实例（数据面端口）
- **ManagedDomain** — 外部接入的受管域名，必须 DNS TXT 验证后才能激活
- **Exposure** — TCP/UDP 端口转发（`tcp://` / `udp://` / `unix://` 目标）
- **Credential** — AES-256-GCM 加密的数据库凭据，Endpoint 可用 `credential://` 引用
- **DistNode** — 跨节点抽象层：静态 peer + HMAC + Transport RPC，无 HTTP 心跳
- **ServiceAuth** — Ed25519 ticket 服务间认证，按名字寻址调用
- **FlowBridge** — 数据面实例（控制面 /health 探活），域名绑定后 Aegis 只做 TLS 终止 + 转发

## 文档索引

| 文档 | 内容 |
|------|------|
| `docs/external-api-guide.md` | 对外 API 使用指南（面向接入方） |
| `docs/flowbridge-integration.md` | FlowBridge 实例接入 |
| `docs/serviceauth.md` | ServiceAuth 使用说明 |
| `docs/apply-safety.md` | Apply 安全机制 / 回滚 |
| `docs/runbooks/` | 安装 / 回滚 / 重启 / 验收运维手册 |
| `docs/design/` | 各项设计文档（能力边界、网关抽象等） |
| `CLAUDE.md` | 项目全貌手册（面向 AI Agent / 贡献者） |

## License

MIT
