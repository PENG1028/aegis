# Smoke Command Verification Audit — v1.7V

## Methodology

Each smoke command was traced through source code in `internal/smoke/service.go` and `internal/cli/smoke.go` to determine its actual behavior, data access pattern, and proof value.

---

## 1. `aegis smoke golden`

**Source:** `internal/smoke/service.go:RunGoldenPath()` (line ~52)

| Property | Value |
|----------|-------|
| **Read-only** | ✅ Yes — only reads state, never writes |
| **Creates temp resources** | ❌ No |
| **Triggers safe apply** | ❌ No |
| **Calls real provider** | ⚠️ Indirectly via `provider.CheckHAProxyStatus()`/`provider.CheckCaddyStatus()` which run `exec.LookPath` |
| **Uses fake provider** | ❌ No — uses real provider checks |
| **Needs Caddy/HAProxy** | ⚠️ Checks binary existence via `exec.LookPath`; reports WARN if missing but doesn't fail |
| **Needs real domain** | ❌ No |
| **Writes operation log** | ❌ No |
| **Writes apply log** | ❌ No |
| **Writes audit log** | ❌ No |

**What it checks (code evidence):**
1. `checkConfig()` — reads `Config.Store.SQLitePath`, checks non-empty
2. `checkDatabase()` — runs `DB.Ping()`, verifies DB accessible
3. `checkListeners()` — calls `ListenerSvc.ListAll()`, counts listeners
4. `checkProviders()` — calls `provider.CheckHAProxyStatus()` and `provider.CheckCaddyStatus()`
5. `checkStateVersion()` — calls `StateVer.Current()`, checks != 0
6. `checkPendingApply()` — calls `PendingSt.Status()`, verifies `pending=false`
7. `checkRoutesQueryable()` — calls `RouteSvc.ListRoutes()`, counts routes

**What it proves:**
- Aegis has a working DB connection
- Listeners are registered (bootstrap ran)
- Provider binaries exist in PATH
- State version is initialized
- No pending apply (clean state)
- Routes table is queryable

**What it DOES NOT prove:**
- ❌ Does NOT prove `bind-http-domain` mutation works end-to-end
- ❌ Does NOT prove apply pipeline works (no config rendered, validated, or reloaded)
- ❌ Does NOT prove traffic reaches any target
- ❌ Does NOT prove trace works
- ❌ Does NOT prove auth works
- ❌ Does NOT prove domain ownership enforcement works

**Verdict:** `smoke golden` is a **system health ping**, not a golden path E2E test. It proves "Aegis is alive" but not "Aegis can bind a domain and serve traffic."

---

## 2. `aegis smoke provider`

**Source:** `internal/smoke/service.go:RunProviderSmoke()` (line ~131)

| Property | Value |
|----------|-------|
| **Read-only** | ✅ Yes |
| **Creates temp resources** | ❌ No |
| **Triggers safe apply** | ❌ No |
| **Calls real provider** | ✅ Yes — `provider.CheckHAProxyStatus()` + `provider.CheckCaddyStatus()` |
| **Uses fake provider** | ❌ No |
| **Needs Caddy/HAProxy** | ⚠️ Checks binary existence, version |
| **Needs real domain** | ❌ No |
| **Writes operation log** | ❌ No |
| **Writes apply log** | ❌ No |
| **Writes audit log** | ❌ No |

**What it checks (code evidence):**
1. `provider.CheckHAProxyStatus()` — `exec.LookPath("haproxy")` + version check
2. `provider.CheckCaddyStatus()` — `exec.LookPath("caddy")` + version check
3. Optional config file existence via `os.Stat(configPath)`

**What it proves:**
- HAProxy binary exists and version can be read
- Caddy binary exists and version can be read

**What it DOES NOT prove:**
- ❌ Does NOT call `Diagnose()` on the actual provider structs
- ❌ Does NOT test config validation, reload, or runtime verify
- ❌ Does NOT test any of the 7 diagnostic error codes
- ❌ Uses `provider.Check*Status()` helper functions, not the `Diagnoser` interface methods

**Verdict:** `smoke provider` is a **binary availability check**, not a provider diagnostic. It's equivalent to `which haproxy && which caddy`. It does NOT exercise the Diagnoser interface.

---

## 3. `aegis smoke trace <domain>`

**Source:** `internal/smoke/service.go:RunTraceSmoke()` (line ~171)

| Property | Value |
|----------|-------|
| **Read-only** | ✅ Yes |
| **Creates temp resources** | ❌ No |
| **Triggers safe apply** | ❌ No |
| **Calls real provider** | ⚠️ Indirectly via `TraceSvc.TraceDomain()` which may call provider diag |
| **Uses fake provider** | ❌ No |
| **Needs Caddy/HAProxy** | ❌ No (trace reads from DB) |
| **Needs real domain** | ✅ Yes — must be a domain already bound in Aegis |
| **Writes operation log** | ❌ No |
| **Writes apply log** | ❌ No |
| **Writes audit log** | ❌ No |

**What it checks (code evidence):**
1. Calls `TraceSvc.TraceDomain(ctx, domain)` — the real trace service
2. Checks `trace_status` (complete/incomplete/not_found/error)
3. Checks step count
4. Checks `final_target.reachable` and `final_target.error_code`
5. Reports warnings and errors from trace output

**What it proves:**
- TraceService can resolve a domain through the access path
- Target connectivity is checked (if trace service does it)
- Missing routes/edge rules are detected as "missing" steps

**What it DOES NOT prove:**
- ❌ Does NOT verify that trace output matches actual `curl`/`openssl` behavior
- ❌ Does NOT verify that HAProxy/Caddy config actually routes the domain
- ❌ Just runs trace and reports what it returns

**Verdict:** `smoke trace` is a **trace service wrapper** — it calls the real TraceService and formats output. The quality depends entirely on TraceService implementation (audited separately in `trace-verification-audit.md`).

---

## 4. `aegis smoke failure-matrix`

**Source:** `internal/smoke/service.go:RunFailureMatrix()` (line ~212)

| Property | Value |
|----------|-------|
| **Read-only** | ✅ Yes (only interacts with FakeProvider in memory) |
| **Creates temp resources** | ❌ No |
| **Triggers safe apply** | ❌ No |
| **Calls real provider** | ❌ No |
| **Uses fake provider** | ✅ Yes — exclusively `fake.NewFakeProvider()` |
| **Needs Caddy/HAProxy** | ❌ No |
| **Needs real domain** | ❌ No |
| **Writes operation log** | ❌ No |
| **Writes apply log** | ❌ No |
| **Writes audit log** | ❌ No |

**What it checks (code evidence):**
1. Creates `fake.NewFakeProvider("test-provider", "http")` for each case
2. Injects failure via flags (e.g., `fp.MissingBinary = true`)
3. Calls `fp.Diagnose()` or `fp.Validate()` and verifies error code
4. Resets with `fp.ResetErrors()` between cases
5. 9 cases: 7 provider error codes + APPLY_LOCKED + GATEWAY_MUTATION_FROZEN

**What it proves:**
- FakeProvider correctly implements all 7 diagnostic error codes
- FakeProvider correctly implements Diagnoser interface
- Error codes are returned as expected from the fake provider

**What it DOES NOT prove:**
- ❌ Does NOT test any real provider
- ❌ Does NOT test CaddyHTTPProvider.Diagnose()
- ❌ Does NOT test HAProxyEdgeMuxProvider.Diagnose()
- ❌ Does NOT test HAProxyTCPProvider.Diagnose()
- ❌ APPLY_LOCKED is tested only by checking `Info().Status == "ready"` — not a real lock test
- ❌ GATEWAY_MUTATION_FROZEN is tested only by checking provider health — not by calling the handler

**Verdict:** `smoke failure-matrix` is a **FakeProvider interface compliance test**. It proves the fake provider works, not that the real providers work. This is FAKE_ONLY verification.

---

## 5. `aegis smoke restart-check`

**Source:** `internal/smoke/service.go:RunRestartCheck()` (line ~307)

| Property | Value |
|----------|-------|
| **Read-only** | ✅ Yes |
| **Creates temp resources** | ❌ No |
| **Triggers safe apply** | ❌ No |
| **Calls real provider** | ❌ No |
| **Uses fake provider** | ❌ No |
| **Needs Caddy/HAProxy** | ❌ No |
| **Needs real domain** | ❌ No |
| **Writes operation log** | ❌ No |
| **Writes apply log** | ❌ No |
| **Writes audit log** | ❌ No |

**What it checks (code evidence):**
1. `checkDatabase()` — `DB.Ping()`
2. `StateVer.Current()` — checks not reset to 0
3. `PendingSt.Status()` — checks not erroneously true
4. `ListenerSvc.ListAll()` — checks listeners preserved
5. Config file existence via `os.Stat()`

**What it proves:**
- After a restart, DB is still accessible
- State version is not reset to 0
- pending_apply is not erroneously set
- Listeners still exist in DB
- Config file still exists on disk

**What it DOES NOT prove:**
- ❌ Does NOT prove Aegis was actually restarted (it's just a state check)
- ❌ Does NOT prove HAProxy/Caddy continued forwarding during downtime
- ❌ Does NOT prove node re-registered (just checks DB state)
- ❌ Does NOT prove no duplicate resources were created
- ❌ Does NOT do a before/after comparison — it's a point-in-time check
- ❌ Cannot verify data plane behavior at all

**Verdict:** `smoke restart-check` is a **post-restart state integrity check**. It verifies that Aegis control plane state looks clean after restart. It does NOT prove restart safety — it cannot verify data plane continuity or process lifecycle. To truly prove restart safety, you need:
1. Real process stop/start (manual)
2. External traffic verification during downtime (curl/openssl)
3. Before/after state comparison

---

## Summary Matrix

| Command | Type | Real Provider | DB Write | Proves |
|---------|------|:---:|:---:|--------|
| `smoke golden` | Health ping | Partial (binary check) | No | System alive, DB ok, listeners exist |
| `smoke provider` | Binary check | Yes (LookPath only) | No | HAProxy/Caddy binaries in PATH |
| `smoke trace` | Trace wrapper | Via TraceSvc | No | Trace service returns structured output |
| `smoke failure-matrix` | Fake test | No (FakeProvider) | No | FakeProvider interface compliance |
| `smoke restart-check` | State check | No | No | Post-restart state looks clean |
