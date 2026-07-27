# Single-Node Production Boundary — v1.7Z

## Status: Release Lockdown

This document defines what Aegis v1.7Z explicitly supports and does not support in single-node production use.

---

## ✅ Supported (Production Ready)

| Capability | Verified In | Notes |
|------------|-------------|-------|
| Single VPS control plane | v1.7Y (VPS) | Ubuntu 24.04, systemd |
| Single admin user | v1.7Y (VPS) | Cookie-based admin session |
| Multi-scope API keys | v1.7Y (VPS) | Space isolation, scope enforcement |
| HTTP domain binding | v1.7Y (VPS) | Action API: bind-http-domain |
| TLS backend binding | v1.7Y (VPS) | Action API: bind-tls-backend |
| Caddy config management | v1.7Y (VPS) | Render → validate → replace → reload |
| HAProxy config management | v1.7Y (VPS) | Render → validate → replace → reload |
| Safe apply pipeline | v1.7Y (VPS) | Lock → plan → render → validate → backup → hash → replace → reload → verify |
| Access path trace | v1.7Y (VPS) | Domain/SNI/Route trace with connectivity check |
| Provider diagnostics | v1.7Y (VPS) | 7 diagnostic codes for Caddy + HAProxy |
| Operation logging | v1.7E | Action-level operation_log |
| Apply step logging | v1.7W | Step-level apply_log |
| Audit logging | v1.7R | Auth failures, access denied |
| Restart safety | via architecture | Control/data plane separation (Caddy/HAProxy independent of Aegis process) |
| Manual rollback | via architecture | Config backup + binary rollback documented |
| Gateway mutation frozen | v1.7R | All gateway mutation APIs return 405 |

## ❌ Not Supported (Production Use Not Advised)

| Capability | Reason |
|------------|--------|
| Multi-node cluster | Only HAProxy + Caddy managed on single node; no cross-node sync, no consensus |
| Automatic binary upgrade | No upgrade session automation; manual binary swap only |
| Canary/staged deployment | No canary routing, no traffic splitting, no staged rollout |
| Cloudflare/tunnel automation | No Cloudflare API integration, no tunnel management |
| New providers (non-Caddy/non-HAProxy) | Only CaddyHTTP and HAProxy variants supported |
| SaaS/multi-tenant | Single admin, single VPS; no tenant isolation, no billing |
| Full observability protocol | operation/apply/audit logs exist but no structured tracing protocol (OTel, etc.) |
| Automatic self-healing | Detects drift but does not auto-fix; manual apply required |
| UI dashboard | CLI-only; no web UI |
| Multi-admin | One admin session at a time; no RBAC |

## ⚠️ Partial/Constrained Support

| Capability | Constraint |
|------------|-----------|
| Drift detection | Detected but not auto-repaired; store state (DB) vs actual provider config drift not monitored |
| Listener conflict detection | Only reported as `error_code` — no automatic port conflict resolution |
| Runtime provider verify | Caddy has curl-based verify; HAProxy runtime verify hardcodes `true` |
| Restart automation | Runbook exists but no automated process restart test |
| Log retention | No log rotation, no retention policy; manually managed via SQLite |

## Production Test Status

```
24 capabilities verified on real Ubuntu VPS:
  Caddy 2.6.2   ✅
  HAProxy 2.8.16 ✅
  Port 80/443   ✅
  admin login   ✅
  API key auth  ✅
  bind domain   ✅
  safe apply    ✅
  trace         ✅
  diagnose      ✅
  gateway frozen ✅
  auth denial   ✅

5 bugs found and fixed during VPS acceptance:
  login bypass, EnsureAdmin, middleware order,
  config path, AdminAuth wiring
```

## Recommended Monitoring

- `aegis smoke golden` — periodic health check
- `aegis smoke provider` — provider binary availability
- `POST /api/admin/v1/providers/diagnose` — full diagnostic check
- `GET /api/admin/v1/apply-logs` — check for failed applies
- `GET /api/admin/v1/audit-logs` — check for auth failures

---

*Last updated: 2026-06-25, v1.7Z Release Lockdown*
