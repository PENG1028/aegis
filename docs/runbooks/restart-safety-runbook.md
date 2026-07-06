# Aegis Restart Safety Runbook — v1.7U

## Overview

This runbook verifies that **Aegis process lifecycle does not affect the data plane**. HAProxy and Caddy continue forwarding traffic regardless of whether Aegis is running, starting, or stopping.

## Design Principle

> Aegis is a **control plane only**. It renders configs and tells HAProxy/Caddy to reload them. Once configs are applied, HAProxy and Caddy serve traffic independently. Aegis is not in the data path.

---

## Prerequisites

- Aegis instance with at least one domain bound and applied (per [runtime-acceptance-runbook.md](runtime-acceptance-runbook.md) Steps 1-8)
- HAProxy and Caddy running with Aegis-managed configs
- A test domain that resolves to the server (or test via `curl -H "Host: domain"`)

---

## Test Procedure

### Phase 1: Establish Baseline

#### 1.1 Verify traffic works
```bash
# Test via Caddy HTTP (port 80)
curl -s -o /dev/null -w "%{http_code}" -H "Host: acceptance-test.example.com" http://127.0.0.1:80/
# EXPECTED: HTTP status code (200 from target, or 502 if target down — either proves proxy works)

# Test via HAProxy EdgeMux (port 443) — if TLS configured
openssl s_client -servername acceptance-test.example.com -connect 127.0.0.1:443 </dev/null 2>&1 | grep "CONNECTED"
# EXPECTED: "CONNECTED(00000003)"
```

#### 1.2 Record control plane state
```bash
# Record state version
curl -s http://localhost:9000/api/system/status | jq '{state_version, pending_apply, leader}'

# Record node info
curl -s http://localhost:9000/api/admin/v1/nodes -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[0] | {node_id, hostname, is_current, state_version}'

# Record provider status
curl -s http://localhost:9000/api/admin/v1/providers -H "Authorization: Bearer $ADMIN_COOKIE" | jq .
```

Save these values for comparison after restart.

---

### Phase 2: Stop Aegis

#### 2.1 Graceful shutdown
```bash
kill -TERM $AEGIS_PID
# or if running as service:
# systemctl stop aegis

sleep 2
```

#### 2.2 Verify Aegis is stopped
```bash
curl -s http://localhost:9000/api/health || echo "Aegis stopped (expected)"
# EXPECTED: "Aegis stopped (expected)" or connection refused
```

---

### Phase 3: Verify Data Plane During Aegis Downtime

#### 3.1 HTTP traffic still works
```bash
# Caddy HTTP should still serve
curl -s -o /dev/null -w "%{http_code}" -H "Host: acceptance-test.example.com" http://127.0.0.1:80/
# EXPECTED: Same HTTP status as Phase 1

# HAProxy should still accept connections
echo "QUIT" | openssl s_client -servername acceptance-test.example.com -connect 127.0.0.1:443 2>&1 | grep -c "CONNECTED"
# EXPECTED: 1 (connected)
```

#### 3.2 Provider processes are still running
```bash
pgrep -a haproxy
pgrep -a caddy
# EXPECTED: Process IDs shown for both
```

#### 3.3 Config files are intact
```bash
cat /etc/haproxy/haproxy.cfg | head -5
cat /etc/caddy/Caddyfile | head -5
# EXPECTED: Config files exist and contain expected content
```

**Acceptance:**
- HTTP(S) traffic continues uninterrupted
- HAProxy and Caddy processes remain running
- Config files are not modified or removed

---

### Phase 4: Restart Aegis

#### 4.1 Start Aegis
```bash
./aegis serve --port 9000 &
AEGIS_PID=$!
sleep 2
```

#### 4.2 Verify Aegis is up
```bash
curl -s http://localhost:9000/api/health | jq .
# EXPECTED: JSON health response
```

---

### Phase 5: Verify Control Plane State Recovery

#### 5.1 Node re-registers correctly
```bash
curl -s http://localhost:9000/api/admin/v1/nodes -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[0]'
```

**Expected:**
- `is_current: true`
- `hostname` matches
- `last_seen` updated to current time
- Capabilities re-detected

#### 5.2 Leader state intact
```bash
curl -s http://localhost:9000/api/system/status | jq '{leader, state_version}'
```

**Expected:**
- `leader` present (should be current node if single-node)
- `state_version` >= previous value (not reset to 0)

#### 5.3 pending_apply state preserved
```bash
curl -s http://localhost:9000/api/system/status | jq '.pending_apply'
# EXPECTED: false (or same as before restart)
```

**IMPORTANT:** If `pending_apply` was `false` before restart, it MUST still be `false` after restart. Aegis must not erroneously set `pending_apply=true` on restart when config is clean.

#### 5.4 Provider diagnostics work
```bash
curl -s -X POST http://localhost:9000/api/admin/v1/providers/diagnose \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '{healthy, issue_count}'
```

**Expected:**
- `healthy: true` (if providers are running)
- `issue_count` matches pre-restart state

#### 5.5 Trace still works
```bash
curl -s "http://localhost:9000/api/admin/v1/trace/domain/acceptance-test.example.com" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '{trace_status, steps_count: (.steps | length)}'
```

**Expected:**
- `trace_status` matches pre-restart state
- Same number of steps as before restart

#### 5.6 Logs are queryable
```bash
# Operation logs still there
curl -s http://localhost:9000/api/admin/v1/operations \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq 'length'
# EXPECTED: > 0 (pre-restart logs preserved)

# Apply logs still there
curl -s http://localhost:9000/api/admin/v1/apply-logs \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq 'length'
# EXPECTED: > 0

# Audit logs still there
curl -s http://localhost:9000/api/admin/v1/audit-logs \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq 'length'
# EXPECTED: > 0
```

---

### Phase 6: Post-Restart Mutation Test

#### 6.1 Bind a new domain
```bash
curl -s -X POST http://localhost:9000/api/v1/actions/bind-http-domain \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"domain":"restart-test.example.com","target_host":"127.0.0.1","target_port":8081}' | jq .
# EXPECTED: status=success
```

#### 6.2 Apply succeeds
```bash
curl -s -X POST http://localhost:9000/api/admin/v1/system/apply \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq .
# EXPECTED: success
```

#### 6.3 No duplicate resources
```bash
# Count routes — should have increased by exactly 1 (the new domain)
curl -s http://localhost:9000/api/admin/v1/routes \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq 'length'
```

**Acceptance:**
- No duplicate routes, services, or edge rules created on restart
- Pre-existing resources are not re-created

---

### Phase 7: Config Integrity Check

#### 7.1 Config hash consistency
```bash
# Get current config
curl -s http://localhost:9000/api/config/current | jq -r '.config' > /tmp/config_after_restart.txt

# Compare with pre-restart (if saved)
diff /tmp/config_before_restart.txt /tmp/config_after_restart.txt
# EXPECTED: No diff (unless new domain added in Phase 6)
```

#### 7.2 Apply history intact
```bash
curl -s http://localhost:9000/api/apply/history | jq 'length'
# EXPECTED: >= 1 (pre-restart apply records preserved)
```

---

## Acceptance Criteria

| # | Criterion | Phase | Status |
|---|-----------|-------|--------|
| 1 | Traffic continues during Aegis downtime | 3 | |
| 2 | HAProxy/Caddy stay running | 3.2 | |
| 3 | Config files not modified | 3.3 | |
| 4 | Node re-registers after restart | 5.1 | |
| 5 | Leader state intact | 5.2 | |
| 6 | state_version not reset | 5.2 | |
| 7 | pending_apply preserved | 5.3 | |
| 8 | Provider diagnostics work | 5.4 | |
| 9 | Trace works after restart | 5.5 | |
| 10 | Logs preserved across restart | 5.6 | |
| 11 | New mutations work after restart | 6.1-6.2 | |
| 12 | No duplicate resources created | 6.3 | |
| 13 | Config hash consistent | 7.1 | |

---

## Known Limitations

1. **In-memory state loss**: Session cookies are invalidated on restart (HttpOnly sessions stored in DB survive, but in-memory caches reset). Admin must re-login.
2. **Apply lock cleared**: `sync.Mutex` is in-memory, so any held lock is released on restart. This is safe — no partial apply survives.
3. **Node capabilities re-detected**: Capabilities are re-detected on restart via `RegisterCurrent()`. If binaries were installed/removed while Aegis was down, capabilities will reflect the new state.
4. **No pending apply on restart**: Aegis does NOT re-trigger pending applies on restart. If `pending_apply=true` at shutdown and config differs from applied state, an admin must manually trigger apply.
