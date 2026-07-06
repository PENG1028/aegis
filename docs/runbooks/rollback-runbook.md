# Aegis Rollback Runbook — v1.7Z

## Principle: Data Plane Never Touched

Aegis is a control plane only. Rolling back Aegis does NOT affect HAProxy/Caddy data plane traffic. The data plane continues serving from the last applied config files.

---

## Rollback Scenarios

### Scenario 1: Aegis Binary Bug (Most Common)

**Trigger:** New Aegis binary has a bug (e.g., login broken, apply failing, trace not working).

**Impact:** Control plane unavailable. Existing traffic continues. No new domain binds possible.

**Rollback steps:**

```bash
# 1. Stop Aegis
sudo systemctl stop aegis

# 2. Verify data plane still works
curl -s -o /dev/null -w "%{http_code}" -H "Host: your-domain.com" http://127.0.0.1:80/
# Expected: 200 (traffic continues)

# 3. Restore previous binary
sudo cp /usr/local/bin/aegis.bak /usr/local/bin/aegis

# 4. Restart Aegis
sudo systemctl start aegis

# 5. Verify control plane
sudo aegis doctor
curl -s -b /tmp/cookies.txt http://127.0.0.1:7380/api/admin/v1/auth/me

# 6. Verify provider health
curl -s -X POST http://127.0.0.1:7380/api/admin/v1/providers/diagnose
```

**Recovery time:** ~30 seconds

---

### Scenario 2: Apply Failed (Config Broken)

**Trigger:** `POST /api/system/apply` returns error, no config change applied.

**Impact:** Desired state changed in DB, but provider config NOT updated. No traffic impact.

**Rollback steps:**

```bash
# 1. Check what changed
curl -s http://127.0.0.1:7380/api/admin/v1/operations

# 2. Check apply log for error
curl -s http://127.0.0.1:7380/api/admin/v1/apply-logs | python3 -m json.tool

# 3. Revert the desired state (example: disable the broken route)
curl -s -X POST http://127.0.0.1:7380/api/v1/actions/disable-domain \
  -H "Content-Type: application/json" \
  -d '{"domain":"problematic-domain.com"}'

# 4. Try apply again with corrected state
curl -s -X POST http://127.0.0.1:7380/api/admin/v1/system/apply
```

---

### Scenario 3: Provider Config Corrupted

**Trigger:** Apply succeeded but HAProxy/Caddy config has a problem (rare due to validate step).

**Impact:** Provider reload may fail. Old config still running.

**Rollback steps:**

```bash
# 1. Check provider status
caddy validate --config /etc/caddy/Caddyfile
haproxy -c -f /etc/haproxy/haproxy.cfg

# 2. Restore from Aegis backup
sudo cp /var/lib/aegis/backups/Caddyfile.*.bak /etc/caddy/Caddyfile
sudo systemctl reload caddy

# 3. Or restore from manual backup
sudo cp /etc/caddy/Caddyfile.rollback-before /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

---

### Scenario 4: SQLite State Corruption

**Trigger:** DB file corrupted (rare with SQLite).

**Impact:** Aegis crashes on startup, cannot read state.

**Rollback steps:**

```bash
# 1. Stop Aegis
sudo systemctl stop aegis

# 2. Restore from SQLite snapshot
sudo cp /var/lib/aegis/aegis.db.snapshot /var/lib/aegis/aegis.db

# 3. Restart Aegis
sudo systemctl start aegis

# 4. Verify state
sudo aegis doctor
curl -s http://127.0.0.1:7380/api/system/status
```

---

## Prepared Recovery

### Before Upgrade Actions

Before deploying a new Aegis binary:

```bash
# 1. Backup current binary
sudo cp /usr/local/bin/aegis /usr/local/bin/aegis.bak

# 2. Backup SQLite DB
cp /var/lib/aegis/aegis.db /var/lib/aegis/aegis.db.pre-upgrade

# 3. Backup provider configs
sudo cp /etc/caddy/Caddyfile /etc/caddy/Caddyfile.pre-upgrade
sudo cp /etc/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg.pre-upgrade

# 4. Record current state
curl -s http://127.0.0.1:7380/api/system/status > /tmp/aegis-state-pre-upgrade.json
```

### Verify Rollback Path

```bash
# Test that rollback procedure works (without actual rollback):
echo "Rollback binary: $(ls -la /usr/local/bin/aegis.bak)"
echo "Rollback DB: $(ls -la /var/lib/aegis/aegis.db.pre-upgrade)"
echo "Rollback Caddy: $(ls -la /etc/caddy/Caddyfile.pre-upgrade)"
echo "Rollback HAProxy: $(ls -la /etc/haproxy/haproxy.cfg.pre-upgrade)"
```

---

## Quick Reference

| Symptom | Action | Downtime |
|---------|--------|:--------:|
| Aegis crash loop | Restore old binary | 30s |
| Apply fails | Fix desired state, re-apply | 0s |
| Config corrupted | Restore from backup | 0s (old config still running) |
| DB corrupted | Restore DB snapshot | 30s |
| Full recovery | Binary + DB + config restore | 60s |

All rollbacks preserve data plane traffic. Aegis is NOT in the request path.

---

*Last updated: 2026-06-25, v1.7Z Release Lockdown*
