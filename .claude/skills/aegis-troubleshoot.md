---
name: aegis-troubleshoot
description: Diagnose and fix Aegis runtime issues
model: sonnet
---

# Aegis Troubleshoot

## Health Check
```bash
TARGET="43.160.211.232"
ssh ubuntu@$TARGET "systemctl is-active aegis caddy haproxy"
ssh ubuntu@$TARGET "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://127.0.0.1:7380/api/healthz"
```

## Logs
```bash
ssh ubuntu@$TARGET "sudo journalctl -u aegis --no-pager -n 100"
ssh ubuntu@$TARGET "sudo journalctl -u caddy --no-pager -n 50"
```

## Common Fixes

**Panel 502:** Check Caddyfile has `reverse_proxy 127.0.0.1:7380`, re-apply config.

**Node offline:** Check cross-server connectivity on port 80 between VPS.

**Forwarding broken:** Preview config via API, check Caddyfile, force re-apply.

**Gateway Link auth fail:** Check clock sync between servers (HMAC 5-min window).

**DB corrupt:** Restore from `/var/lib/aegis/backups/` and restart.

## Key Paths
- Config: /etc/aegis/config.yaml (0600)
- DB: /var/lib/aegis/aegis.db
- Backups: /var/lib/aegis/backups/
