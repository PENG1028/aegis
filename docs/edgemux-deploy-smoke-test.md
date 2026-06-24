# Aegis EdgeMux 部署烟雾测试

目标：验证 EdgeMux 模式在真实 Linux VPS 上能跑通。

## 前提

- Ubuntu 20.04+ / Debian 11+ / CentOS 8+
- 有公网 IP 或域名解析到此机器
- 已有 DNS 记录指向此机器

## Step 1 — 安装依赖

```bash
# HAProxy (EdgeMux TLS SNI passthrough)
sudo apt update && sudo apt install -y haproxy
haproxy -v    # ≥ 1.8 required for req.ssl_sni

# Caddy (HTTP/HTTPS application proxy)
sudo apt install -y debian-keyring debian-archive-keyring
curl -1sLf 'https://dl.cloudcaddy.com/com/caddy/stable?os=linux&arch=amd64' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update && sudo apt install -y caddy
caddy version

# OpenSSL (for SNI testing)
openssl version

# Aegis
go build -o aegis ./cmd/aegis/
```

## Step 2 — 初始化 Aegis

```bash
# 使用生产配置初始化
sudo mkdir -p /etc/aegis /var/lib/aegis/backups

cat > /etc/aegis/config.yaml << 'EOF'
proxy:
  provider: caddy
  caddyfile_path: /etc/caddy/Caddyfile
  caddy_binary: caddy
  reload_command: systemctl reload caddy
  validate_command: caddy validate --config {{config_path}}
  backup_dir: /var/lib/aegis/backups
  email: "admin@example.com"

store:
  sqlite_path: /var/lib/aegis/aegis.db

server:
  addr: 127.0.0.1:7380
  admin_token: "change-me-on-production"

managed_domain:
  gateway_domain: ""

runtime:
  config_dir: /etc/aegis
  data_dir: /var/lib/aegis
EOF

sudo ./aegis --config /etc/aegis/config.yaml init
```

## Step 3 — 检查部署就绪

```bash
sudo ./aegis doctor
```

确认：
- haproxy binary: found, version ≥ 1.8
- caddy binary: found
- /etc/haproxy/haproxy.cfg: writable
- /etc/caddy/Caddyfile: writable
- port 80/443/8443 状态

## Step 4 — 创建测试服务

```bash
# 创建项目和测试服务
./aegis project create demo
./aegis service add demo-app --project demo --kind http
./aegis endpoint add demo-app --type local --address http://127.0.0.1:3001
```

## Step 5 — 创建 HTTP Route

```bash
# 用真实域名替换 app.yourdomain.com
./aegis route add app.yourdomain.com --service demo-app
```

这将自动创建：
- Caddy route: `https://app.yourdomain.com:8443 { reverse_proxy 127.0.0.1:3001 }`
- EdgeMux rule: `SNI app.yourdomain.com → 127.0.0.1:8443`

## Step 6 — 预览配置

```bash
# 查看 Caddy 配置
./aegis config preview --provider caddy

# 查看 HAProxy 配置
./aegis config preview --provider haproxy

# 查看 EdgeMux 规则
./aegis edge rule list
```

## Step 7 — Apply

```bash
# Dry-run
sudo ./aegis apply --dry-run

# 正式 apply
sudo ./aegis apply --all
```

## Step 8 — 检查端口监听

```bash
sudo ss -ltnp | grep -E ':80|:443|:8443'
```

期望：
- `0.0.0.0:80` → caddy
- `0.0.0.0:443` → haproxy
- `127.0.0.1:8443` → caddy

## Step 9 — OpenSSL SNI 测试

```bash
# known SNI → 期望连接成功
openssl s_client -connect 127.0.0.1:443 -servername app.yourdomain.com -quiet

# unknown SNI → 期望被拒绝
openssl s_client -connect 127.0.0.1:443 -servername unknown.example.com -quiet

# no SNI → 期望被拒绝
openssl s_client -connect 127.0.0.1:443 -quiet
```

## Step 10 — curl 验证

```bash
# HTTP → 期望 308 redirect 或 200
curl -I http://app.yourdomain.com

# 启动测试后端
python3 -m http.server 3001 &

# HTTPS → 期望 200 或证书验证（自签或 Let's Encrypt）
curl -k https://app.yourdomain.com
```

## Step 11 — Runtime 检查

```bash
sudo ./aegis edge check --runtime
```

检查输出中：
- [providers] HAProxy 和 Caddy 状态
- [ports] 端口监听状态
- [runtime] SNI 命中/拒绝测试

## Step 12 — 诊断导出

```bash
./aegis diagnostics export
cat aegis-diagnostics-*.json
```

## Troubleshooting

### HAProxy 未监听 443
```bash
sudo journalctl -u haproxy --no-pager -n 50
sudo haproxy -c -f /etc/haproxy/haproxy.cfg
```

### Caddy 证书错误
```bash
sudo journalctl -u caddy --no-pager -n 50
# 如果 HTTP-01 challenge 失败：
# - 确保 DNS 已指向此机器
# - 确保 80 端口可以从公网访问
# - 否则考虑 DNS-01 challenge
```

### 端口已被占用
```bash
sudo ss -ltnp | grep <port>
sudo lsof -i :<port>
```

### Aegis apply 权限不足
```bash
# reload 需要 systemctl 权限
sudo ./aegis apply --all

# 或配置 sudo 免密：
# echo "aegis ALL=(ALL) NOPASSWD: /usr/bin/systemctl reload haproxy, /usr/bin/systemctl reload caddy" | sudo tee /etc/sudoers.d/aegis
```
