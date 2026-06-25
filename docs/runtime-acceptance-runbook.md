# Aegis Runtime Acceptance Runbook — v1.7U

## Overview

This runbook defines the **Golden Path E2E** verification for Aegis. It proves:
1. Complete forward path works (bootstrap → action → apply → traffic → logs)
2. Every step produces verifiable artifacts
3. Trace output matches real request path
4. All log types are populated

## Prerequisites

- Clean VPS or local VM with Go 1.22+
- Aegis binary built (`go build ./cmd/aegis/`)
- No pre-existing Aegis state (`rm -rf ~/.aegis`)
- HAProxy and Caddy installed (binaries present, services don't need to be running)
- `curl`, `openssl`, `jq` available

---

## Step 1: Bootstrap

```bash
./aegis bootstrap
```

**Expected output:**
```
=== Aegis Bootstrap ===

[config] ~/.aegis/config.yaml
[database] ~/.aegis/aegis.db (migrations applied)
[listeners] 3 registered
  haproxy_edge_mux 0.0.0.0:443 (tls_passthrough) → active
  caddy_http 0.0.0.0:80 (http) → active
  caddy_internal_https 127.0.0.1:8443 (https) → active

Bootstrap complete.
```

**Verify:**
```bash
ls -la ~/.aegis/
# EXPECTED: aegis.db, config.yaml exist
```

---

## Step 2: Doctor

```bash
./aegis doctor
```

**Expected output includes:**
```
[os]        OS/Arch/Hostname populated
[user]      UID shown
[binaries]  haproxy, caddy status (MISSING or version)
[providers] haproxy_edge_mux status, caddy_http status
[config paths] all paths show writable or (not configured)
[ports]     0.0.0.0:443, 0.0.0.0:80, 127.0.0.1:8443
[listeners] 3 listeners registered
[firewall]  hints
[acme]      port status
```

**Acceptance:** doctor runs without error exit code.

---

## Step 3: Start Aegis Server

```bash
./aegis serve --port 9000 &
AEGIS_PID=$!
sleep 2
```

**Verify:**
```bash
curl -s http://localhost:9000/api/health | jq .
# EXPECTED: JSON with health status
```

---

## Step 4: Admin Login

```bash
# First-time login creates default admin
LOGIN_RESP=$(curl -s -X POST http://localhost:9000/api/admin/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}')

echo $LOGIN_RESP | jq .

# Extract session cookie
ADMIN_COOKIE=$(echo $LOGIN_RESP | jq -r '.token')
```

**Expected:** Returns session token. HttpOnly cookie set.

**Acceptance:** `token` field present, status 200.

---

## Step 5: Create Space (Scope)

```bash
SPACE_RESP=$(curl -s -X POST http://localhost:9000/api/admin/v1/scopes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_COOKIE" \
  -d '{"name":"test-space","description":"acceptance test space"}')

echo $SPACE_RESP | jq .
SPACE_ID=$(echo $SPACE_RESP | jq -r '.id')
```

**Expected:** Returns space with `id` field.

---

## Step 6: Create Service API Key

```bash
KEY_RESP=$(curl -s -X POST "http://localhost:9000/api/admin/v1/scopes/${SPACE_ID}/api-keys" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_COOKIE" \
  -d '{"name":"test-key"}')

echo $KEY_RESP | jq .
API_KEY=$(echo $KEY_RESP | jq -r '.key')
```

**Expected:** Returns API key with `key` field (raw key value).

**Verify:**
```bash
curl -s http://localhost:9000/api/admin/v1/api-keys \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq .
# EXPECTED: key listed (token shown masked)
```

---

## Step 7: Bind HTTP Domain (via Service Action API)

```bash
BIND_RESP=$(curl -s -X POST http://localhost:9000/api/v1/actions/bind-http-domain \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"domain":"acceptance-test.example.com","target_host":"127.0.0.1","target_port":8080}')

echo $BIND_RESP | jq .
```

**Expected:**
```json
{
  "operation_id": "op_...",
  "status": "success",
  "message": "domain bound successfully"
}
```

**Verify resources created:**
```bash
# Check services
curl -s http://localhost:9000/api/admin/v1/services \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[] | select(.domain == "acceptance-test.example.com")'

# Check routes
curl -s http://localhost:9000/api/admin/v1/routes \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[] | select(.domain == "acceptance-test.example.com")'
```

**Acceptance:**
- `status: "success"`
- service created with correct domain
- route created pointing to target
- `operation_id` present

---

## Step 8: Verify Safe Apply

```bash
# Trigger apply
APPLY_RESP=$(curl -s -X POST http://localhost:9000/api/admin/v1/system/apply \
  -H "Authorization: Bearer $ADMIN_COOKIE")

echo $APPLY_RESP | jq .
```

**Expected:** apply succeeds, `pending_apply` becomes false.

```bash
# Check system status
curl -s http://localhost:9000/api/system/status | jq '.pending_apply'
# EXPECTED: false
```

**Acceptance:**
- Apply returns success
- `pending_apply` = false
- `state_version` incremented

---

## Step 9: Trace Domain Access Path

```bash
TRACE_RESP=$(curl -s "http://localhost:9000/api/admin/v1/trace/domain/acceptance-test.example.com" \
  -H "Authorization: Bearer $ADMIN_COOKIE")

echo $TRACE_RESP | jq .
```

**Expected trace structure:**
```json
{
  "input": "acceptance-test.example.com",
  "input_type": "domain",
  "trace_status": "complete",
  "steps": [
    {"order": 1, "component": "route", "name": "route_lookup", "status": "matched"},
    {"order": 2, "component": "listener", "name": "entry", "status": "matched"},
    ...
  ],
  "final_target": {
    "host": "127.0.0.1",
    "port": 8080,
    "protocol": "http",
    "reachable": true
  }
}
```

**CLI equivalent:**
```bash
./aegis trace domain acceptance-test.example.com
```

**Acceptance:**
- `trace_status` = "complete" or "incomplete" (complete if target is reachable)
- steps include: route → listener → edge_mux (if TLS) → caddy → target
- `final_target` matches the configured target

---

## Step 10: HTTP Request Verification

```bash
# Test via curl against port 80 (Caddy direct)
curl -v -H "Host: acceptance-test.example.com" http://127.0.0.1:80/ 2>&1

# If TLS configured, test via openssl against port 443
openssl s_client -servername acceptance-test.example.com -connect 127.0.0.1:443 </dev/null 2>&1 | head -20
```

**Acceptance:**
- HTTP request reaches target (or gets connection refused if target not running — that's expected, not a Caddy/HAProxy failure)
- openssl shows certificate negotiation (if TLS)
- 502/504 from proxy is OK if target is down — proves path works to proxy level

---

## Step 11: Log Verification

### Operation Logs

```bash
curl -s http://localhost:9000/api/admin/v1/operations \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[0]'
```

**Expected fields present:**
- `id`, `action`, `target_type`, `target_id`, `result`, `message`, `actor`, `created_at`

**Verify bind-http-domain logged:**
```bash
curl -s http://localhost:9000/api/admin/v1/operations \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[] | select(.action == "bind-http-domain")'
```

### Apply Logs

```bash
curl -s http://localhost:9000/api/admin/v1/apply-logs \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[0]'
```

**Expected fields present:**
- `id`, `operation_id`, `state_version`, `config_hash_before`, `config_hash_after`
- `provider`, `validate_status`, `reload_status`, `runtime_verify_status`
- `stderr`, `step_log`, `created_at`

### Audit Logs

```bash
curl -s http://localhost:9000/api/admin/v1/audit-logs \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[0]'
```

**Expected fields present:**
- `id`, `actor_type`, `actor_id`, `event_type`, `ip`, `user_agent`
- `target_type`, `target_id`, `result`, `error_code`, `created_at`

---

## Step 12: Provider Diagnostics

```bash
# List providers
curl -s http://localhost:9000/api/admin/v1/providers \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq .

# Full diagnose
curl -s -X POST http://localhost:9000/api/admin/v1/providers/diagnose \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq .
```

**Expected:**
- Provider list returns haproxy_edge_mux and caddy_http status
- Diagnose returns structured diagnostic with `healthy` flag and `issue_count`

---

## Step 13: Cleanup

```bash
# Stop Aegis
kill $AEGIS_PID

# Optionally remove state
rm -rf ~/.aegis
```

---

## Acceptance Criteria Summary

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | Bootstrap creates DB + listeners | Step 1 |
| 2 | Doctor runs without error | Step 2 |
| 3 | Admin can login | Step 4 |
| 4 | Admin can create space + API key | Steps 5-6 |
| 5 | Service key can bind HTTP domain | Step 7 |
| 6 | Safe apply succeeds | Step 8 |
| 7 | pending_apply = false after apply | Step 8 |
| 8 | Trace shows complete access path | Step 9 |
| 9 | Trace target matches configured target | Step 9 |
| 10 | HTTP request reaches proxy layer | Step 10 |
| 11 | Operation log has bind-http-domain record | Step 11 |
| 12 | Apply log has provider/step/stderr fields | Step 11 |
| 13 | Audit log has actor/event/result fields | Step 11 |
| 14 | Provider diagnostics return structured output | Step 12 |

---

## Troubleshooting

### "bind-http-domain" returns error
- Check provider diagnostic: `POST /api/admin/v1/providers/diagnose`
- Check logs: `GET /api/admin/v1/operations`
- Verify Caddy/HAProxy binaries exist: `which caddy haproxy`

### Trace shows "incomplete" status
- Check edge rules exist for domain: `GET /api/admin/v1/edge-rules`
- Check listener registration: `GET /api/admin/v1/gateway/listeners`
- Run doctor to verify provider health

### Apply fails
- Check apply logs for stderr: `GET /api/admin/v1/apply-logs`
- Verify config path permissions
- Check for listener conflicts: `POST /api/admin/v1/providers/diagnose`
