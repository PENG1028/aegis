# Aegis 流量路由 — 全场景

## 场景矩阵

```
两台机器: A (43.160.211.232, 内网 10.0.0.3, network=vpc-tokyo)
         B (43.159.34.11,  内网 10.0.0.5, network=vpc-tokyo)
```

---

## 一、域名入站 (Domain Ingress)

外部流量通过域名进入 Aegis，Caddy 反向代理到后端。

### 1.1 域名 :80 → 本机后端

```
外部 curl https://api.example.com/resource
  │
  ▼ DNS 解析 → 43.160.211.232 (Machine A 公网 IP)
  │
  ▼ connect(43.160.211.232, 80) → Machine A Caddy
  │
  ▼ Caddy: Host=api.example.com → 匹配 Route
  │   Caddyfile: api.example.com { reverse_proxy http://127.0.0.1:3001 }
  │
  ▼ 127.0.0.1:3001 → 本机后端 ✅
```

| 属性 | 值 |
|------|-----|
| 实现 | `internal/proxy/caddy/render.go` — `renderCaddyfile()` + `writeReverseProxy()` |
| 路由匹配 | `internal/apply/planner.go:189-201` — `Route → UpstreamURL` |
| 状态 | ✅ 已实现，E2E 测试通过 |

### 1.2 域名 :80 → 远程机器后端 (Gateway Link)

```
外部 curl https://b-service.example.com/data
  │
  ▼ DNS → 43.160.211.232 (Machine A)
  │
  ▼ Machine A Caddy :80
  │   Host=b-service.example.com, Route 绑定了 GatewayLink
  │   Caddyfile: b-service.example.com {
  │       reverse_proxy http://43.159.34.11:80 {
  │           header_up Host "b-service.example.com"
  │           header_up X-Aegis-Gateway-Token "..."
  │       }
  │   }
  │
  ▼ connect(43.159.34.11, 80) → Machine B Caddy
  │
  ▼ Machine B Caddy: Host=b-service.example.com
  │   → reverse_proxy http://127.0.0.1:3002 → 本机后端 ✅
```

| 属性 | 值 |
|------|-----|
| 实现 | `internal/apply/planner.go:207-218` — Gateway Link 时改写 UpstreamURL 为远程 IP:80 |
| 路由匹配 | `internal/routingtable/generator.go` — per-node candidate |
| 状态 | ✅ 已实现 (v1.8H)，E2E 测试通过 |

### 1.3 域名 :443 (TLS) → 后端

```
外部 curl https://api.example.com (TLS)
  │
  ▼ connect(43.160.211.232, 443) → HAProxy (EdgeMux) 或 Caddy HTTPS
  │
  ▼ TLS 终止 / SNI 路由 → 同 1.1 或 1.2 的 HTTP 后端链路
```

| 属性 | 值 |
|------|-----|
| 实现 | `internal/listener/listener.go:24-32` — EdgeMuxDefaults (443 → haproxy_edge_mux) |
| 状态 | ✅ 已实现 |

---

## 二、IP 出站 (IP Egress — 透明代理)

本机进程直接用 IP:port 连接目标，iptables 劫持后走 Aegis。

### 2.1 同机 — 连自己的公网 IP

```
Machine A 进程 → connect(43.160.211.232, 8080)
  │                              ↑ 自己的公网 IP
  │
  ▼ iptables OUTPUT DNAT: -d 43.160.211.232 --dport 8080 → 127.0.0.1:18100
  │
  ▼ TransparentProxy(127.0.0.1:18100) → 同节点 → 127.0.0.1:8080 ✅
  │
  └─ 不走网卡，不经过安全组，本机闭环
```

| 属性 | 值 |
|------|-----|
| 规则生成 | `cmd/aegis/main.go:syncTransparentRules()` — 匹配 `ep.NodeID == currentNode` |
| 代理转发 | `internal/transparent/manager.go:StartRedirect()` — `isCrossNode=false → targetPort=OriginalPort` |
| iptables | `internal/transparent/redirect_linux.go:addRule()` |
| 状态 | ✅ 已实现 |

### 2.2 同 VPC 内网 — 连对方的内网 IP

```
Machine A 进程 → connect(10.0.0.5, 9100)
  │                       ↑ Machine B 的内网 IP (同 vpc-tokyo)
  │
  ▼ iptables: -d 10.0.0.5 --dport 9100 → 127.0.0.1:18101
  │   (network_id 相同 → 拦截 private_ip)
  │
  ▼ TransparentProxy → 异节点 → Caddy :80
  │
  ▼ Caddy 路由 → Gateway Link → 10.0.0.5:80 (内网直连)
  │
  ▼ Machine B Caddy → 127.0.0.1:9100 ✅
```

| 属性 | 值 |
|------|-----|
| 规则生成 | `cmd/aegis/main.go:syncTransparentRules()` — `sameNetwork=true → 拦截 PrivateIP` |
| 跨节点判断 | `internal/transparent/manager.go:StartRedirect()` — `isCrossNode=true → targetPort=80` |
| 内网检测 | `internal/dns/reachability.go:probe()` — TCP :80 探活 |
| 状态 | ✅ 已实现 |

### 2.3 跨区域 — 连对方的公网 IP

```
Machine A(东京) 进程 → connect(52.74.11.22, 9100)
  │                            ↑ Machine C(新加坡) 的公网 IP
  │
  ▼ iptables: -d 52.74.11.22 --dport 9100 → 127.0.0.1:18102
  │   (public_ip 永远拦截，不限 network_id)
  │
  ▼ TransparentProxy → Caddy :80
  │
  ▼ Caddy → Gateway Link → 52.74.11.22:80 (公网)
  │
  ▼ Machine C Caddy → 127.0.0.1:9100 ✅
```

| 属性 | 值 |
|------|-----|
| 规则生成 | `cmd/aegis/main.go:syncTransparentRules()` — `PublicIP 永远加入 ips` |
| 路由降级 | `internal/dns/resolver.go:resolveBestIP()` — private 不可达 → public |
| 状态 | ✅ 已实现 |

### 2.4 不同 VPC 的内网 IP — 不拦截

```
Machine A(东京) 进程 → connect(10.0.0.3, 9100)
  │                        ↑ Machine C(新加坡) 注册了这个内网 IP
  │                          但 network_id 不同 (vpc-tokyo ≠ vpc-sg)
  │
  ▼ iptables: 无规则 (network_id 不同 → 不拦截 private_ip)
  │
  ▼ OS 正常处理 → 10.0.0.3 在东京 VPC 内是另一台机器
  │   → 连到东京的 10.0.0.3:9100 (不是新加坡的)
```

| 属性 | 值 |
|------|-----|
| 规则生成 | `cmd/aegis/main.go:syncTransparentRules()` — `sameNetwork=false → 跳过 PrivateIP` |
| 状态 | ✅ 已实现 |

### 2.5 未受管的 IP:port — 不拦截

```
Machine A 进程 → connect(8.8.8.8, 53)
  │
  ▼ iptables: 无规则 (8.8.8.8 不在任何节点的 IP 里)
  │
  ▼ OS 正常处理 ✅
```

| 属性 | 值 |
|------|-----|
| 规则生成 | `syncTransparentRules()` — 没有匹配的端点 → 不生成规则 |
| 状态 | ✅ 已实现 |

---

## 三、DNS 出站 (DNS Egress)

本机进程用域名连接，Aegis DNS 劫持返回最佳 IP。

### 3.1 域名 → 同机

```
Machine A 进程 → curl http://api.internal/data
  │
  ▼ DNS 查询: api.internal → Aegis DNS (:53)
  │   → Resolver.Lookup("api.internal")
  │   → Endpoint 在 node_a → resolveBestIP() → 127.0.0.1
  │
  ▼ DNS 返回: 127.0.0.1
  │
  ▼ connect(127.0.0.1, 80) → Caddy → reverse_proxy → 127.0.0.1:3001 ✅
```

| 属性 | 值 |
|------|-----|
| DNS 拦截 | `internal/dns/server.go:handlePacket()` — 匹配 Resolver table |
| IP 选择 | `internal/dns/resolver.go:resolveBestIP()` — 同节点 → 127.0.0.1 |
| 状态 | ✅ 已实现 |

### 3.2 域名 → 同 VPC 远程

```
Machine A 进程 → curl http://b-service.internal/data
  │
  ▼ DNS: b-service.internal → Endpoint 在 node_b (同 vpc-tokyo)
  │   → resolveBestIP():
  │       node_b.PrivateIP=10.0.0.5, reachability probe :80 → 通
  │       → 返回 10.0.0.5
  │
  ▼ connect(10.0.0.5, 80) → Machine B Caddy
  │
  │   ⚠️ 注意: connect 到 10.0.0.5:80 — 端口是 80, 不是后端端口
  │   DNS 只改了 IP，没改端口。应用必须用 :80 才能走 Caddy。
  │
  ▼ Machine B Caddy → reverse_proxy → 127.0.0.1:3002 ✅
```

| 属性 | 值 |
|------|-----|
| IP 选择 | `internal/dns/resolver.go:resolveBestIP()` — 私网可达 → private_ip |
| 可达性 | `internal/dns/reachability.go:probe()` — TCP :80 |
| ⚠️ 限制 | DNS 只返回 IP，不返回端口。应用必须用 :80 才能走 Caddy |
| 状态 | ✅ DNS 端已实现，⚠️ 端口问题需应用配合 |

### 3.3 域名 → 跨区域远程

```
Machine A 进程 → curl http://c-service.internal/data
  │
  ▼ DNS: c-service.internal → Endpoint 在 node_c (vpc-sg, 跨区域)
  │   → resolveBestIP():
  │       node_c.PrivateIP=10.0.0.3 (不同 NetworkID, 不可达)
  │       → 返回 node_c.PublicIP=52.74.11.22
  │
  ▼ connect(52.74.11.22, 80) → Caddy → Gateway Link → Machine C ✅
```

| 属性 | 值 |
|------|-----|
| IP 选择 | `internal/dns/resolver.go:resolveBestIP()` — 私网不通 → public_ip |
| 状态 | ✅ 已实现 |

### 3.4 域名 + 非 80 端口 — DNS + 透明代理组合

```
Machine A 进程 → curl http://b-service.internal:9100/data
  │
  ▼ DNS: b-service.internal → 10.0.0.5 (Machine B 内网)
  │
  ▼ connect(10.0.0.5, 9100)
  │   ↑ DNS 返回了 IP，端口 9100 由应用指定
  │
  ▼ iptables: -d 10.0.0.5 --dport 9100 → 127.0.0.1:18103
  │   ↑ 透明代理拦截 (10.0.0.5:9100 有端点)
  │
  ▼ TransparentProxy → Caddy :80 → Gateway Link → Machine B → 127.0.0.1:9100 ✅
```

| 属性 | 值 |
|------|-----|
| DNS 部分 | 同 3.2 |
| 透明代理部分 | 同 2.2 |
| ⚠️ 注意 | DNS 返回了 10.0.0.5，透明代理又拦截了 10.0.0.5:9100 — 两层各自工作 |
| 状态 | ✅ DNS + 透明代理独立协作 |

---

## 四、汇总

```
入站 (外部 → Aegis)
┌────────────────────┬──────────┬────────────────────────────────┐
│ 场景               │ 状态     │ 入口                           │
├────────────────────┼──────────┼────────────────────────────────┤
│ 域名 :80 → 本机    │ ✅       │ Caddy reverse_proxy            │
│ 域名 :80 → 远程    │ ✅       │ Caddy + Gateway Link           │
│ 域名 :443 (TLS)    │ ✅       │ HAProxy/Caddy TLS              │
│ 公网 IP:80 → 远程  │ ✅       │ Caddy (有 Route 则匹配)        │
└────────────────────┴──────────┴────────────────────────────────┘

出站 (Aegis 拦截本机发出的连接)
┌──────────────────────────┬──────────┬───────────────────────────┐
│ 场景                     │ 状态     │ 机制                      │
├──────────────────────────┼──────────┼───────────────────────────┤
│ 域名 → DNS 返回最佳 IP   │ ✅       │ Aegis DNS :53             │
│ 域名:80 → 远程           │ ⚠️ 端口  │ DNS 改 IP 不改端口         │
│ 同机 IP:port → 本机      │ ✅       │ iptables + 透明代理       │
│ 同 VPC IP:port → 远程    │ ✅       │ iptables + Caddy :80      │
│ 跨区 IP:port → 远程      │ ✅       │ iptables + Gateway Link   │
│ 不同 VPC 内网 IP         │ ✅ 不拦  │ NetworkID 过滤            │
│ 未受管 IP:port           │ ✅ 不拦  │ 无规则，OS 正常           │
│ 域名:非80端口 → 远程     │ ✅       │ DNS + 透明代理协作        │
└──────────────────────────┴──────────┴───────────────────────────┘
```

### 实现速查

| 模块 | 文件 | 功能 |
|------|------|------|
| Caddy 渲染 | `internal/proxy/caddy/render.go` | `renderCaddyfile()` — 域名→reverse_proxy |
| 路由规划 | `internal/apply/planner.go` | `Plan()` — Route→UpstreamURL, Gateway Link 改写 |
| DNS 服务器 | `internal/dns/server.go` | UDP :53 监听，拦截 A 记录查询 |
| DNS 解析器 | `internal/dns/resolver.go` | `resolveBestIP()` — 同机/内网/公网选择 |
| 可达性检测 | `internal/dns/reachability.go` | TCP :80 探活 |
| 透明代理规则 | `cmd/aegis/main.go:syncTransparentRules()` | NetworkID 过滤 + IP 收集 |
| 透明代理管理 | `internal/transparent/manager.go` | `StartRedirect()` — 同/跨节点决策 |
| iptables | `internal/transparent/redirect_linux.go` | OUTPUT DNAT 规则 |
| TCP 转发 | `internal/transparent/proxy.go` | 双向 io.Copy |
