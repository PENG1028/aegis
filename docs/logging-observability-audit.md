# Logging & Observability Audit — v1.7R

## Executive Summary

The logging system has the right tables and models but **gaps exist in what gets logged and when**. Most CRUD operations on service/route/endpoint handlers do not write operation or audit logs. The deployment model writes no logs at all. The key audit fields (scope_id, request_id, state_version, stderr) are not consistently populated.

---

## 1. Log Table Inventory

| Table | Purpose | Created? | Populated? | By Whom? |
|-------|---------|----------|-----------|----------|
| operation_logs | User/system operations | ✅ (v1.0) | ⚠️ Partial | Action API, Apply, Rollback |
| apply_logs | Step-level apply tracing | ✅ (v1.6B) | ⚠️ Partial | Apply service only |
| audit_logs | Security events | ✅ (v1.6B) | ⚠️ Minimal | Only admin auth (login/logout) |
| node_events | Cluster lifecycle events | ✅ (v1.6B) | ⚠️ Partial | Leader election, drift detection |
| upgrade_sessions | Upgrade lifecycle | ✅ (pre-v1.6) | ❓ | Unknown |

---

## 2. Traceability Field Audit

Each log table should capture enough context to trace a failure from end to end.

### operation_logs (OperationLog model)

| Field | Defined? | Populated? | Gap |
|-------|----------|-----------|-----|
| actor | ✅ | ⚠️ | "system"/"cli"/"api" — but not user/service key identity |
| action | ✅ | ✅ | |
| target_type | ✅ | ✅ | |
| target_id | ✅ | ✅ | |
| result | ✅ | ✅ | |
| message | ✅ | ✅ | |
| scope_id | ❌ | ❌ | **MISSING** — cannot filter by space |
| request_id | ❌ | ❌ | **MISSING** — cannot correlate API call to log |
| state_version | ❌ | ❌ | **MISSING** — cannot correlate to cluster state |
| error_code | ❌ | ❌ | **MISSING** — must parse from message string |
| stderr | ❌ | ❌ | **MISSING** — provider errors lost |

### apply_logs (ApplyLog model)

| Field | Defined? | Populated? | Gap |
|-------|----------|-----------|-----|
| operation_id | ✅ | ❓ | Not consistently linked |
| state_version | ✅ | ❌ | **Not populated** |
| config_hash_before | ✅ | ❌ | **Not populated** |
| config_hash_after | ✅ | ✅ | |
| provider | ✅ | ❌ | **Not populated** — always empty |
| validate_status | ✅ | ❌ | **Not populated** |
| reload_status | ✅ | ❌ | **Not populated** |
| runtime_verify_status | ✅ | ❌ | **Not populated** |
| stderr | ✅ | ❌ | **Not populated** |
| step_log | ✅ | ⚠️ | JSON helper exists but never used |

### audit_logs (AuditLog model)

| Field | Defined? | Populated? | Gap |
|-------|----------|-----------|-----|
| actor_type | ✅ | ✅ | |
| actor_id | ✅ | ✅ | |
| event_type | ✅ | ✅ | |
| ip | ✅ | ❓ | Not clear if set |
| user_agent | ✅ | ❓ | Not clear if set |
| target_type | ✅ | ✅ | |
| target_id | ✅ | ✅ | |
| result | ✅ | ✅ | |
| error_code | ✅ | ❌ | **Not populated** — always empty |
| scope_id | ❌ | ❌ | **MISSING** |

### node_events (NodeEvent model)

| Field | Defined? | Populated? | Gap |
|-------|----------|-----------|-----|
| node_id | ✅ | ✅ | |
| event_type | ✅ | ✅ | |
| state_version | ✅ | ❌ | **Not consistently populated** |
| severity | ✅ | ⚠️ | Used inconsistently |
| message | ✅ | ✅ | |

---

## 3. Operation Coverage Audit

| Operation | Writes op_log? | Writes apply_log? | Writes audit_log? | Writes node_event? |
|-----------|---------------|-------------------|-------------------|-------------------|
| BindHTTPDomain (action) | ✅ | ✅ (via apply) | ❌ | ❌ |
| BindTLSBackend (action) | ✅ | ✅ (via apply) | ❌ | ❌ |
| UpdateTarget (action) | ✅ | ✅ (via apply) | ❌ | ❌ |
| DisableDomain (action) | ✅ | ✅ (via apply) | ❌ | ❌ |
| DeleteDomain (action) | ✅ | ✅ (via apply) | ❌ | ❌ |
| AdminLogin | ❌ | N/A | ✅ | ❌ |
| AdminLogout | ❌ | N/A | ✅ | ❌ |
| AdminCreateSpace | ❌ | N/A | ❌ | ❌ |
| AdminCreateAPIKey | ❌ | N/A | ❌ | ❌ |
| AdminRevokeAPIKey | ❌ | N/A | ❌ | ❌ |
| AdminRotateAPIKey | ❌ | N/A | ❌ | ❌ |
| AdminSystemApply | ❌ | ✅ | ❌ | ❌ |
| CreateService | ❌ | ❌ | ❌ | ❌ |
| UpdateService | ❌ | ❌ | ❌ | ❌ |
| CreateRoute | ❌ | ❌ | ❌ | ❌ |
| UpdateRoute | ❌ | ❌ | ❌ | ❌ |
| CreateEndpoint | ❌ | ❌ | ❌ | ❌ |
| CreateManagedDomain | ❌ | ❌ | ❌ | ❌ |
| CreateExposure | ❌ | ❌ | ❌ | ❌ |
| Apply (manual) | ✅ | ✅ | ❌ | ❌ |
| Rollback | ✅ | ✅ | ❌ | ❌ |
| CreateDeployment | ❌ | ❌ | ❌ | ❌ |
| RollbackDeployment | ❌ | ❌ | ❌ | ❌ |
| Node register/heartbeat | ❌ | ❌ | ❌ | ✅ |
| Leader election | ❌ | ❌ | ❌ | ✅ |
| Drift detected | ❌ | ❌ | ❌ | ✅ |

**Coverage: ~25% of mutating operations write logs**

---

## 4. Specific Gaps

### Gap 1: CRUD operations don't log
Creating/updating/deleting services, routes, endpoints, managed domains, exposures — none of these write operation logs or audit logs. This means:
- No trace of who changed what
- No way to reconstruct state changes
- No scope tracking for changes

### Gap 2: Apply log fields are mostly empty
The `ApplyLog` model has rich fields (state_version, config_hash_before, provider, validate_status, reload_status, stderr) but the apply service writes minimal data. The step_log JSON helper exists but is never called.

### Gap 3: No provider stderr capture
When provider validate/reload fails, `stderr` is concatenated into the message string but not stored in the dedicated `stderr` field. This makes programmatic error analysis impossible.

### Gap 4: Deployment operations untracked
CreateDeployment and RollbackDeployment write no logs. This was acceptable when the model was pure tracking, but for audit purposes, even tracking-only mutations should be logged.

### Gap 5: No scope_id on logs
All logs are at the global level. It's impossible to filter logs by space/scope. For a multi-space system, this is a significant observability gap.

### Gap 6: No unauthorized access audit
Failed authentication/authorization attempts are not logged to audit_logs. Security events like scope violations, token expiration, and forbidden accesses should be auditable.

---

## 5. Fix Plan

### Immediate (v1.7R)

1. **Add audit logging for unauthorized access**: When bearer token auth fails (invalid token, wrong scope, space token on system route), write to audit_logs with:
   - actor_type: "service_key"
   - actor_id: token_id (when available)
   - event_type: "unauthorized_access"
   - error_code: the specific error (UNAUTHORIZED, FORBIDDEN, SCOPE_DENIED)
   - ip: from request

2. **Add operation logging to deployment service**: Write operation_log on CreateDeployment and RollbackDeployment.

3. **Add operation logging to admin key management**: Write operation_log on AdminCreateAPIKey, AdminRevokeAPIKey, AdminRotateAPIKey.

### Recommended (post v1.7R)

1. Add `scope_id` field to operation_logs and audit_logs
2. Add `request_id` field to operation_logs (from X-Request-ID header or generated)
3. Populate apply_log state_version, provider, validate_status, reload_status, stderr
4. Write step_log JSON during apply
5. Add log retention policy (keep last N entries or last M days)
6. Add `GET /api/admin/v1/logs/export` for log download

---

## 6. Current Actual Logging Paths

### Paths that DO log:
```
Action API → logs.Log() → operation_logs
                   → apply.TryApply() → logs.LogApply() → apply_logs
Admin Auth → service.Login()/Logout() → audit_logs
Node heartbeat → node_event (via reconcile loop)
Leader election → node_event
Drift detection → node_event
```

### Paths that DON'T log:
```
All service/route/endpoint/managed_domain/exposure CRUD
Gateway abstraction (frozen now)
Deployment CRUD
API key management
Failed auth attempts
Scope violations
```
