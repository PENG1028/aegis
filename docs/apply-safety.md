# Apply 安全性模型

## 概述

Aegis 的配置下发（apply）是最高危操作——生成的配置错误可能导致 Caddy 重载后所有服务不可用。

因此 apply 流程设计为 **多阶段事务**，任何阶段失败都不会影响当前运行的配置。

## Staged Apply Flow

```
1. Plan         → 读取 routes + managed_domains，解析 endpoints
2. Render       → ProxyAdapter.Render() 生成配置
3. Write Temp   → 写入临时文件（不影响正式配置）
4. Validate     → caddy validate（或自定义 validate 命令）
5. Backup       → 备份当前正式配置到 backup_dir
6. Replace      → 原子替换正式配置
7. Reload       → 重载代理
8. On Failure   → 恢复备份，重载恢复的配置
9. Record       → 写入 apply_versions + operation_logs
```

## 失败处理规则

### Validate 失败
- 临时文件被删除
- 正式配置 **不被替换**
- 错误记录到 operation_logs
- 返回 `PROXY_VALIDATE_FAILED`

### Reload 失败
- 从备份恢复旧配置
- 对恢复的配置执行 validate
- 重载恢复的配置
- 如果恢复后重载也失败 → 记录 CRITICAL 日志
- 返回 `PROXY_RELOAD_FAILED`

### Backup 失败
- 不替换配置（安全失败）
- 记录失败日志

## Dry-run

`aegis apply --dry-run` 或 `POST /api/apply/dry-run`：

- 执行 Plan + Render
- **不写文件**
- **不备份**
- **不替换**
- **不重载**

返回结构化结果：

```json
{
  "rendered_config": "...",
  "warnings": [
    {
      "code": "NO_AVAILABLE_ENDPOINT",
      "message": "service demo-web has no available endpoint",
      "target": "svc_xxx",
      "severity": "critical"
    }
  ],
  "route_count": 3,
  "managed_domain_count": 1,
  "skipped_count": 2
}
```

## Rollback

### 默认回滚
`aegis rollback` 或 `POST /api/rollback`：
- 找到最近一次 status=success 的 apply
- 从其 backup_path 恢复配置
- Validate 恢复的配置
- Reload

### 指定版本回滚
`aegis rollback --version v20260623-001` 或 `POST /api/rollback {"version": "v20260623-001"}`：
- 在 apply_versions 中查找指定版本
- 该版本必须有 backup_path 且 status=success
- 执行同样恢复流程

### 回滚记录
回滚成功后写入：
- `apply_versions.status = rolled_back`
- `operation_logs.action = rollback.success`

## ApplyPlan 结构

```go
type ApplyPlan struct {
    Routes             []RouteConfig
    Warnings           []ApplyWarning
    RenderedConfig     string
    TempPath           string
    BackupPath         string
    RouteCount         int
    ManagedDomainCount int
    SkippedCount       int
}
```

## Warning 类型

| Code | Severity | 说明 |
|------|----------|------|
| SERVICE_DISABLED | warning | 服务已禁用，对应路由跳过 |
| NO_AVAILABLE_ENDPOINT | critical | 服务无可用端点 |
| ENDPOINT_UNREACHABLE | warning | 端点 TCP 不可达 |
| ROUTE_SKIPPED | critical | 路由因配置问题被跳过 |

## FakeProxyAdapter

测试环境使用 `FakeProxyAdapter`，支持：
- `ValidateShouldFail` — 模拟校验失败
- `ReloadShouldFail` — 模拟重载失败

不依赖真实 Caddy 即可验证 apply 安全性。
