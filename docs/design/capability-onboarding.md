# 能力 / 组合 / Provider 接入清单

加东西时**必须一起改的地方**。漏改的后果多数不是编译错误，而是 UI 显示一个执行不了的能力，
或模式切换预览少报一步迁移动作。

三条独立的接入路径，按代价从小到大。

---

## 一、加一个能力（Capability）

能力是协议原语，`internal/hostdep/provider/capability.go` 的封闭枚举。

| # | 改哪里 | 漏改的后果 |
|---|--------|-----------|
| 1 | `Capability` 常量 | — |
| 2 | `AllCapabilities()` 加一行 `CapabilityDef` | 能力矩阵 UI 静默少一行（`TestEveryCapabilityKeyInEnumerationParses` 会失败） |
| 3 | `SemanticsOf()` **如果它带状态** | 落到 default = `declarative`/`re_render`，模式切换预览承诺"纯改配置"，漏掉资产重载或状态重建 |
| 4 | 至少一个 provider 的 `xxxCapabilities()` | 无人声明 = 永不可用 |
| 5 | 对应 `xxx_render.go` 真的渲染出东西 | **声明但不渲染** → `HasCapability` 与 UI `capabilityIsReady` 都报就绪，实际无效（`TestNoProviderDeclaresUnimplementedCapability` 会失败） |

第 3 步是最容易漏的。判断标准：**这个能力是否消费或产出磁盘/进程里的东西**。

- 只影响生成的配置文本 → `declarative` / `re_render`，走 default 即可
- 消费磁盘上的文件（证书、CA 包） → `portable_asset` / `reload_asset` / `AffinityNone`
- Provider 自己管的状态（ACME 账户与私钥） → `provider_managed` / `recreate` / `AffinityExecutorSticky`
- 端口与监听 → `runtime` / `restart` / `AffinityBundleSticky`

`portable_asset` 与 `provider_managed` 的区别决定迁移能否跨执行器：文件能搬走，
Provider 内部状态搬不走——所以后者的绑定是执行器粘性的，换模式要重建。

### 尚未实现的能力

`CapMTLSTerminate`、`CapTLSMasquerade` 已在枚举里但**无 provider 声明、无渲染实现**，
且任何 Composition 都到不了。它们的键先存在，是为了让语义和本文档可以引用。

要启用其中一个：渲染它 → 从 `capability_guardrail_test.go` 的 `unimplementedCapabilities` 移除 →
在 provider 的 `xxxCapabilities()` 声明。三步必须同一次改完。

---

## 二、加一个组合（Composition）

组合是用户能选的端到端形态，`AllCompositions()` 是唯一来源，6 条。

| # | 改哪里 | 说明 |
|---|--------|------|
| 1 | `CompKey` 常量 + `AllCompositions()` 加 `CompDef` | 单点，两个 Lookup 函数自动跟上 |
| 2 | **每个** `RuntimeMode.Providers` 的 atom 绑定表 | 手写映射，每模式一份；不填 = 该模式下组合不可用 |
| 3 | `routeExecutionForMode()` 的 required-capability switch | 决定这个组合需要哪个能力，进而决定目标执行器 |
| 4 | 渲染层 | 真的产出配置 |

第 2 步是真正的成本：`RuntimeModeLegacy` 与 `RuntimeModeEdgeMux` 各有一份手写的
`map[string][]AtomSlot`。加组合要在两处都填，`nil` 表示该模式下此 atom 不由这个 provider 承担。

---

## 三、加一个 Provider

最贵的一条。`"caddy"` / `"haproxy"` 字面量在非测试代码里出现 **79 处、跨 20+ 文件**。

真正设计好的缝只有两个：`Registry.Register()`（`cmd/aegis/main.go:201-202`）和 `AllCompositions()`。
其余要逐个处理。

| # | 改哪里 | 说明 |
|---|--------|------|
| 1 | 实现 `Provider` 接口 | 必需 |
| 2 | 实现 `ConfigStager` + `ServiceController` | **模式切换的隐式硬要求**——`SwitchMode` 对二者做类型断言，缺任一则该 provider 无法参与任何模式切换（`workflow.go` 里是明确的 error，不是 panic） |
| 3 | `xxxCapabilities()` | 声明必须与渲染一致，见第一节第 5 步 |
| 4 | `Registry.Register()` | `main.go` |
| 5 | 相关 `RuntimeMode.Providers` 加 `ProviderAtoms` | 不加则新 provider 不属于任何模式，`DetectRuntimeMode` 永不选中 |
| 6 | `capability_guardrail_test.go` 的 `known` 列表 | `TestRuntimeModeProvidersAreConstructible` 会失败提醒 |
| 7 | `providerLabel()`（`mode_switch.go`） | 当前是 `return id`，UI 直接显示裸 ID；加中文名就在这里 |
| 8 | `certstore/caddy_discover.go` 类似物 | 若新 provider 自管证书，自动证书发现需要对应实现 |

### 模式检测的前提

`DetectRuntimeMode()` 只看 provider 是否 `Installed && Running && 无错误`，**不校验配置内容**，
并在满足的模式中选 provider 数最多的那个。这意味着新 provider 一旦装上并运行，
就会参与模式判定——即使它的配置不属于任何模式。

`SwitchMode` 末尾有 `DetectRuntimeMode` 回验，切换那一刻是安全的。但切换之后
（systemd 残留、手工重启）没有守卫：`DetectDrift()` 只比对路由集合，不校验模式身份。

---

## 四、证书与模式的交叉

绑定证书时链路是：`tlslifecycle.BindCertificate` → 检查 `providerForCapability(CapLoadCert)` →
`certstore.CoversDomain`（RFC 6125 单标签通配）→ `SetTLSBinding`。

自动 TLS 是**执行器粘性**的：`UseProviderAuto` 拒绝把已绑定的 `provider_auto` 换到别的执行器，
换执行器只能走显式迁移流程。这是有意的——ACME 账户与私钥在 Provider 内部，搬不走。

因此 `routeExecutionForMode()` 有一处优化：目标模式里同一个执行器仍能提供 `CapAutoCert` 时，
迁移策略从 `recreate` 降级为 `re_render`。legacy ↔ edge_mux 两边证书都归 caddy，
所以实际不需要重新签发——只是 caddy 从 `:443` 换到 `:8443`。

### 不相干的两个子系统

`ManagedDomain` 与 `Route` / `certstore` **没有引用关系**。受管域名只做 DNS 归属验证，
`EnableDomain` 只翻转 `Status`，不建路由、不签证书。`ManagedDomain.TLSStatus`
恒为 `not_requested`，不是活状态（见 `manageddomain/model.go` 的字段注释）。

要把两者接起来，需要先有签发路径，那是一条新功能，不是补一个字段。
