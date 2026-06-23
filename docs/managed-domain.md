# ManagedDomain 受管域名

## 概述

ManagedDomain 是外部业务方或客户接入的受控域名。

**与 Route 的区别：**
- Route 是管理员维护的内部路由
- ManagedDomain 带有 DNS 验证流程，面向外部接入

## 状态机

```
pending_verification ──→ verified ──→ active ──→ disabled
        │                    ↑                       │
        └──→ failed ────────┘ ←──────────────────────┘
```

### 允许的状态转换

| 从 | 到 | 说明 |
|----|----|------|
| pending_verification | verified | DNS TXT 验证通过 |
| pending_verification | failed | DNS 验证失败 |
| verified | active | 管理员启用 |
| active | disabled | 管理员禁用 |
| disabled | active | 重新启用 |
| failed | pending_verification | 重新尝试验证 |

### 禁止的转换
- `pending_verification → active` — 必须先验证
- `verified → disabled` — 先激活再禁用
- 任何跳跃转换

### Force Enable
仅在 `admin:*` scope 下允许 force enable（跳过验证直接激活）。

## DNS 验证

创建 ManagedDomain 时自动生成 TXT 验证令牌：

```json
{
  "domain": "login.customer.com",
  "status": "pending_verification",
  "verification_type": "dns_txt",
  "verification_name": "_aegis.login.customer.com",
  "verification_value": "aegis-verify-xxx"
}
```

### 验证检查项

`aegis managed-domain verify` 执行以下 DNS 检查：

| 检查 | 类型 | 说明 |
|------|------|------|
| TXT | 必需 | `_aegis.<domain>` 的 TXT 记录必须匹配令牌 |
| CNAME | 可选 | 域名 CNAME 记录 |
| A | 信息 | 域名 A 记录 |
| AAAA | 信息 | 域名 AAAA 记录 |

### 结构化验证结果

```json
{
  "id": "md_xxx",
  "status": "verified",
  "checks": {
    "txt": {
      "expected": "aegis-verify-xxx",
      "actual": ["aegis-verify-xxx"],
      "ok": true
    },
    "cname": { "actual": ["gateway.example.com"], "ok": true, "warning": false },
    "a": { "actual": ["1.2.3.4"] },
    "aaaa": { "actual": [] }
  }
}
```

## Caddyfile 生成规则

只有 `status = active` 的 ManagedDomain 才进入 Caddyfile。

以下状态 **不生成** 配置：
- `pending_verification`
- `verified`
- `failed`
- `disabled`

## 安全约束

- ManagedDomain 只能绑定到 Aegis 已注册的 Service
- 不允许传入任意 upstream URL
- DNS 验证是必须的安全门禁
- 验证失败后可以重试（状态回到 pending_verification）
