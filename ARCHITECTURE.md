# Aegis 三维度架构设计

> v1.8L 重构 · 2026-07-03

---

## 一、问题

当前代码三个职责搅在一起：把中间件包成统一接口（维度 1）、决定多个中间件怎么组合（维度 2）、管理中间件的安装启停（维度 3），全部混在 `internal/provider/` 和 `internal/apply/` 里。任何一个改动横跨 3-4 个文件，每个文件都跨了维度边界。

---

## 二、三个维度

```
┌──────────────────┐   ┌──────────────────┐   ┌──────────────────┐
│   维度 3          │   │   维度 2          │   │   维度 1          │
│   生命周期管理    │   │   拓扑组合决策    │   │   Provider 适配   │
│                  │   │                  │   │                  │
│  · install       │   │  · Intent        │   │  · Capabilities  │
│  · uninstall     │   │  · Planner       │   │  · Render        │
│  · start/stop    │   │  · Template      │   │  · Apply         │
│  · status        │   │  · Fallback      │   │  · Diagnose      │
└────────┬─────────┘   └────────┬─────────┘   └────────┬─────────┘
         │                      │                      │
         └──────────┬───────────┴──────────┬───────────┘
                    ▼                      ▼
              ProviderState           ProviderState
              (写：维度 3)            (读：维度 1 + 2)
```

### 维度 1：Provider 适配层

**职责**：把 Caddy / HAProxy / Nginx 包成统一形状，让上层不关心底层是谁。

**边界**：
- 只管单个中间件。不管"Caddy + HAProxy 怎么组合"——那是维度 2。
- 只管能力声明 + 配置生成 + 配置下发。不管下载安装——那是维度 3。

**核心接口**：

```go
type Capability string  // 26 个常量，按 L3-L7 网络层级分组

type ProviderState struct {
    ID           string
    Name         string
    Installed    bool
    Running      bool
    Version      string
    BinaryPath   string
    Capabilities []Capability
    Ports        []PortBinding
    Diagnostic   *Diagnostic
}

type Provider interface {
    State() ProviderState
    Diagnose() Diagnostic
    Render(plan Plan) ([]ConfigFile, error)
    Apply(configs []ConfigFile) error
}
```

**一个中间件 = 一个 Provider**。HAProxy 只有一个 `HAProxyProvider`，不按配置文件拆分（`haproxy_edge_mux` + `haproxy_tcp` 是历史错误）。

**当前已知的 Provider 映射**：

| Provider ID | 中间件 | 代表能力 |
|---|---|---|
| `caddy` | Caddy v2+ | listen_tcp, tls_terminate, auto_cert, http1, http2, http3, websocket, grpc, sse, route_host, route_path, upstream_http, upstream_unix, hot_reload, validate_config |
| `haproxy` | HAProxy 1.8+ | listen_tcp, tls_passthrough, tls_terminate, sni_preread, raw_tcp, upstream_tcp, health_check, hot_reload, validate_config |
| `nginx` | Nginx（预留） | listen_tcp, listen_udp, tls_terminate, tls_passthrough, sni_preread, raw_tcp, raw_udp, http1, http2, websocket, grpc, route_host, route_path, upstream_http, upstream_tcp, upstream_udp, upstream_unix |

---

### 维度 2：拓扑组合决策

**职责**：给定当前安装了哪些中间件，用户的流量需求能用什么方案实现？

**边界**：
- 只管组合决策。不管具体中间件怎么生成配置——那是维度 1。
- 只管流量路径规划。不管中间件的安装升级——那是维度 3。

**核心流程**：

```
GatewayIntent（用户声明：我要什么）
    │  transport: tcp, tls: passthrough, sni: app.example.com, upstream: 127.0.0.1:3000
    ▼
requirementsOf(intent) → []Capability
    │  [listen_tcp, tls_passthrough, sni_preread, upstream_tcp]
    ▼
Planner.Plan(intent)
    │  1. 收集所有 healthy Provider 的能力
    │  2. 遍历拓扑模板，找匹配的
    │  3. 无完全匹配 → 生成降级方案
    ▼
TopologyPlan
    ├─ primary:      { template: "haproxy+caddy", providers: [...], listeners: [...] }
    └─ alternatives: [{ level: 0, template: "nginx+caddy" }, { level: 2, template: "dedicated_ports" }]
```

**拓扑模板（预置的组合方案）**：

| 模板 | 中间件组合 | 适用场景 |
|---|---|---|
| Single Caddy | Caddy :80, :443 | Plain HTTP, HTTPS 终止, WebSocket, gRPC |
| Single Nginx | Nginx :80, :443, stream | HTTP + TCP + UDP 全覆盖 |
| Single HAProxy | HAProxy :80, :443, TCP | TCP/TLS 为主，高强度代理 |
| HAProxy + Caddy | HAProxy :443 SNI → Caddy :8443 | 443 混合 HTTP + TCP/TLS passthrough |
| Nginx + Caddy | Nginx stream :443 → Caddy :8443 | HAProxy 缺失时的等价替代 |
| Caddy + Aegis TCP | Caddy :80/:443 + Aegis TCP 独立端口 | 无 SNI preread 时的降级方案（TCP 放独立端口） |
| Full Stack | HAProxy :443 + Caddy :8443 + Aegis UDP :443 | HTTP/3 QUIC + TCP passthrough 全场景 |

**4 级降级**：

| Level | 含义 | 示例 |
|---|---|---|
| 0 | 等价替代，换中间件功能不变 | HAProxy SNI → Nginx stream_ssl_preread |
| 1 | 功能等价，运维变差 | Caddy auto_cert → Nginx + certbot 手动 |
| 2 | 功能降级，仍可运行 | 放弃 443 复用，TCP 放独立端口 |
| 3 | 不可实现，必须安装新中间件 | 要求 UDP/443 HTTP3 但无任何 QUIC Provider |

**核心接口**：

```go
type GatewayIntent struct {
    Routes []RouteIntent
}
type RouteIntent struct {
    Port        int
    Transport   string         // "tcp" | "udp"
    TLSMode     string         // "none" | "terminate" | "passthrough"
    Match       MatchSpec      // { sni, host, path, alpn, port, src_ip }
    AppProtocol string         // "http" | "grpc" | "websocket" | "raw"
    HTTPVersion string         // "h1" | "h2" | "h3" (可选)
    Upstream    UpstreamSpec   // { type: "tcp"|"udp"|"unix"|"http", target }
}

type Planner interface {
    Plan(intent GatewayIntent) (*TopologyPlan, error)
}
```

**Planner 输出给透明代理的转发目标**：

```go
type ForwardTarget struct {
    Host string  // "127.0.0.1"
    Port int     // 80 (Caddy) 或 8443 (Caddy EdgeMux) 或 8080 (Nginx)
}
```

透明代理不需要知道 Caddy 存在。它只需要知道"跨节点流量转发到 `127.0.0.1:80`"，这个值由 Planner 根据当前可用的 Provider 动态决定——任何声明了 `[route_host, upstream_tcp]` 的 Provider 都可以作为转发目标。

---

### 维度 3：生命周期管理

**职责**：安装、更新、卸载、启停、插件管理、状态展示、配置查看。

**边界**：
- 只管单个中间件的运维操作。不管组合——那是维度 2。
- 只管操作执行 + 状态上报。不管流量——那是维度 1 和 2。

**包含的操作**：

```
安装/卸载:    Install() / Uninstall()     → apt-get + systemctl
更新:        Update()                     → apt-get upgrade
运行控制:    Start() / Stop() / Restart() / Reload()
状态查询:    Status()                     → ProviderState
配置查看:    GetConfig()                  → 当前配置文件内容
诊断:        Diagnose()                   → 完整诊断报告
插件管理:    ListPlugins() / InstallPlugin() / RemovePlugin()（预留）
```

**核心接口**：

```go
type Manager interface {
    Install(providerID string) error
    Uninstall(providerID string) error
    Start(providerID string) error
    Stop(providerID string) error
    Reload(providerID string) error
    Status(providerID string) (ProviderState, error)
    Diagnose(providerID string) (Diagnostic, error)
}
```

---

## 三、透明代理与三维度的交叉

### 当前问题

透明代理硬依赖 Caddy :80：

```go
// manager.go:87 — 写死了具体中间件
targetPort = 80  // 换了 Nginx 就断
```

### 正确的抽象

透明代理需要一个**能力**，不是一个**中间件名**：

```
透明代理的需求：
  "我拦截了 TCP 连接，需要把它转发给一个能处理 HTTP 路由 + 有 Gateway Link 能力的入口"

翻译成 Capability：
  [route_host, upstream_tcp]

可能是 Caddy :80
可能是 Nginx :8080
可能是 HAProxy http mode :80
可能是任何实现了这两个能力的 Provider
```

### 口子怎么开

透明代理不直接依赖 Provider。Planner 在生成 TopologyPlan 时分配一个 `ForwardTarget`：

```go
// transparent.Manager 新增方法
func (m *Manager) SetForwardTarget(target ForwardTarget) {
    m.forwardHost = target.Host
    m.forwardPort = target.Port
}

// StartRedirect 不再硬编码 :80
func (m *Manager) StartRedirect(rule RedirectRule) error {
    if isCrossNode {
        targetPort = m.forwardPort  // ← 从 Planner 来
        targetHost = m.forwardHost
    }
}
```

### 交叉关系图

```
                    ┌─────────────────────────┐
                    │    TopologyPlanner       │
                    │    (维度 2)              │
                    │                         │
                    │  扫描可用 Capability     │
                    │  匹配模板               │
                    │  决定 ForwardTarget     │
                    └───────────┬─────────────┘
                                │ 输出 TopologyPlan (含 ForwardTarget)
                    ┌───────────┼─────────────┐
                    ▼           ▼             ▼
          ┌─────────────┐ ┌──────────┐ ┌──────────────┐
          │ transparent │ │ Caddy    │ │ HAProxy      │
          │ .Manager    │ │ Provider │ │ Provider     │
          │             │ │          │ │              │
          │ 读取        │ │ Render() │ │ Render()     │
          │ ForwardTarget│ │ Apply() │ │ Apply()      │
          │ 启动 iptables│ │          │ │              │
          │ 启动 Proxy  │ │          │ │              │
          └─────────────┘ └──────────┘ └──────────────┘
```

**没有交集的部分（不动）**：iptables 规则管理、端口分配（18100-18199）、SO_ORIGINAL_DST 读取。这些是透明代理的内部实现，和三维度无关。

---

## 四、三个维度的交叉

### 共享数据：ProviderState

三个维度**唯一共享的数据结构**。维度 3 写入（操作改变状态），维度 1 和 2 只读。

```
维度 3 操作 ──→ ProviderState 改变 ──→ 维度 2 重新评估
  install("haproxy")          capabilities 从 [] 变成 [sni_preread, ...]
                              Planner 下次 scan 发现新能力
                              → 生成新的拓扑方案
```

### Import 规则（防乱机制）

```
允许：
  main.go  → 三个维度都 import
  topology → provider （只读 Provider.State()）
  lifecycle → provider（只调 install/uninstall/start/stop/diagnose）
  provider → 不 import topology 或 lifecycle

禁止：
  provider → topology   (Provider 不该知道拓扑组合)
  provider → lifecycle  (Provider 不该自己安装自己)
  topology → lifecycle  (Planner 不该自己安装中间件)
  topology ↔ lifecycle  (平级 import 不允许)
```

如果必须跨维度通信 → 用事件通知，不直接调用。

---

## 五、L3-L7 能力清单

26 个能力值，按网络层级分组。新协议只需在对应层加常量，中间件支持就声明，不支持就不声明。

```
L3 网络层 (2):
  route_src_ip             按源 IP 路由
  transparent_proxy        iptables DNAT 劫持出站流量，SO_ORIGINAL_DST 获取原始目标

L4 传输层 (4):
  listen_tcp               监听 TCP 端口
  listen_udp               监听 UDP 端口
  upstream_tcp             TCP 转发到 upstream
  upstream_udp             UDP 转发到 upstream

L5 会话层 (4):
  tls_terminate            TLS 终止（持有证书、解密）
  tls_passthrough          TLS 透传（不解密，只读 SNI）
  mtls_terminate           mTLS 终止（双向证书验证）
  tls_masquerade           对外 TLS，对内明文

L6 表示层 (4):
  sni_preread              从 ClientHello 读 SNI
  alpn_match               根据 ALPN 协商结果路由
  proto_detect             自动检测协议（HTTP/1.1 vs h2）
  ocsp_stapling            OCSP 装订

L7 应用层 (12):
  http1                    HTTP/1.1
  http2                    HTTP/2 (h2, h2c)
  http3                    HTTP/3 over QUIC
  websocket                WebSocket upgrade
  grpc                     gRPC (HTTP/2 + trailers)
  sse                      Server-Sent Events
  raw_tcp                  Raw TCP 流转发
  raw_udp                  Raw UDP 报转发
  route_host               Host header 路由
  route_path               URL path 路由
  auto_cert                自动 ACME 证书
  health_check             后端健康检查
```

---

## 六、12 种流量类型的层级分解

```
                     L3    L4     L5           L6      L7
                     ────  ────   ────         ────    ────
1.  Plain HTTP        —     TCP    none         —       HTTP/1.1
2.  HTTPS 终止        —     TCP    terminate    —       HTTP/1.1, h2
3.  TLS 透传 SNI      —     TCP    passthrough  SNI     —（不可见）
4.  443 混合          —     TCP    passthrough  SNI     分支：HTTP / Raw TCP
                                   +terminate
5.  HTTP/2 gRPC       —     TCP    terminate    ALPN h2  gRPC
6.  WebSocket         —     TCP    terminate    —        HTTP→WS
7.  HTTP/3 QUIC       —     UDP    terminate    内建      HTTP/3
8.  Raw TCP           —     TCP    none         —        Raw TCP
9.  TLS-wrapped TCP   —     TCP    passthrough  —        Raw TCP（加密）
10. Raw UDP           —     UDP    none         —        Raw UDP
11. Unix upstream     —     —      不适用        —        不适用
12. ACME              —     TCP    none         —        HTTP/1.1
```

---

## 七、目标目录结构

```
internal/
  provider/             ← 维度 1：Provider 适配
    capability.go       ← 26 个 Capability 常量 + Layer() + IsIngress()
    state.go            ← ProviderState, Diagnostic, Plan, ListenerSpec, RouteSpec, ConfigFile
    provider.go         ← Provider 接口
    caddy.go            ← Caddy 实现
    haproxy.go          ← HAProxy 实现（合并 edge+tcp）
    nginx.go            ← Nginx 实现（预留）
    registry.go         ← Provider 注册表
    discovery.go        ← 运行时检测（精简，仅 DiscoverProviders + CurrentPortPolicyMode）

  topology/             ← 维度 2：拓扑组合决策
    intent.go           ← GatewayIntent, RouteIntent, MatchSpec, UpstreamSpec
    plan.go             ← TopologyPlan, Solution, ForwardTarget
    planner.go          ← Planner 接口 + 实现 + requirementsOf
    fallback.go         ← 4 级降级逻辑
    templates/          ← 拓扑模板（每个实现 Template 接口）
      single_caddy.go
      single_nginx.go
      single_haproxy.go
      haproxy_caddy.go
      nginx_caddy.go
      dedicated_ports.go
      full_stack.go

  lifecycle/            ← 维度 3：生命周期管理
    manager.go          ← Manager 接口 + 实现
    apt.go              ← apt-get 安装/卸载（从 install.go 迁移）
    systemd.go          ← systemctl 启停

  transparent/          ← 透明代理（内部实现不变，加 SetForwardTarget）
    manager.go          ← 新增 SetForwardTarget，不再硬编码 :80
    proxy.go            ← 不变
    redirect_linux.go   ← 不变
    redirect_other.go   ← 不变
    original_dst_linux.go  ← 不变
    original_dst_other.go  ← 不变
    types.go            ← 不变
    doc.go              ← 不变
```

---

## 八、完整执行计划

### 设计模式清单

| 模式 | 用在哪 | 解决什么问题 |
|---|---|---|
| **Strategy** | Provider 接口 + Caddy/HAProxy/Nginx 实现 | 不同中间件同一套接口 |
| **Template Method** | Provider.Apply() 的 6 步流程 | validate→backup→write→reload→verify→rollback |
| **Registry** | provider.Registry | 运行时发现所有可用 Provider |
| **Chain of Responsibility** | 拓扑模板链 | 按优先级挨个尝试匹配 |
| **Builder** | GatewayIntent 构造 | 复杂对象分步构建 |
| **Value Object** | ProviderState, Plan, ForwardTarget | 跨维度共享的纯数据 |
| **Facade** | lifecycle.Manager | 封装 apt + systemctl |
| **Observer** (隐式) | 维度 3 操作 → 维度 2 重评估 | install 完成 → Planner 重新 Plan |

### Phase 1：基础类型（0 外部依赖）

| Step | 文件 | 操作 | 内容 |
|---|---|---|---|
| 1.1 | `capability.go` | ✅ 已完成，新增 1 个常量 | 加 `CapTransparentProxy`，总 26 个 |
| 1.2 | `state.go` | 新建 | ProviderState, Diagnostic, Plan, ListenerSpec, RouteSpec, MatchSpec, UpstreamSpec, ConfigFile |
| 1.3 | `types.go` | 清理 | 删除残留的 Protocol / ProviderCapabilities 引用，保留 GatewayType / PortPolicy / PortBinding / DependencyEdge |

### Phase 2：Provider 接口 + 实现（依赖 Phase 1）

| Step | 文件 | 操作 | 内容 |
|---|---|---|---|
| 2.1 | `provider.go` | 重构接口 | 删除 ID()/Name()/Type()/ConfigPath()/BinaryPath()/ServiceName() 独立方法 → 合并到 State()；删除 Capabilities()/UIHints()/CanInstall()/Install()/CanUninstall()/Uninstall() → 移到维度 1 的 State() 和维度 3；Render 参数改为 Plan；返回值改为 []ConfigFile |
| 2.2 | `caddy_http.go` → `caddy.go` | 重写 | CaddyProvider 实现新接口，Render 从 Plan 生成 Caddyfile，Apply 6 步流程 |
| 2.3 | `haproxy.go` | 新建 | HAProxyProvider 合并 edge+tcp，一个 Provider 声明完整能力集，Render 内部区分 SNI 路由和 TCP 转发 |
| 2.4 | `registry.go` | 适配 | 适配新 Provider 接口，删除 SelectForProtocol |

### Phase 3：拓扑组合（依赖 Phase 2）

| Step | 文件 | 操作 | 内容 |
|---|---|---|---|
| 3.1 | `topology/intent.go` | 新建 | GatewayIntent, RouteIntent (5 维度模型) |
| 3.2 | `topology/plan.go` | 新建 | TopologyPlan, Solution, ForwardTarget |
| 3.3 | `topology/templates/` | 新建 7 个文件 | 每个实现 Template 接口 { Name, RequiredCapabilities, TryMatch } |
| 3.4 | `topology/planner.go` | 新建 | Planner 实现 + requirementsOf（意图 → 能力需求提取） |
| 3.5 | `topology/fallback.go` | 新建 | 4 级降级方案生成逻辑 |

### Phase 4：生命周期管理（依赖 Phase 2）

| Step | 文件 | 操作 | 内容 |
|---|---|---|---|
| 4.1 | `lifecycle/manager.go` | 新建 | Manager 接口 + 实现，封装所有生命周期操作 |
| 4.2 | `lifecycle/apt.go` | 从 install.go 迁移 | installPackage / uninstallPackage |
| 4.3 | `lifecycle/systemd.go` | 新建 | start / stop / restart / reload |

### Phase 5：集成 + 透明代理改造（依赖 Phase 3 + 4）

| Step | 文件 | 操作 | 内容 |
|---|---|---|---|
| 5.1 | `main.go` | 重组装配 | 三个维度清晰分离 |
| 5.2 | `transparent/manager.go` | 新增 SetForwardTarget | 不再硬编码 :80，从 Planner 读取 ForwardTarget |
| 5.3 | `discovery.go` | 精简 | 适配新 Provider 接口，删除死代码 |

### 统计

```
新建 16 个文件
重写 3 个文件
修改 4 个文件
删除 0 个（已在 cleanup 阶段完成）
~1400 行新代码 + ~150 行修改
```

---

## 九、关键设计决策

1. **Provider 只暴露端口输入输出**。Render 接受 `Plan`（这个 Provider 要监听哪些端口、转发哪些路由），返回配置文件。Provider 不知道也不关心其他 Provider 的存在。

2. **一个中间件 = 一个 Provider**。HAProxy 只有一个 `HAProxyProvider`，内部可以管理多个配置文件（haproxy.cfg + haproxy_tcp.cfg），但对外是一个统一的能力集合。

3. **Planner 把流量意图翻译成 Provider 分配**。用户不说"用 Caddy"，而是说"我要把 :443 上的 app.example.com 转发到 127.0.0.1:3000"。Planner 计算需要哪些能力，找匹配的 Provider 组合。

4. **拓扑模板是声明式的**。每个模板声明"我需要哪些能力"，Planner 拿当前可用的能力去匹配。匹配上了 → 按模板的 buildPlan() 生成 Provider 分配。匹配不上 → 降级。

5. **透明代理的转发目标由 Planner 决定**。透明代理不硬编码 Caddy :80，而是从 TopologyPlan.ForwardTarget 读取。任何声明了 `[route_host, upstream_tcp]` 的 Provider 都可以作为转发目标。

6. **三层代码互相只通过接口耦合**。改维度 1 的实现不影响维度 2。加一个新的拓扑模板不影响维度 1 和 3。透明代理只通过 ForwardTarget 纯数据和 Planner 交叉。
