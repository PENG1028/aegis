# Provider Architecture — 三大真理源 + 红线规则

> **自动加载范围：** `internal/provider/**`, `internal/topology/**`, `internal/apply/**`, `internal/httpapi/handlers/transparent.go`, `internal/httpapi/handlers/system.go`
>
> **目的：** 防止 agent 写出分叉代码。当你要加"判断能不能用"的逻辑时，先读这个文件。

---

## 三大真理源（只能从这三处读，不能自己另算）

### 真理源 A：组合能力注册表 — "能做哪些事"

**文件：** `internal/provider/composition.go`

6 个 CompDef：HTTP Route / HTTPS Route / TLS Passthrough / HTTP/3 / Raw TCP Forward / Raw UDP Forward

每个 CompDef 定义了：
- `Atoms`（原子链）→ `Requirements()`（Capability 列表）→ `IsTransparentForwardTarget()`（是否可作为透明代理入口）

**消费方式：**
```
查"能不能用" → mode.Compositions[i].Status（已由 EvalAllCompositions 计算）
查"需要什么能力" → CompDef.Requirements()
查"谁是转发入口" → 所有 Compositions 都是，过滤靠 Status
```

### 真理源 B：能力常量 — "Provider 能做什么"

**文件：** `internal/provider/capability.go`

30 个 Capability 常量，L3-L7 分层。每个 Provider 声明自己的 `[]Capability`。

**消费方式：**
```
查"Provider X 能做 Y 吗" → p.HasCapability(CapXXX)
查"协议类型需要什么能力" → registry.protocolCaps[protoType]
查"模板匹配" → RequiredCapabilities() vs p.HasCapability()
```

### 真理源 C：运行时模式 — "端口怎么分"

**文件：** `internal/provider/runtime_mode.go`（类型+检测）+ `internal/provider/mode_legacy.go` + `internal/provider/mode_edgemux.go`

定义每个模式下哪个 Provider 在哪个端口提供哪个原子。

**消费方式：**
```
查"Provider 监听哪个端口" → mode.ListenerSpecsFor(providerID)
查"是 Legacy 还是 EdgeMux" → DetectRuntimeMode(states)
查"某个原子的端口" → mode.PortFor(providerID, atomKey)
```

---

## 红线规则（永远不能做的事）

### ❌ 硬编码 Provider 名

```go
// ❌ 永远不要
if p.ID == "caddy" { ... }
caddy := registry.Get("caddy")
tlsProvider = "caddy"

// ✅ 应该用
p.HasCapability(CapXXX)  // 按能力找，不按名字找
```

### ❌ 硬编码端口号

```go
// ❌ 永远不要
port = 8443
httpsPort = 8443
target := "127.0.0.1:8443"

// ✅ 应该用
mode.ListenerSpecsFor("caddy")  // 从模式读
listener.EdgeMuxDefaults()      // 从监听器定义读
```

### ❌ 手写能力判断

```go
// ❌ 永远不要
if p.HasCapability(CapRouteHost) && p.HasCapability(CapUpstreamTCP) { ... }

// ✅ 应该用——同一个判断逻辑
compDef := LookupComp(CompHTTPSRoute)
for _, cap := range compDef.Requirements() { ... }
// 或者直接用 mode.Compositions[i].Status
```

### ❌ 自己另写一份组合能力判断

```go
// ❌ 永远不要——这会分叉！
for _, comp := range AllCompositions() {
    if comp.IsTransparentForwardTarget() {
        // 自己算状态
    }
}

// ✅ 应该用——已经在真理源 A 算好了
mode.EvalAllCompositions(states)
for _, c := range mode.Compositions {
    // c.Status 已经有值
}
```

### ❌ 前端硬编码能力列表

```ts
// ❌ 永远不要
const MODES = [{ id: 'legacy', columns: [...] }]

// ✅ 应该用
runtimeModeApi.get()  // 从后端 API 读
compositionApi.list()  // 从后端 API 读
transparentApi.status()  // 从后端 API 读
```

---

## 正确的消费模式（改了这里，别的地方自动跟）

| 需求 | 入口 | 自动跟新 |
|------|------|---------|
| 加新组合能力 | `AllCompositions()` 加一条 | 诊断表 + 透明代理 + Planner 全自动 |
| 加新模式 | `AllRuntimeModes()` 加一条 | 前端模式切换 + API + 模板全自动 |
| 加新 Provider | `main.go` Register | 能力查询 + 协议选择全自动 |
| 加新原子 | `AllAtomsInDisplayOrder()` | 矩阵渲染全自动 |
| 透明代理加新转发目标 | 不需要改——所有组合自动出现 | ✅ |
