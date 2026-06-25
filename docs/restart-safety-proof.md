# Restart Safety Proof — v1.7V

## What "restart-check" Actually Proves

**Source:** `internal/smoke/service.go:RunRestartCheck()` (line ~307)

`aegis smoke restart-check` performs 5 read-only checks:

| # | Check | Method | Evidence |
|---|-------|--------|----------|
| 1 | DB accessible | `DB.Ping()` | smoke/service.go |
| 2 | state_version not 0 | `StateVer.Current()` | smoke/service.go |
| 3 | pending_apply = false | `PendingSt.Status()` | smoke/service.go |
| 4 | listeners preserved | `ListenerSvc.ListAll()` | smoke/service.go |
| 5 | config file exists | `os.Stat(configPath)` | smoke/service.go |

**All 5 checks are read-only.** None verify process lifecycle, data plane behavior, or before/after state comparison.

---

## What restart-check CANNOT prove

| Claim | Can restart-check prove it? | Why not? |
|-------|:---:|------|
| "Aegis restart doesn't affect data plane" | ❌ | restart-check never touches HAProxy/Caddy processes. It's a SQLite state check. |
| "Traffic continues during downtime" | ❌ | No curl/openssl test during Aegis downtime |
| "Node re-registers after restart" | ❌ | No before/after comparison of node records |
| "Leader recovers after restart" | ❌ | Checks only state_version != 0, not leader election |
| "No duplicate routes after restart" | ❌ | Checks listener count only, not route uniqueness |
| "Config hash consistent" | ❌ | Checks config file EXISTS, not content hash |
| "HAProxy/Caddy config files unchanged" | ❌ | Only checks aegis config file, not /etc/haproxy/haproxy.cfg |

---

## Restart Safety: Real Architecture

The actual restart safety guarantee comes from ARCHITECTURE, not from smoke tests:

### True Safety Property 1: Control Plane / Data Plane Separation
- **Evidence:** Aegis runs as a separate process from HAProxy/Caddy
- **Why it's safe:** `kill aegis` does NOT kill haproxy or caddy
- **Proof method:** Manual test — start aegis, start traffic, kill aegis, verify traffic continues
- **Can restart-check prove this?** ❌ No — restart-check doesn't manage processes

### True Safety Property 2: Config File Persistence
- **Evidence:** `apply/service.go:172` — `executor.Replace()` writes to disk
- **Why it's safe:** Config files survive process restart because they're on disk, not in memory
- **Proof method:** Manual test — check `cat /etc/haproxy/haproxy.cfg` before and after restart
- **Can restart-check prove this?** ❌ No — only checks if file exists, not content integrity

### True Safety Property 3: SQLite Persistence
- **Evidence:** `store.OpenSQLite()` opens a file-based DB
- **Why it's safe:** SQLite is file-based, all state persists across process restart
- **Proof method:** Manual test — query DB before and after restart
- **Can restart-check prove this?** ✅ Partially — `DB.Ping()` proves DB is accessible

### True Safety Property 4: No Auto-Reapply on Restart
- **Evidence:** `cmd/aegis/main.go` — bootstrap/apply are explicit CLI commands, not automatic on `serve`
- **Why it's safe:** Aegis doesn't auto-apply on startup — stale config won't be accidentally deployed
- **Can restart-check prove this?** ❌ No — just checks `pending_apply` flag state

---

## What restart-check IS good for

1. **Post-restart health ping:** Quick sanity check that Aegis came back up clean
2. **State drift detection:** If `state_version == 0` after restart, something went wrong
3. **pending_apply anomaly detection:** If `pending_apply == true` but no recent mutations, may indicate unclean shutdown

---

## Restart Safety: Actual Verification Required

To truly prove restart safety, a manual runbook is required:

```bash
# Phase 1: Before restart
aegis smoke golden > /tmp/before_restart.txt
curl -s http://localhost:9000/api/admin/v1/nodes | jq . > /tmp/nodes_before.json
curl -s http://localhost:9000/api/admin/v1/routes | jq . > /tmp/routes_before.json
md5sum /etc/haproxy/haproxy.cfg > /tmp/haproxy_md5_before.txt
md5sum /etc/caddy/Caddyfile > /tmp/caddy_md5_before.txt

# Phase 2: During downtime
kill $AEGIS_PID
curl -H "Host: test.example.com" http://127.0.0.1:80/  # MUST return (200 or 502, not connection refused)
sleep 2

# Phase 3: After restart
./aegis serve --port 9000 &
sleep 2
aegis smoke golden > /tmp/after_restart.txt
diff /tmp/before_restart.txt /tmp/after_restart.txt
md5sum /etc/haproxy/haproxy.cfg > /tmp/haproxy_md5_after.txt
diff /tmp/haproxy_md5_before.txt /tmp/haproxy_md5_after.txt  # MUST be identical
```

**Status:** This manual runbook is documented in `docs/restart-safety-runbook.md` but has NOT been executed as an automated test.

---

## Verdict

| Property | Verification Level |
|----------|-------------------|
| Aegis stop → data plane continues | DOC_ONLY (architectural claim, manual test needed) |
| Aegis restart → DB accessible | ✅ REAL (restart-check proves DB.Ping) |
| Aegis restart → state_version preserved | ✅ REAL (restart-check verifies != 0) |
| Aegis restart → pending_apply preserved | ✅ REAL (restart-check verifies false) |
| Aegis restart → listeners preserved | ✅ REAL (restart-check counts listeners) |
| Aegis restart → config files intact | PARTIAL (checks file exists, not content hash) |
| Aegis restart → no duplicate resources | ❌ NOT VERIFIED (restart-check doesn't check) |
| Aegis restart → traffic unchanged | ❌ NOT VERIFIED (no traffic test) |
| Aegis restart → node re-registration | ❌ NOT VERIFIED (no before/after comparison) |

**Overall:** `restart-check` proves **SQLite state integrity** after restart. It does NOT prove data plane continuity or config file integrity. The true restart safety guarantee is architectural (process separation + file-based state), not test-verified.
