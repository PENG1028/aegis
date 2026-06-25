# Pilot Rollback Drill Result — v1.7Z-RC

## Status: NOT_EXECUTED

**Reason:** Rollback drill requires:
1. A known-good previous binary to roll back TO
2. A SQLite snapshot to restore FROM
3. Provider config snapshots

During this pilot session, the Aegis binary was only deployed once (the v1.7Z-RC build). There is no "previous version" to roll back to. The provider configs (Caddy/HAProxy) are managed by the system and were not manually modified.

## Prepared Rollback State

| Artifact | Path | Status |
|----------|------|--------|
| Current binary | `/home/ubuntu/aegis` | ✅ In use |
| Backup binary | Not created | ❌ |
| SQLite snapshot | Not created | ❌ |
| Caddyfile backup | `/etc/caddy/Caddyfile.acceptance-backup` | ✅ From v1.7Y |
| HAProxy config backup | `/etc/haproxy/haproxy.cfg.acceptance-backup` | ✅ From v1.7Y |

## Rollback Procedure (Documented Only)

Per `docs/rollback-runbook.md`, the procedure for a binary rollback is:

1. `sudo systemctl stop aegis`
2. Swap binary: `sudo cp /usr/local/bin/aegis.bak /usr/local/bin/aegis`
3. `sudo systemctl start aegis`
4. Verify: `aegis doctor` + `aegis trace domain <domain>` + `POST /providers/diagnose`

This procedure is documented but was NOT executed during this pilot because:
- No known-bad binary was deployed (no regression to roll back from)
- Aegis architecture ensures rollback is safe (data plane unaffected by control plane changes)

## Rollback Safety Confirmation

Even without executing the drill, the architectural guarantee was confirmed by the **restart drill**:
- Aegis process can be stopped without affecting traffic
- Aegis process can be replaced (new binary) and restarted
- All state (SQLite) survives restart
- Provider configs (Caddy/HAProxy) are not modified by Aegis process lifecycle

## Recommended Pre-Rollback Actions

Before the next upgrade:
1. `cp /usr/local/bin/aegis /usr/local/bin/aegis.bak` — backup current binary
2. `cp /var/lib/aegis/aegis.db /var/lib/aegis/aegis.db.pre-upgrade` — snapshot DB
3. `cp /etc/caddy/Caddyfile /etc/caddy/Caddyfile.pre-upgrade`
4. `cp /etc/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg.pre-upgrade`

## Verdict

Rollback is architecturally safe but the drill was **NOT_EXECUTED**. Recommend executing before production use.
