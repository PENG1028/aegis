# Aegis Logging Acceptance — v1.7U

## Overview

This document defines the logging acceptance criteria for Aegis. Every mutation operation must produce log entries with enough context to diagnose failures. Every auth-relevant event must produce audit log entries.

## Log Types

| Log Type | Table | Purpose |
|----------|-------|---------|
| Operation Log | `operation_logs` | Records every mutation attempt (success or failure) |
| Apply Log | `apply_logs` | Records each apply execution with per-step detail |
| Audit Log | `audit_logs` | Records security-relevant events (auth, access control) |
| Node Event | `node_events` | Records cluster lifecycle events |

---

## Required Log Fields

### Operation Log Fields

| Field | Type | Description | Required For |
|-------|------|-------------|-------------|
| `id` | string | Unique log entry ID | All |
| `action` | string | Action name (e.g., `bind-http-domain`) | All |
| `target_type` | string | Resource type (e.g., `route`, `service`) | All |
| `target_id` | string | Resource ID created/modified | All |
| `result` | string | `success` or `failed` | All |
| `message` | string | Human-readable description | All |
| `actor` | string | `cli`, `api`, or `system` | All |
| `created_at` | RFC3339 | Timestamp | All |

### Apply Log Fields

| Field | Type | Description | Required For |
|-------|------|-------------|-------------|
| `id` | string | Unique log entry ID | All applies |
| `operation_id` | string | Correlated operation ID | Linked applies |
| `state_version` | uint64 | State version at apply time | All applies |
| `config_hash_before` | string | SHA-256 before apply | All applies |
| `config_hash_after` | string | SHA-256 after apply | Successful applies |
| `provider` | string | Provider name (e.g., `caddy_http`) | Per-provider applies |
| `validate_status` | string | `success`, `failed`, `skipped` | All applies |
| `reload_status` | string | `success`, `failed`, `skipped` | All applies |
| `runtime_verify_status` | string | `success`, `failed`, `skipped` | All applies |
| `stderr` | string | Captured stderr from provider | Failed applies |
| `step_log` | JSON array | Step-by-step apply trace | All applies |
| `created_at` | RFC3339 | Timestamp | All applies |

### Audit Log Fields

| Field | Type | Description | Required For |
|-------|------|-------------|-------------|
| `id` | string | Unique log entry ID | All |
| `actor_type` | string | `admin`, `service_key`, `system` | All |
| `actor_id` | string | Admin user ID or token ID | All |
| `event_type` | string | Event classification | All |
| `ip` | string | Source IP | HTTP requests |
| `user_agent` | string | User-Agent header | HTTP requests |
| `target_type` | string | Resource type | Resource events |
| `target_id` | string | Resource ID | Resource events |
| `result` | string | `success`, `failed`, `blocked` | All |
| `error_code` | string | Structured error code | Failed events |
| `created_at` | RFC3339 | Timestamp | All |

### Node Event Fields

| Field | Type | Description | Required For |
|-------|------|-------------|-------------|
| `id` | string | Unique event ID | All |
| `node_id` | string | Target node ID | Node events |
| `event_type` | string | Event classification | All |
| `state_version` | uint64 | State version at event time | Versioned events |
| `severity` | string | `info`, `warning`, `error`, `critical` | All |
| `message` | string | Event description | All |
| `created_at` | RFC3339 | Timestamp | All |

---

## Scenario Verification

### Scenario 1: Bind Domain Success

**Trigger:** `POST /api/v1/actions/bind-http-domain` with valid API key and free domain.

**Operation Log expected:**
```json
{
  "action": "bind-http-domain",
  "target_type": "route",
  "target_id": "<route-id>",
  "result": "success",
  "message": "domain acceptance-test.example.com bound successfully",
  "actor": "api"
}
```

**Audit Log expected:**
```json
{
  "actor_type": "service_key",
  "actor_id": "<token-id>",
  "event_type": "domain_bound",
  "target_type": "domain",
  "target_id": "acceptance-test.example.com",
  "result": "success"
}
```

**Apply Log expected:** Triggered by action (if auto-apply) or manual apply.
```json
{
  "provider": "caddy_http",
  "validate_status": "success",
  "reload_status": "success",
  "runtime_verify_status": "success"
}
```

**CLI verify:**
```bash
curl -s http://localhost:9000/api/admin/v1/operations \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[] | select(.action == "bind-http-domain")'
```

---

### Scenario 2: Bind Domain Provider Failure

**Trigger:** `POST /api/v1/actions/bind-http-domain` when Caddy is missing or config invalid.

**Operation Log expected:**
```json
{
  "action": "bind-http-domain",
  "result": "failed",
  "message": "provider missing: caddy_http binary not found"
}
```

**Apply Log expected:**
```json
{
  "provider": "caddy_http",
  "validate_status": "failed",
  "stderr": "<error output from caddy validate>"
}
```

**CLI verify:**
```bash
curl -s http://localhost:9000/api/admin/v1/apply-logs \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[] | select(.validate_status == "failed")'
```

---

### Scenario 3: Scope Denied

**Trigger:** Service key from space-A tries to modify domain owned by space-B.

**Operation Log expected:**
```json
{
  "action": "update-target",
  "result": "failed",
  "message": "route rt_xxx does not belong to space spc_aaa"
}
```

**Audit Log expected:**
```json
{
  "actor_type": "service_key",
  "actor_id": "<token-id>",
  "event_type": "scope_denied",
  "result": "failed",
  "error_code": "SCOPE_DENIED"
}
```

---

### Scenario 4: Apply Locked

**Trigger:** Two concurrent apply requests.

**Operation Log expected:**
```json
{
  "action": "apply",
  "result": "failed",
  "message": "APPLY_LOCKED: another apply is in progress"
}
```

**CLI verify:**
```bash
curl -s http://localhost:9000/api/admin/v1/operations \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[] | select(.message | contains("APPLY_LOCKED"))'
```

---

### Scenario 5: Gateway Frozen

**Trigger:** `POST /api/admin/v1/gateway/domains`

**Audit Log expected:**
```json
{
  "actor_type": "admin",
  "event_type": "gateway_mutation_blocked",
  "result": "blocked",
  "error_code": "GATEWAY_MUTATION_FROZEN"
}
```

**HTTP Response:**
```
HTTP 405
{"error": "GATEWAY_MUTATION_FROZEN", "message": "Gateway mutations are frozen..."}
```

---

### Scenario 6: Target Unreachable

**Trigger:** `GET /api/admin/v1/trace/domain/<domain>` where target is down.

**Trace response:**
```json
{
  "final_target": {
    "host": "10.0.0.5",
    "port": 9999,
    "reachable": false,
    "error_code": "TARGET_CONNECTION_REFUSED",
    "connect_error": "dial tcp 10.0.0.5:9999: connect: connection refused"
  }
}
```

**Note:** Trace is read-only — no operation/audit log generated. Trace output itself is the diagnostic.

---

### Scenario 7: Admin Login Failed

**Trigger:** `POST /api/admin/v1/auth/login` with wrong password.

**Audit Log expected:**
```json
{
  "actor_type": "admin",
  "actor_id": "unknown",
  "event_type": "login_failed",
  "ip": "<request-ip>",
  "result": "failed",
  "error_code": "INVALID_CREDENTIALS"
}
```

**CLI verify:**
```bash
curl -s http://localhost:9000/api/admin/v1/audit-logs \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[] | select(.event_type == "login_failed")'
```

---

### Scenario 8: Revoked Key Access

**Trigger:** Use API key after `POST /api/admin/v1/api-keys/{id}/revoke`

**Audit Log (revocation event):**
```json
{
  "actor_type": "admin",
  "event_type": "token_revoked",
  "target_type": "api_key",
  "target_id": "<key-id>",
  "result": "success"
}
```

**Audit Log (attempted use):**
```json
{
  "actor_type": "service_key",
  "actor_id": "<token-id>",
  "event_type": "access_denied",
  "result": "failed",
  "error_code": "TOKEN_REVOKED"
}
```

---

## Log Field Coverage Matrix

| Scenario | operation_log | apply_log | audit_log | node_event |
|----------|:---:|:---:|:---:|:---:|
| 1. Bind domain success | ✓ | ✓ (on apply) | ✓ | - |
| 2. Bind domain provider failure | ✓ | ✓ | - | - |
| 3. Scope denied | ✓ | - | ✓ | - |
| 4. Apply locked | ✓ | - | - | - |
| 5. Gateway frozen | - | - | ✓ | - |
| 6. Target unreachable | - | - | - | - (trace handles) |
| 7. Admin login failed | - | - | ✓ | - |
| 8. Revoked key access | - | - | ✓ | - |
| Leader elected | - | - | - | ✓ |
| Drift detected | - | - | - | ✓ |
| Node stale | - | - | - | ✓ |

---

## Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | Every mutation produces operation_log with action/target/result | Scenarios 1-4 |
| 2 | Every apply produces apply_log with provider/steps/stderr | Scenarios 1-2 |
| 3 | Every auth event produces audit_log with actor/event/error_code | Scenarios 3, 5, 7, 8 |
| 4 | Every cluster event produces node_event with severity | Node events section |
| 5 | Failed operations include error_code in log | Scenarios 2-5, 7, 8 |
| 6 | Failed applies include stderr in log | Scenario 2 |
| 7 | Trace failures include target error_code (no log needed) | Scenario 6 |
| 8 | Log timestamps are RFC3339 | All scenarios |

---

## Log Query Reference

```bash
# Recent operation logs
GET /api/admin/v1/operations?limit=50

# Operation logs by action
GET /api/admin/v1/operations?action=bind-http-domain

# Recent apply logs
GET /api/admin/v1/apply-logs?limit=20

# Recent audit logs
GET /api/admin/v1/audit-logs?limit=50

# Audit logs by event type
GET /api/admin/v1/audit-logs?event_type=access_denied

# Node events for a specific node
GET /api/admin/v1/node-events?node_id=<node-id>
```
