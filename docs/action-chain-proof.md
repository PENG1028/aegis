# BindHTTPDomain Complete Action Chain Proof — v1.7V

## Methodology

Every step is traced through actual source files and functions. No design descriptions — only code evidence.

---

## Phase 1: HTTP API Layer

### 1.1 Route Registration

**File:** `internal/httpapi/routes.go:113`
```go
mux.HandleFunc("POST /api/v1/actions/bind-http-domain", h.BindHTTPDomain)
```

**Auth:** Route is under `/api/v1/actions/` prefix. Goes through `AuthMiddleware` which checks bearer token. NOT under `/api/admin/v1/` — service keys CAN access it.

### 1.2 Handler Function

**File:** `internal/httpapi/handlers/actions.go:10`
**Function:** `func (h *Handlers) BindHTTPDomain(w http.ResponseWriter, r *http.Request)`

**What it does:**
1. Decodes JSON body into `action.BindHTTPDomainInput` (domain, target_host, target_port)
2. Validates: domain required, target_host required, target_port defaults to 80
3. Calls `h.Action.BindHTTPDomain(r.Context(), input)`
4. Maps `action.ActionError` codes to HTTP status codes:
   - `SCOPE_DENIED` → 403
   - `DOMAIN_ALREADY_OWNED` → 409
   - `APPLY_LOCKED` → 423
   - `RESOURCE_NOT_FOUND` → 404
5. Returns JSON result with `operation_id`, `status`, `message`, `details`

### 1.3 Token/Scope Resolution

**File:** `internal/token/middleware.go`
**Path:** `AuthMiddleware` extracts token from `Authorization: Bearer <token>` header, looks up token in DB, validates:
- Token not revoked
- Token not expired
- Populates `action.ActionContext` with `TokenID`, `SpaceID`, scopes

### 1.4 ActionContext Injection

**File:** `internal/action/context.go`
Token middleware injects `ActionContext` into `context.Context`. The ActionService extracts it via `GetActionContext(ctx)`.

---

## Phase 2: ActionService Layer

### 2.1 Entry Point

**File:** `internal/action/bind_http_domain.go:23`
**Function:** `func (s *ActionService) BindHTTPDomain(ctx context.Context, input BindHTTPDomainInput) (*ActionResult, error)`

**Execution order (code evidence):**

### Step 1: Space Permission Validation
**Line 27:** `ac, err := s.requireSpace(ctx)`
**File:** `internal/action/service.go:64`
```go
func (s *ActionService) requireSpace(ctx context.Context) (*ActionContext, error) {
    ac := GetActionContext(ctx)
    if ac == nil { return nil, ErrScopeDenied("no action context found") }
    if ac.IsAdmin() { return ac, nil }  // Admin bypass
    if ac.SpaceID == "" { return nil, ErrScopeDenied("space tokens must have a space_id") }
    return ac, nil
}
```
**Status:** ✅ REAL — `requireSpace` enforces space scope

### Step 2: Domain Ownership Check
**Line 33:** `ownerSpaceID, err := s.checkDomainOwnership(input.Domain)`
**File:** `internal/action/service.go:100`
```go
func (s *ActionService) checkDomainOwnership(domain string) (string, error) {
    routes, err := s.routeSvc.ListRoutes(context.Background())
    // iterates routes checking rt.Domain == domain && rt.SpaceID != ""
    edgeRule, err := s.edgeSvc.FindBySNIHost(context.Background(), domain)
    // checks edge rules by SNI
    return "", nil
}
```
**Line 37-38:** If `ownerSpaceID != "" && ownerSpaceID != ac.SpaceID` → returns `ErrDomainAlreadyOwned`

**Status:** ✅ REAL — domain ownership enforced

### Step 3: Target Validation
**Line 42-47:** Validates target_host not empty, target_port in range [1, 65535]

### Step 4: Service Creation
**Line 59-78:** Creates `service.Service` struct with:
```go
svc := &service.Service{
    ID:               id.New("svc"),
    Name:             fmt.Sprintf("http-%s", input.Domain),
    Kind:             "http", Env: "prod", Status: "active",
    SpaceID:          spaceID,
    OwnerType:        ownerType,   // "admin" or "space"
    OwnerID:          ownerID,
    CreatedByTokenID: tokenID,
}
```
**Line 76:** `createServiceDirect(ctx, s.serviceSvc, svc)`
**Line 157-161:** Helper calls `svcSvc.CreateServiceDirect(s)` — bypasses project validation

**Status:** ✅ REAL — service created with space ownership

### Step 5: Endpoint Creation
**Line 80-92:** Creates `endpoint.Endpoint` with target address:
```go
ep := &endpoint.Endpoint{
    ID:        id.New("ep"),
    ServiceID: svc.ID,
    Type:      "private",
    Address:   fmt.Sprintf("%s:%d", input.TargetHost, input.TargetPort),
    Enabled:   true,
}
```
**Line 90:** `s.endpointRepo.Create(ep)`

**Status:** ✅ REAL — endpoint created

### Step 6: Route Creation
**Line 94-113:** Creates `route.Route`:
```go
rt := &route.Route{
    ID:       id.New("rt"),
    Domain:   input.Domain,
    ServiceID: svc.ID,
    TLSEnabled: true,
    Status:   "active",
    SpaceID:  spaceID, OwnerType: ownerType, OwnerID: ownerID,
    CreatedByTokenID: tokenID,
}
```
**Line 111:** `createRouteDirect(ctx, s.routeSvc, rt)`

**Status:** ✅ REAL — route created with space ownership

### Step 7: Edge Rule Auto-Create
**Line 116:** `s.edgeSvc.EnsureRuleForHTTPRoute(ctx, rt.Domain, rt.ID)`
**Status:** ✅ REAL — edge rule ensured. Failure is non-fatal (warning logged).

### Step 8: Edge Rule Ownership
**Line 122-131:** Finds edge rule by SNI host, attempts to set ownership fields.
**Note:** Code reads edge rule but ownership fields assignment with `_ = edgeRule` on line 130 is a NO-OP:
```go
edgeRule.SpaceID = spaceID       // line 124
edgeRule.OwnerType = ownerType   // line 125
edgeRule.OwnerID = ownerID       // line 126
edgeRule.CreatedByTokenID = tokenID  // line 127
_ = edgeRule                     // line 130 — ownership NEVER PERSISTED to DB!
```
**Status:** ⚠️ PARTIAL — edge rule ownership fields set in-memory but NOT written to DB. Comment says "ownership is set via route lifecycle sync."

### Step 9: Safe Apply
**Line 134:** `s.safeApply(ctx)`

**File:** `internal/action/service.go:125`
```go
func (s *ActionService) safeApply(ctx context.Context) error {
    if s.applySvc == nil { return nil }
    _, err := s.applySvc.TryApply(ctx)
    if err != nil {
        if IsActionError(err, ErrCodeApplyLocked) { return err }
        return NewError(ErrCodeConfigValidateFailed, fmt.Sprintf("apply failed: %v", err))
    }
    return nil
}
```
**Status:** ✅ REAL — `TryApply` called with mutex lock

### Step 10: Logging
**Line 135-136 (failure):** `s.logSvc.Log(ctx, "action.bind-http-domain", "action", opID, "failed", ...)`
**Line 145-146 (success):** `s.logSvc.Log(ctx, "action.bind-http-domain", "action", opID, "success", ...)`

**Status:** ✅ REAL — operation_log written for both success and failure

---

## Phase 3: Apply Pipeline

### 3.1 Apply Lock

**File:** `internal/apply/service.go:91`
```go
func (s *AppService) TryApply(ctx context.Context) (*ApplyPlan, error) {
    if !s.mu.TryLock() { return nil, fmt.Errorf("APPLY_LOCKED: ...") }
    defer s.mu.Unlock()
    return s.Apply(ctx)
}
```
**Status:** ✅ REAL — `sync.Mutex.TryLock` prevents concurrent applies

### 3.2 Plan (Route Collection)
**File:** `internal/apply/service.go:116`
`plan, err := s.planner.Plan(s.cfg.Proxy.Email)` — collects all active routes

### 3.3 Render (Config Generation)
**File:** `internal/apply/service.go:123`
`rendered, err := s.adapter.Render(proxy.GatewayConfig{Routes: plan.Routes, ...})`
Calls real provider adapter (Caddy or HAProxy).

### 3.4 Write Temp File
**File:** `internal/apply/service.go:134`
`tempPath, err := s.executor.WriteTemp(rendered)`

### 3.5 Validate Config
**File:** `internal/apply/service.go:143`
`s.executor.ValidateAdapter(s.adapter, tempPath)`
Runs `caddy validate --config <path>` or `haproxy -c -f <path>`.

### 3.6 Backup Current Config
**File:** `internal/apply/service.go:150`
`backupPath, err := s.executor.Backup()`

### 3.7 Hash Compare (Idempotency)
**File:** `internal/apply/service.go:159-169`
```go
newHash := computeHash(string(rendered))
lastSuccess, _ := s.applyRepo.FindLastSuccess()
if lastSuccess != nil && computeHash(lastSuccess.RenderedConfig) == newHash {
    // skip reload — config unchanged
    return plan, nil
}
```
**Status:** ✅ REAL — hash comparison prevents unnecessary reloads

### 3.8 Atomic Replace
**File:** `internal/apply/service.go:172`
`s.executor.Replace(tempPath)` — replaces live config file

### 3.9 Reload Provider
**File:** `internal/apply/service.go:178`
`s.executor.ReloadAdapter(s.adapter)` — runs `systemctl reload caddy` or `systemctl reload haproxy`

**Failure recovery (lines 182-198):** On reload failure:
1. Restore backup config
2. Reload again (recovered config)
3. Log critical if restore also fails

### 3.10 Clear Pending Apply
**File:** `internal/apply/service.go:207-209`
```go
if s.pendingState != nil {
    s.pendingState.ClearPending()
}
```
**Status:** ✅ REAL — Clears `pending_apply` on successful apply

### 3.11 Record Apply Version
**File:** `internal/apply/service.go:212-229`
`ApplyVersion` recorded in `apply_versions` table with config hash, backup path, status.

### 3.12 Operation Log (Apply Success)
**File:** `internal/apply/service.go:203-204`
`s.logSvc.Log(ctx, "apply", "", "", "success", ...)` 

**Note:** Apply logs are written via `s.logSvc.Log()` (operation_logs table) but apply_logs table (step-level details) is NOT populated. The `applyStepLog` type exists (line 371-391) with `record()` and `toJSON()` methods but is NEVER instantiated or called in `Apply()`.

**Status:** ⚠️ PARTIAL — operation_log written, but apply_log (step-level) NOT populated. ApplyStep infrastructure exists but is unused.

---

## Phase 4: Log Points

| Log Type | Written? | Code Evidence |
|----------|:---:|------|
| `operation_log` (action success) | ✅ | `bind_http_domain.go:145-146` |
| `operation_log` (action failure) | ✅ | `bind_http_domain.go:135-136` |
| `operation_log` (apply success) | ✅ | `apply/service.go:203-204` |
| `operation_log` (apply failure) | ✅ | `apply/service.go:118,128,137,179,183-191` |
| `apply_log` (step-level) | ❌ MISSING | `applyStepLog` type exists but never instantiated |
| `audit_log` (bind domain) | ❌ MISSING | No `LogAudit()` call in `BindHTTPDomain` |
| `audit_log` (auth) | ✅ | `token/middleware.go:logAuditEvent()` |

---

## Summary: Verified Chain

```
POST /api/v1/actions/bind-http-domain
  │
  ├─ [✅] Handler: decode JSON, validate inputs
  ├─ [✅] Auth: Bearer token → ActionContext (space_id, token_id)
  ├─ [✅] requireSpace: enforce space scope or admin bypass
  ├─ [✅] checkDomainOwnership: scan routes + edge rules
  ├─ [✅] Create service (svc_xxx) with space ownership
  ├─ [✅] Create endpoint (ep_xxx) with target address
  ├─ [✅] Create route (rt_xxx) with space ownership
  ├─ [✅] EnsureRuleForHTTPRoute: auto-create edge rule
  ├─ [⚠️] Edge rule ownership: in-memory only, NOT persisted
  ├─ [✅] safeApply → TryApply (mutex.TryLock)
  │   ├─ [✅] Plan: collect all routes
  │   ├─ [✅] Render: provider adapter generates config
  │   ├─ [✅] WriteTemp: save rendered config
  │   ├─ [✅] Validate: provider validate command
  │   ├─ [✅] Backup: snapshot current config
  │   ├─ [✅] Hash compare: skip if unchanged
  │   ├─ [✅] Replace: atomic config swap
  │   ├─ [✅] Reload: provider graceful reload
  │   ├─ [✅] ClearPending: reset pending_apply flag
  │   └─ [✅] Record apply version
  ├─ [✅] operation_log: success/failure logged
  ├─ [❌] apply_log: step-level log NOT populated
  └─ [❌] audit_log: domain_bound audit event NOT logged
```

**Overall:** The BindHTTPDomain action chain is **REAL** — it creates services, endpoints, routes, edge rules, runs safe apply, and writes operation logs. Two items are PARTIAL (edge rule ownership not persisted, apply_log step-level unused).

---

## Issues Found

### ISSUE-1: Edge rule ownership NOT persisted
**File:** `internal/action/bind_http_domain.go:122-131`
**Severity:** Medium — edge rules created by service keys don't have ownership recorded
**Fix:** Call `edgeSvc.UpdateRule()` to persist ownership fields

### ISSUE-2: apply_log step-level entries NOT written
**File:** `internal/apply/service.go:371-391` (infrastructure exists but unused)
**Severity:** Low — operation_log covers success/failure; step-level detail would require wiring `applyStepLog` into the `Apply` method
**Fix:** Instantiate `applyStepLog` in `Apply()`, call `record()` at each step, write to apply_logs table
