# Aegis Install Runbook — v1.7Z

## Prerequisites

- Ubuntu 22.04 or 24.04
- Root or sudo access
- Caddy 2.x installed
- HAProxy 2.x installed
- Aegis binary (ELF, ~16MB, no CGO)

## Step 1: Install Caddy

```bash
sudo apt install -y caddy
caddy version
# Expected: v2.x.x
```

Configure Caddy for EdgeMux mode (Aegis will manage this file, but it must exist):

```bash
sudo touch /etc/caddy/Caddyfile
sudo systemctl enable caddy
sudo systemctl start caddy
```

## Step 2: Install HAProxy

```bash
sudo apt install -y haproxy
haproxy -v
# Expected: HAProxy version 2.x.x

sudo touch /etc/haproxy/haproxy.cfg
sudo systemctl enable haproxy
sudo systemctl start haproxy
```

## Step 3: Deploy Aegis Binary

```bash
# Copy binary
sudo cp aegis /usr/local/bin/aegis
sudo chmod +x /usr/local/bin/aegis

# Create data directories
sudo mkdir -p /var/lib/aegis/backups
sudo mkdir -p /etc/aegis
```

## Step 4: Create Config

```bash
cat > /etc/aegis/config.yaml << 'EOF'
proxy:
    provider: caddy
    caddyfile_path: /etc/caddy/Caddyfile
    caddy_binary: caddy
    caddy_data_dir: /var/lib/caddy/.local/share/caddy
    reload_command: systemctl reload caddy
    validate_command: '{{caddy_binary}} validate --config {{config_path}}'
    backup_dir: /var/lib/aegis/backups
    email: ""
    # 首次上线先使用 https://acme-staging-v02.api.letsencrypt.org/directory
    acme_server: ""
store:
    sqlite_path: /var/lib/aegis/aegis.db
server:
    addr: 127.0.0.1:7380
    admin_token: change-me
managed_domain:
    gateway_domain: ""
runtime:
    config_dir: /etc/aegis
    data_dir: /var/lib/aegis
EOF
```

## Step 5: Bootstrap

```bash
# Run as root or with sudo
sudo aegis bootstrap
# Expected: DB created, 3 listeners registered
```

## Step 6: Systemd Service

```bash
cat > /etc/systemd/system/aegis.service << 'EOF'
[Unit]
Description=Aegis Infrastructure Gateway Control
After=network.target caddy.service haproxy.service
Wants=caddy.service haproxy.service

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/aegis serve --addr 127.0.0.1:7380
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable aegis
sudo systemctl start aegis
```

At startup Aegis grants the existing `caddy` group read-only access to managed
certificate assets: `/var/lib/aegis/certs` is `0750` and `.crt`/`.key` files are
`0640`. Assets remain owned by Aegis and private keys are never world-readable.
If the `caddy` group is unavailable, Aegis logs a warning and keeps the files
private; install Caddy and restart Aegis before using explicit certificate
bindings. Caddy validate and reload commands are limited to 30 seconds so a
stuck service operation cannot hold the Apply lock indefinitely.

## Step 7: Verify Service

```bash
sudo systemctl status aegis
# Expected: active (running)
```

## Step 8: Run Doctor

```bash
sudo aegis doctor
# Verify: caddy found, haproxy found, ports listening
```

## Step 9: Provider Diagnose

```bash
curl -s -X POST http://127.0.0.1:7380/api/admin/v1/providers/diagnose \
  -H "Authorization: Bearer $(sudo cat /etc/aegis/config.yaml | grep admin_token | awk '{print $2}')"
# Expected: healthy=true
```

## Step 10: First Admin Login

```bash
# First admin user is auto-created on first serve
curl -s -c /tmp/aegis-cookies.txt -X POST http://127.0.0.1:7380/api/admin/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'
# Expected: {"user":{"id":"admin_...","username":"admin"}}

# Create first scope
curl -s -b /tmp/aegis-cookies.txt -X POST http://127.0.0.1:7380/api/admin/v1/scopes \
  -H "Content-Type: application/json" \
  -d '{"name":"default"}'
# Expected: {"id":"space_...","name":"default"}
```

## Step 11: Smoke Test

```bash
sudo aegis smoke golden
sudo aegis smoke provider
sudo aegis smoke failure-matrix
```

## Done

Aegis is now running and ready for domain binding via the Action API.

---

*Last updated: 2026-06-25, v1.7Z Release Lockdown*
