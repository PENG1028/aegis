# Gateway Ports Boundary

Current Aegis infrastructure only opens **ports 80 (TCP) and 443 (TCP/UDP)** on the cloud security group. No other ports are open between servers.

## Connectivity Pre-flight Checklist

Before any cross-server operation involving `target_host:port`:

1. Check if the port is in the allowed set (80, 443). If not, stop — don't use it.
2. Verify connectivity from the source server to the target:
   ```bash
   ssh -o StrictHostKeyChecking=no ubuntu@<source> \
     "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://<target_ip>:<port>/"
   ```
   Expected: 200, 401, or 403 (service responds). Anything else → port not open or service not running.
3. Only after step 2 passes, proceed with Aegis bind-http-domain / apply / trace.

## Allowed Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 80 | TCP | HTTP traffic via Caddy |
| 443 | TCP + UDP | HTTPS / TLS via HAProxy + Hysteria |
