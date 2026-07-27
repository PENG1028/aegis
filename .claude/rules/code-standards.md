# Aegis 代码规范 — 防分叉 + 防补丁

> **自动加载范围：** `internal/**`, `cmd/**`
>
> **目的：** 防止 agent 写出补丁代码、重复逻辑、不一致的实现。

---

## 零分叉规则

### 1. 查 Provider 能力：用 Capability，不用名字

```go
// ❌ 分叉
if p.ID == "caddy" || p.Name == "Caddy HTTP" { ... }
caddy := reg.Get("caddy")

// ✅ 统一
p.HasCapability(provider.CapRouteHost)
// 或从 CompDef.Requirements() 遍历
```

### 2. 查端口：用 RuntimeMode，不写数字

```go
// ❌ 分叉
port := 8443
port := 443

// ✅ 统一
mode.ListenerSpecsFor(providerID)
// 或 listener.EdgeMuxDefaults()
```

### 3. 查组合能力可用性：读 Status，不重算

```go
// ❌ 分叉
for _, comp := range provider.AllCompositions() {
    // 自己再算一遍 Provider 能力
}

// ✅ 统一
mode.EvalAllCompositions(states)
c.Status  // 已经算好了
```

### 4. 前端展示能力数据：读 API，不硬编码

```ts
// ❌ 分叉
const compositions = ['HTTP Route', 'HTTPS Route', ...]

// ✅ 统一
runtimeModeApi.get()       // 模式 + 原子矩阵
compositionApi.list()      // 组合定义
transparentApi.status()    // 透明代理诊断
providerApi.list()         // Provider 列表
```

### 5. 不重复实现工具函数

```go
// ❌ 分叉——每个包自己写一遍
func containsStr(slice []string, s string) bool { ... }
func contains(slice []string, item string) bool { ... }

// ✅ 统一——用标准库
slices.Contains(slice, item)
```

### 6. HTTP 状态码映射只写一次

```go
// ❌ 分叉——三个 handler 各写一遍 switch ae.Code
// ✅ 统一——所有 handler 调 writeActionError(w, err)
```

### 7. 诊断流程参数化，不复制粘贴

```go
// ❌ 分叉——quickDiagnoseCaddy 和 quickDiagnoseHAProxy 各 55 行
// ✅ 统一——diagnoseExternal(providerID, serviceName, configPath, versionFlag, validateArgs, versionOK)
```

---

## 文件组织规则

| 原则 | 说明 |
|------|------|
| 一个模式一个文件 | `mode_legacy.go` / `mode_edgemux.go`，不加到 `runtime_mode.go` 里 |
| 接口+类型在根 | `provider.go` + `model.go` |
| 实现在子文件 | `caddy.go` / `caddy_render.go` / `haproxy.go` / `haproxy_render.go` |
| 能力定义独立 | `capability.go` 只有常量 + Layer()/IsIngress() |
| 组合定义独立 | `composition.go` 只有 CompDef 注册表 |

---

## Go 语言规范

- 用 `slices.Contains` 不用自定义 `contains`
- switch 必须有 `default` 分支（即使空操作也要写注释）
- 回滚路径的 `_` 错误必须改为 `log.Printf`
- 文件权限用常量不用裸数字：`ConfigFilePerm` 非 `0640`
- `time.Parse` 的错误不能吞掉——至少 `log.Printf`

---

## 前端规范

- 组合流卡片状态从前端 `compCardStatus()` → 后端 API `comp.status`（已完成）
- 透明代理转发目标从后端 `forward_targets` 读（已完成）
- 不加新的 `const MODES = [...]` 或手写能力数组
