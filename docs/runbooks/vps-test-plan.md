# VPS 部署测试计划

## 前置条件

```bash
# Server A (43.160.211.232): 面板 + 网关
ssh ubuntu@43.160.211.232

# Server B (43.159.34.11): 远程节点
ssh ubuntu@43.159.34.11

# 端口规则: 只有 80 TCP + 443 TCP/UDP 对外开放
# 跨机测试前必须验证连通性
```

## Phase 1: ServiceAuth 基础功能

### 1.1 部署新版本
```bash
cd /path/to/aegis
make deploy-server-a   # 面板节点
make deploy-server-b   # 远程节点
```

### 1.2 验证 ServiceAuth 启动
```bash
curl -s http://127.0.0.1:7380/api/healthz  # 返回 200
# 检查日志: serviceauth 初始化
sudo journalctl -u aegis --no-pager -n 20 | grep serviceauth
```

### 1.3 手动注册测试服务
```bash
# 生成 Ed25519 密钥对
openssl genpkey -algorithm Ed25519 -out test_priv.pem
openssl pkey -in test_priv.pem -pubout -out test_pub.pem

# 注册
curl -X POST http://127.0.0.1:7380/api/service-auth/v1/register \
  -H "Content-Type: application/json" \
  -d '{
    "service_name": "test-service",
    "host": "127.0.0.1",
    "port": 9999,
    "node_host": "server-a",
    "pubkey": "<base64-encoded public key>",
    "apis": [{"name":"health","path":"/health","method":"GET"}]
  }'

# 预期: 200 + service_id + instances + public_keys
```

### 1.4 验证注册成功
```bash
# 列出所有服务
curl -s http://127.0.0.1:7380/api/admin/v1/service-auth/services \
  -H "Cookie: aegis_admin_session=<session_token>"

# 检查数据库
ssh ubuntu@43.160.211.232 "sqlite3 /var/lib/aegis/aegis.db 'SELECT name,host,port,public_key FROM svc_auth_services'"
```

### 1.5 跨节点注册
```bash
# Server B 上注册服务，验证 Server A 能发现
ssh ubuntu@43.159.34.11 "curl -X POST http://127.0.0.1:7380/api/service-auth/v1/register ..."
# 在 Server A 验证能看到 Server B 的服务
```

## Phase 2: 出口网关 + 透明代理

### 2.1 验证 DNS 解析器（需要 dnsmasq）
```bash
# 安装 dnsmasq
ssh ubuntu@43.160.211.232 "sudo apt install -y dnsmasq"
# 检查状态
curl -s http://127.0.0.1:7380/api/admin/v1/dns/status
# 预期: running=true
```

### 2.2 验证透明代理（需要 iptables + root）
```bash
# 检查透明代理状态
curl -s http://127.0.0.1:7380/api/admin/v1/transparent/status
# 预期: available=true (Linux)
# 检查 iptables 规则
ssh ubuntu@43.160.211.232 "sudo iptables -t nat -L OUTPUT"
```

### 2.3 验证基础设施检测
```bash
curl -s http://127.0.0.1:7380/api/admin/v1/infra/status
# 预期: certbot, iptables, dnsmasq 全部就绪
```

## Phase 3: 完整端到端流程

### 3.1 创建路由 + 绑定证书
```bash
# 1. 上传证书
curl -X POST http://127.0.0.1:7380/api/admin/v1/certificates \
  -H "Content-Type: application/json" \
  -d '{"cert_pem":"...", "key_pem":"...", "note":"test"}'

# 2. 创建 HTTPS 路由
curl -X POST http://127.0.0.1:7380/api/v1/actions/bind-http-domain \
  -H "Content-Type: application/json" \
  -d '{"domain":"test.example.com", "target_host":"127.0.0.1", "target_port":3000}'

# 3. 验证 Caddyfile 包含正确配置
ssh ubuntu@43.160.211.232 "cat /etc/caddy/Caddyfile"
```

### 3.2 跨节点 Gateway Link + ServiceAuth
```bash
# 1. 创建 Gateway Link (Server A → Server B)
# 2. 在 Server B 注册服务
# 3. 在 Server A 创建路由指向 Server B 的服务
# 4. 通过 Server A 的 Caddy 访问 Server B 的服务 → 验票通过
```

## Phase 4: 容灾测试

### 4.1 Aegis 重启
```bash
# 停止 Aegis
ssh ubuntu@43.160.211.232 "sudo systemctl stop aegis"
# 验证 Caddy 仍正常（配置已在磁盘）
curl -s -o /dev/null -w '%{http_code}' http://43.160.211.232/
# 启动 Aegis
ssh ubuntu@43.160.211.232 "sudo systemctl start aegis"
# 验证服务恢复
```

### 4.2 iptables 恢复
```bash
# 停止 Aegis（iptables 规则残留）
ssh ubuntu@43.160.211.232 "sudo systemctl stop aegis"
ssh ubuntu@43.160.211.232 "sudo iptables -t nat -L OUTPUT | grep aegis"
# 预期: 规则仍在
# 启动 Aegis → CleanupStaleRules 清理 → 重建
ssh ubuntu@43.160.211.232 "sudo systemctl start aegis"
```

## 验证清单

| 测试项 | 预期结果 | 实际 |
|--------|---------|------|
| Service 注册 | 200 + service_id | |
| 同节点验票 | 200 | |
| 跨节点验票 | 200 | |
| DNS 解析器 | running=true | |
| 透明代理 | available=true | |
| 基础设施检测 | 3/3 就绪 | |
| 证书上传 | 201 | |
| 路由创建 | 200 + Caddyfile 包含 tls | |
| Aegis 停止 → Caddy 正常 | 200 | |
| Aegis 重启 → iptables 恢复 | 规则重建 | |
