# DistNode 节点接入修复 — 定案文档 (v1.9B-fix)

> **状态:** 已定案,开始实施
> **起因:** UI「加入节点」显示成功但节点不出现在列表。排查后发现 distnode 跨机路径从未在生产端到端跑通,并牵出多套并存的跨节点机制(设计分叉)。
> **本文档是实施与验收的唯一依据。**

---

## 一、问题全貌(排查结论)

原始问题集中在 **distnode 节点接入**,但排查暴露出一整片问题。根因是**设计分叉**:同一件事「两台机器怎么通信」并存三套互不认识的机制,完成度天差地别。

| 机制 | 载体 / 路由键 | 状态 |
|------|-------------|------|
| **A. Gateway-Link → Caddy `reverse_proxy`** | 域名入口重写到远端节点(planner 改写上游 + 渲染进 Caddyfile) | ✅ **唯一真正接通、且在 apply/render 链路里** |
| **B. `/__aegis/relay`** | Route/Endpoint + HTTP 头 | ⚠️ 接收端已挂载,**发送端 `gateway.Gateway` 生产从未启动** → 死 |
| **C. transparent (iptables)** | 裸 IP:端口 | ❌ 双 bug 死透:`SetForwardTarget` 生产从未调用;`syncTransparentRules` 因 hook 里 `endpointRepo=nil` 每次 early-return,一条规则都不装 |

distnode(v1.9B)是**第 4 套**,被刻意做成"只导入标准库"的孤岛,健康检查/调用硬编码直连 `<ip>:7380`。但:
- `7380` 只监听 `127.0.0.1`,云安全组只开 `80/443` → 跨机 `A→B:7380` 实测 `000`(不通)。
- `POST /api/distnode/v1/call` 只在测试里挂载,生产 `routes.go` 未挂。
- join handler 只把配置写到 B(且用 `cat >>` 无 sudo、不查 ExitCode,静默失败),从不把 B 写进 A 的 peers。
- A 的 distnode `enabled=false`,无身份。

**断点在包与包的"接缝"里,每层单独看都"正常",且到处 success-masking,所以看起来做完了、实际从未跑通。**

---

## 二、修复原则:骑 Design A,不碰死机制,不分叉

**A 是唯一 canonical 且活着的跨机载体。修复必须复用 A 的渲染管线(`RouteSpec → Plan → renderCaddyfile`),绝不复活 B/C,绝不新增第 5 套。**

核心洞察:**distnode 健康检查是「进程直接外拨」,不经过本机 Caddy。**
```
A 的 distnode ──直拨──► B边缘:80/api/healthz ──► B 的 Caddy(有控制路由)──► B 本机 127.0.0.1:{controlPort}
```
因此:
- 只需在**每个节点的 Caddy** 上有一条「控制面预留路由」(按路径、Host 无关),作为该节点控制面的入口。
- A 直拨 B 边缘:80,B 的 Caddy 终结并落到 B 本机控制口。
- **不需要 gateway-link**(那是给"入口路由改写上游"用的;distnode 是对等直拨,用它属于套错抽象)。
- **distnode 核心逻辑一行不改;7380 永远只在本机监听(真·本机预留口),公网零新开端口。**

「加一条 Caddy 路由的正确姿势」(已确认,防分叉):**注入合成 `RouteSpec`,不改渲染器、不加 composition、不硬编码端口。** 先例 = `config.go:258 PanelCaddyfile()` 的 `handle /api/* → {Server.Addr}`。

---

## 三、变更清单(精确到文件)

| # | 文件 | 改动 |
|---|------|------|
| 1 | `internal/config/config.go` `Load()` | distnode 默认 `enabled`;`id` 空→`os.Hostname()`;`addr` 空→由 `Server.Addr` 推;`secret` 空→`crypto/rand` 生成 hex 并 `Config.Save()` 持久化(重启不变)。**controlPort 一律 `safety.SplitHostPort(cfg.Server.Addr)`,无 7380 字面量。** 权衡:显式 `enabled:false` 也会被强开(Phase 0 可接受) |
| 2 | `internal/topology/planner.go` (`Dependencies`) + `cmd/aegis/main.go` 装配 | `Dependencies` 加 `ControlPort int`,main.go 从 `cfg.Server.Addr` 用 `safety.SplitHostPort` 填入 |
| 3 | `internal/topology/planner.go`(build 后注入,**能力化**) | `tmpl.BuildPlan` 返回后,遍历 `best.Plans`,按能力(`CapRouteHost`+`CapUpstreamTCP`,从 `healthy` 里 `HasCapability` 选,**绝不按 provider 名**)选出 HTTP 路由 provider,向其 `Plan.Routes` 追加**一条控制面合成 RouteSpec**。**模板零改动、渲染器零改动。** |
| 4 | `internal/httpapi/routes.go` | 挂 `POST /api/distnode/v1/call` = `h.DistNode.Transport.Handler()`(`h.DistNode!=nil` 时);**必须在 admin-token 中间件之外**,用 distnode 自己的 HMAC 鉴权 |
| 5 | `internal/distnode/distnode.go` | 加 `Membership.AddPeer(pc PeerConfig)`(线程安全、跳过 self/重复、可选立即探测一次)。保持零第三方依赖 |
| 6 | `internal/httpapi/handlers/deploy_node.go` `AdminJoinNode` | peer.Addr=`net.JoinHostPort(TargetIP,"80")`;**A 侧**:写 `Config.DistNode.Peers`+`Save`+`Membership.AddPeer`+建 `nodes` 记录(带 IP,状态 pending,不等 alive → 破死锁);**B 侧**:`cat >>` 改 `sudo tee -a` 且查 `ExitCode`,secret 用 `h.Config.DistNode.Secret`(不再硬编码),写完触发 B 的 apply |
| 7 | 运维一次性 | `make update-all` 部署 + 重启 A/B(用户已同意) |

**控制面 RouteSpec 的渲染决定(已定,避免 caddy validate 崩):**
- `Match.Host = "http://"` → 渲染成 **HTTP catch-all** `http:// { ... }`(与自动 HTTPS 域名共存不冲突;`:80` 会和 auto-HTTPS 抢 :80 → 弃用)。
- `Match.Path = "/api"` → 渲染器 `pathPattern` 自动补 `/api/*`,一条覆盖 `/api/healthz`(distnode 硬编码、精确路径)与 `/api/distnode/v1/call`。等价于 `config.go:258 PanelCaddyfile` 的 `/api/*` 先例。
- `Upstream.Target = "http://127.0.0.1:{controlPort}"`。
- 只影响"以 IP 为 Host"的请求(即 distnode 对等直拨);业务域名块特异性更高,不受影响。
- **安全注记:** 这会把节点自身 `/api/*` 暴露在公网 :80(catch-all)。与面板既有 `PanelCaddyfile` 同姿态,`/api/admin/*` 有会话鉴权、`/api/distnode/*` 有 HMAC。可接受;如需收窄再议。peer.Addr = `<边缘IP>:80`,无需 DNS/hosts。

**不在本次范围(留 Stage 2):** C 的两个 bug(`main.go:288-292` 传 `endpointRepo`、apply 里调 `SetForwardTarget`);B 的死发送端 `gateway.Gateway`;删废弃的 nodeagent/noderuntime/nodestate。

---

## 四、最终验收标准(AC)

- **AC1 默认开:** 全新 `aegis serve` 后 `config.yaml` 中 distnode 自动 `enabled`、有 `id/addr/secret`;重启后 `secret` 不变。
- **AC2 路由渲染 & 不写死:** apply 后每台节点生成的 Caddyfile 含 `/api/distnode/*` 与 `/api/healthz → 127.0.0.1:{controlPort}`;把 `Server.Addr` 端口改成非 7380,该路由的上游端口跟着变(证明无字面量)。
- **AC3 跨机可达:** `curl http://<B边缘IP>:80/api/healthz` 返回 **B 自己**的 `{"status":"alive"}`;带正确 HMAC 的 `POST .../api/distnode/v1/call` 返回非 401;不带 secret 返回 401。
- **AC4 接入生效:** A 上 UI 加入 B → B 在 **≤30s** 出现在节点列表且 `online`;A 日志出现 `peer <hostname> is alive`。
- **AC5 存活翻转:** 停 B 服务 → A 列表 B 转 `offline`;恢复 → 转回 `online`。
- **AC6 distnode 纯净:** `go list -m all` distnode 无新增第三方依赖;distnode 包内无 `aegis/internal/*` 业务导入。
- **AC7 死机制未动:** transparent(C)/relay(B) 代码与行为不变(Stage 2 处理)。
- **AC8 构建门禁:** `make test`、`go build ./...`、`go vet ./...` 全过。
- **AC9 端口边界:** 每台节点 `7380` 仍只 `127.0.0.1` 监听;公网无新开端口;安全组无需改动。

---

## 五、验证步骤

```bash
# 本地
make test && go build ./... && go vet ./...
go list -m all            # 确认 distnode 无第三方依赖 (AC6)

# 部署后跨机 (AC3) — 只开 80/443
ssh ubuntu@<A> "curl -s http://<B边缘IP>:80/api/healthz"                       # 期望 {"status":"alive"}
ssh ubuntu@<A> "curl -s -o /dev/null -w '%{http_code}' -X POST http://<B边缘IP>:80/api/distnode/v1/call"  # 期望 401(无 secret)

# 端到端 (AC4/AC5)
#  UI 加入 B → 刷新节点列表看 B online
ssh ubuntu@<A> "sudo journalctl -u aegis | grep distnode | tail"               # 期望 'peer ... is alive'
ssh ubuntu@<B> "sudo grep -c 7380 /etc/aegis/config.yaml"                      # 仅本机监听项,无跨机 7380

# 回归 (AC1/AC9)
ssh ubuntu@<A> "sudo ss -ltnp | grep 7380"                                     # 期望仅 127.0.0.1:7380
```

---

## 五点五、验证中发现并修复的既有 bug(distnode HTTP/RPC 层从未工作的真正原因)

部署验证时发现,distnode 的 HTTP/RPC 层因两处**既有装配缺失**从未工作过(与"distnode 从未端到端跑通"的结论一致):

1. **`Handlers.DistNode` 从未从 `Services` 装配** —— `routes.go` 的 `Handlers{}` 字面量缺 `DistNode: svcs.DistNode`,导致 `h.DistNode` 永远 nil → 所有 `/api/admin/v1/distnode/*` 返回 "distnode not enabled",且我新挂的 call 端点 `if h.DistNode != nil` 永不触发。已补该字段。
2. **`RegisterAegisTransportHandlers` 定义了但从未被调用** —— `Aegis.ProxyRequest` 等跨节点方法从未注册 → 跨机 RPC 报 "unknown method"。已在 `routes.go` 挂 call 端点处调用它。

> 节点能出现在列表(Membership→OnEvent)走的是 `main.go` 的 `dn` 引用,与 `h.DistNode` 是两条引用,所以"列表能显示"但"RPC/status 不工作"——正是这两个 bug 的表现。

## 六、一次性运维修复(当前 A/B)

1. `make update-all`(先 B 后 A)部署修好的二进制。
2. A/B 重启后 `config.Load` 自动 distnode-on 并生成 secret;以 A 的 secret 为准同步到 B(通过修好的 UI「加入」重走一次)。
3. A/B 各 apply 一次,渲染出控制面预留路由。
4. 刷新节点列表 → B 应 ≤30s 出现。
