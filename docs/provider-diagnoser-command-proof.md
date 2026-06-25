# Provider Diagnoser Command Proof — v1.7X

## Overview

Each diagnostic code is traced to its actual system command or check function.

---

## Caddy HTTP Provider

**Functions:** `CaddyHTTPProvider.Diagnose()` (`internal/provider/caddy_http.go:131`) and `DiagnoseCaddy()` (`internal/provider/diagnostics.go`)

| # | Error Code | Detection Method | Command / Function | stderr Preserved |
|---|-----------|-----------------|-------------------|:---:|
| 1 | PROVIDER_MISSING | PATH lookup | `exec.LookPath("caddy")` | N/A |
| 2 | VERSION_UNSUPPORTED | Version check + prefix match | `exec.Command("caddy", "version")` → `strings.HasPrefix(ver, "v2")` | ✅ `diag.Stderr = string(verOut)` |
| 3 | CONFIG_FILE_MISSING | File stat | `os.Stat(CaddyfilePath)` | N/A |
| 4 | CONFIG_VALIDATE_FAILED | Config validation | `exec.Command("caddy", "validate", "--config", caddyfilePath)` | ✅ `diag.Stderr = string(validOut)` |
| 5 | SERVICE_NOT_RUNNING | systemd check | `exec.Command("systemctl", "is-active", "--quiet", "caddy")` | ❌ Exit code only |
| 6 | LISTENER_CONFLICT | NONE | `diag.ListenerOK = true` (hardcoded) | N/A |
| 7 | RUNTIME_VERIFY_FAILED | HTTP smoke test | `exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "--connect-timeout", "3", "http://127.0.0.1:80")` | ❌ Exit code + HTTP status |

**Listener conflict status:** REAL_ENV_REQUIRED — requires root privileges for port scanning.

**Runtime verify:** Falls back to `rtOK = true` if `curl` not found in PATH.

---

## HAProxy EdgeMux Provider

**Functions:** `HAProxyEdgeMuxProvider.Diagnose()` (`internal/provider/haproxy_edge.go:185`) and `DiagnoseHAProxy()` (`internal/provider/diagnostics.go`)

| # | Error Code | Detection Method | Command / Function | stderr Preserved |
|---|-----------|-----------------|-------------------|:---:|
| 1 | PROVIDER_MISSING | PATH lookup | `exec.LookPath("haproxy")` | N/A |
| 2 | VERSION_UNSUPPORTED | Version check + parse | `exec.Command("haproxy", "-v")` → `parseHAProxyVersionParts()` | ✅ `diag.Stderr = string(verOut)` |
| 3 | CONFIG_FILE_MISSING | File stat | `os.Stat(configPath)` | N/A |
| 4 | CONFIG_VALIDATE_FAILED | Config validation | `exec.Command("haproxy", "-c", "-f", configPath)` | ✅ `diag.Stderr = string(validOut)` |
| 5 | SERVICE_NOT_RUNNING | systemd check | `exec.Command("systemctl", "is-active", "--quiet", "haproxy")` | ❌ Exit code only |
| 6 | LISTENER_CONFLICT | NONE | `diag.ListenerOK = true` (hardcoded) | N/A |
| 7 | RUNTIME_VERIFY_FAILED | NONE | `rtOK := true` (hardcoded) | N/A |

**LISTENER_CONFLICT:** REAL_ENV_REQUIRED — requires root/`CAP_NET_RAW` for port scanning.

**RUNTIME_VERIFY:** REAL_ENV_REQUIRED for HAProxy variants — no curl check, no TCP connect implemented. Always returns true.

---

## HAProxy TCP Provider

**Function:** `HAProxyTCPProvider.Diagnose()` (similar to EdgeMux)

Same detection methods as HAProxy EdgeMux (same binary, different config rendering).

---

## Command Summary

| Diagnostic Code | Caddy Command | HAProxy Command |
|----------------|--------------|----------------|
| PROVIDER_MISSING | `which caddy` | `which haproxy` |
| VERSION_UNSUPPORTED | `caddy version` | `haproxy -v` |
| CONFIG_FILE_MISSING | `stat $Caddyfile` | `stat /etc/haproxy/haproxy.cfg` |
| CONFIG_VALIDATE_FAILED | `caddy validate --config $Caddyfile` | `haproxy -c -f /etc/haproxy/haproxy.cfg` |
| SERVICE_NOT_RUNNING | `systemctl is-active --quiet caddy` | `systemctl is-active --quiet haproxy` |
| LISTENER_CONFLICT | REAL_ENV_REQUIRED | REAL_ENV_REQUIRED |
| RUNTIME_VERIFY_FAILED | `curl http://127.0.0.1:80` | REAL_ENV_REQUIRED |

---

## Test Coverage

### Verified by Unit Tests

```go
// Test 1: Fake provider covers all 7 error codes (fake_test.go)
fp := fake.NewFakeProvider("test", "http")
fp.MissingBinary = true
diag := fp.Diagnose()
assert diag.LastErrorCode == "PROVIDER_MISSING"
// ... (7 sub-tests for all codes)

// Test 2: Smoke failure matrix covers all codes (smoke_test.go)
svc.RunFailureMatrix(ctx) // 9 cases including all 7 diag codes

// Test 3: Version parsing (HAProxy)
major, minor := parseHAProxyVersionParts("HAProxy version 2.4.22")
assert major == 2 && minor == 4
```

### REAL_ENV_REQUIRED

| Test | Why |
|------|-----|
| `caddy validate` with real invalid Caddyfile | Needs Caddy installed |
| `haproxy -c` with real invalid config | Needs HAProxy installed |
| `systemctl is-active caddy` returns non-zero | Needs systemd + caddy service |
| `systemctl is-active haproxy` returns non-zero | Needs systemd + haproxy service |
| Port conflict detection | Needs root + actual listener |
| curl runtime verify | Needs Caddy running on :80 |

### Fake Coverage Only

All 7 diagnostic codes are tested via `fake.FakeProvider` — this tests the *interface contract* but not the *real system commands*. For real command verification, see `docs/real-vps-verification-plan.md`.

### stderr Preservation

| Provider | Version Check | Config Validate | Service Check | Runtime Verify |
|----------|:---:|:---:|:---:|:---:|
| Caddy | ✅ (verOut) | ✅ (validOut) | ❌ | ❌ |
| HAProxy | ✅ (verOut) | ✅ (validOut) | ❌ | ❌ |

**Gap:** Service check and runtime verify don't capture stderr. This is acceptable for the current phase — the primary failure modes (binary missing, config invalid) do capture stderr.
