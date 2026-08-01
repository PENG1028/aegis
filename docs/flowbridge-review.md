# FlowBridge 配合调研审查报告

> 日期：2026-08-01（第一阶段只读调研）。本报告为控制台交付物的落盘版：差距说明、提交状态盘点、注释/结构/小问题三张清单，以及第二步必须改/可推迟的划分。所有结论均基于代码阅读与官方文档核实，未做任何代码改动。

---

## 一、当前系统与配合目标的差距

### 1. 模型差距（核心）

当前链路是两级映射：`Route.Domain → ServiceID → Endpoint（local/private/public）→ 地址`，另有一个 Gateway Link 覆盖分支。FlowBridge 配合需要**一级映射**：`Route.Domain → flowbridge 实例`。

- 不存在 flowbridge 实例实体（无表、无包、无 API、无 UI）。
- Route 无 flowbridge 引用字段；`routes.service_id` 为 NOT NULL（`internal/store/migrations.go:418`），绑定必须沿用合成 service 模式，不能简单把 ServiceID 置空。
- 上游决策点集中在 `internal/topology/planner.go` 的 `resolveIntents()`（endpoint 解析 → gateway link 覆盖），是加入 flowbridge 分支的唯一合理位置。

### 2. EdgeRule 落点核实

**结论：EdgeRule 不是落点，Route 是。** EdgeRule（`internal/edgemux`）是 SNI（L5）层直通规则，HAProxy 按 ClientHello SNI 转发原始 TCP 流，无 L7、无 Host 解析、不终止 TLS。HTTP 路由的自动 edge rule 只是固定 `SNI → 127.0.0.1:8443`（`edgemux/service.go:73-119`）。FlowBridge binding 依赖 Host 头（L7），必须走 Route → Caddy TLS 终止 → 转发。EdgeRule 层零改动。

### 3. 转发约束差距（比预期小）

| 约束 | 现状核实 | 差距 |
|---|---|---|
| Host 头原样转发 | Caddy 官方文档：默认透传所有入站头（含 Host），明文 HTTP upstream 不受 v2.11 Host 改写影响；gateway-link 已有 `header_up Host` 防御性先例（`planner.go:212`） | **无代码差距**，需真机验收（验收信号 A1/A2） |
| X-Forwarded-Proto/For | Caddy 自动附加，默认防伪造 | **零差距** |
| keep-alive | Caddy http transport 默认 keepalive 2m、32 conns/host | **零差距** |
| 健康探针指向 flowbridge 控制 `/health` | 现状只有 service→endpoint 的 TCP connect（`internal/health/checker.go`） | **必改**：新增实例级 HTTP 探活 |

### 4. FlowBridge 侧接口事实

控制面 `/health` **免认证**（`flowbridge/internal/control/control.go`），探活可直接用；`/status`、`/bindings` 等需 Bearer token（非 loopback 强制）→ 二期 O1。目标切换 `/switch`/`/drain` 是 flowbridge 内部操作，Aegis v1 不调用。

---

## 二、提交状态盘点

### 分支与提交

- 当前分支：`codex/cert-lifecycle` @ `1bb39f6`（v1.9C 系列，31 个提交），**与 `origin/codex/cert-lifecycle` 同步（up to date，无未推送提交）**。
- 相对 `origin/main` 领先 31 个提交（v1.9C 证书生命周期 + 近期修复，全部已推）。
- 另有本地分支 `deploy/hy2-list`，也已推送 `origin/deploy/hy2-list`。

### 工作区状态

| 状态 | 内容 |
|---|---|
| 唯一未提交改动 | `deleted: aegis_linux_amd64`（25MB 已跟踪二进制被删除） |
| 未跟踪且被 .gitignore 忽略 | `_notes/`（6 个调研草稿文件）、`.git-rewrite/`（filter-branch 残留）、`acme/`、`data/`、构建产物（aegis*.exe 等）、`ui/node_modules/`、`internal/uiassets/dist/` |
| 工作区是否干净 | 基本干净：仅上述二进制删除，无任何代码改动 |

### 遗留痕迹（只列不改）

1. **`aegis_linux_amd64` 被 git 跟踪**：该文件同时存在于 `.gitignore` 中（说明是加入 ignore 之前被误提交的构建产物），现工作区已删除。建议第二步顺手 `git rm --cached` 从历史剔除跟踪（不删历史）。
2. **`_notes/`**：`api-key-pattern.go`、`authz-patterns.go`、`go-auth-comparison.go`、`go-http-patterns.go`、`serviceauth-architecture.md`、`serviceauth-next.ts` —— 调研/设计草稿，被忽略未提交。属个人笔记，无动作。
3. **`.git-rewrite/`**：git filter-branch 运行残留，已忽略。无动作。
4. **`DISCUSS-TODO.md`**（已跟踪）：待讨论问题清单，其中 B3 引用 `internal/provider/install.go`（该包已删除）——文档漂移，见问题清单 C3。

> 注：本机 PowerShell 控制台为 GBK 代码页，UTF-8 中文文件在控制台显示乱码是**显示问题**，非文件损坏（用 UTF-8 读取正常）。

---

## 三、问题清单

### A. 注释 / 文档需要补充或已过时

| # | 位置 | 问题 | 建议 |
|---|---|---|---|
| C1 | `ARCHITECTURE.md`（全文） | 引用已删除包：`internal/provider/`、`internal/lifecycle/`、`internal/apply/planner.go`（现为 `internal/hostdep/provider/` + `internal/topology/planner.go`）；能力数量、目录结构均与现状不符 | 重写或标注「历史设计，代码已演化」 |
| C2 | `README.md` | 版本号写 `v1.7AD`（现状 v1.9C）；「不是」清单含「多节点分布式网关」，与 v1.9B distnode 已实现矛盾 | 更新版本与定位描述 |
| C3 | `DISCUSS-TODO.md` B3 | 引用 `internal/provider/install.go`（已删除） | 更新路径 |
| C4 | `internal/route/model.go:18` | `TLSEnabled` 标注 deprecated，但仍被 `CompDef()`（:48）作为回退分支使用，注释未说明回退语义 | 补充「仅作为旧数据回退」说明 |
| C5 | `internal/action/bind_http_domain.go:128-144` | 步骤 8 注释声称「Set ownership on the auto-created edge rule」，实际代码是死操作（赋值本地副本后 `_ = edgeRule`，归属从未持久化） | 注释删除或实现真实归属同步 |
| C6 | `internal/edgemux/model.go:78` | `if net.ParseIP(host) != nil {` 前导缩进错乱（7 个 tab） | 格式化 |

### B. 结构可能需要调整

| # | 位置 | 问题 | 级别 |
|---|---|---|---|
| S1 | `internal/topology/planner.go` `resolveIntents()` | 上游来源决策点目前有 endpoint + gateway-link 两处逻辑叠加；flowbridge 加入后必须在此**单点**新增分支，禁止在渲染层或其它地方再实现第三套 | 必须（新逻辑唯一入口）；现有两分支不重构 |
| S2 | `internal/health/checker.go:135` vs `internal/endpoint/resolver.go:134` | 地址解析重复实现两份（`parseAddress` / `parseHostPort`），且 `internal/addr.Parse` 已是统一抽象（`internal/addr/addr.go`） | 建议：实施期顺手收敛 health 侧到 addr 包 |
| S3 | `internal/action/bind_http_domain.go` | 死代码（见 C5）；action 层创建 service 用「反射式」注释描述的 `CreateServiceDirect` 直连，绕过 service 校验，是长期权宜 | 建议：flowbridge 绑定复用同一模式，不新增旁路 |
| S4 | `internal/route/model.go` | 字段累积版本标记（v1.7AB / v1.8L-22 / v1.9C / v2.0-RPCB），无统一目标模型视图 | 建议：flowbridge 字段沿用现有模式（带版本注释），不借机重构 |
| S5 | `DISCUSS-TODO.md` B2 | 指出 `internal/gateway/relay_resolver.go` 与 `internal/routingtable/generator.go` 两套域名解析逻辑重叠，仍未决 | 建议：flowbridge 解析**不得**新增第三套，只进 planner.resolveIntents |

### C. 小问题（按严重程度排序）

| # | 严重度 | 位置 | 问题 |
|---|---|---|---|
| P1 | 中低（潜在 panic） | `internal/health/checker.go:147` | `addr[:5] == "https"` 对 <5 字符的地址切片越界 panic（endpoint 地址为用户可配的 hostname）。同行 `endpoint/resolver.go:144` 用的是安全写法 `strings.HasPrefix`。触发概率低（需无端口短 hostname 地址），修复成本一行 |
| P2 | 低 | 仓库 | 已跟踪二进制 `aegis_linux_amd64` 被误提交（现已被 .gitignore 忽略但跟踪仍在），当前工作区已删除该文件——需 `git rm --cached` 清理跟踪 |
| P3 | 低 | `README.md` / `ARCHITECTURE.md` | 版本号与实现描述漂移（见 C1/C2） |
| P4 | 低 | `internal/edgemux/model.go:78` | 缩进错乱（见 C6） |
| P5 | 信息 | 根目录 | `_notes/`、`.git-rewrite/` 残留（已忽略，无动作） |

### D. 测试缺口（与 FlowBridge 相关）

1. Caddy 渲染无 Host 透传断言——flowbridge 的核心依赖（Host 匹配）目前只靠 Caddy 默认行为，需真机验收（A1/A2）或补 render 断言测试。
2. `internal/health` 无短地址输入测试（P1 场景）。
3. flowbridge 实体/解析分支的单元测试随 M1-M4 新建。

---

## 四、第二步必须改 vs 可推迟

### 必须（按实施顺序）

1. **M1** 实例实体（表 048 + `internal/flowbridge` 包 + main.go 装配）
2. **M2** Route.FlowBridgeID（迁移 + model + repo）
3. **M3** planner.resolveIntents 的 flowbridge 分支（**核心**，先写单测再改）
4. **M4** bind-http-domain 支持 flowbridge_id
5. **M5** CRUD + check handler（MarkPending 规则）
6. **M6** 实例健康检查器（GET 控制面 /health）
7. **M7** UI（实例页 + NewEntry 二选一 + 路由列表展示）
8. **M8** 文档同步（CLAUDE.md + 本文档状态）
9. **S1** 真机验收 A1-A8

### 可推迟（二期或按需）

- S4 地址解析去重（P1 修一行可提前，属于顺手项）
- O1 控制面 token 加密存储 + /status 展示（需 FlowBridge 侧配 token）
- O2 distnode 聚合实例状态
- O3 数据面 TLS 直通、O4 实例自动部署
- P2 仓库清理（`git rm --cached aegis_linux_amd64`，随时可做）
- C1/C2 文档重写（可与 M8 同批或独立）

---

*本报告对应交付：`docs/flowbridge-integration.md`（改动需求文档）。第二阶段实施以该文档 M1-M8 + S1 为准。*
