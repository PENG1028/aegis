# Aegis — Agent Project Guide

> **给 AI Agent 的完整项目手册。** 每当你被派到这个项目工作，阅读此文件即可理解全貌、完成部署、排查问题。

**架构规则（修改 Provider/能力/组合/透明代理前必读）：**
- `.claude/rules/provider-architecture.md` — 三大真理源 + 红线规则
- `.claude/rules/code-standards.md` — 防分叉规范

---

## 一、项目身份

Aegis 是一个**个人基础设施网关控制平面**（Go + SQLite + React UI），管理 HTTP/TCP/UDP 流量的入口路由。

**Aegis 是什么：**
- 一个管理跨项目服务入口的 CLI / API / Web UI 工具
- 一个安全的配置下发器（校验 → 备份 → 替换 → 重载 → 审计）
- Caddy（80 HTTP）+ HAProxy（443 TLS SNI）的配置生成器
- 双机 Gateway Link（HMAC-SHA256 跨网关认证）

**Aegis 不是什么：**
- NOT SaaS, NOT multi-admin, NOT multi-tenant
- NOT 在数据路径中 — Caddy/HAProxy 独立服务流量
- NOT PaaS, NOT Service Mesh, NOT Docker 管理平台

**版本：** v1.8L | **Go 1.22+** | **SQLite（嵌入式）** | **React + TypeScript 前端**

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
| 7380 | Aegis | 内部 API（仅 localhost） |

---

## 三、项目结构（68 个包）

```
cmd/aegis/main.go            # 入口 & 依赖装配
internal/
  app/          # 服务容器接口
  config/       # YAML 配置（0600 权限）
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
  health/       # TCP 健康检查
  logs/         # 操作日志审计
  token/        # API Token 认证
  adminauth/    # 管理员登录（bcrypt + rate limiting）
  space/        # 多空间隔离
  action/       # Action API（bindHTTPDomain 等高级操作）
  proxy/        # 代理抽象层
    caddy/      # Caddy 适配器（Caddyfile 渲染/校验/重载）
    nginx/      # Nginx 桩
  edgemux/      # HAProxy SNI 路由
  listener/     # 端口监听管理
  gateway/      # 网关抽象
  gateway_link/ # 跨机 Gateway Link（HMAC-SHA256）
  node/         # 节点模型/仓库
  nodeagent/    # 节点代理（心跳、同步）
  nodeauth/     # 节点注册认证
  nodestate/    # 期望状态 vs 实际状态同步
  noderuntime/  # 节点运行时
  localgateway/ # 本地网关运行时
  routingpolicy/# 网关策略路由
  routingtable/ # 路由表
  topology/     # 网络拓扑
  relay/        # 跨节点中继
  trace/        # 访问路径追踪
  safety/       # 路由安全检查
  dns/          # DNS 管理
  tcp/          # TCP 端口代理
  udp/          # UDP 端口代理（含 Unix socket 支持）
  transparent/  # 透明代理（IP:端口拦截）
  credential/   # 凭证加密管理（AES-256-GCM）
  recovery/     # Panic 恢复 + stack trace
  validate/     # 输入验证
  sloglog/      # 结构化日志
  testutil/     # 测试辅助（SetupTestDB）
  addr/         # 统一地址类型（TCP/UDP/unix/unixgram）
  id/           # ID 生成器
  consistency/  # 一致性检查
  snapshot/     # 配置快照
  upgrade/      # 升级管理
  importcfg/    # Caddyfile 导入
  provider/     # Provider 抽象
  secrets/      # 加密/解密
  failure/      # 故障矩阵
  smoke/        # 冒烟测试
  e2e/          # 端到端测试
  quota/        # 配额
  sync/         # 同步循环
  cluster/      # 集群协调
  fake/         # 测试假数据
  errors/       # 错误类型
  uri/          # URI 解析
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

部署脚本 (`scripts/update.sh`) 传输逻辑 (v1.8L-10)：
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
| `/api/admin/v1/` | Admin Session Cookie | 管理后台 |
| `/api/node/v1/` | Node Bearer Token | 节点注册/心跳/同步 |

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
- `POST /api/admin/v1/gateway-links` — 创建跨机链路
- `GET /api/admin/v1/trace/egress` — 出站追踪
- `GET /api/admin/v1/routes/{id}/safety` — 路由安全检查

**节点 API（无需 admin session，用 node credential）：**
- `POST /api/node/v1/join` — 节点注册
- `POST /api/node/v1/heartbeat` — 心跳上报
- `GET /api/node/v1/binary` — 下载更新二进制
- `GET /api/node/v1/desired-state` — 拉取期望状态
- `POST /api/node/v1/actual-state` — 上报实际状态

**二进制更新：**
- `POST /api/admin/v1/system/upload-binary` — 上传新版本（≤50MB）
- `POST /api/admin/v1/nodes/{id}/update` — 触发节点自更新

### 认证机制

- **Admin Session：** Cookie-based，HTTP-only，bcrypt 密码 + rate limiting（5 次/分钟/IP）
- **API Token：** `Authorization: Bearer <token>`，可按 scope 控制权限
- **Node Auth：** `Authorization: Bearer <node_token>`，节点注册时下发
- **Gateway Link：** HMAC-SHA256 + 时间戳防重放（5 分钟窗口）

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
                              Caddyfile 生成 ← EndpointResolver(local→private→public)
                                        ↓
                              caddy validate → backup → replace → reload
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

### Gateway Link（跨机认证转发）

```
Server A 创建 GatewayLink → 拿到 raw_secret
    → 绑定 Route 到 gateway_link_id
    → Apply → Caddyfile 中 upstream 指向 Server B
    → Server B 验证 HMAC-SHA256(secret, timestamp)
    → 合法请求转发到本地服务
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
2. **Gateway Link 不是路由数据源** — 它只是路由的元数据属性
3. **Endpoint 解析顺序固定** — `local → private → public → fail`，不做智能调度
4. **ManagedDomain 必须 DNS 验证** — 绑定已有 Service，不允许任意 upstream
5. **Caddyfile 语法不污染 domain model** — 通过 GatewayConfig / RouteConfig 隔离
6. **所有配置变更可回滚** — Apply 前自动备份，Rollback 恢复
7. **API 默认只监听 127.0.0.1:7380** — 不直接暴露公网
8. **Unix socket 目标** — `unix:///run/app.sock` 格式，TCP check 自动跳过

---

## 八、安全清单

- Admin 密码：bcrypt cost 12，首次启动随机生成打印一次
- Admin token：64 位随机 hex，GET /api/settings 中已脱敏
- Session cookie：Secure flag 可配置（dev false, prod true）
- 请求体限制：1 MB
- 登录频率限制：5 次/分钟/IP
- Gateway link secret：HMAC-SHA256 hash 存 DB，原始值仅返回一次
- 配置文件权限：0600
- Panic 恢复：`internal/recovery` 包 + 关键 goroutine 内联 defer/recover

---

## 九、常见操作（Agent 执行指南）

### 如何新增一个 API 端点

1. 在 `internal/httpapi/handlers/` 写 handler 方法
2. 在 `internal/httpapi/routes.go` 注册路由
3. 如需新的 service 依赖，在 `internal/httpapi/server.go` 的 Services struct 添加字段
4. 在 `cmd/aegis/main.go` 的装配代码中注入依赖
5. 前端在 `ui/src/lib/real-api-client.ts` 添加 fetch 函数
6. 如需 mock，在 `ui/src/lib/api-bridge.ts` 添加 mock 实现

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
v1.8L: 中文或英文描述变更内容
v1.8L-2: 后续小修复
```

版本号在 `cmd/aegis/main.go` 的 `var Version = "dev"` 中，release 构建时由 Makefile 注入。

---

## 十一、当前已知限制

- 节点自更新执行代码未实现（agent 收到 update_available 但不执行下载替换）
- 部分 slog 迁移未完成（DNS、nodeagent、transparent 还在用 log.Printf）
- 多节点支持为 FAKE_ONLY 状态
- 无 Dockerfile
- 部分老旧 doc（README.md、docs/api.md）路由前缀可能已过时，以 `internal/httpapi/routes.go` 为准

---

## 十二、文档索引

| 文档 | 内容 |
|------|------|
| `CLAUDE.md` | 本文件 — Agent 全项目指南 |
| `docs/api.md` | API 文档（部分过时，以 routes.go 为准） |
| `docs/install-runbook.md` | 手动安装步骤 |
| `docs/deployment-model.md` | 部署版本模型 |
| `docs/apply-safety.md` | Apply 安全机制 |
| `docs/rollback-runbook.md` | 回滚操作手册 |
| `docs/restart-safety-runbook.md` | 重启安全检查 |
| `docs/single-node-production-boundary.md` | 单节点生产边界 |
| `docs/boundary/dimension-map.md` | 能力矩阵 |
| `docs/v1.8/` | v1.8 各项设计文档 |
| `scripts/deploy.sh` | 一键部署脚本 |
| `scripts/update.sh` | 安全更新脚本 |
| `scripts/update-all.sh` | 双机批量更新 |
| `tests/e2e/` | 7 个 E2E 测试脚本 |
