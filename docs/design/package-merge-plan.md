# 包合并计划

> 目标：把"每个名词一个包"压成"每个领域一个包"。
> 60 → 46（已完成）→ 目标 18~25

---

## 已完成（Wave 1）：60 → 46

| 合并 | 效果 | 风险 |
|------|------|------|
| `errors + sloglog + recovery + id + testutil` → `core` | 5→1 | 无 |
| `uri` → `addr` | 2→1 | 无 |
| `importcfg` → `config` | 2→1 | 无 |
| `consistency` → `maintenance` | 2→1 | 无 |
| `snapshot` → `deployment` | 2→1 | 无 |
| `gateway_link + relay + localgateway` → `gateway` | 4→1 | 命名冲突（已解决） |
| 删除空包 `proxy/` `quota/` | — | — |

**遇到的问题**：gateway 合并时 `Service`/`Repository`/`StatusActive` 各两套命名冲突，采用了 `LinkService`/`LinkRepository`/`LinkStatusActive` 前缀方案——但这只是机械前缀化，事后应该做语义重命名。

---

## 待做（Wave 2+）

### network → transport：5→1（最优先）

```
tcp/proxy.go          → transport/tcp.go
udp/proxy.go           → transport/udp.go
transparent/manager.go → transport/transparent.go
dns/server.go          → transport/dns.go
```

- 零命名冲突
- 零循环风险
- health 不合并（独立概念）
- endpoint 不合并（属于 routing 层）

**命名注意**：叫 `transport` 不叫 `network`——`network` 太大容易变垃圾桶。

### auth 家族：5→1

```
token + adminauth + serviceauth + credential + secrets → auth/
```

- 合并条件：共享同一套认证语义 + 身份模型
- nodeauth 归 `auth/node.go`（如果算身份认证）或 `node/auth.go`（如果算节点注册）
- 注意：不要机械前缀化，要做语义重命名

### routing 家族：先修循环再合并

```
route + routingpolicy + routingtable + safety
```

**阻塞问题**：`topology → safety → nodestate → topology` 循环。
合并不能解决循环——需要先抽离快照接口，让 routing 只依赖数据快照而非完整实现。

### node 家族：最后做

```
node + nodeagent + nodeauth + nodestate + noderuntime
```

**阻塞问题**：
- `Service`/`Repository`/`HeartbeatResponse` 各两套命名冲突
- node 要先定义所有权边界：只负责"节点是什么、当前怎么样、如何和 agent 通信"
- 不负责网关路由、服务暴露、Provider 配置生成
- 需要用语义命名替代前缀命名

---

## 原则

1. **不是"每个名词一个包"，是"每个稳定领域一个包"**
2. **不为了压缩而制造巨型包**
3. **合并前先看 import 图，不能把循环藏进包内部**
4. **命名冲突时做语义重命名，不用机械前缀**
5. **"为什么不能合并"比"为什么能单独存在"更重要**

## 验收标准

- import 图单向，非网状
- 新增功能不跳 8 个包
- 包内类型名语义化（`Registry` 非 `Service`）
- 最终 18~25 个 internal 包
