# Provider Diagnostics Audit — v1.7R

## Executive Summary

Provider diagnostics is **partially implemented**. Caddy and HAProxy have status check functions with binary detection, version parsing, config validation, and service status. However, the `Diagnoser` interface is defined but **never implemented** by any provider, the diagnostic error codes are not consistently mapped, and there are gaps in the structured error output, listener conflict detection, and runtime verification.

---

## 1. Diagnostic Error Code Coverage

| Error Code | Defined? | Detectable? | Actually Detected? | Notes |
|-----------|----------|-------------|-------------------|-------|
| PROVIDER_MISSING | ✅ | ✅ | ✅ | `missing_binary` status |
| PROVIDER_VERSION_UNSUPPORTED | ✅ | ❌ | ❌ | No version gating implemented |
| CONFIG_FILE_MISSING | ✅ | ⚠️ | ❌ | Not checked separately — falls through to validate failure |
| CONFIG_VALIDATE_FAILED | ✅ | ✅ | ✅ | HAProxy: `-c -f`, Caddy: `validate --config` |
| SERVICE_NOT_RUNNING | ✅ | ✅ | ✅ | `systemctl is-active` |
| LISTENER_CONFLICT | ✅ | ❌ | ❌ | No port conflict detection in diagnostics |
| RUNTIME_VERIFY_FAILED | ✅ | ❌ | ❌ | No runtime smoke test in diagnostics |

**Coverage: 4/7 detectable, 3/7 actually implemented**

---

## 2. Current Diagnostic Functions

### CheckCaddyStatus (diagnostics.go:84-131)
- Binary lookup: ✅ `exec.LookPath("caddy")`
- Version: ✅ `caddy version`
- Config validate: ✅ `caddy validate --config <path>`
- Service running: ✅ `systemctl is-active caddy`
- Missing: listener conflict, runtime verify, version gating, structured stderr

### CheckHAProxyStatus (diagnostics.go:24-81)
- Binary lookup: ✅ `exec.LookPath("haproxy")`
- Version: ✅ `haproxy -vv` with version parsing
- SNI passthrough check: ✅ (version >= 1.8)
- Config validate: ✅ `haproxy -c -f <path>`
- Service running: ✅ `systemctl is-active haproxy`
- Missing: listener conflict, runtime verify, version gating, structured stderr

### FakeProvider (fake/provider.go)
- FailValidate: ✅
- FailReload: ✅
- FailBackup/FailRestore: ✅
- Missing: MissingBinary, InvalidConfig (separate from Validate), RuntimeVerifyFailure, ListenerConflict

---

## 3. Diagnoser Interface — Not Implemented

The `Diagnoser` interface is defined in `diagnostic.go`:
```go
type Diagnoser interface {
    Diagnose() ProviderDiagnostic
}
```

But **no provider implements it**. Neither `CaddyHTTPProvider` nor the HAProxy providers have a `Diagnose()` method. The diagnostic functions are standalone (`CheckCaddyStatus`, `CheckHAProxyStatus`) and return `ProviderStatus`, not `ProviderDiagnostic`.

This means:
- Admin API has no structured diagnostic endpoint for providers
- The `ProviderDiagnostic` struct with its `Stderr`, `LastErrorCode`, `LastErrorMessage` fields is unused
- There's no unified diagnostic query path

---

## 4. Structured Error Gap

Current error output example (from `CheckCaddyStatus`):
```go
status.Message = "caddy validate failed: " + string(validOut)
```

The stderr/stdout is concatenated into a single `Message` string. The `ProviderDiagnostic` struct has a separate `Stderr` field, but it's never populated because `Diagnose()` is never called.

Expected structured output:
```json
{
  "provider": "caddy_http",
  "installed": true,
  "binary_path": "/usr/bin/caddy",
  "version": "v2.7.6",
  "version_supported": true,
  "config_path": "/etc/caddy/Caddyfile",
  "config_exists": true,
  "config_valid": false,
  "service_running": true,
  "listener_ok": true,
  "last_error_code": "CONFIG_VALIDATE_FAILED",
  "last_error_message": "validate: validating config: unexpected token",
  "stderr": "Error: adapting config using caddyfile: parsing: unknown directive..."
}
```

---

## 5. Missing Diagnostic Checks

### LISTENER_CONFLICT
Neither provider checks for port conflicts. The listener system exists in `internal/listener/` but diagnostics doesn't cross-reference:
- Which ports are configured in Caddy/HAProxy config
- Whether those ports conflict with system listeners
- Whether the same port is claimed by both providers

### RUNTIME_VERIFY_FAILED
After apply+reload, there is no smoke test in the diagnostic path:
- No curl/wget check to verify the gateway responds
- No health endpoint probe
- No TLS certificate validity check
- The apply service has a step 10 ("Post-reload health check") but it's a comment — no actual check is performed

### PROVIDER_VERSION_UNSUPPORTED
No minimum version check:
- Caddy: v2.0.0+ required for API/config
- HAProxy: v1.8+ required for SNI passthrough (checked informally but no hard gate)

---

## 6. FakeProvider Diagnostic Coverage

Current `FakeProvider` failure modes:
```go
FailValidate bool   // → Validate returns CONFIG_VALIDATE_FAILED
FailReload   bool   // → Reload returns SERVICE_NOT_RUNNING
FailBackup   bool
FailRestore  bool
```

Missing failure modes:
```go
MissingBinary       bool   // → Info() returns "unavailable"
InvalidConfig       bool   // → Validate returns error with stderr
RuntimeVerifyFailed bool   // → new method RuntimeVerify() error
ListenerConflict    bool   // → new method CheckListeners() error
```

---

## 7. Admin API Diagnostic Endpoint

Current: `GET /api/diagnostics/export` — exists but implementation is a stub (`DiagnosticsExport` handler).

No admin endpoint for:
- `GET /api/admin/v1/providers/{name}/diagnostics` — structured provider diagnostic
- `GET /api/admin/v1/providers` — list all providers with basic status

---

## 8. Fix Plan

### Immediate
1. **Implement `Diagnose()` on CaddyHTTPProvider** — returns `ProviderDiagnostic` with all 7 error codes
2. **Implement `Diagnose()` on HAProxy providers** — same
3. **Add `GET /api/admin/v1/providers` handler** — lists all providers with Info()
4. **Add `POST /api/admin/v1/providers/{name}/diagnose` handler** — runs Diagnose() and returns structured result
5. **Enhance FakeProvider** with MissingBinary, RuntimeVerifyFailure, ListenerConflict injection

### Tests to add
1. Fake caddy missing binary → PROVIDER_MISSING
2. Fake haproxy missing binary → PROVIDER_MISSING
3. Fake config validate failed with stderr → CONFIG_VALIDATE_FAILED
4. Fake reload failed → SERVICE_NOT_RUNNING
5. Fake listener conflict → LISTENER_CONFLICT
6. Fake runtime verify failed → RUNTIME_VERIFY_FAILED

### Recommended
1. Add version gating (Caddy >= 2.0, HAProxy >= 1.8)
2. Add port conflict detection by cross-referencing listener table
3. Add runtime smoke test (curl localhost:8443 after apply)
4. Store last diagnostic result and expose via admin API
