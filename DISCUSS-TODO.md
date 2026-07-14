# Aegis 待讨论问题清单（临时文档）

> 创建于 2026-07-13。这些是**模糊 / 需要设计讨论**的问题，不适合直接开工。
> 集中讨论定方向后再执行。清晰可快速执行的项不在此文档，见对话中的「可执行清单」。

---

## B1. 模式切换拆 SwitchMode + 回滚

**现状**：网关模式切换（Legacy ↔ EdgeMux）寄生在 `apply/workflow.go` 的 `Apply()` 里，
通过 side-channel 变量 `targetMode` 触发。`Apply()` 有一个 `if w.targetMode != ""` 分支。

**问题**：
- 切换和普通 Apply 耦合，同一个方法两种行为
- 切到一半失败（旧 provider 已停、新配置没写）→ 系统卡在中间状态，无自动回滚

**待讨论**：
- 是否值得拆成独立 `SwitchMode()` 方法？
- 回滚需要快照什么？（provider 配置文件 + systemctl 运行状态）
- 验证用 `Diagnose()` 够不够，还是要发真实烟雾测试请求？
- 切换频率低（人工触发），投入产出比如何？

**涉及文件**：`internal/apply/workflow.go`、`internal/provider/mode_switch.go`、
`internal/httpapi/handlers/mode.go`

---

## B2. 域名解析逻辑合并（现在剩 2 套）

**现状**：「域名 → 后端地址」有 2 套独立实现（删 Design B 后从 3 套减到 2 套）：
- `internal/gateway/relay_resolver.go` — DAG 解析 + 模式选择（trace API 用）
- `internal/routingtable/generator.go` — 预计算路由表（apply 用）

**问题**：两者对「候选节点选择（local/private/public）」的逻辑重叠但代码不同。

**待讨论**：
- 两者消费方不同（一个给 trace 预览，一个给 apply 生成），能否真正合并？
- 还是只抽取共享的「候选选择」子逻辑成一个函数，两边都调？
- 合并的风险：apply 是生产关键路径，动它要非常小心

**涉及文件**：`internal/gateway/relay_resolver.go`、`internal/routingtable/generator.go`

---

## B3. deploy_node 的 provider-aware 部署

**现状**：`internal/httpapi/handlers/deploy_node.go` 里 SSH 到新节点执行
`apt-get install caddy` 是**写死 Caddy 的**。

**问题**：将来换 Nginx 或别的中间件，这段 SSH 命令不知道。违反 provider 抽象原则。

**待讨论**：
- provider 的安装逻辑（`LifecycleProvider.Install()`）目前是本机 `apt-get`，
  怎么变成「通过 SSH 在远程机器上安装」？
- 是在远程也跑一个 aegis，让它自己调 `LifecycleProvider.Install()`？
  （新节点部署时本来就装 aegis，可以让 aegis 自己装 provider）
- 还是把 provider 的「需要哪些包」暴露成元数据，deploy 读元数据生成 SSH 命令？

**涉及文件**：`internal/httpapi/handlers/deploy_node.go`、`internal/provider/install.go`

---

## B4. 半成品功能：接不接、接多少

以下功能「后端就绪，UI/跨节点没接」，需要确认是否要完成：

| 功能 | 后端 | 缺什么 | 要不要做 |
|------|------|--------|---------|
| ServiceAuth CallService 跨节点 | 同机转发就绪 | 跨节点未接 distnode Transport | ? |
| deploy 装中间件 | 写死 caddy | provider-aware（见 B3） | ? |

**待讨论**：
- ServiceAuth 跨节点调用：目前 `ServiceAuthCall` handler 只处理同机
  （查注册表 → 转发到 target:listen_port）。跨节点需要：
  发现 target 服务在哪个节点 → `Transport.Call(nodeID, "Aegis.ProxyRequest", ...)`。
  模式和 aggregate 一样，但要先在注册表里记录「服务在哪个节点」。
- 这个跨节点服务调用是否是真实需求？还是单机 + GatewayLink 已经够用？

---

## B5. 架构可复用性边界

**现状**：`internal/distnode/` 设计为零依赖可移植底座（规则强制）。
`internal/provider/` 是能力接口抽象。

**待讨论**：
- 是否要把 distnode 真正抽成独立 Go module，供其他项目 import？
  （现在是「复制目录」复用，不是「import 依赖」复用）
- provider 抽象是否要文档化成「如何接入新中间件」的指南？

---

## B6. 跨节点扇出的环路保护（反证审查发现）

**现状**：`fanOutToPeers` + `forwardServiceCall` 组成跨节点调用链。删掉的
Design B 里有 `MaxHopLimit = 1` 显式跳数限制，我没有替代它。

**触发条件（近乎不可能，但非零）**：A locate 到 B（陈旧数据）→ 转发给 B →
B 本地也查不到（微秒级注销竞态）→ B 又 locate → 可能弹跳。

**待讨论**：
- 你的场景（单管理员、小集群、服务注册后稳定）几乎不触发，是否值得加？
- 如果加：`forwardServiceCall` 注入 `X-Aegis-Hop` 头，>1 拒绝。修复入口
  唯一——就在 `fanOutToPeers` 那一层。

---

## B7. 多实例服务的跨节点路由非确定性（反证审查发现）

**现状**：`locateServiceNode` 返回「遍历到的第一个」有该服务的 peer。
`AlivePeers()` 顺序未定义。

**待讨论**：
- 只有「同名服务跑在多个节点」才有意义。你大概率一个服务一个节点。
- 若要多实例：需要负载均衡策略（轮询/一致性哈希），修复入口同样在
  `fanOutToPeers` / `locateServiceNode` 一处。

---

## 优先级建议（讨论时参考）

```
高价值:  B4 (半成品功能要不要完成) — 直接影响能不能用
中价值:  B3 (provider-aware 部署) — 影响多中间件扩展
中价值:  B1 (模式切换回滚) — 影响切换安全性
低价值:  B2 (解析合并) — 纯内部整洁，风险高收益低
低价值:  B5 (可复用抽 module) — 长期演进
```
