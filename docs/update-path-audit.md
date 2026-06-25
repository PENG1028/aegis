# Update Path Audit — v1.7R

## Executive Summary

Only the **Action API** (`/api/v1/actions/*`) and explicit **apply endpoints** go through the safe apply pipeline. Most service CRUD endpoints (routes, services, endpoints, managed domains, exposures) mutate desired state but do **NOT** trigger safe apply. The gateway abstraction endpoints mutate a separate state table that is completely invisible to the apply pipeline.

---

## 1. Safe Apply Pipeline Definition

The canonical safe apply flow (from `internal/apply/service.go`):

```
1. Log operation (operation_log)
2. Acquire apply lock (sync.Mutex TryLock)
3. Plan (read desired state from routes/managed_domains/exposures/services)
4. Render config (via provider adapter)
5. Write temp config file
6. Validate config (provider validate command)
7. Backup current config
8. Config hash comparison (skip reload if unchanged)
9. Atomic replace config file
10. Reload provider (graceful reload)
11. On failure: restore backup + reload backup
12. Record apply version (apply_versions table)
13. Log apply result (apply_log + operation_log)
```

---

## 2. Entry Point Audit

### 2.1 Action API (`POST /api/v1/actions/bind-http-domain`)

| Step | Status |
|------|--------|
| Changes desired state? | Yes — creates service, endpoint, route |
| Triggers safe apply? | **Yes** — calls `s.safeApply(ctx)` at step 9 |
| Writes operation log? | **Yes** — via `logSvc.Log()` |
| Writes apply log? | **Yes** — via apply service |
| Can bypass lock? | **No** — uses `TryApply` |
| Can bypass scope check? | **No** — `requireSpace()` validates |

**Verdict: ✅ COMPLIANT**

### 2.2 Action API (`POST /api/v1/actions/bind-tls-backend`)

Same pattern — creates edge rule, triggers safe apply. **Verdict: ✅ COMPLIANT**

### 2.3 Action API (`PATCH /api/v1/actions/update-target`)

Updates service/edge target, triggers safe apply. **Verdict: ✅ COMPLIANT**

### 2.4 Action API (`POST /api/v1/actions/disable-domain`)

Disables route, triggers safe apply. **Verdict: ✅ COMPLIANT**

### 2.5 Action API (`DELETE /api/v1/actions/domain`)

Deletes route + edge rule, triggers safe apply. **Verdict: ✅ COMPLIANT**

### 2.6 Admin System Apply (`POST /api/admin/v1/system/apply`)

| Step | Status |
|------|--------|
| Changes desired state? | No — reads current state |
| Triggers safe apply? | **Yes** — calls `h.Apply.TryApply(r.Context())` |
| Writes operation log? | **Yes** |
| Writes apply log? | **Yes** |
| Can bypass lock? | **No** |

**Verdict: ✅ COMPLIANT**

### 2.7 Service Apply (`POST /api/apply`)

| Step | Status |
|------|--------|
| Triggers safe apply? | **Yes** — calls apply service |
| Writes logs? | **Yes** |

**Verdict: ✅ COMPLIANT**

### 2.8 Service Rollback (`POST /api/rollback`)

| Step | Status |
|------|--------|
| Triggers safe apply? | Direct rollback path (restore backup + reload) |
| Writes operation log? | **Yes** |
| Writes apply log? | Records rollback as ApplyVersion |
| Config hash compare? | **No** — rollback skips this |
| Provider validate? | **Yes** — validates restored config |

**Verdict: ✅ COMPLIANT** (rollback has its own safe path)

### 2.9 Service CRUD: CreateRoute (`POST /api/routes`)

| Step | Status |
|------|--------|
| Changes desired state? | **Yes** — inserts into routes table |
| Triggers safe apply? | **No** — handler only creates route |
| Writes operation log? | **Maybe** — depends on handler implementation |
| Must manually call apply? | **Yes** |

**Verdict: ⚠️ PARTIAL — config change without automatic apply**

### 2.10 Service CRUD: UpdateRoute (`PATCH /api/routes/{id}`)

| Step | Status |
|------|--------|
| Changes desired state? | **Yes** |
| Triggers safe apply? | **No** |

**Verdict: ⚠️ PARTIAL**

### 2.11 Service CRUD: EnableRoute/DisableRoute

| Step | Status |
|------|--------|
| Changes desired state? | **Yes** |
| Triggers safe apply? | **No** |

**Verdict: ⚠️ PARTIAL**

### 2.12 Service CRUD: SwitchRouteService

| Step | Status |
|------|--------|
| Changes desired state? | **Yes** |
| Triggers safe apply? | **No** |

**Verdict: ⚠️ PARTIAL**

### 2.13 Service CRUD: MaintenanceOn/MaintenanceOff

| Step | Status |
|------|--------|
| Changes desired state? | **Yes** |
| Triggers safe apply? | **No** |

**Verdict: ⚠️ PARTIAL**

### 2.14 Service CRUD: CreateService/UpdateService/EnableService/DisableService

| Step | Status |
|------|--------|
| Changes desired state? | **Yes** (service status affects routing) |
| Triggers safe apply? | **No** |

**Verdict: ⚠️ PARTIAL**

### 2.15 Endpoint CRUD: Create/Update/Enable/Disable/Delete

| Step | Status |
|------|--------|
| Changes desired state? | **Yes** (endpoint changes affect routing) |
| Triggers safe apply? | **No** |

**Verdict: ⚠️ PARTIAL**

### 2.16 Managed Domain CRUD: Create/Enable/Disable/Delete

| Step | Status |
|------|--------|
| Changes desired state? | **Yes** |
| Triggers safe apply? | **No** |

**Verdict: ⚠️ PARTIAL**

### 2.17 Exposure CRUD: Create/Update/Activate/Disable

| Step | Status |
|------|--------|
| Changes desired state? | **Yes** (exposure controls provider routing) |
| Triggers safe apply? | **No** |

**Verdict: ⚠️ PARTIAL**

### 2.18 Gateway Abstraction: All Mutation Endpoints

| Step | Status |
|------|--------|
| Changes desired state? | **Yes** — but in gateway_* tables only |
| Affects real config? | **No** — gateway_* tables are invisible to apply planner |
| Triggers safe apply? | **No** |
| Writes operation log? | **No** |
| Writes apply log? | **No** |

**Verdict: 🔴 DANGEROUS — creates shadow state with zero effect on real config**

### 2.19 Deployment: RollbackDeployment

| Step | Status |
|------|--------|
| Changes desired state? | **No** — only marks status = rolled_back |
| Affects real config? | **No** |
| Triggers safe apply? | **No** |
| Writes operation log? | **No** |
| Writes apply log? | **No** |

**Verdict: ⚠️ PARTIAL — rollback is tracking-only, not dangerous, but missing logs**

### 2.20 CLI Commands (aegis route create, aegis service create, etc.)

Same as their HTTP API counterparts — mutate desired state but may not trigger apply. The CLI has `aegis apply` as a separate explicit command.

**Verdict: ⚠️ PARTIAL — consistent with API behavior, but apply is manual**

---

## 3. Summary Matrix

| Entry Point | Desired State Change | Safe Apply | Op Log | Apply Log | Bypass Lock | Bypass Scope |
|------------|---------------------|------------|--------|-----------|-------------|--------------|
| Action API (5 endpoints) | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| Admin System Apply | ❌ (read-only) | ✅ | ✅ | ✅ | ❌ | ❌ |
| POST /api/apply | ❌ (read-only) | ✅ | ✅ | ✅ | ❌ | ❌ |
| POST /api/rollback | ❌ (read-only) | ✅ (own path) | ✅ | ✅ | ❌ | ❌ |
| Service CRUD | ✅ | ❌ | ⚠️ | ❌ | N/A | N/A |
| Route CRUD | ✅ | ❌ | ⚠️ | ❌ | N/A | N/A |
| Endpoint CRUD | ✅ | ❌ | ⚠️ | ❌ | N/A | N/A |
| Managed Domain CRUD | ✅ | ❌ | ⚠️ | ❌ | N/A | N/A |
| Exposure CRUD | ✅ | ❌ | ⚠️ | ❌ | N/A | N/A |
| Gateway Mutation | ✅ (shadow) | ❌ | ❌ | ❌ | N/A | N/A |
| Deployment Rollback | ❌ | ❌ | ❌ | ❌ | N/A | N/A |
| CLI commands | ✅ | ❌ (manual) | ⚠️ | ❌ | N/A | N/A |

---

## 4. Design Decision: Separate Mutation from Apply

The current architecture has a deliberate split:
- **Mutation** (CRUD) changes the desired state in the database
- **Apply** renders desired state to provider config and reloads

This is intentional for the admin workflow — an admin may want to make several changes, then apply once. However, the Action API auto-applies because actions represent complete intent from external service API keys.

**This split is acceptable for admin operations** but must be clearly documented. The risk is that an admin makes a mutation and forgets to apply — the actual running config is stale.

---

## 5. Fix Plan

### Immediate
1. **Freeze gateway mutation endpoints** (already identified in gateway audit)
2. **Add operation logging to all mutation handlers** that currently lack it
3. **Add audit log on unauthorized access attempts** to admin endpoints

### Recommended
1. Add a "pending changes" indicator: track `state_version` vs `last_applied_version`
2. Return `X-Aegis-Pending-Apply: true` header on responses for routes that have unapplied changes
3. Notify admin on GET /api/admin/v1/system/overview if unapplied changes exist
