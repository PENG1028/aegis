# Two-Node Network Analysis — v1.7AA

## 问题复盘

### 之前为什么卡住

测试流程是：

```
Dev Machine ──SSH──▶ Server A (<SERVER_A_IP>)
                       │
                       │ 需要能连
                       ▼
                  Server B (<SERVER_B_IP>:3000)
```

我在 Server A 上用 `curl http://10.3.0.11:3000/` 测试连通性，超时了。

**这里的混淆点：**
- 我测试的是 **Server A → Server B:3000** 的 TCP 连通性
- 这不是 SSH，这是 **Caddy/HAProxy 转发流量的真实数据路径**
- Aegis 不负责节点间通信，Caddy/HAProxy 才负责

### 为什么现在说连不上

**你开了 80 TCP 和 443 TCP/UDP，但测试端口是 3000。**
这两个不是同一个端口。Server B 的云安全组可能只开了 80/443，没开 3000。

---

## 数据路径分析

用户请求到达 Server A 后的完整路径：

```
Internet
  │
  ▼
Server A :443 (HAProxy EdgeMux)  ── SNI 匹配 ──▶ Server A :8443 (Caddy)
                                                       │
                                                       │ 需要 Server B 放行
                                                       ▼
                                              Server B :3000 (target)
```

**关键依赖：Server A（Caddy）必须能 TCP 连到 Server B:3000。**

这不是 SSH，不是 Aegis 内部通信，是 Caddy 的反向代理转发。

---

## 当前状态

| 路径 | 协议 | 端口 | 状态 |
|------|------|------|:---:|
| Dev → Server A | SSH | 22 | ✅ |
| Dev → Server B | SSH | 22 | ✅ |
| Server A → Server B | TCP | 3000 | ❌ TIMEOUT |
| Server A → Server B | TCP | 80 | ❓ 未测（python3 不在 80） |
| Aegis bind-http-domain | — | — | ✅ action success |
| Aegis safe apply | — | — | ✅ apply completed |
| Trace 识别远端目标 | — | — | ✅ final_target 正确 |
| Trace 检测不可达 | — | — | ✅ TARGET_TIMEOUT |

---

## 需要确认

要让完整链路跑通，需要 Server B 放行 Server A 对端口 3000 的入站流量：

```
Server B 安全组 / 防火墙允许：
  来源: <SERVER_A_IP>
  协议: TCP
  端口: 3000
  动作: ACCEPT
```

或者把测试 target 移到 Server B 的 80 端口（如果 80 已开放）。

---

## 通信模型纠正

Aegis 的节点间通信模型是**控制面独立，数据面走 Caddy**：

| 通信类型 | 谁负责 | 协议 | 是否需要开端口 |
|----------|--------|------|:---:|
| Control: API 调用 | curl/HTTP client → Aegis | HTTP :9000 | 本地 127.0.0.1 |
| Data: 流量转发 | Caddy → target | 目标协议 :target_port | **需要** |
| Data: TLS SNI | HAProxy → Caddy | TCP :443 → :8443 | 本地 127.0.0.1 |
| Aegis sync（将来） | Aegis → Aegis | HTTP | 需要 |
