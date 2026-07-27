# ServiceAuth 设计文档

> **为什么设计、怎么拆、各种场景怎么接。**
> 这不是操作手册。操作见 `docs/serviceauth.md`。

---

## 一、词汇表（专有名词速查）

### 三面（Plane）—— 认证系统的三个正交维度

| 面 | 英文专有名词 | 回答 | 在你的系统里 |
|---|---|---|---|
| **身份面** | **Authentication / Identity** | 你是谁？ | ticket（服务）/ API key（外部）/ session（用户） |
| **权限面** | **Authorization（AuthZ）** | 你能干嘛？ | scope（服务/外部）/ RBAC（用户） |
| **供给面** | **Provisioning（JIT Provisioning）** | 新来的自动开通什么？ | OnNewService 钩子 |

### 三种主体（Principal）—— 谁在操作

| 主体 | 专有名词 | 认证方式 | 权限方式 |
|---|---|---|---|
| 服务（自己人） | **Workload Identity** / Non-Human Identity (NHI) | ticket（Ed25519 签名） | scope |
| 外部团队 | **API Client** | API key | scope |
| 终端用户 | **End User** | session / JWT | RBAC |

### 关键概念

| 术语 | 英文 | 一句话 |
|---|---|---|
| 统一身份 | **Principal** | 认证后的折平结果，业务代码只认这个 |
| 服务身份 | Workload Identity | 服务之间互相调用用，不是人 |
| 即时供给 | **JIT Provisioning** | 新身份第一次出现时自动开通资源 |
| 供给钩子 | Provisioning Hook | 放"新服务自动建 project/权限"的地方 |
| 第一方 SDK | **First-party Client** | 自己人用的 SDK，内部带 ticket |
| 第三方 SDK | **Third-party Client** | 外部团队用的 SDK，带 API key |
| 服务分类 | A / B / C / D | 见第二节 |

---

## 二、服务 4 分类（每个服务必属一类）

分类由两个问题决定：**它调不调别人？** + **它被谁调？**

### 类型 A：纯消费者

**特征：** 只调别人，不开接口。
**真实业务：** 定时任务（cron job）、数据同步 worker、消息消费者。
**接入方式：**

```go
client, _ := serviceauth.New(Config{ServiceName: "report-cron"})
// 只调，不用 Guard
client.Post(ctx, "http://other-svc/api/xxx", body)
```

**建议：** 不动。现在就是够用的。

---

### 类型 B：纯内部提供者

**特征：** 只被自己人调，不对外。
**真实业务：** 风控服务、用户查询服务、配置服务。
**接入方式：**

```go
client, _ := serviceauth.New(Config{ServiceName: "risk-svc"})
// 被调：Guard 验票
mux.Handle("POST /api/check", client.Guard(handler))
// 如果它也调别的：照常 client.Post
```

**建议：** 不动。Guard 默认锁内网 IP，天然拒外部。

---

### 类型 C：对外提供者 ★最复杂★

**特征：** 同一个接口，自己人带 ticket 调、外部带 API key 调。
**真实业务：** 认证/登录服务、短信服务、支付网关。
**接入方式：**

关键原则：**不要把"if 是ticket 是key"写在业务 handler 里。** 在认证层折平。

```go
// —— 认证层（写一次） ——
type Principal struct {
    ID     string   // 服务名 / key持有者ID
    Kind   string   // "service" | "external"
    Scopes []string // 该身份被授权的能力列表
    Space  string   // 归属空间
}

func dualAuth(serviceKeyMap map[string][]string, validateKey func(string) (*Principal, error)) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            var p *Principal

            // 路径 A：ServiceAuth ticket（自己人）
            if ticket := r.Header.Get("X-Service-Ticket"); ticket != "" {
                p = verifyTicketPrincipal(ticket, serviceKeyMap)
            }

            // 路径 B：API Key（外部）
            if p == nil {
                if key := r.Header.Get("X-API-Key"); key != "" {
                    p, _ = validateKey(key)
                }
            }

            if p == nil {
                http.Error(w, `{"error":"unauthorized"}`, 401)
                return
            }

            ctx := context.WithValue(r.Context(), "principal", p)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// —— 业务 handler（只写一次，不管来源） ——
func CreateProject(w http.ResponseWriter, r *http.Request) {
    p := r.Context().Value("principal").(*Principal)
    // p.Kind 是 "service" 还是 "external"？
    // 大部分时候不用管——操作统一用 p.Space 判断归属。
}
```

**建议改动：** 这个 `dualAuth` 现在只是模板，不是 SDK 内置。C 类服务多时可以收进 SDK。

---

### 类型 D：中心（authserver）

**特征：** 发身份、记归属、新服务接入时自动供给。
**真实业务：** 就是你的 authserver。
**关键设计：** 留 `OnNewService` 钩子。

```go
// 中心在"第一次见到新服务名"时，回调这个：
// 钩子接收服务信息，做业务决策（建 project、配归属等）
// 钩子可以留空——不做任何自动操作。
type OnNewService func(svc NewServiceInfo) error

type NewServiceInfo struct {
    Name    string
    Key     string // 公钥
    SourceIP string
}
```

**建议改动：** 加钩子（可留空）。这是第 5 幕（供给面）的家。

---

## 三、三面拆分（身份 / 权限 / 供给）

一个复杂服务（类型 C）的内部结构：

```
┌─────────────────────────────────────────────────────┐
│                接收的请求（入口）                        │
│    X-Service-Ticket（自己人） / X-API-Key（外部）       │
└─────────────────────┬───────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  身份面（Authentication）  identity.go                 │
│  功能：折平 ticket 和 key → Principal                   │
│  输出：Principal{ID, Kind, Scopes, Space}              │
└─────────────────────┬───────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  权限面（Authorization）   authz.go                    │
│  功能：Can(principal, action) → bool                   │
│  服务/外部 查 scope；用户查 RBAC                       │
└─────────────────────┬───────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  供给面（Provisioning） provisioning.go               │
│  功能：新身份第一次出现时，自动建资源 + 配权限             │
│  位置：独立文件，高度耦合业务（它是"接线板"）              │
│  启动时一行装配                                       │
└─────────────────────┬───────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  业务 handler（只调 Can()，不碰认证/供给细节）            │
└─────────────────────────────────────────────────────┘
```

---

### 供给面（provisioning.go）—— 你一直纠结的"预处理"

这个文件是**独立业务文件**，不是 SDK 的一部分。它：
- **高度耦合业务**（import project、permission 等服务）—— 这是对的，因为它的职责就是"接线"。
- **和认证机制无关**（不 import ticket、不 import Guard）。
- **只回答一个问题：** 当我第一次见到一个新服务时，自动做什么。

```go
// provisioning.go —— 你的业务项目里自己写

func registerProvisioning(center *serviceauth.Center, proj *project.Service, perm *permission.Service) {
    center.OnNewService(func(svc serviceauth.NewServiceInfo) error {
        // ↓↓↓ 业务决策，完全你说了算 ↓↓↓

        // 1. 建默认 project（幂等，已建跳过）
        p, err := proj.Ensure(svc.Name)
        if err != nil { return err }

        // 2. 给默认 scope（新服务默认只能读自己）
        perm.Grant(svc.Name, []string{"read:self"})

        // 3. 归属挂到默认管理员
        proj.SetOwner(p.ID, "default-admin")

        return nil

        // ↑↑↑ 改这段 = 改你默认接入策略，不动任何其他代码 ↑↑↑
    })
}
```

启动时一行装配：

```go
// main.go
registerProvisioning(center, projectSvc, permSvc)
```

**要点：**
- 幂等（`Ensure` 而不是 `Create`），因为消息可能重复。
- 逻辑复杂度只在 `provisioning.go` 这一个文件里膨胀，不会漏到 handler 或认证机制里。
- 不想做自动供给 → 不调 `registerProvisioning`，钩子留空。

---

## 四、对内 / 对外 SDK 分壳

### 问题

同一套业务能力（比如登录服务），自己人和外部都要用。但：
- 自己人想用 ticket（零配置、自动身份）。
- 外部团队需要用 API key（手动注册、签名）。

对外 SDK 里不能出现 ticket 逻辑——否则泄漏内网架构。

### 方案：两壳一核

```
           ┌──────────────────┐
           │  业务核心（写一次）│  ← 拼 URL、解析响应…… 和认证无关
           └──────┬───────────┘
                  │
       ┌──────────┴──────────┐
       ↓                     ↓
┌──────────────┐    ┌──────────────┐
│ 对内壳        │    │ 对外壳        │
│ first-party  │    │ third-party  │
│ 内部包        │    │ 外部包        │
│              │    │              │
│ New("svc")   │    │ New(key)     │
│ 底层 client. │    │ 底层 API key │
│ Post 自动带票 │    │ 签名         │
└──────────────┘    └──────────────┘
```

**关键原则：两个壳共享业务方法定义，不共享凭证逻辑。**

```go
// ── 业务核心（authcore/）—— 和被认证无关的封装 ──
package authcore

type DoFunc func(method, url string, body any) ([]byte, error)

func CreateProject(do DoFunc, name string) (*Project, error) {
    // 拼 URL、序列化 body、解析响应——纯业务，一行认证都没有
    data, _ := json.Marshal(map[string]string{"name": name})
    resp, err := do("POST", "/api/v2/projects", data)
    if err != nil { return nil, err }
    var p Project
    json.Unmarshal(resp, &p)
    return &p, nil
}

// ── 对内壳（sdk-internal/）—— 只有几十行 ──
package sdkinternal

import "aegis/pkg/serviceauth"
import "authcore"

type Client struct {
    *serviceauth.Client
}

func New(svcName string) *Client {
    c, _ := serviceauth.New(Config{ServiceName: svcName})
    return &Client{c}
}

func (c *Client) CreateProject(name string) (*Project, error) {
    // 底层用 client.Post——自动带 ticket
    return authcore.CreateProject(c.Post, name)
}

// ── 对外壳（sdk-external/）—— 也只有几十行 ──
package sdkexternal

import "authcore"

type Client struct {
    key string
}

func New(apiKey string) *Client {
    return &Client{key: apiKey}
}

func (c *Client) CreateProject(name string) (*Project, error) {
    // 底层用 http.Post——header 带 X-API-Key
    return authcore.CreateProject(c.apiKeyDo, name)
}
```

**效果：**
- 对外壳里**没有一行 ticket**，外部团队反推不出内网。
- 对内壳里**没有 API key 逻辑**，自己人不用手动配 key。
- 业务方法（拼 URL、解析响应）**只写一次**在 `authcore` 里。

---

## 五、服务作为主体时，授权怎么做（Scope-based AuthZ）

服务做主体时，授权不依赖 RBAC（角色继承），依赖 **scope（作用域/能力标签）**。

```go
// 权限判断的一次调用
func CanDo(principal Principal, action string) bool {
    // 服务 / 外部查 scope
    if slices.Contains(principal.Scopes, action) {
        return true
    }
    // 用户查角色
    if hasRolePermission(principal.Roles, action) {
        return true
    }
    return false
}
```

实际项目里，`CanDo` 查的 scope 来源有两种模式：

| 模式 | scope 存在哪 | 谁决定 | 适合 |
|---|---|---|---|
| **内聚** | 中心（authserver） | 中心在 JIT provisioning 时授予 | 服务权限简单、中心说了算 |
| **分散** | 各业务服务自己存 | 各服务管理员配 | 权限复杂、每个服务自己管 |

**建议：** 先从内聚开始（中心的供给钩子里给默认 scope），复杂了再下沉。你的 C 类服务在 `dualAuth` 里已经知道 Principal 的 Kind，授权是下一步。

---

## 六、全景速查

| 你的服务是… | 类型 | 身份面 | 权限面 | 供给面 | SDK 形态 |
|---|---|---|---|---|---|
| 定时任务 | A | ticket 调用 | — | — | 无（直接用 client.Post） |
| 风控服务 | B | Guard 验票 | 简单 if | — | 无 |
| 登录/认证服务 | C | dualAuth（ticket+key） | scope | 供给钩子 | 内壳(ticket) + 外壳(key) |
| 中心（authserver） | D | 作为验证方 | scope 授予 | **定义钩子** | 对内壳 |

---

## 七、相关文件

| 文件 | 职责 |
|---|---|
| `docs/serviceauth.md` | 操作手册（怎么用） |
| `docs/serviceauth-design.md` | **本文（为什么这么设计）** |
| `pkg/serviceauth/doc.go` | 包注释 |
| `pkg/serviceauth/*.go` | SDK 实现 |
| `internal/serviceauth/*.go` | 服务端实现 |
