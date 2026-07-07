# 运行时模式切换安全设计

> 目标：让用户能在 Legacy / EdgeMux 之间切换，
> 不影响可用能力，不丢配置，不中断流量。

---

## 一、现在有什么

### 1.1 能力矩阵（只读）

```
Provider 声明自己能做什么:
  Caddy:   [listen_tcp, tls_terminate, http1, route_host, ...]
  HAProxy: [listen_tcp, sni_preread, tls_passthrough, ...]

Mode 定义端口归谁:
  Legacy:  Caddy :80 + :443
  EdgeMux: HAProxy :443 + Caddy :80 + Caddy :8443
```

**能力声明和实际路由之间没有任何关联。** 矩阵只能看「这个 Provider 能做什么」，看不到「有多少路由依赖这个能力」。

### 1.2 模式切换（自动 + 不安全）

```go
func DetectRuntimeMode(states []ProviderState) RuntimeMode {
    // HAProxy 活着 → EdgeMux
    // 只有 Caddy  → Legacy
}
```

没有用户控制，没有预览，没有确认，没有回滚。

### 1.3 路由存储（独立于模式）

Route 存在 DB 里，带 `Composition` 字段标记它属于哪个组合（如 `"https_route"`、`"tls_passthrough"`）。模式切换时：

- DB 里的路由**不丢**
- Planner 根据新模式过滤：不支持的 Composition 跳过
- 跳过的路由**没有标记、没有警告**

---

## 二、需要加什么

### 2.1 能力关联分析

每个 Route 关联它依赖的能力链：

```
HTTPS Route:    tcp → tls_terminate → http → route_host
TLS Passthrough: tcp → sni_preread
TCP Forward:     tcp → raw_tcp
```

这不在 DB 里额外存——Composition 定义已经包含了：

```go
// internal/provider/composition.go
var CompHTTPSRoute = CompDef{
    Key: "https_route",
    Atoms: []string{"tcp", "tls", "http"},
}
```

Planner 已经在用这个判断支持不支持。缺的是：**把判断结果暴露给 API 和 UI**。

### 2.2 模式切换预览 API

```
POST /api/admin/v1/mode/preview
  { "target_mode": "legacy" }

→ {
    "current_mode": "edge_mux",
    "target_mode": "legacy",
    "total_routes": 47,
    "affected_routes": {
      "kept": 42,       // 新模式能正常服务的路由
      "degraded": 0,    // 功能降级但有替代
      "unsupported": 5  // 新模式不支持，将静默
    },
    "unsupported": [
      { "domain": "db.example.com", "composition": "tls_passthrough", "reason": "Legacy 不支持 SNI 透传" },
      { "domain": "vpn.example.com", "composition": "tcp_forward", "reason": "Legacy 下 :443 被 Caddy 占用" }
    ],
    "provider_changes": [
      { "provider": "haproxy", "action": "stop", "reason": "EdgeMux→Legacy 不需要 HAProxy" },
      { "provider": "caddy", "action": "reconfig", "detail": ":443 从 HAProxy 移回 Caddy" }
    ],
    "risks": [
      "5 条透传路由在 Legacy 下无法服务",
      "HAProxy 停止期间已有连接将被断开",
      "Caddy 重载配置时有秒级连接中断"
    ]
  }
```

### 2.3 模式切换执行 API

```
POST /api/admin/v1/mode/switch
  { "target_mode": "legacy", "confirm_risks": true }

→ {
    "status": "success",  // 或 "rolled_back"
    "backup_snapshot_id": "snap_20260707_140000",
    "provider_results": [
      { "provider": "haproxy", "action": "stopped", "ok": true },
      { "provider": "caddy", "action": "reloaded", "ok": true }
    ],
    "drift_check": { "consistent": true },
    "message": "已切换到 Legacy，5 条透传路由无法服务（见清单）"
  }
```

### 2.4 执行流程

```
1. 导出全量快照（snapshot export）
2. 停止不需要的 Provider（HAProxy）
3. 重新生成配置（Planner + 新模式模板）
4. Validate 新配置
5. 写入并重载
6. Verify（健康检查 + Drift 检测）
7. 如果 4/5/6 任何一步失败 → 从快照恢复
```

这 7 步**全部自动化，但每一步都报告状态**。

---

## 三、UI 布局

```
运行时模式
┌─────────────────────────────────────────────────────────┐
│  当前模式: EdgeMux                                       │
│                                                         │
│  ┌─ 能力概览 ───────────────────────────────────────┐  │
│  │  HTTPS Route:  42 条 · Caddy :80 + :443 ✅       │  │
│  │  TLS Passthrough: 5 条 · HAProxy :443 ✅         │  │
│  │  TCP Forward:  0 条                              │  │
│  │  UDP Forward:  0 条                              │  │
│  └──────────────────────────────────────────────────┘  │
│                                                         │
│  [切换模式]                                              │
│                                                         │
│  ┌─ 模式迁移预览 ────── (展开后) ─────────────────────┐  │
│  │  ⚠️ 切换到 Legacy 将影响:                           │  │
│  │    保持: 42 条                                      │  │
│  │    不可用: 5 条 TLS 透传                             │  │
│  │       db.example.com → 10.0.0.5:5432                │  │
│  │       vpn.example.com → 10.0.0.6:443                │  │
│  │    Provider 变更:                                    │  │
│  │       HAProxy → 停止                                 │  │
│  │       Caddy → 重载配置 (:443 从 HAProxy 移回)        │  │
│  │                                                     │  │
│  │  [确认切换]  [取消]                                  │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘

数据保护
┌─────────────────────────────────────────────────────────┐
│  自动备份: 每 6 小时 · 保留 7 天                         │
│  最近快照: 2026-07-07 14:00                             │
│  [立即导出] [从快照恢复]                                 │
└─────────────────────────────────────────────────────────┘
```

两条原则：
- **模式切换和数据保护分开** — 切换前自动触发快照，但用户也可以独立操作备份
- **不支持的路线灰化但显示** — 在路由列表中标明"当前模式不支持"，不隐藏

---

## 四、外部服务注册域名时

```
POST /api/v1/actions/bind-http-domain
  → 写入 DB ✅
  → Apply → Planner 判断当前模式支持 → 生效 ✅
  → 如果不支持 → 写入 DB ✅，路由标记 status=inactive
     → 返回: { status: "created_inactive", reason: "当前模式不支持此能力" }
```

**路由不会丢。** 切换回支持的模式后 Apply 即可生效。

---

## 五、实现顺序

| 步 | 内容 | 依赖 |
|----|------|------|
| 1 | Route → Capability 映射（从 Composition 定义推导） | 无 |
| 2 | 模式预览 API（只算不改） | 步 1 |
| 3 | 模式执行 API（备份→切换→验证→回滚） | 步 2 + Snapshot |
| 4 | UI 模式管理页面 | 步 2 + 步 3 |
| 5 | 不支持的 Composition 在路由列表中标记 | 步 1 |

要按这个顺序做吗？
