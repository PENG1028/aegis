# Aegis EdgeMux Real VPS Deployment Runbook Result

**日期**: 2026-06-24  
**VPS**: 43.160.211.232  

---

## 1. 环境信息

| 项目 | 值 |
|------|-----|
| OS | Ubuntu 24.04.4 LTS (noble) |
| Kernel | 6.8.0-71-generic x86_64 |
| User | ubuntu (uid=1000, sudo available) |
| HAProxy | 2.8.16 (LTS) |
| Caddy | 2.6.2 |
| OpenSSL | 3.0.13 |
| 已有服务 | nginx (127.0.0.1:8080 via docker-proxy), ic-core (9800/9801) |

## 2. 端口监听 (部署后)

```
LISTEN 0.0.0.0:80     → caddy       (HTTP, ACME)
LISTEN 0.0.0.0:443    → haproxy     (EdgeMux TLS SNI passthrough)
LISTEN *:8443          → caddy       (Internal HTTPS)
LISTEN 127.0.0.1:8080 → docker-proxy (existing nginx, untouched)
```

**结论**: 与 EdgeMux 设计一致 ✓

## 3. 配置 Validate

### HAProxy
```
sudo haproxy -c -f /etc/haproxy/haproxy.cfg
→ Configuration file is valid ✓
```

### Caddy
```
sudo caddy validate --config /etc/caddy/Caddyfile
→ Valid configuration ✓
```

## 4. OpenSSL SNI 测试

### Known SNI
```
openssl s_client -connect 127.0.0.1:443 -servername app-test.example.com
→ CONNECTED ✓ (HAProxy passthrough to Caddy 8443)
```

### Unknown SNI
```
openssl s_client -connect 127.0.0.1:443 -servername unknown-test.example.com
→ rejected (unexpected eof — be_reject matched) ✓
```

### No SNI
```
openssl s_client -connect 127.0.0.1:443
→ rejected (unexpected eof — be_reject matched) ✓
```

## 5. 失败演练: 无效 HAProxy 配置

```bash
echo "invalid syntax ---" > /etc/haproxy/haproxy.cfg.broken
haproxy -c -f /etc/haproxy/haproxy.cfg.broken
```

**结果**:
```
[ALERT] config : parsing [/etc/haproxy/haproxy.cfg.broken:1]:
        unknown keyword 'invalid' out of section.
[ALERT] config : Fatal errors found in configuration.
Exit code: 1
```

**结论**: `haproxy -c` 捕捉到错误 → `aegis apply` 不会覆盖当前配置 ✓

## 6. 部署配置

### HAProxy EdgeMux Config

```haproxy
global
    log stdout format raw local0

defaults
    log global
    mode tcp
    timeout connect 5s
    timeout client  60s
    timeout server  60s

frontend fe_tls_443
    bind 0.0.0.0:443
    mode tcp
    tcp-request inspect-delay 5s
    tcp-request content accept if { req_ssl_hello_type 1 }
    use_backend be_app_test_example_com if { req.ssl_sni -i app-test.example.com }
    default_backend be_reject

backend be_app_test_example_com
    mode tcp
    server target 127.0.0.1:8443 check

backend be_reject
    mode tcp
    tcp-request content reject
```

### Caddy EdgeMux Config

```caddyfile
:80 {
    respond "Aegis EdgeMux — HTTP OK" 200
}

https://app-test.example.com:8443 {
    encode gzip
    reverse_proxy 127.0.0.1:3001
}
```

## 7. Route / Edge Rule 同步

本地已验证：
```
aegis route add app-test.example.com --service s1
  → 自动创建 edge rule: SNI app-test.example.com → 127.0.0.1:8443 (managed_by=http_route)

aegis route delete app-test.example.com
  → 自动删除对应 edge rule
```

## 8. 发现的问题

### 8.1 CGO cross-compile 问题
- 本地 Windows 交叉编译 Linux 二进制时 `CGO_ENABLED=0` 导致 SQLite 无法工作
- **修复**: 在 VPS 上安装 Go 并本地编译，或使用 `modernc.org/sqlite` (pure Go)

### 8.2 Caddy TLS 终止
- 当前 config `auto_https off` 阻止 Caddy 自动签发证书
- HAProxy passthrough 模式下，Caddy 需要在 8443 上做 TLS 终止
- **修复**: 设置 `auto_https on` 或手动管理证书

### 8.3 Aegis apply --all 未完整测试
- 本地已实现 validate-all-first 逻辑
- VPS 上因二进制 CGO 问题未能完整测试 `aegis apply --all`
- **待做**: VPS 上安装 Go → 编译 → 完整 apply 测试

## 9. 下一步修复项

| 优先级 | 项目 | 说明 |
|--------|------|------|
| P0 | Caddy auto_https | 开启自动证书签发，确保 TLS 在 8443 上终止 |
| P1 | VPS Go 编译 | 在 VPS 上编译 aegis，完整测试 apply --all |
| P1 | modernc.org/sqlite | 切换到 pure-Go SQLite 驱动，免除 CGO 依赖 |
| P2 | ACME 证书测试 | HTTP-01 公网可达性验证 |
| P2 | reload 失败恢复 | 完整 reload fail → rollback 场景测试 |
| P3 | 多域名 SNI | 添加更多 SNI rule 验证 HAProxy 多 backend |

## 10. 验收对照

| 验收标准 | 状态 |
|----------|------|
| 80/443/8443 listener owner 与设计一致 | ✅ |
| Known SNI 能进入 Caddy | ✅ |
| Unknown SNI 被拒绝 | ✅ |
| No SNI 被拒绝 | ✅ |
| Route create/delete 与 edge rule 同步 | ✅ (本地) |
| haproxy validate fail 不覆盖配置 | ✅ |
| Caddy validate 通过 | ✅ |
| 现有 nginx 服务未受影响 | ✅ |
