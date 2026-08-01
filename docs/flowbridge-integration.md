# FlowBridge 集成设计（第一阶段调研文档）

> 状态：**已实施（2026-08-01）**。M1-M8 全部完成，验证见 `docs/flowbridge-dod.md` 与 `docs/flowbridge-review.md`。
> 配合仓库：FlowBridge（独立 HTTP 数据面中间件，`F:\Work Document\project\flowbridge`）。
> 本文档使用的术语与现有代码一致：Route / EdgeRule / Service / Endpoint / Planner / Provider / UpstreamSpec。

---

## 一、配合模型（已确认结论 + 与现状的差距）

### 1.1 Aegis = 「域名 → 单一 upstream」映射表

**已确认结论：** 一个域名只指向一个目标，不做多目标回退 / 负载分发。绑定了 flowbridge 的域名，流量以 flowbridge 为准——Aegis 只做 TLS 终止 + 转发，不关心 flowbridge 内部的路由 / 切换 / 健康。

**现状对应：** `internal/route.Route` 的 `Domain → ServiceID` 即此映射（`internal/route/model.go`）。Service 再经 EndpointResolver 解析出具体地址（local → private → public → fail，`internal/endpoint/resolver.go`）。即当前模型是**两级映射**：域名 → 服务 → 端点。FlowBridge 模型是**一级映射**：域名 → flowbridge 实例。

**差距：** 当前 Route 的 target 来源只有「服务端点」和「Gateway Link」（`internal/topology/planner.go` 的 `resolveIntents` 内两种来源）。缺少第三个来源：flowbridge 实例。

### 1.2 upstream 增加 flowbridge 子类型

**已确认结论：** 域名可以指向旧后端（IP:端口），也可以指向某个 flowbridge 实例（机器 IP + 数据面端口）。

**现状对应：** 上游在代码中的载体是 `provider.UpstreamSpec{Type, Target}`（`internal/hostdep/provider/model.go`），Type 取值 `tcp|udp|unix|http`。**这里没有 flowbridge 类型，而且不需要加**——对 Provider 渲染层来说，flowbridge 数据面就是一个 `http://<machine_ip>:<data_plane_port>` 的普通 HTTP upstream，`UpstreamSpec.Type="http"` 已经够用。**flowbridge 的区别在上游的来源决策（planner 层），不在渲染层（provider 层）。**

**落点核实结论（任务要求核实 EdgeRule 是否为最小落点）：**
- **EdgeRule 不是落点。** `internal/edgemux` 是 SNI 层（L5）直通规则：HAProxy 按 SNI 把原始 TCP 流转发到 `TargetHost:TargetPort`，**不做任何 Host 头解析、不终止 TLS**。HTTP 路由自动创建的 edge rule 只是固定的 `SNI → 127.0.0.1:8443`（`edgemux/service.go:EnsureRuleForHTTPRoute`）。FlowBridge 的 binding 匹配依赖 **Host 头（L7）**，必须由 Aegis 终止 TLS 后按 Host 转发，因此走 **Route（L7 路由）**，EdgeRule 层无需改动。
- **Route 是落点。** 具体为 Route 增加可选字段 `FlowBridgeID *string`（仿现有 `CertID *string` 模式，`route/model.go:32`）。`service_id TEXT NOT NULL` 约束保持不变（`internal/store/migrations.go:418`）——绑定流程沿用 `bind-http-domain` 现有的「合成 service + route + edge rule」模式（`internal/action/bind_http_domain.go`），flowbridge 绑定时创建合成 service 作为归属锚点、route 携带 `FlowBridgeID`、**不创建 endpoint**。

### 1.3 flowbridge 实例 = 新的被管理实体

**已确认结论：** 实例字段：id、名称、机器 IP、数据面端口、控制面地址、健康状态、启用/禁用。Aegis 管理多个实例。

**现状差距：** 不存在该实体。需要新增 `internal/flowbridge` 包（model / repository / service），新表 `flowbridge_instances`（下一个迁移版本号为 **048**，当前最新 047 `certificate_reference_integrity`）。

### 1.4 转发关键约束（逐条对照现状）

| 约束 | 现状 | 结论 |
|---|---|---|
| 终止 TLS 后保留 Host 头原样转发 | **Caddy 默认行为已满足**：官方文档明确「默认透传所有入站头，包括 Host」（仅对 HTTPS upstream 自 v2.11 起自动改写 Host，我们到 flowbridge 是明文 HTTP，不受影响）。现有 gateway-link 代码显式 `header_up Host`（`planner.go:212`）属防御性写法，机制（`writeReverseProxy` 的 ExtraHeaders，`caddy_render.go:179`）可复用 | 大概率零改动，**必须真机验收一次**（验收信号 A1/A2） |
| 补 X-Forwarded-Proto / X-Forwarded-For | Caddy 自动附加 X-Forwarded-For / X-Forwarded-Proto / X-Forwarded-Host，并忽略客户端传入值防伪造 | 零改动 |
| 连接 keep-alive 复用 | Caddy http transport 默认 keepalive 2m、每 host 32 条空闲连接 | 零改动 |
| 健康探针指向 FlowBridge 控制端点 `/health`，不探业务 target | 现状健康检查只做**到 endpoint 地址的 TCP connect**（`internal/health/checker.go`），没有 HTTP 探活、没有实例级概念 | **必改**：新增实例级 HTTP 探活（GET `http://<control_address>/health`，FlowBridge 该端点**免认证**——已核实 `flowbridge/internal/control/control.go`） |

### 1.5 FlowBridge 侧接口事实（调研确认，供实施引用）

- 数据面：YAML 配置的 listeners（host:port），bindings 按 `(listener, host, path, pathPrefix) → service` 匹配，**Host 匹配是核心依赖**。
- 控制面：`/health`（免认证，纯存活探针）、`/status`、`/bindings`、`/services`、`/reload` 等（配置了 token 时需要 `Authorization: Bearer <token>`；**非 loopback 控制面强制要求 auth_token**）。
- 目标切换（`/switch`、`/drain`）是 flowbridge 内部操作，**Aegis v1 不调用**。

---

## 二、改动清单

分层：**必改**（第二步实施范围） / **建议**（同批实施但可裁剪） / **可选**（推迟）。每条含：改动点、涉及文件/模块、影响面、风险、FlowBridge 侧配合。

### 2.1 必改

#### M1. 新实体 flowbridge_instances（表 + model + repository + service）

- **改动点：** 新增迁移 048 建表；新包 `internal/flowbridge/`（model.go / repository.go / service.go，仿 `internal/certstore` 结构）；字段：`id, name, machine_ip, data_plane_port, control_address, control_token_enc (可选), enabled, last_health_status, last_health_latency_ms, last_health_message, last_checked_at, space_id, owner_type, owner_id, created_by_token_id, created_at, updated_at`。
- **涉及文件：** `internal/store/migrations.go`（+migration048）、新建 `internal/flowbridge/*`、`cmd/aegis/main.go`（装配）。
- **影响面：** 新增代码，无存量影响；SQLite 迁移自动执行（项目现有迁移机制）。
- **风险：** 低。注意 `control_token` 需用 `internal/credential`（AES-256-GCM）加密存储，不能明文落库——若 v1 只用 `/health`（免认证），token 字段可先留空（见 O1）。
- **FlowBridge 侧配合：** 无。

#### M2. Route 增加 FlowBridgeID

- **改动点：** 迁移 049（或并入 048）`ALTER TABLE routes ADD COLUMN flowbridge_id TEXT`；`route/model.go` 增加 `FlowBridgeID *string json:"flowbridge_id,omitempty"`（仿 `CertID`）；`CreateRouteInput` / action 输入相应扩展。
- **涉及文件：** `internal/store/migrations.go`、`internal/route/model.go`（+repository 读写列，+model_test 补字段断言）。
- **影响面：** Route 是核心模型，UI / API 序列化自动带上新字段（`json` tag），无需逐处改；注意 `NormalizeManagement` / `CapabilityKeys` 等派生逻辑不受影响（composition 仍是 https_route，ingress TLS 语义不变）。
- **风险：** 低。唯一注意点：`service_id NOT NULL` 保持，flowbridge 路由仍绑定合成 service（见 M4）。
- **FlowBridge 侧配合：** 无。

#### M3. Planner 第三上游来源：flowbridge

- **改动点：** `internal/topology/planner.go` 的 `resolveIntents()` 增加分支：`ri.FlowBridgeID != ""` 时——加载实例；实例不存在 → warning + 跳过该路由；实例 `enabled=false` → warning + 跳过（语义对齐现有「service disabled 跳过」）；实例正常 → `ri.Upstream = fmt.Sprintf("http://%s:%d", instance.MachineIP, instance.DataPlanePort)`，并设置 `UpstreamSpec{Type:"http"}`，**不回退到 endpoint 解析**。Endpoint 解析仅在不带 FlowBridgeID 时执行。
- **涉及文件：** `internal/topology/planner.go`、`internal/topology/intent.go`（RouteIntent 增加 FlowBridgeID 字段）、`internal/topology/planner_tls_test.go` 或新增 planner 测试。
- **影响面：** Apply / preview / dry-run / config diff 全走此路径，一处改动全局生效。**这是唯一决策点，不得在渲染层或其它地方重复实现。**
- **风险：** 中。planner 是生产关键路径，改动需覆盖：flowbridge 路由的 `collectIntents` 中 service 存在性校验分支（合成 service 一定存在，不受影响）、禁用实例的跳过语义要出 warning 而非 error（与现有行为一致）。
- **FlowBridge 侧配合：** 无。

#### M4. bind-http-domain action 支持 flowbridge 目标

- **改动点：** `BindHTTPDomainInput` 增加可选 `FlowBridgeID string`；当设置时：校验实例存在且启用、跳过 endpoint 创建、route 带 `FlowBridgeID`、其余（合成 service、edge rule、safeApply、ownership）不变。日志消息区分两种目标。
- **涉及文件：** `internal/action/bind_http_domain.go`（+action_test）。
- **影响面：** 现有非 flowbridge 调用路径零变化（参数可选）。
- **风险：** 低。
- **FlowBridge 侧配合：** 无。

#### M5. 实例 CRUD + 健康检查 HTTP API

- **改动点：** `internal/httpapi/handlers/flowbridge.go` + `routes.go` 注册：
  - `GET /api/admin/v1/flowbridge`（列表，含健康状态）
  - `POST /api/admin/v1/flowbridge`（创建，**必须调用 `MarkPending()`**——admin mutation 规则）
  - `GET/PATCH/DELETE /api/admin/v1/flowbridge/{id}`；PATCH 支持 `enabled` 切换（同样 MarkPending）
  - `POST /api/admin/v1/flowbridge/{id}/check`（触发即时健康检查）
- **涉及文件：** `internal/httpapi/handlers/*`、`internal/httpapi/routes.go`、`internal/httpapi/server.go`（Services 加字段）、`cmd/aegis/main.go`。
- **影响面：** 新增路由，无存量影响。
- **风险：** 低。注意：管理端点一律走 admin 认证；若未来开放给 service ticket 需主动加白名单（默认关闭，符合「白名单是加法」规则）。
- **FlowBridge 侧配合：** 无（`/health` 免认证）。

#### M6. 实例健康检查器

- **改动点：** `internal/flowbridge/checker.go`：HTTP GET `http://<control_address>/health`，3s 超时，2xx 判 healthy；结果写回实例行（`last_health_status/latency/message/checked_at`）。触发方式：创建/启用时立即一次 + `POST /{id}/check` 手动 + 可选的周期循环（仿 `internal/sync` 或 certstore 的同步模式）。**只探控制端点，不探数据面业务 target。**
- **涉及文件：** 新建 `internal/flowbridge/checker.go`、`cmd/aegis/main.go`（goroutine 装配）。
- **影响面：** 独立于 `internal/health`（那是 service→endpoint 体系），两套并存，互不干扰。
- **风险：** 低。注意控制面地址为 loopback 场景（本机 flowbridge）直接可用；跨机场景 `/health` 免认证无鉴权问题。
- **FlowBridge 侧配合：** 无。

#### M7. 管理 UI

- **改动点：**
  - 新页面「FlowBridge 实例」列表 + 创建/编辑/启用/禁用/健康状态（状态点：healthy / unhealthy / unknown / disabled）。
  - `pages/exposure/NewEntry.tsx` 绑定表单：目标类型二选一「旧后端 IP:端口」/「FlowBridge 实例」（下拉选实例），提交仍走 `bind-http-domain`（带 `flowbridge_id`）。
  - 路由列表（`EntryList.tsx` 等）目标列支持显示「→ flowbridge: <name>」。
- **涉及文件：** `ui/src/pages/`（新页面 + NewEntry/EntryList 改动）、`ui/src/lib/real-api-client.ts`（fetch 函数）、`ui/src/types/`。
- **影响面：** 纯前端。
- **风险：** 低。
- **FlowBridge 侧配合：** 无。

#### M8. 文档同步

- **改动点：** CLAUDE.md 包清单加 `internal/flowbridge`、API 表加 flowbridge 端点、本文档状态改为已实施。
- **风险：** 无。

### 2.2 建议（同批实施，但可裁剪）

#### S1. Host 头透传真机验收 + （必要时）防御性 header_up

- 验收信号 A1 覆盖。若验收发现 Host 未透传（Caddy 版本行为差异），在 `planner.go` flowbridge 分支复用现有 ExtraHeaders 机制加 `header_up Host`（gateway-link 先例，`planner.go:212`）。
- **涉及：** 无代码（或 planner 两行）。**FlowBridge 侧配合：** 无。

#### S2. 创建实例时连通性预检

- 创建时（或 `POST /{id}/check`）对控制面 `/health` 做一次探活，不可达给 warning 但不阻断创建（实例可能稍后上线）。

#### S3. 路由详情 / 健康页展示 flowbridge 目标状态

- `GET /api/routes/{id}` 或 admin 路由列表 join 实例健康（`unhealthy` 实例的路由在 UI 高亮）。

#### S4. 顺手修复健康检查器地址解析隐患

- `internal/health/checker.go` 的 `parseAddress` 有短地址越界 panic 风险（见审查报告问题清单），改一行 `strings.HasPrefix`。此文件本来就在健康体系内，同批改成本最低。

#### S5. 实例启停联动 apply

- 启用/禁用实例仅改状态位，不自动触发 apply（避免无谓重载）；对绑定了该实例的路由，下次 Apply 时按 M3 语义生效。是否自动触发由 UI 交互决定（沿用现有「变更后手动 Apply」习惯）。

### 2.3 可选（推迟到二期）

| 编号 | 内容 | FlowBridge 侧配合 |
|---|---|---|
| O1 | 控制面 token 加密存储（`internal/credential`）+ `/status`、`/bindings` 拉取展示（实例健康之外的信息） | 需要：非 loopback 控制面配置 `auth_token` 并告知 Aegis |
| O2 | distnode 聚合：经现有 `/api/admin/v1/distnode/aggregate` 聚合各节点 flowbridge 实例状态 | 无 |
| O3 | flowbridge 实例数据面 TLS 直通（EdgeRule 指向 flowbridge 上的 TLS 监听） | 数据面需配 TLS |
| O4 | 实例自动部署 / SSH 下发 flowbridge 进程 | 无 |

### 2.4 明确不动的部分

- **Provider 渲染层（`internal/hostdep/provider/`）：** 不加 flowbridge 能力、不加新 composition。`https_route` 已足够，flowbridge 是上游来源差异，不是入口能力差异。
- **EdgeRule（`internal/edgemux/`）：** 不动。
- **Endpoint 模型：** 不加 flowbridge 子类型（避免污染「服务端点」语义；flowbridge 引用挂在 Route 上）。
- **`internal/health`：** 不动（service 健康体系保持 TCP 探测；实例健康独立实现）。

---

## 三、验收信号

前置：Server A 部署新二进制；flowbridge 实例（示例配置含 `a.test` binding，数据面监听可访问）。

| # | 信号 | 命令/操作 | 期望 |
|---|---|---|---|
| A1 | Host 头透传（核心） | `curl -s -H "Host: a.test" http://<AEGIS_80>/`（或 https 入口） | 命中 flowbridge 的 a.test binding，返回对应 upstream 响应（如 `X-FlowBridge-Service: a.test` 头或 App A 内容）；**不是** 502/错误页 |
| A2 | Host 透传负例 | `curl -s -H "Host: b.test" http://<AEGIS_80>/` | 命中 b.test binding（多 binding 场景）；`Host: nonexist.test` → 非 a.test 内容 |
| A3 | X-Forwarded-* | A1 响应所在 upstream 日志/响应头 | `X-Forwarded-Proto: http/https`、`X-Forwarded-For` 为客户端 IP |
| A4 | 实例健康 | `GET /api/admin/v1/flowbridge` | 实例 `last_health_status=healthy`（控制面 `/health` 200） |
| A5 | 禁用语义 | 禁用实例 → `POST /api/apply` | 绑定的域名不再转发到该实例；preview/apply 输出该路由 warning（如「route x: flowbridge instance y disabled」） |
| A6 | 绑定链路完整 | UI：创建实例 → NewEntry 选 flowbridge 绑定域名 → Apply | 域名可达（A1 同期望）；路由列表显示 flowbridge 目标 |
| A7 | 回滚/预览一致性 | `GET /api/config/preview` 与 `POST /api/apply` 前后对比 | 渲染的 Caddyfile 中该域名为 `reverse_proxy http://<machine_ip>:<data_plane_port>`，无其它意外变更 |
| A8 | keep-alive（可选） | 连续两次 curl + upstream 侧连接观察 | 复用同一连接（Caddy 默认行为，验证即可） |

---

## 四、不做的事（明确排除，二期也不做除非另行决策）

1. **域名多目标回退 / 负载分发**：Aegis 层一个域名恒一个目标；不配置 Caddy 多 upstream、不引入 lb 策略。flowbridge 内部的多 target / 切换 / 回退是 flowbridge 自己的事，Aegis 不参与、不感知。
2. **跨实例状态同步**：不调用 flowbridge 的 `/switch`、`/drain`、`/reload`；不在 Aegis 侧复制 flowbridge 的服务/目标/版本状态。
3. **业务级负载分发**：不做按域名的 round-robin、灰度权重、健康驱动的流量搬移。
4. **flowbridge 配置管理**：bindings / modules / listeners 仍由 flowbridge 自己的 YAML 管理，Aegis UI 不编辑、不展示其内部配置。
5. **数据面 TLS 直通**（O3 之前的默认）：flowbridge 数据面按明文 HTTP 接入；如需 TLS 由后续决策。
6. **实例自动部署**：Aegis 不负责安装 / 启动 / 升级 flowbridge 进程。

---

## 五、实施顺序建议（供第二步参考）

1. M1 实体（表 + 包 + 装配）→ 2. M2 Route 字段 + M3 planner 分支（**核心链路**，先做）+ 对应单测 → 3. M4 action → 4. M5/M6 handler + 健康检查器 → 5. M7 UI → 6. S1 真机验收（A1/A2/A3）→ 7. M8 文档。

风险排序：M3 最高（生产关键路径），M7 工作量最大，其余为常规 CRUD。所有改动保持「最小充分修改」：不重构 resolver、不动 provider 层、不动 EdgeRule。

---

## 六、实施记录（2026-08-01，M1-M8 完成）

| 项 | 内容 | 涉及文件 |
|---|---|---|
| M1 | 实例实体 | `internal/flowbridge/{model,repository,service,checker}.go`、迁移 048（`internal/store/migrations.go`）、`cmd/aegis/main.go` |
| M2 | Route.FlowBridgeID | 迁移 048（routes 加列 + 触发器）、`internal/route/{model,repository,service}.go` |
| M3 | planner flowbridge 分支 | `internal/topology/{planner,intent}.go` + `flowbridge_planner_test.go`（6 个用例，含「不回退 endpoint」「nil repo 降级」） |
| M4 | action 支持 flowbridge_id | `internal/action/{service,bind_http_domain}.go`；顺带删除步骤 8 死代码 |
| M5 | CRUD + check handler | `internal/httpapi/handlers/flowbridge.go`、`routes.go`、`server.go`、`handlers/system.go`（Handlers 字段） |
| M6 | 健康检查器 | `internal/flowbridge/checker.go`（GET 控制面 /health，3s 超时）+ main.go 30s 周期循环 |
| M7 | UI | `ui/src/pages/fabric/FlowBridge.tsx`（新页面）、`NewEntry.tsx`（后端类型二选一）、`EntryList.tsx`（flowbridge 标记）、`real-api-client.ts` / `api-bridge.ts` / `App.tsx` / `constants.ts` |
| M8 | 文档 | 本文档、`docs/flowbridge-dod.md`（收工标准）、`docs/flowbridge-review.md`、CLAUDE.md |
| 附加 | P1 修复 + 格式 | `internal/health/checker.go`（`addr[:5]` 越界 → HasPrefix）、`internal/edgemux/model.go`（缩进） |

**已知待办（部署环境项）：** 真机链路验收（A1：`curl -H "Host: a.test"` 命中 flowbridge binding）需在 VPS 上执行，本机已完成除真机外的全部验证（见 `flowbridge-dod.md` 验收记录）。
