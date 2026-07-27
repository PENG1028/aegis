# deploy — SSH 部署工具包

> 位置：`internal/deploy/` + `internal/httpapi/handlers/deploy_node.go`
> 前端：`ui/src/pages/runtime/DeployNode.tsx`
> 参考价值：高。展示了 Go 服务远程部署的完整实现：SSH 抽象、多认证模式、结构化日志、前后端配合。

---

## 架构

```
POST /api/admin/v1/nodes/deploy
         │
         ▼
  deploy.Connect(SSHConfig)        ← internal/deploy/executor.go: interface
         │
         ├─ Executor.Run()         ← internal/deploy/ssh.go: 执行命令
         ├─ Files.CopyTo()         ← internal/deploy/ssh.go: 传输文件
         └─ Services.Install()     ← internal/deploy/ssh.go: 管理服务
```

## 认证模式

| 模式 | 前端字段 | 后端处理 |
|------|---------|---------|
| `key` | SSH 私钥（粘贴或文件上传） | 写入 temp file → `ssh -i` |
| `password` | SSH 密码 | 写入 temp file → `sshpass -f` |
| `token` | Join Token | 无 SSH，仅注册到面板 |

## 参考点

- `internal/deploy/executor.go` — 接口定义（Executor, FileTransfer, ServiceManager）
- `internal/deploy/ssh.go` — SSH 实现（密钥/密码/temp file 安全处理）
- `internal/httpapi/handlers/deploy_node.go` — HTTP handler + 7 步部署流程
- `ui/src/pages/runtime/DeployNode.tsx` — 前端表单 + 日志展示

## UI 设计 (@ui 标注)

后端代码中用 `// @ui:` 标注了前端实现建议，全文搜索即可找到。
包括：表单字段映射、验证规则、日志渲染、错误状态处理。
