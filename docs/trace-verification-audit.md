# Trace Verification Audit — v1.7V

## Methodology

Every trace step is traced through `internal/trace/service.go` to verify it queries real tables, runs real checks, and produces structured output.

---

## TraceDomain — Full Code Trace

**File:** `internal/trace/service.go:40`
**Function:** `func (s *Service) TraceDomain(ctx context.Context, domain string) *AccessPathTrace`

### Step 1: Route Lookup
**Code (line 48):** `rt, err := s.deps.RouteRepo.FindByDomain(domain)`
- **Queries real table:** ✅ `routes` table via `RouteRepository.FindByDomain()`
- **Missing detection:** ✅ If `rt == nil` → `TraceStatus = "not_found"`, step.status = "missing"
- **Error detail:** `"no route found for domain '<domain>'"`

### Step 2: EdgeMux Listener Detection
**Code (lines 71-83):** `listeners, _ := s.deps.ListenerSvc.ListAll()`
- **Queries real table:** ✅ `listeners` table via `ListenerService.ListAll()`
- **TLS vs HTTP branching:** ✅ If `rt.TLSEnabled` → checks for port 443; else → checks for port 80
- **Missing detection:** ✅ If no matching port listener → step.status = "missing", warning added

### Step 3: EdgeMux SNI Match (TLS only)
**Code (line 93):** `edgeRule, edgeErr := s.deps.EdgeSvc.FindBySNIHost(ctx, domain)`
- **Queries real table:** ✅ `edge_mux_rules` table via `EdgeSvc.FindBySNIHost()`
- **Missing detection:** ✅ If `edgeRule == nil` → step.status = "missing", `TraceStatus = "incomplete"`, warning: "TLS domain has no matching edge_mux_rule"
- **Match detail:** ✅ Shows `edgeRule.ID`, `edgeRule.SNIHost`, `edgeRule.TargetHost:TargetPort`

### Step 4: Caddy Step
**Code (lines 111-115 for TLS, 139-142 for HTTP):**
- **TLS path:** Hardcoded `"Caddy handles TLS termination on 127.0.0.1:8443"` — assumes Caddy is at 8443
- **HTTP path:** Hardcoded `"Caddy handles HTTP directly on port 80"` — assumes Caddy is at 80
- **No real check:** ❌ Does NOT verify Caddy is actually listening on these ports
- **Missing detection:** ❌ Always shows "matched" — never detects if Caddy is down

### Step 5: Route Detail
**Code (lines 146-150):** Shows `rt.ID`, `rt.ServiceID`, `rt.Status`
- **Queries real table:** ✅ Data from Step 1 route object

### Step 6: Provider Diagnostics
**Code (lines 153-189):**
- Calls `provider.CheckHAProxyStatus()` — runs `exec.LookPath("haproxy")` + `haproxy -v`
- Calls `provider.CheckCaddyStatus()` — runs `exec.LookPath("caddy")` + `caddy version`
- **Real provider check:** ✅ Uses real system commands
- **Missing detection:** ✅ Shows "error" if provider missing or config invalid
- **Note:** Only runs for TLS routes; HTTP routes skip provider diagnostics

### Target Connectivity Check
**Code (lines 353-390):** `s.checkTargetConnectivity(target)`
- **DNS resolution:** ✅ `net.LookupHost(target.Host)` — detects DNS failure
- **TCP connect:** ✅ `net.DialTimeout("tcp", addr, 2s)` — 2-second timeout
- **Error classification:** ✅ 4 error types:
  - `TARGET_DNS_FAILED` — DNS resolution fails
  - `TARGET_TIMEOUT` — TCP connect times out
  - `TARGET_CONNECTION_REFUSED` — connection actively refused
  - `TARGET_UNREACHABLE` — other network errors
- **IMPORTANT:** `checkTargetConnectivity` is ONLY called from `TraceSNI()`, NOT from `TraceDomain()`. In `TraceDomain()`, `FinalTarget` is never set and connectivity is never checked.

**This is a BUG:** `TraceDomain()` (line 40) never calls `checkTargetConnectivity()` and never sets `FinalTarget`. Only `TraceSNI()` (lines 269, 279) does.

---

## TraceSNI — Full Code Trace

**File:** `internal/trace/service.go:199`
**Function:** `func (s *Service) TraceSNI(ctx context.Context, sniHost string) *AccessPathTrace`

### Step 1: Entry Listener
**Code (lines 207-226):** `listeners, _ := s.deps.ListenerSvc.ListAll()`
- **Queries real table:** ✅ `listeners` table
- **Missing detection:** ✅ If no port 443 listener → step = "missing"

### Step 2: EdgeMux SNI Match
**Code (line 229):** `edgeRule, err := s.deps.EdgeSvc.FindBySNIHost(ctx, sniHost)`
- **Queries real table:** ✅ `edge_mux_rules`
- **Missing detection:** ✅ Returns `StatusNotFound` if no match

### Step 3: Caddy vs Direct
**Code (lines 249-280):**
- If `edgeRule.TargetHost == "127.0.0.1" && edgeRule.TargetPort == 8443` → routes to Caddy, looks up matching route
- Else → direct TLS passthrough to backend
- **Target connectivity:** ✅ Calls `checkTargetConnectivity(target)` in BOTH branches

### Step 4: Provider Diagnostics
**Code (lines 283-295):** Calls `provider.CheckHAProxyStatus()`

---

## TraceRoute — Full Code Trace

**File:** `internal/trace/service.go:305`
**Function:** `func (s *Service) TraceRoute(ctx context.Context, routeID string) *AccessPathTrace`

### Step 1: Route Lookup
**Code (line 313):** `rt, err := s.deps.RouteRepo.FindByID(routeID)`
- **Queries real table:** ✅ `routes` table by ID
- **Missing detection:** ✅ Returns `StatusNotFound` if not found

### Step 2: Delegate to TraceDomain
**Code (line 332):** `domainTrace := s.TraceDomain(ctx, rt.Domain)`
- Delegates remainder of path to `TraceDomain()`
- Inherits the bug: target connectivity is NOT checked in `TraceDomain()`, so `TraceRoute()` also doesn't check it unless the route happens to be TLS.

### Steps 3: Edge Rule Verification
**Code (lines 338-347):** Independent edge rule check for TLS routes, adds warning if missing.

---

## Verification Matrix

| Capability | TraceDomain | TraceSNI | TraceRoute | Evidence |
|------------|:---:|:---:|:---:|------|
| Queries real routes table | ✅ | N/A | ✅ | `RouteRepo.FindByDomain()` / `FindByID()` |
| Queries real edge_mux_rules | ✅ (TLS) | ✅ | ✅ (TLS) | `EdgeSvc.FindBySNIHost()` |
| Queries real listeners | ✅ | ✅ | Via TraceDomain | `ListenerSvc.ListAll()` |
| Detects edge rule missing | ✅ | ✅ | ✅ | step.status = "missing" |
| Detects route missing | ✅ | N/A | ✅ | TraceStatus = "not_found" |
| Detects target unreachable | ❌ BUG | ✅ | ❌ BUG | Only TraceSNI calls checkTargetConnectivity |
| Checks actual config files | ❌ | ❌ | ❌ | Trace reads DB, not files |
| Calls provider diagnostics | ✅ (TLS) | ✅ | Via TraceDomain | `provider.CheckHAProxyStatus()` / `CheckCaddyStatus()` |
| Calls Diagnoser interface | ❌ | ❌ | ❌ | Uses `Check*Status()` helpers, NOT `Diagnose()` |
| Verifies Caddy is running | ❌ | ❌ | ❌ | Always shows "matched" |
| Verifies HAProxy is running | ❌ | ❌ | ❌ | Always shows "matched" |

---

## Key Findings

### FINDING-1: Target connectivity NOT checked in TraceDomain (BUG)
**File:** `internal/trace/service.go:40-196`
`TraceDomain()` constructs `FinalTarget` only in some code paths and never calls `checkTargetConnectivity()`. This means `trace domain <domain>` will NEVER report TARGET_UNREACHABLE/TIMEOUT/CONNECTION_REFUSED/DNS_FAILED. Only `trace sni <host>` does.

### FINDING-2: Trace uses CheckStatus helpers, NOT Diagnoser interface
The trace calls `provider.CheckHAProxyStatus()` and `provider.CheckCaddyStatus()` which only check binary existence and version. It does NOT call `Diagnose()` which would check config validation, service state, runtime verify, etc.

### FINDING-3: Caddy step is always "matched"
The Caddy step in TraceDomain is hardcoded to "matched" status. It never actually checks if Caddy is listening, running, or has valid config.

### FINDING-4: Trace is DB-only, not config-file-aware
Trace reads from SQLite tables (routes, edge_mux_rules, listeners). It never reads the actual HAProxy or Caddy config files to verify they match the DB state. This means trace cannot detect config drift between DB and rendered files.

### FINDING-5: Trace is genuinely read-only
Confirmed: TraceService has no DB write operations, no log writes, no state mutations. It only reads from RouteRepo, EdgeSvc, ListenerSvc, and runs provider status checks.

---

## Summary

- ✅ Trace queries REAL database tables (routes, edge_mux_rules, listeners)
- ✅ Trace detects missing routes and edge rules
- ✅ TraceSNI checks real target connectivity
- ❌ TraceDomain does NOT check target connectivity (BUG)
- ❌ Trace does NOT call Diagnoser.Diagnose() — uses lighter helpers
- ❌ Trace cannot detect Caddy/HAProxy config drift from DB state
- ❌ Caddy step is always "matched" — never actually verified
