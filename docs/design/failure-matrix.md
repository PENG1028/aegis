# Aegis Failure Matrix — v1.7U

## Overview

This document catalogs every expected failure mode and verifies that each produces:
- Correct HTTP status code
- Specific `error_code`
- Operation log entry
- Apply log or audit log entry
- Trace or diagnose output that can locate the failure

## Test Methodology

All failure cases below can be tested using the **FakeProvider** (`internal/fake/provider.go`) which supports all 7 diagnostic error codes plus runtime failures. Use `aegis smoke failure-matrix --fake` for automated verification.

---

## 1. Provider Failures

### 1.1 Caddy Missing

| Field | Expected Value |
|-------|---------------|
| **Trigger** | FakeProvider: `MissingBinary = true` |
| **HTTP Status** | 503 (Service Unavailable) or provider info shows `unavailable` |
| **error_code** | `PROVIDER_MISSING` |
| **Operation Log** | action=diagnose, result=failed, message contains "binary not found" |
| **Apply Log** | N/A (apply not attempted) |
| **Audit Log** | N/A |
| **Trace/Diagnose** | `Diagnose()` returns `last_error_code: "PROVIDER_MISSING"`, `installed: false` |
| **CLI verify** | `aegis smoke provider` shows caddy as MISSING |

### 1.2 HAProxy Missing

| Field | Expected Value |
|-------|---------------|
| **Trigger** | FakeProvider: `MissingBinary = true` for HAProxy |
| **HTTP Status** | Provider info returns `unavailable` |
| **error_code** | `PROVIDER_MISSING` |
| **Operation Log** | action=diagnose, message contains "binary not found" |
| **Apply Log** | N/A |
| **Audit Log** | N/A |
| **Trace/Diagnose** | `Diagnose()` returns `PROVIDER_MISSING` |

### 1.3 Caddy Config Invalid

| Field | Expected Value |
|-------|---------------|
| **Trigger** | FakeProvider: `FailValidate = true` |
| **HTTP Status** | 422 (Unprocessable Entity) or apply returns error |
| **error_code** | `CONFIG_VALIDATE_FAILED` |
| **Operation Log** | action=apply, result=failed, message contains "validate failed" |
| **Apply Log** | validate_status=failed, stderr contains "syntax error" |
| **Audit Log** | N/A |
| **Trace/Diagnose** | `Diagnose()` returns `last_error_code: "CONFIG_VALIDATE_FAILED"`, `config_valid: false` |
| **CLI verify** | `aegis smoke failure-matrix --fake` reports config_invalid |

### 1.4 HAProxy Config Invalid

| Field | Expected Value |
|-------|---------------|
| **Trigger** | FakeProvider: `FailValidate = true` for HAProxy |
| **HTTP Status** | 422 |
| **error_code** | `CONFIG_VALIDATE_FAILED` |
| **Operation Log** | action=apply, result=failed |
| **Apply Log** | provider=haproxy_edge_mux, validate_status=failed |
| **Trace/Diagnose** | HAProxy diagnose shows `CONFIG_VALIDATE_FAILED` |

### 1.5 Reload Failed

| Field | Expected Value |
|-------|---------------|
| **Trigger** | FakeProvider: `FailReload = true` |
| **HTTP Status** | 500 (Internal Server Error) |
| **error_code** | `RELOAD_FAILED` or `SERVICE_NOT_RUNNING` |
| **Operation Log** | action=apply, result=failed, message contains "reload failed" |
| **Apply Log** | reload_status=failed, stderr contains "service not running" |
| **Audit Log** | N/A |
| **Trace/Diagnose** | `Diagnose()` returns `service_running: false` |

### 1.6 Service Not Running

| Field | Expected Value |
|-------|---------------|
| **Trigger** | FakeProvider: `Running = false` |
| **HTTP Status** | Provider info returns `degraded` or `unavailable` |
| **error_code** | `SERVICE_NOT_RUNNING` |
| **Operation Log** | action=diagnose, result=failed |
| **Apply Log** | N/A (apply blocked by pre-check) |
| **Audit Log** | N/A |
| **Trace/Diagnose** | `Diagnose()` returns `service_running: false`, `last_error_code: "SERVICE_NOT_RUNNING"` |

### 1.7 Listener Conflict

| Field | Expected Value |
|-------|---------------|
| **Trigger** | FakeProvider: `ListenerConflict = true` |
| **HTTP Status** | 409 (Conflict) |
| **error_code** | `LISTENER_CONFLICT` |
| **Operation Log** | action=apply or action=validate, result=failed |
| **Apply Log** | validate_status=failed, stderr contains "port already in use" |
| **Audit Log** | N/A |
| **Trace/Diagnose** | `Diagnose()` returns `listener_ok: false`, `last_error_code: "LISTENER_CONFLICT"` |

### 1.8 Runtime Verify Failed

| Field | Expected Value |
|-------|---------------|
| **Trigger** | FakeProvider: `RuntimeVerifyFailed = true` |
| **HTTP Status** | 500 |
| **error_code** | `RUNTIME_VERIFY_FAILED` |
| **Operation Log** | action=apply, result=failed, message contains "health check" |
| **Apply Log** | runtime_verify_status=failed, stderr contains "502" |
| **Audit Log** | N/A |
| **Trace/Diagnose** | `Diagnose()` returns `last_error_code: "RUNTIME_VERIFY_FAILED"` |

---

## 2. Target Failures

### 2.1 Target Unreachable

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Bind domain to non-existent IP:port (e.g., 10.255.255.1:9999) |
| **HTTP Status** | 200 (trace returns data) or trace status is `incomplete` |
| **error_code** | `TARGET_UNREACHABLE` (in trace result) |
| **Operation Log** | N/A (trace is read-only) |
| **Apply Log** | N/A |
| **Audit Log** | N/A |
| **Trace** | `final_target.reachable: false`, `final_target.error_code: "TARGET_UNREACHABLE"` |
| **CLI verify** | `aegis trace domain <domain>` shows UNREACHABLE with error |

### 2.2 Target Timeout

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Target IP that drops packets (firewall DROP, not REJECT) |
| **HTTP Status** | 200 (trace returns data) |
| **error_code** | `TARGET_TIMEOUT` |
| **Operation Log** | N/A |
| **Trace** | `final_target.reachable: false`, `final_target.error_code: "TARGET_TIMEOUT"` |
| **CLI verify** | `aegis smoke trace <domain>` shows TARGET_TIMEOUT |

### 2.3 Target Connection Refused

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Target IP:port with no service listening (e.g., 127.0.0.1:19999) |
| **HTTP Status** | 200 (trace returns data) |
| **error_code** | `TARGET_CONNECTION_REFUSED` |
| **Operation Log** | N/A |
| **Trace** | `final_target.reachable: false`, `final_target.error_code: "TARGET_CONNECTION_REFUSED"` |
| **CLI verify** | `aegis trace domain <domain>` shows TARGET_CONNECTION_REFUSED |

### 2.4 Target DNS Failed

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Target hostname that doesn't resolve (e.g., `nonexistent.invalid`) |
| **HTTP Status** | 200 (trace returns data) |
| **error_code** | `TARGET_DNS_FAILED` |
| **Operation Log** | N/A |
| **Trace** | `final_target.reachable: false`, `final_target.error_code: "TARGET_DNS_FAILED"` |
| **CLI verify** | `aegis trace domain <domain>` shows TARGET_DNS_FAILED |

---

## 3. Auth / Scope Failures

### 3.1 Service Key Access Admin API

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Use service API key to call `/api/admin/v1/*` endpoint |
| **HTTP Status** | 403 (Forbidden) |
| **error_code** | `SCOPE_DENIED` or `FORBIDDEN` |
| **Operation Log** | N/A (rejected at auth layer) |
| **Apply Log** | N/A |
| **Audit Log** | event_type=access_denied, actor_type=service_key, result=failed, error_code=SCOPE_DENIED |
| **CLI verify** | `aegis smoke failure-matrix --fake` reports auth denied |

### 3.2 Revoked Key Access

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Admin revokes API key, then use it |
| **HTTP Status** | 401 (Unauthorized) |
| **error_code** | `TOKEN_REVOKED` or `UNAUTHORIZED` |
| **Operation Log** | N/A |
| **Apply Log** | N/A |
| **Audit Log** | event_type=token_revoked (revocation) + event_type=access_denied (attempted use), error_code=TOKEN_REVOKED |

### 3.3 Scope Denied

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Space-scoped key tries to access resource from another space |
| **HTTP Status** | 403 |
| **error_code** | `SCOPE_DENIED` |
| **Operation Log** | action=bind-http-domain, result=failed, message contains "does not belong to space" |
| **Apply Log** | N/A |
| **Audit Log** | event_type=scope_denied, actor_type=service_key, error_code=SCOPE_DENIED |

### 3.4 Resource Not Owned

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Space key tries to modify another space's domain |
| **HTTP Status** | 403 |
| **error_code** | `RESOURCE_NOT_OWNED` |
| **Operation Log** | action=update-target, result=failed, message contains "does not belong to space" |
| **Audit Log** | error_code=RESOURCE_NOT_OWNED |

### 3.5 Duplicate Domain Ownership

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Second space tries to bind domain already owned by first space |
| **HTTP Status** | 409 (Conflict) |
| **error_code** | `DOMAIN_ALREADY_OWNED` |
| **Operation Log** | action=bind-http-domain, result=failed, message contains "already owned by another space" |
| **Audit Log** | error_code=DOMAIN_ALREADY_OWNED |

---

## 4. Apply Failures

### 4.1 Apply Locked

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Two concurrent apply requests (or `sync.Mutex.TryLock` returns false) |
| **HTTP Status** | 409 (Conflict) or 423 (Locked) |
| **error_code** | `APPLY_LOCKED` |
| **Operation Log** | action=apply, result=failed, message contains "another apply is in progress" |
| **Apply Log** | N/A (apply not started) |
| **Audit Log** | N/A |
| **CLI verify** | `aegis smoke failure-matrix --fake` reports apply_locked |

### 4.2 Apply Validate Failed

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Provider returns validation error (see 1.3, 1.4) |
| **HTTP Status** | 422 |
| **error_code** | `CONFIG_VALIDATE_FAILED` |
| **Operation Log** | action=apply, result=failed |
| **Apply Log** | validate_status=failed, stderr populated |

### 4.3 Apply Reload Failed

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Provider returns reload error, restore succeeds (see 1.5) |
| **HTTP Status** | 500 |
| **error_code** | `RELOAD_FAILED` |
| **Operation Log** | action=apply, result=failed, message contains "reload failed, restored old config" |
| **Apply Log** | reload_status=failed, stderr populated |
| **Audit Log** | N/A |

### 4.4 Apply Runtime Verify Failed

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Provider reloads but runtime check fails (see 1.8) |
| **HTTP Status** | 500 |
| **error_code** | `RUNTIME_VERIFY_FAILED` |
| **Operation Log** | action=apply, result=failed |
| **Apply Log** | runtime_verify_status=failed |

---

## 5. Gateway Failures

### 5.1 Gateway Mutation Frozen

| Field | Expected Value |
|-------|---------------|
| **Trigger** | `POST /api/admin/v1/gateway/domains` |
| **HTTP Status** | 405 (Method Not Allowed) |
| **error_code** | `GATEWAY_MUTATION_FROZEN` |
| **Operation Log** | N/A (rejected before service layer) |
| **Apply Log** | N/A |
| **Audit Log** | event_type=gateway_mutation_blocked, result=blocked |
| **Response Body** | `{"error": "GATEWAY_MUTATION_FROZEN", "message": "Gateway mutations are frozen. Use /api/v1/actions/* or resource-specific endpoints."}` |

### 5.2 Gateway Read-Only State Mismatch

| Field | Expected Value |
|-------|---------------|
| **Trigger** | Query gateway domains; compare with routes + managed_domains |
| **HTTP Status** | 200 (read succeeds) |
| **error_code** | N/A (read-only, no error) |
| **Operation Log** | N/A |
| **Audit Log** | N/A |
| **Verification** | `GET /api/admin/v1/gateway/domains` results match `GET /api/routes` + `GET /api/managed-domains` domain list |

---

## 6. Log Tracing Matrix

For each failure case, verify at least one of these log types is populated:

| Failure Case | Operation Log | Apply Log | Audit Log | Trace/Diagnose |
|---|---|---|---|---|
| Provider missing | ✓ (diagnose=failed) | - | - | ✓ (PROVIDER_MISSING) |
| Config invalid | ✓ (apply=failed) | ✓ (validate=failed) | - | ✓ (CONFIG_VALIDATE_FAILED) |
| Reload failed | ✓ (apply=failed) | ✓ (reload=failed) | - | ✓ (SERVICE_NOT_RUNNING) |
| Listener conflict | ✓ (validate=failed) | ✓ (validate=failed) | - | ✓ (LISTENER_CONFLICT) |
| Runtime verify failed | ✓ (apply=failed) | ✓ (verify=failed) | - | ✓ (RUNTIME_VERIFY_FAILED) |
| Target unreachable | - | - | - | ✓ (TARGET_UNREACHABLE) |
| Service key → admin API | - | - | ✓ (SCOPE_DENIED) | - |
| Revoked key | - | - | ✓ (TOKEN_REVOKED) | - |
| Scope denied | ✓ (action=failed) | - | ✓ (SCOPE_DENIED) | - |
| Resource not owned | ✓ (action=failed) | - | ✓ (RESOURCE_NOT_OWNED) | - |
| Domain already owned | ✓ (action=failed) | - | ✓ (DOMAIN_ALREADY_OWNED) | - |
| Apply locked | ✓ (apply=failed) | - | - | - |
| Gateway frozen | - | - | ✓ (GATEWAY_MUTATION_FROZEN) | - |

---

## 7. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | Every provider failure has error_code | Sections 1.1-1.8 |
| 2 | Every target failure has error_code | Sections 2.1-2.4 |
| 3 | Every auth failure has audit log | Sections 3.1-3.5 |
| 4 | Every apply failure has apply log | Sections 4.1-4.4 |
| 5 | Gateway mutations return 405 | Section 5.1 |
| 6 | All failures locatable via log or trace | Section 6 |
| 7 | `aegis smoke failure-matrix --fake` covers all provider cases | Smoke CLI |
| 8 | Service key cannot access admin routes | Section 3.1 |
