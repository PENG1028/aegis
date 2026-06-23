# Aegis HTTP API 文档

## 认证

所有 API 端点需要 `Authorization: Bearer <token>` 头。

Admin token（配置文件中 `server.admin_token`）拥有 `admin:*` scope，可访问所有端点。

### 错误响应格式

```json
{
  "error": {
    "code": "RESOURCE_NOT_FOUND",
    "message": "route not found",
    "details": {}
  }
}
```

| HTTP 状态 | 说明 |
|-----------|------|
| 200 | 成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误 |
| 401 | 未认证（无 token 或 token 无效） |
| 403 | 无权限（scope 不足） |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |

## Scopes

| Scope | 说明 |
|-------|------|
| `admin:*` | 超级管理员（admin token 默认） |
| `system:read` | 读取系统状态 |
| `project:read` / `project:write` | 项目管理 |
| `service:read` / `service:write` | 服务管理 |
| `endpoint:read` / `endpoint:write` | 端点管理 |
| `route:read` / `route:write` | 路由管理 |
| `managed_domain:read` / `managed_domain:write` / `managed_domain:verify` | 受管域名 |
| `config:read` | 查看配置 |
| `apply:run` | 执行 apply |
| `rollback:run` | 执行 rollback |
| `health:read` / `health:run` | 健康检查 |
| `logs:read` | 查看日志 |
| `settings:read` / `settings:write` | 设置管理 |

## 端点列表

### System
```
GET /api/system/status
```

返回增强状态信息：代理状态、数据库 schema 版本、资源计数、最近 apply 状态、健康摘要。

### Projects
```
GET    /api/projects
POST   /api/projects
GET    /api/projects/:id
PATCH  /api/projects/:id
POST   /api/projects/:id/archive
```

### Services
```
GET    /api/services
POST   /api/services
GET    /api/services/:id
PATCH  /api/services/:id
POST   /api/services/:id/enable
POST   /api/services/:id/disable
```

### Endpoints
```
GET    /api/services/:id/endpoints
POST   /api/services/:id/endpoints
PATCH  /api/endpoints/:id
POST   /api/endpoints/:id/enable
POST   /api/endpoints/:id/disable
DELETE /api/endpoints/:id
```

### Routes
```
GET    /api/routes
POST   /api/routes
GET    /api/routes/:id
PATCH  /api/routes/:id
POST   /api/routes/:id/enable
POST   /api/routes/:id/disable
POST   /api/routes/:id/switch-service
POST   /api/routes/:id/maintenance-on
POST   /api/routes/:id/maintenance-off
```

### Managed Domains
```
GET    /api/managed-domains
POST   /api/managed-domains
GET    /api/managed-domains/:id
POST   /api/managed-domains/:id/verify
POST   /api/managed-domains/:id/enable
POST   /api/managed-domains/:id/disable
DELETE /api/managed-domains/:id
```

### Config / Apply
```
GET  /api/config/current      # 当前正式配置
GET  /api/config/preview      # 预览即将生成的配置（dry-run）
GET  /api/config/diff         # unified diff
POST /api/apply               # 执行 apply
POST /api/apply/dry-run       # dry-run（同 preview）
POST /api/rollback            # 回滚（可选 body: {"version": "v20260623-001"}）
GET  /api/apply/history       # apply 历史
```

### Diagnostics
```
GET /api/diagnostics/export   # 导出完整诊断 JSON（不含明文 token）
```

### Health
```
GET  /api/health
POST /api/health/check-all
GET  /api/health/services/:id
```

### Logs
```
GET /api/logs?action=apply&target=svc_xxx
```

### Settings
```
GET  /api/settings
PATCH /api/settings
```
