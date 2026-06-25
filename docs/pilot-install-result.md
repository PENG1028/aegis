# Pilot Install Result — v1.7Z-RC

## Environment

| Property | Value |
|----------|-------|
| **Date** | 2026-06-25 |
| **OS** | Ubuntu 24.04.4 LTS |
| **Caddy** | 2.6.2 |
| **HAProxy** | 2.8.16-0ubuntu0.24.04.3 |
| **Aegis binary** | ELF 64-bit, ~13.9 MB, modernc sqlite |
| **Config path** | `/home/ubuntu/.aegis/config/config.yaml` |
| **DB path** | `/home/ubuntu/.aegis/aegis.db` |
| **Test domain** | `pilot.aegis.local` (hosts mapping) |
| **Test target** | `python3 -m http.server 3000` |

## Install Steps Executed

### Bootstrap
```
[config] /home/ubuntu/.aegis/config/config.yaml
[database] /home/ubuntu/.aegis/aegis.db (migrations applied)
[listeners] 3 registered
  caddy_http 0.0.0.0:80 (http) -> active
  haproxy_edge_mux 0.0.0.0:443 (tls_mux) -> active
  caddy_http 127.0.0.1:8443 (https) -> active
```

### Doctor
```
  haproxy:     /usr/sbin/haproxy (HAProxy version 2.8.16)
  caddy:       /usr/bin/caddy (2.6.2)
  openssl:     /usr/bin/openssl (OpenSSL 3.0.13)
  ports 80/443/8443: LISTENING
  3 listeners registered
```

### Config Path Consistency
Config saved at nested path `~/.aegis/config/config.yaml` and symlinked to `~/.aegis/config.yaml` for compatibility with both bootstrap save path and config loader lookup path.

### Login
```
{"expires_at":"2026-06-26T12:23:17+08:00",
 "user":{"id":"admin_f9a749267ce51592","username":"admin"}}
```

## Install Runbook Accuracy

| Runbook Step | Status | Notes |
|-------------|--------|-------|
| Install Caddy | ✅ | Already installed |
| Install HAProxy | ✅ | Already installed |
| Deploy Aegis binary | ✅ | scp + chmod |
| Create config | ✅ | Written to ~/.aegis/config/config.yaml |
| Bootstrap | ✅ | 3 listeners |
| Systemd service | ⚠️ | Not tested (running in foreground) |
| Doctor | ✅ | All checks pass |
| Provider diagnose | ✅ | healthy=True |
| First admin login | ✅ | Auto-created by EnsureAdmin |
| Smoke test | ✅ | failure-matrix 9/9 |

## Install Runbook Issues Found

| Issue | Severity | Fix |
|-------|----------|-----|
| Config path mismatch | Medium | bootstrap saves to `config/config.yaml` but loader looked for `config.yaml` — symlink workaround in use |
| No systemd test | Low | Runbook has unit file but it wasn't tested in this pilot (manual `nohup` used) |

## Verdict

Clean install procedure is repeatable and matches the documented install-runbook. The config path issue identified in v1.7Y is partially mitigated by symlink but should be fully fixed by adding `config/config.yaml` to the loader's search paths.
