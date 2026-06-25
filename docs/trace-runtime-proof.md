# Trace Runtime Proof — v1.7X

## TraceDomain: Complete Path with Connectivity

**File:** `internal/trace/service.go:40`

### Step Chain

| Order | Component | Source | Code Evidence |
|-------|-----------|--------|--------------|
| 1 | route_lookup | `RouteRepo.FindByDomain(domain)` — queries `routes` table | service.go:48 |
| 2 | listener | `ListenerSvc.ListAll()` — checks port 443 (TLS) or 80 (HTTP) | service.go:71-83, 118-130 |
| 3 | edge_mux (TLS) | `EdgeSvc.FindBySNIHost(ctx, domain)` — queries `edge_mux_rules` | service.go:93 |
| 4 | caddy | Static: TLS→127.0.0.1:8443, HTTP→port 80 | service.go:111-115, 139-142 |
| 5 | route_detail | Route fields from step 1 | service.go:146-150 |
| 6 | target | `EndpointRepo.FindEnabledByServiceID()` → `checkTargetConnectivity()` | service.go:156-175 |
| 7 | provider | `provider.DiagnoseHAProxy()` + `provider.DiagnoseCaddy()` | service.go:179-215 |

### Target Connectivity Check (v1.7W fix)

**Code:** `service.go:156-175`
```go
if s.deps.EndpointRepo != nil {
    endpoints, epErr := s.deps.EndpointRepo.FindEnabledByServiceID(rt.ServiceID)
    if epErr == nil && len(endpoints) > 0 {
        host, port := parseHostPort(endpoints[0].Address)
        finalTarget = &TargetInfo{Host: host, Port: port, Protocol: "http"}
        if rt.TLSEnabled { finalTarget.Protocol = "https" }
    }
}
// ...
if finalTarget != nil {
    s.checkTargetConnectivity(finalTarget)
    t.FinalTarget = finalTarget
    if finalTarget.Reachable != nil && !*finalTarget.Reachable {
        steps = append(steps, TraceStep{Status: "error", Detail: "target unreachable: ..."})
        t.Warnings = append(t.Warnings, "target is unreachable")
        t.TraceStatus = StatusIncomplete
    }
}
```

**Real connectivity check:** `service.go:353-399`
```go
func (s *Service) checkTargetConnectivity(target *TargetInfo) {
    ips, err := net.LookupHost(target.Host)  // DNS resolution
    conn, err := net.DialTimeout("tcp", addr, s.tcpTimeout)  // TCP connect, 2s timeout
    // Classifies: TARGET_DNS_FAILED | TARGET_TIMEOUT | TARGET_CONNECTION_REFUSED | TARGET_UNREACHABLE
}
```

### Provider Diagnostic Integration (v1.7W fix)

**Code:** `service.go:179-215`
```go
// HTTPS: HAProxy + Caddy diagnostics
haproxyDiag := provider.DiagnoseHAProxy()  // → ProviderDiagnostic struct
diagStep.ProviderDiagnostic = &haproxyDiag  // attached to trace step

caddyDiag := provider.DiagnoseCaddy()
caddyStep.ProviderDiagnostic = &caddyDiag

// HTTP: Caddy diagnostic only
caddyDiag := provider.DiagnoseCaddy()
caddyStep.ProviderDiagnostic = &caddyDiag
```

ProviderDiagnostic fields in trace output:
- `provider`, `installed`, `binary_path`, `version`, `version_supported`
- `config_path`, `config_exists`, `config_valid`
- `service_running`, `listener_ok`, `runtime_verify_ok`
- `last_error_code`, `last_error_message`, `stderr`

## TraceSNI: SNI Host Path

**File:** `internal/trace/service.go:196`

### Step Chain
| Order | Component | Source |
|-------|-----------|--------|
| 1 | listener | `ListenerSvc.ListAll()` — port 443 |
| 2 | edge_mux | `EdgeSvc.FindBySNIHost()` |
| 3 | caddy or direct | If 127.0.0.1:8443 → Caddy TLS termination; else → direct backend |
| 4 | route | `RouteRepo.FindByDomain()` (if Caddy path) |
| 5 | provider | `provider.DiagnoseHAProxy()` |

### Target Connectivity
Called in BOTH branches:
- Caddy path: endpoint lookup → `checkTargetConnectivity()`
- Direct backend: uses `edgeRule.TargetHost:TargetPort` → `checkTargetConnectivity()`

## TraceRoute: Route ID Path

**File:** `internal/trace/service.go:260`

### Step Chain
| Order | Component | Source |
|-------|-----------|--------|
| 1 | route_lookup | `RouteRepo.FindByID(routeID)` |
| 2 | target | Endpoint lookup → `checkTargetConnectivity()` |
| 3-N | (delegates) | `TraceDomain(ctx, rt.Domain)` |

## Failure Detection Matrix

| Scenario | TraceDomain | TraceSNI | TraceRoute | Evidence |
|----------|:---:|:---:|:---:|------|
| Route missing | StatusNotFound + error | N/A | StatusNotFound | steps[0].status="missing" |
| Edge rule missing | Warning + StatusIncomplete | StatusNotFound | Warning (if TLS) | steps[*].status="missing" |
| Target DNS failed | Warning + FinalTarget.ErrorCode=TARGET_DNS_FAILED | ✅ | ✅ | checkTargetConnectivity |
| Target connection refused | Warning + FinalTarget.ErrorCode=TARGET_CONNECTION_REFUSED | ✅ | ✅ | checkTargetConnectivity |
| Target timeout | Warning + FinalTarget.ErrorCode=TARGET_TIMEOUT | ✅ | ✅ | checkTargetConnectivity |
| Target unreachable | Warning + FinalTarget.ErrorCode=TARGET_UNREACHABLE | ✅ | ✅ | checkTargetConnectivity |
| HAProxy missing | Error step + warning | Error step | Via TraceDomain | DiagnoseHAProxy() |
| Caddy missing | Error step + warning | Via TraceDomain | Via TraceDomain | DiagnoseCaddy() |
| HAProxy config invalid | ProviderDiagnostic.config_valid=false | ✅ | Via TraceDomain | DiagnoseHAProxy() |
| Caddy config invalid | ProviderDiagnostic.config_valid=false | Via TraceDomain | Via TraceDomain | DiagnoseCaddy() |

## Test Coverage

### Real Tests (with net.Listen)

```go
// Start a real TCP listener on a random port
l, _ := net.Listen("tcp", "127.0.0.1:0")
defer l.Close()

// Test: target reachable
target := &TargetInfo{Host: "127.0.0.1", Port: l.Addr().(*net.TCPAddr).Port}
svc.checkTargetConnectivity(target)
assert *target.Reachable == true

// Close listener
l.Close()

// Test: target connection refused
target2 := &TargetInfo{Host: "127.0.0.1", Port: port}
svc.checkTargetConnectivity(target2)
assert *target2.Reachable == false
assert target2.ErrorCode == TARGET_CONNECTION_REFUSED
```

### Fake Tests (model/constant verification)
- TraceStep model fields
- ProviderDiagnostic attachment
- Status constant values
- Error code constant values

### REAL_ENV_REQUIRED Tests
- Caddy actually running and config valid → DiagnoseCaddy returns healthy
- HAProxy actually running → DiagnoseHAProxy returns healthy  
- Real domain trace with real Caddy/HAProxy routing
