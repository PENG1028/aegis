# Aegis — Agent Project Guide

> **给 AI Agent 的完整项目手册。** 每当你被派到这个项目工作，阅读此文件即可理解全貌、完成部署、排查问题。

**架构规则（修改 Provider/能力/组合/透明代理/分布式节点前必读）：**
- `.claude/rules/provider-architecture.md` — 三大真理源 + 红线规则
- `.claude/rules/code-standards.md` — 防分叉规范
- `.claude/rules/distnode-architecture.md` — distnode 唯一跨节点抽象层
- `.claude/rules/distnode-standards.md` — distnode 零依赖复用守则

---

## 一、项目身份

Aegis 是一个**个人基础设施网关控制平面**（Go + SQLite + React UI），管理 HTTP/TCP/UDP 流量的入口路由，并协调多个节点。

**Aegis 是什么：**
- 一个管理跨项目服务入口的 CLI / API / Web UI 工具
- 一个安全的配置下发器（校验 → 备份 → 替换 → 重载 → 审计）
- Caddy（80 HTTP）+ HAProxy（443 TLS SNI）的配置生成器
- 一个通过 `internal/provider/` 统一渲染/生命周期管理中间件的控制器
- 一个分布式节点运行时（`internal/distnode/`，静态 peer + HMAC 认证 + Transport RPC）
- 一个服务间认证网关（`internal/serviceauth/`，Ed25519 ticket + 拓扑发现）
- 内置 ACME 客户端（`internal/acme/`，lego，替代 certbot）

**Aegis 不是什么：**
- NOT SaaS, NOT multi-admin, NOT multi-tenant
- NOT 在数据路径中 — Caddy/HAProxy 独立服务流量
- NOT PaaS, NOT Service Mesh, NOT Docker 管理平台

**版本：** v1.9B | **Go 1.22+** | **SQLite（嵌入式）** | **React + TypeScript 前端**

功能子版本（同一二进制内共存）：
- **v1.9A** — ServiceAuth（服务间认证 + Egress 出站网关）
- **v1.9B** — DistNode（分布式节点运行时，默认启用）
- **v1.9C** — CertStore + ACME（证书存储 + 内嵌 lego ACME 客户端）

---

## 二、基础设施

### VPS 节点

| 节点 | IP | 角色 | SSH |
|------|-----|------|-----|
| Server A | <SERVER_A_IP> | 面板 + 网关 + Caddy/HAProxy | `ssh ubuntu@<SERVER_A_IP>` |
| Server B | <SERVER_B_IP> | 远程节点 + 后端服务 | `ssh ubuntu@<SERVER_B_IP>` |

### 端口规则 ⚠️ 极其重要

**云安全组只开放了 80 (TCP) 和 443 (TCP+UDP)。** 任何跨机测试只能用这两个端口。

```bash
# 跨机连通性预检（必须先做）
ssh ubuntu@<source> "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://<target_ip>:<port>/"
# 期望: 2xx/4xx。其他 → 端口不通
```

### 端口分工

| 端口 | 服务 | 用途 |
|------|------|------|
| 80 | Caddy | HTTP 反向代理 + Let's Encrypt |
| 443 | HAProxy | TLS SNI 直通 → Caddy TLS |
| 7380 | Aegis | 内部 API（默认仅 localhost） |

**7380 从不硬编码。** Aegis API 端口从 `cfg.Server.Addr`（默认 `127.0.0.1:7380`）派生，
`cmd/aegis/main.go` 用 `safety.SplitHostPort(cfg.Server.Addr)` 拆出端口再跨节点使用。
改端口只改配置，代码里永远不写字面量 `7380` / `443` / `8443`（见 `code-standards.md`）。

---

## 三、项目结构（50 个内部包 / 57 个含子包）

```
cmd/aegis/main.go            # 入口 & 依赖装配（含 distnode / serviceauth / acme 装配）
pkg/serviceauth/             # 对外 SDK（其他项目 import：Client / Guard / Ticket）
internal/
  app/          # 服务容器接口
  config/       # YAML 配置（0600 权限，含 distnode 默认值）
  core/         # 公共底座：errors / id / recovery(panic 恢复) / sloglog / testutil
  store/        # SQLite + 迁移
  httpapi/      # HTTP API（130+ 路由）
    handlers/   # 请求处理器
  cli/          # Cobra CLI 命令
  project/      # 项目 CRUD
  service/      # 服务 CRUD
  endpoint/     # 端点模型 + 解析器（local→private→public→fail）
  route/        # 路由模型
  manageddomain/# 受管域名（DNS TXT 验证）
  exposure/     # TCP/UDP 端口暴露
  apply/        # 配置下发（计划→校验→备份→替换→重载）
  provider/     # Provider 抽象 + 渲染 + 生命周期（Caddy/HAProxy 全在此）
  edgemux/      # HAProxy SNI 路由
  listener/     # 端口监听管理
  topology/     # 网络拓扑 + 模板（topology/templates）
  health/       # TCP 健康检查
  logs/         # 操作日志审计
  token/        # API Token 认证
  adminauth/    # 管理员登录（bcrypt + rate limiting）
  space/        # 多空间隔离
  action/       # Action API（bind-http-domain 等高级操作）
  gateway/      # 网关抽象
  node/         # 节点模型/仓库
  nodeauth/     # 节点注册认证（历史，distnode 之前）
  distnode/     # ⭐ 唯一跨节点抽象层（Identity/Membership/Transport/Role）
  cluster/      # Leader 选举 + 集群健康聚合（读 distnode.Membership）
  serviceauth/  # ⭐ 服务间认证（Ed25519 ticket + 拓扑发现）
    aegis/      # serviceauth 的 Aegis 适配器（secrets/node/logs 落地）
  egress/       # 出站流量 allow/block 规则
  routingpolicy/# 网关策略路由
  routingtable/ # 路由表
  relay/        # 跨节点中继
  trace/        # 访问路径追踪
  safety/       # 路由安全检查 + 统一地址拆分（SplitHostPort）
  dns/          # DNS 管理
  tcp/          # TCP 端口代理
  udp/          # UDP 端口代理（含 Unix socket 支持）
  transparent/  # 透明代理（IP:端口拦截）
  credential/   # 凭证加密管理（AES-256-GCM）
  secrets/      # 加密/解密（AES-256-GCM MasterKey）
  certstore/    # 证书存储 + 生命周期（v1.9C）
  acme/         # 内嵌 lego ACME 客户端（v1.9C，替代 certbot）
  deploy/       # 可复用 SSH 部署工具箱
  deployment/   # 部署记录 + 快照（model/repository/snapshot）
  maintenance/  # 清理 / 冲突检测 / 漂移检测
  infra/        # 基础设施依赖探测
  addr/         # 统一地址类型（TCP/UDP/unix/unixgram）
  sync/         # 同步循环
  smoke/        # 冒烟测试
  fake/         # 测试假数据
  uiassets/     # 嵌入式前端（go:embed ui/dist）
ui/             # React 前端（Vite + TypeScript）
  src/
    pages/      # 40+ 页面
    components/ # 共享组件
    lib/        # API 客户端
scripts/        # 部署/更新/验收脚本
docs/           # 设计文档
tests/e2e/      # E2E 测试脚本
```

> **注意：已删除的包。** `internal/proxy/`（代理抽象层）、`internal/nodeagent/`、
> `internal/noderuntime/`、`internal/nodestate/` 已全部删除。
> - Caddy/HAProxy 的渲染/校验/重载全部收敛到 `internal/provider/`，不再有独立的 proxy 抽象层。
> - 跨节点通信全部收敛到 `internal/distnode/`，旧的 nodeagent/noderuntime/nodestate 三套心跳同步机制已废弃并移除。

---

## 四、构建系统

### Makefile 目标

```bash
make build           # 本地构建（当前 OS）
make build-linux     # 交叉编译 Linux amd64
make release         # 发布构建（注入 VERSION + BUILDTIME）
make test            # 运行所有测试（120s 超时）
make vet             # go vet
make dev-ui          # 启动前端开发服务器
make build-ui        # 构建前端 → ui/dist
make embed-ui        # 复制 ui/dist → internal/uiassets/dist（go:embed 用）

# 部署
make deploy-server-a # 构建 + 一键部署到 Server A
make deploy-server-b # 构建 + 一键部署到 Server B
make update-server-a # 安全更新 Server A（备份→停服→上传→启动→验证）
make update-server-b # 安全更新 Server B
make update-all      # 依次更新 Server B → Server A
```

部署脚本 (`scripts/update.sh`) 传输逻辑：
- gzip 压缩二进制 (~22MB→~11MB) 减少跨海传输窗口
- 上传到 `/tmp/aegis.upload.tmp` 而非直接覆盖运行中文件
- 校验文件大小一致后 `mv` 原子替换 → 无 "Text file busy" 风险
- SSH 中断也只会残留 `/tmp` 临时文件，不影响服务

### 首次部署流程

```bash
# 1. 本地构建并部署到 Server A（面板）
make deploy-server-a
# 输出包含面板 URL 和管理员密码

# 2. 部署到 Server B（远程节点）
make deploy-server-b
# 同样输出节点信息

# 3. 打开 http://<SERVER_A_IP>
#    用步骤 1 打印的密码登录
#    在 UI 中创建路由、Apply 配置
```

### 更新流程

```bash
# 单台更新（带自动备份和健康检查）
make update-server-a
make update-server-b

# 全部依次更新（先 B 后 A）
make update-all
```

更新脚本做的事：健康检查 → 备份（二进制+DB+配置） → 优雅停服 → 上传并验证 → 启动 → 最多 5 次重试健康检查 → 失败自动给出回滚命令

---

## 五、API 结构（130+ 路由）

### 路由分组

| 前缀 | 认证方式 | 用途 |
|------|----------|------|
| `/api/` | Bearer Token (admin/user) | 业务 API |
| `/api/v1/actions/` | Bearer Token | Action API（高级操作） |
| `/api/admin/v1/` | Admin Session Cookie | 管理后台 |
| `/api/distnode/v1/call` | HMAC（distnode Identity） | 节点间 Transport RPC |
| `/api/service-auth/v1/` | Ed25519 Service Ticket | 服务间认证/调用 |

> ⚠️ **`/api/node/v1/*` 已删除。** 旧的节点注册/心跳/同步端点（join / heartbeat /
> binary / desired-state / actual-state）连同中间件里的 node-route 认证旁路一起从
> `routes.go` 移除。**distnode 是现在唯一的跨节点机制** —— 静态 peer + Transport.Call，
> 不再有 HTTP 心跳。永远不要新增 `/api/node/v1/*`（见 `distnode-architecture.md`）。

### 核心 API 端点

**系统：**
- `GET /api/system/status` — 系统状态 + 版本
- `GET /api/healthz` — 存活探针（始终 200）
- `GET /api/readyz` — 就绪探针（检查 DB）

**CRUD 资源：** projects, services, endpoints, routes, managed-domains（均有完整 REST）

**配置下发：**
- `GET /api/config/preview` — 预览 Caddyfile/HAProxy 配置
- `POST /api/apply` — 执行配置下发
- `POST /api/rollback` — 回滚到最近成功版本

**管理后台：**
- `POST /api/admin/v1/auth/login` — 登录
- `GET /api/admin/v1/system/overview` — 系统概览
- `GET /api/admin/v1/nodes` — 节点列表
- `GET /api/admin/v1/trace/egress` — 出站追踪
- `GET /api/admin/v1/routes/{id}/safety` — 路由安全检查

**分布式节点（distnode）：**
- `POST /api/distnode/v1/call` — 节点间 Transport RPC（HMAC 认证，peer 使用）
- `GET /api/admin/v1/distnode/status` — 本节点 distnode 状态
- `POST /api/admin/v1/distnode/check` — 触发成员健康检查
- `POST /api/admin/v1/distnode/ping/{id}` — ping 指定 peer
- `GET /api/admin/v1/distnode/aggregate?path=...` — 聚合所有节点某个 API 的返回
- `GET /api/admin/v1/nodes/{id}/distnode-overview` — 指定节点 distnode 概览

**服务间认证（serviceauth）：**
- `POST /api/service-auth/v1/register` — 服务注册（上报公钥 + listen_port）
- `GET /api/service-auth/v1/sync` — 拉取黑名单/其他服务公钥
- `POST /api/service-auth/v1/call` — 按名字调用其他服务（网关代理转发）
- `POST /api/service-auth/v1/report` — 上报调用关系（构建拓扑）
- `POST /api/service-auth/v1/heartbeat` — 心跳
- `GET /api/admin/v1/service-auth/topology` — 服务调用拓扑
- `GET /api/admin/v1/service-auth/services` — 已注册服务列表

**二进制更新：**
- `POST /api/admin/v1/system/upload-binary` — 上传新版本（≤50MB）
- `POST /api/admin/v1/nodes/{id}/update` — 触发节点自更新

### 认证机制

- **Admin Session：** Cookie-based，HTTP-only，bcrypt 密码 + rate limiting（5 次/分钟/IP）
- **API Token：** `Authorization: Bearer <token>`，可按 scope 控制权限
- **DistNode：** HMAC-SHA256（cluster secret）+ 节点 Identity，保护 `/api/distnode/v1/call`
- **ServiceAuth：** Ed25519 签名 ticket（`X-Service-Ticket` + `X-Caller-Service` 头），
  由 `pkg/serviceauth` 的 Guard 中间件校验

### 重要规则

- 所有 POST/PATCH/PUT 需要 `Content-Type: application/json`
- 所有 admin mutation 端点必须调用 `MarkPending()`
- Service API Key 不能访问 `/api/admin/v1/*`（由 `isSystemRoute()` 阻止）
- 分页：`?limit=&offset=`，默认 limit=50，最大 200

---

## 六、核心数据流

### HTTP 域名转发（主链路）

```
UI 创建 Service → Endpoint → Route → Apply
                                        ↓
                    provider.Render() 生成 Caddyfile ← EndpointResolver(local→private→public)
                                        ↓
                    provider validate → backup → replace → reload
                                        ↓
                              流量：Caddy :80 → upstream
```

### TCP/UDP 端口转发

```
UI 创建 Exposure {type:tcp/udp, entry_host, entry_port, target_host, target_port}
    → POST /api/exposures/{id}/activate
    → tcpMgr.StartProxy() 或 udpMgr.StartProxy()
    → 流量直接转发（Aegis 不在数据路径中）
```

### 跨节点调用（distnode）

```
A 面板注册 handler：dn.Transport.Register("Aegis.ListRoutes", handler)
    → B 调用：dn.Transport.Call(ctx, "node_a", "Aegis.ListRoutes", args, &reply)
    → 走 POST /api/distnode/v1/call（HMAC 认证，经 443 边缘入口）
前端聚合：GET /api/admin/v1/distnode/aggregate?path=/api/admin/v1/routes
    → 一次调用聚合所有节点的返回
```

### 服务间调用（serviceauth）

```
服务 import pkg/serviceauth → client.Register(ctx)（上报 Ed25519 公钥 + listen_port）
    → client.CallService(ctx, "project-service", "POST", "/api/v1/create", body)
    → Aegis 按名字解析 host:port，代理转发（调用方无需知道对方域名/IP）
    → 目标服务用 client.Guard() 中间件校验 ticket
```

### Credential 别名解析

```
credential://pg-db → AES-256-GCM 解密 → 解析 host:port:user:db
    → Endpoint 可用 credential:// 作为目标地址
    → TCP/UDP 代理自动解析
```

---

## 七、关键设计决策

1. **Aegis 不在数据路径中** — Caddy/HAProxy 独立服务流量，Aegis 只负责配置管理
2. **Provider 是中间件的唯一抽象** — 渲染/校验/重载/生命周期全走 `internal/provider/`，无独立 proxy 包
3. **distnode 是唯一跨节点层** — 静态 peer + Transport RPC，无 HTTP 心跳，无 nodeagent
4. **Endpoint 解析顺序固定** — `local → private → public → fail`，不做智能调度
5. **ManagedDomain 必须 DNS 验证** — 绑定已有 Service，不允许任意 upstream
6. **Caddyfile 语法不污染 domain model** — 通过 GatewayConfig / RouteConfig 隔离
7. **所有配置变更可回滚** — Apply 前自动备份，Rollback 恢复
8. **API 默认只监听 127.0.0.1:7380** — 端口从 `cfg.Server.Addr` 派生，从不硬编码
9. **Unix socket 目标** — `unix:///run/app.sock` 格式，TCP check 自动跳过
10. **ServiceAuth 按名字寻址** — 服务用 `CallService(name)` 调用，Aegis 网关解析后端

---

## 七·五、Provider 生命周期管理

中间件（Caddy/HAProxy 等）的所有生命周期操作**都通过 `internal/provider/provider.go`
里的可选扩展接口**完成，绝不按名字 if/else，也不直接 `exec` systemctl。

| 接口 | 方法 | 用途 |
|------|------|------|
| `LifecycleProvider` | `CanInstall/Install/CanUninstall/Uninstall` | 安装 / 卸载中间件 |
| `ReloadableProvider` | `Reload` | 独立重载（不走完整 Apply） |
| `ServiceController` | `Start/Stop/Restart` | systemd 服务控制（模式切换时停掉不再需要的 Provider） |
| `ConfigReader` | `GetCurrentConfig` | 读回当前配置（config preview handler 用） |
| `ConfigCleaner` | `CleanConfig` | 清理陈旧配置文件（模式切换时用） |

**规则：** 加新中间件只需实现这些接口 + 在 `main.go` Register，
install/uninstall/reload/config/service-control 的 HTTP handler 会自动适配。
判断"某 Provider 能不能装/能不能重载"用类型断言（`p.(LifecycleProvider)`），
不要写 `if p.ID == "caddy"`（见 `provider-architecture.md` 红线规则）。

---

## 八、安全清单

- Admin 密码：bcrypt cost 12，首次启动随机生成打印一次
- Admin token：64 位随机 hex，GET /api/settings 中已脱敏
- Session cookie：Secure flag 可配置（dev false, prod true）
- 请求体限制：1 MB
- 登录频率限制：5 次/分钟/IP
- DistNode secret：cluster 共享 secret，未配置时自动生成 64 hex（32 字节）
- ServiceAuth：Ed25519 私钥仅服务本地持有，Aegis 只存公钥；ticket 带时间戳防重放
- 配置文件权限：0600
- Panic 恢复：`internal/core`（recovery）+ 关键 goroutine 内联 defer/recover

---

## 九、常见操作（Agent 执行指南）

### 如何新增一个 API 端点

1. 在 `internal/httpapi/handlers/` 写 handler 方法
2. 在 `internal/httpapi/routes.go` 注册路由
3. 如需新的 service 依赖，在 `internal/httpapi/server.go` 的 Services struct 添加字段
4. 在 `cmd/aegis/main.go` 的装配代码中注入依赖
5. 前端在 `ui/src/lib/real-api-client.ts` 添加 fetch 函数
6. 如需 mock，在 `ui/src/lib/api-bridge.ts` 添加 mock 实现

### 如何加跨节点功能（distnode）

```go
// 注册：不要新增 /api/node/v1/* 端点
dn.Transport.Register("Aegis.XXX", handler)
// 调用远程节点
dn.Transport.Call(ctx, "node_b", "Aegis.XXX", args, &reply)
```

### 如何让服务接入 Aegis（serviceauth SDK）

```go
import "aegis/pkg/serviceauth"

client, _ := serviceauth.New(serviceauth.Config{ServiceName: "my-service", ...})
client.Register(ctx)

// 调本机 Aegis 的 API —— 相对路径，无需知道 cluster URL / 端口
client.Post(ctx, "/api/v1/actions/bind-http-domain", body)

// 按名字调另一个服务 —— 无需知道对方域名/IP，Aegis 网关代理转发
client.CallService(ctx, "project-service", "POST", "/api/v1/create", body)

// 目标服务侧用 Guard 中间件校验 ticket
mux.Handle("POST /api/v1/create", client.Guard(myHandler))
```

### 如何调试部署问题

```bash
# 查看 Aegis 日志
ssh ubuntu@<ip> "sudo journalctl -u aegis --no-pager -n 50"

# 查看 Caddy 状态
ssh ubuntu@<ip> "sudo systemctl status caddy"

# 检查 API 是否响应
ssh ubuntu@<ip> "curl -s http://127.0.0.1:7380/api/healthz"

# 查看系统状态
ssh ubuntu@<ip> "curl -s http://127.0.0.1:7380/api/system/status"

# 查看 distnode 成员状态
ssh ubuntu@<ip> "curl -s http://127.0.0.1:7380/api/admin/v1/distnode/status"

# 测试端口转发
echo "test" | nc -w2 <ip> <port>
```

### 如何回滚

```bash
# 方法 1: API 回滚
curl -X POST http://127.0.0.1:7380/api/rollback \
  -H "Authorization: Bearer <token>"

# 方法 2: 手动回滚（从备份）
ssh ubuntu@<ip> "sudo cp /var/lib/aegis/backups/aegis.<timestamp> /usr/local/bin/aegis"
ssh ubuntu@<ip> "sudo systemctl restart aegis"
```

### 预检清单（部署前必查）

```bash
# 1. 本地测试通过
make test

# 2. 构建检查
go build ./... && go vet ./...

# 3. 前端类型检查（可选）
cd ui && npx tsc --noEmit

# 4. 跨机连通性（如涉及双机功能）
ssh ubuntu@<SERVER_A_IP> "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://<SERVER_B_IP>:80/"
```

---

## 十、提交规范

```
v1.9B: 中文或英文描述变更内容
v1.9B-2: 后续小修复
```

版本号在 `cmd/aegis/main.go` 的 `var Version = "dev"` 中，release 构建时由 Makefile 注入。
功能子版本（v1.9A/v1.9B/v1.9C）按模块划分，不代表独立发布。

---

## 十一、当前已知限制

- 节点自更新执行代码未实现（收到 update_available 但不执行下载替换）
- 部分 slog 迁移未完成（DNS、transparent 还在用 log.Printf）
- 无 Dockerfile
- `internal/nodeauth/` 是 distnode 之前的历史遗留，仍在但不应新增引用
- 部分老旧 doc（README.md、docs/v1.8/）路由前缀可能已过时，以 `internal/httpapi/routes.go` 为准

---

## 十二、文档索引

| 文档 | 内容 |
|------|------|
| `CLAUDE.md` | 本文件 — Agent 全项目指南 |
| `docs/external-api-guide.md` | 对外 API 使用指南（面向接入方，以 routes.go 为准） |
| `docs/serviceauth-design.md` | ServiceAuth 设计文档 |
| `docs/serviceauth.md` | ServiceAuth 使用说明 |
| `docs/distnode-onboarding-fix.md` | distnode 节点加入流程 |
| `docs/apply-safety.md` | Apply 安全机制 |
| `docs/mode-switch-safety.md` | Provider 模式切换安全 |
| `docs/code-fork-audit.md` | 防分叉审计记录 |
| `docs/boundary/` | 能力边界 / 维度矩阵 |
| `docs/runbooks/` | 回滚/重启/安装等运维手册 |
| `docs/design/` | 各项设计文档 |
| `scripts/deploy.sh` | 一键部署脚本 |
| `scripts/update.sh` | 安全更新脚本 |
| `scripts/update-all.sh` | 双机批量更新 |
| `tests/e2e/` | E2E 测试脚本 |
</content>
</invoke>
