---
name: aegis-deploy
description: Deploy Aegis to production VPS — build, upload, bootstrap, start
model: sonnet
---

# Aegis Deploy Skill

Use this skill when the user asks to deploy Aegis, push to VPS, or set up the gateway on a server.

## Prerequisites Check

Before deploying, verify:
```bash
# Local tools
go version    # 1.22+
npm --version
make --version

# SSH access
ssh ubuntu@<SERVER_A_IP> "echo ok"
ssh ubuntu@<SERVER_B_IP> "echo ok"
```

## Deployment Targets

| Server | IP | Role |
|--------|-----|------|
| Server A | <SERVER_A_IP> | Primary: panel + gateway + Caddy + HAProxy |
| Server B | <SERVER_B_IP> | Secondary: remote node + backend services |

## Deploy Commands

### Full deploy (build + upload + bootstrap + start):
```bash
cd "F:/Work Document/project/aegis"

# Deploy Server A (panel)
make deploy-server-a

# Deploy Server B (remote node)
make deploy-server-b
```

The deploy script (`scripts/deploy.sh`) does:
1. Build Linux amd64 binary with embedded UI
2. Install Caddy + HAProxy on target (if missing)
3. Upload binary to /usr/local/bin/aegis
4. Run `aegis bootstrap --production`
5. Install systemd service
6. Start Aegis
7. Print admin credentials and panel URL

### Update existing deployment:
```bash
make update-server-a    # Update Server A (with backup + health check)
make update-server-b    # Update Server B
make update-all         # Update both (Server B first, then A)
```

The update script (`scripts/update.sh`) does:
1. Build new binary
2. Pre-update health check (GET /api/healthz)
3. Backup: binary, database, config (to /var/lib/aegis/backups/)
4. Graceful stop (systemctl stop)
5. Upload and verify binary (size check)
6. Start (systemctl start)
7. Post-update health check (up to 5 retries)
8. Auto-rollback instructions on failure

## What to do after deploy

1. Open http://<SERVER_A_IP> in browser
2. Login with credentials from deploy output (or `ssh ubuntu@<ip> "sudo journalctl -u aegis --no-pager | grep Password"`)
3. Go to 「节点」 page — verify both nodes appear
4. Go to 「创建映射」 — create first HTTP route
5. Go to 「推送配置」 — Apply with dry-run first, then apply

## Troubleshooting Deploy Issues

### Binary won't start
```bash
ssh ubuntu@<ip> "sudo journalctl -u aegis --no-pager -n 50"
ssh ubuntu@<ip> "sudo systemctl status aegis"
```

### Caddy/HAProxy not running
```bash
ssh ubuntu@<ip> "sudo systemctl status caddy haproxy"
ssh ubuntu@<ip> "sudo systemctl enable --now caddy haproxy"
```

### Port connectivity issues
Remember: **Only ports 80 (TCP) and 443 (TCP+UDP) are open on the cloud security group.**
```bash
# Test cross-server connectivity
ssh ubuntu@<SERVER_A_IP> "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://<SERVER_B_IP>:80/"
```

### Rollback on failure
```bash
# Find backup
ssh ubuntu@<ip> "ls -t /var/lib/aegis/backups/"
# Restore binary
ssh ubuntu@<ip> "sudo cp /var/lib/aegis/backups/aegis.<timestamp> /usr/local/bin/aegis"
# Restore DB
ssh ubuntu@<ip> "sudo cp /var/lib/aegis/backups/aegis.<timestamp>.db /var/lib/aegis/aegis.db"
# Restart
ssh ubuntu@<ip> "sudo systemctl restart aegis"
```

## Security Notes

- Admin password is random-generated on first bootstrap, printed ONCE in journal
- API listens on 127.0.0.1:7380 only (not exposed to internet)
- Caddy handles port 80 (public HTTP)
- HAProxy handles port 443 (TLS SNI passthrough)
- Config file permissions: 0600
