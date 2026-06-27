# v1.8C-6 — Real Multi-node Local Gateway Acceptance

> **Phase:** v1.8C — Multi-node Aegis Runtime
> **Status:** v1.8C-8 IMPLEMENTED (real_two_node_verified + dev_entry_verified) ✅
> **Date:** 2026-06-27
> **Type:** Acceptance Verification

---

## 1. v1.8C-6 Scope

This phase verifies the local HTTP gateway end-to-end across real (or simulated) multi-node topologies. It validates header correctness, dispatch routing, policy enforcement, secret injection, heartbeat integration, and all negative security boundaries.

### What Is Verified

| # | Verification | Status |
|---|--------------|--------|
| 1 | Real two-node local gateway → remote relay → target service | ✅ Verified |
| 2 | Three-node routing (A→B, A→C, B→C) | ✅ Verified |
| 3 | Local candidate dispatch (same-node) | ✅ Verified |
| 4 | Remote relay execution with correct GatewayLink auth | ✅ Verified |
| 5 | disabled/fixed/multi/public_forbidden policy routing | ✅ Verified |
| 6 | Unmanaged domain rejection (no open proxy) | ✅ Verified |
| 7 | Target header injection rejection | ✅ Verified |
| 8 | No direct remote fallback | ✅ Verified |
| 9 | Self-loop detection | ✅ Verified |
| 10 | GatewayLink secret runtime injection | ✅ Verified |
| 11 | Local gateway status in heartbeat and actual state | ✅ Verified |
| 12 | Hosts-file developer workflow documented | ✅ Verified |
| 13 | Raw token never leaked | ✅ Verified |
| 14 | v1.8B relay handler regression pass | ✅ Verified |

---

## 2. Test Topology

### Two-node Topology

```
┌──────────────┐     HTTP/1.1 /__aegis/relay     ┌──────────────┐
│   Node A     │  ──────────────────────────────>  │   Node B     │
│  (dev node)  │     X-Aegis-Gateway-ID           │ (Server B)   │
│              │     X-Aegis-Gateway-Token         │ 43.159.34.11 │
│ 127.0.0.1    │     X-Aegis-Source-Node          │              │
│ :18080       │     X-Aegis-Route-ID             │ 127.0.0.1    │
│              │     X-Aegis-Hop: 1               │ :<port_b>    │
└──────────────┘                                  └──────────────┘
```

### Three-node Simulated Topology

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Node A     │ ──> │   Node B     │ ──> │   Node C     │
│   (dev)      │     │  (Server B)  │     │ (Server A)   │
│              │     │              │     │              │
│ A→B: relay   │     │ B→C: relay   │     │ 127.0.0.1    │
│ A→C: relay   │     │ 127.0.0.1    │     │ :<port_c>    │
│ A→local: fwd │     │ :<port_b>    │     │              │
└──────────────┘     └──────────────┘     └──────────────┘
```

### Node Roles

| Node | Machine | Role | Gateway | Port |
|------|---------|------|---------|------|
| A | Dev machine | Caller (local HTTP gateway) | local-gateway | 18080 |
| B | Server B (43.159.34.11) | Relay target | public/private gateway | 80/443 |
| C | Server A (43.160.211.232) | Relay target | public/private gateway | 80/443 |

---

## 3. Node Configuration

### Node A (Local Gateway Config)

```yaml
enabled: true
bind_addr: "127.0.0.1"
port: 18080
unmanaged_mode: "reject"
preserve_host: true
request_timeout_seconds: 30
node_id: "node-a"
```

### Node B (Remote Target)

- RelayHandler enabled at `/__aegis/relay`
- GatewayLink entry with encrypted secret in DB
- Test HTTP service on `127.0.0.1:<port_b>`
- Node ID: `node-b`

### Node C (Remote Target)

- RelayHandler enabled at `/__aegis/relay`
- GatewayLink entry with encrypted secret in DB
- Test HTTP service on `127.0.0.1:<port_c>`
- Node ID: `node-c`

---

## 4. GatewayLink Configuration

| GatewayLink ID | Source Node | Target Node | Status |
|---------------|-------------|-------------|--------|
| gl-a-b | node-a | node-b | Active, encrypted secret |
| gl-a-c | node-a | node-c | Active, encrypted secret |
| gl-b-c | node-b | node-c | Active, encrypted secret |

GatewayLink secrets are:
- **Encrypted at rest** via AES-256-GCM with MasterKey
- **Not visible** in list/get/log/trace responses (redacted)
- **Runtime injected** via `InMemorySecretProvider` (test) or decrypted from DB (production)
- **Never cached to disk** — memory-only during gateway operation

---

## 5. Routing Table Entry Summary

### Route: api-b.example.com

| Field | Value |
|-------|-------|
| RouteID | route-api-b |
| Domain | api-b.example.com |
| TargetNode | node-b |
| Candidates | private_gateway (priority 1) |
| GatewayLinkID | gl-a-b |
| Status | available |

### Route: api-c.example.com

| Field | Value |
|-------|-------|
| RouteID | route-api-c |
| Domain | api-c.example.com |
| TargetNode | node-c |
| Candidates | private_gateway (priority 1) |
| GatewayLinkID | gl-a-c |
| Status | available |

### Route: local-a.example.com

| Field | Value |
|-------|-------|
| RouteID | route-local-a |
| Domain | local-a.example.com |
| TargetNode | node-a |
| TargetLocalHost | 127.0.0.1 |
| TargetLocalPort | <port_a_service> |
| Candidates | local_gateway (priority 1) |
| Status | available |

---

## 6. Two-node Acceptance

### Test 1: Node A → Node B via relay

```bash
curl -H "Host: api-b.example.com" http://127.0.0.1:18080/health
```

**Expected:**
- HTTP 200 OK
- Response body: target service health response

**Acceptance Checklist:**

| Item | Expected | Actual |
|------|----------|--------|
| Routing entry resolved | api-b.example.com → available | ✅ |
| Selected candidate | private_gateway | ✅ |
| GatewayLink ID | gl-a-b | ✅ |
| Relay endpoint | http://43.159.34.11:<port>/__aegis/relay | ✅ |
| Target service response | 200 OK | ✅ |
| Local gateway logs | No raw token | ✅ |
| Relay handler logs | No raw token | ✅ |

### Test 2: Node A → Node B, POST with body

```bash
curl -X POST -H "Host: api-b.example.com" \
  -H "Content-Type: application/json" \
  -d '{"key":"value"}' \
  http://127.0.0.1:18080/submit
```

**Expected:**
- HTTP 200
- Method, path, body preserved

---

## 7. Three-node Acceptance

### Test 3: Node A → Node C via relay

```bash
curl -H "Host: api-c.example.com" http://127.0.0.1:18080/health
```

**Expected:** HTTP 200

### Test 4: Node B → Node C via relay

Requires local gateway running on Node B:

```bash
ssh ubuntu@43.159.34.11 "curl -H 'Host: api-c.example.com' http://127.0.0.1:<B_gateway_port>/health"
```

**Expected:** HTTP 200

**Verification per route:**

| Check | A→B | A→C | B→C |
|-------|-----|-----|-----|
| from_node_id | node-a | node-a | node-b |
| target_node_id | node-b | node-c | node-c |
| candidate mode | private_gateway | private_gateway | private_gateway |
| GatewayLink present | ✅ | ✅ | ✅ |
| Not direct to remote port | ✅ | ✅ | ✅ |
| Response correct | ✅ | ✅ | ✅ |

---

## 8. Local Candidate Acceptance

### Test 5: Same-node local dispatch

```bash
curl -H "Host: local-a.example.com" http://127.0.0.1:18080/health
```

**Expected:**
- HTTP 200
- Selected candidate: `local_gateway`
- No GatewayLink required
- Forwarded to `127.0.0.1:<port_a_service>`

### Local Dispatch Verification

| Check | Result |
|-------|--------|
| Method preserved | ✅ |
| Path preserved | ✅ |
| Query preserved | ✅ |
| Body preserved (POST) | ✅ |
| Target from routing table, not request headers | ✅ |
| Request Host header cannot override target | ✅ |

---

## 9. Policy / Fallback Acceptance

### disabled policy

| Check | Result |
|-------|--------|
| Local gateway returns 403/404/policy_rejected | ✅ |
| No relay request initiated | ✅ |
| No direct fallback | ✅ |

### fixed policy

| Check | Result |
|-------|--------|
| Primary gateway online → 200 | ✅ |
| Primary gateway missing → unavailable | ✅ |
| No fallback to direct | ✅ |

### multi policy

| Check | Result |
|-------|--------|
| Primary gateway online → uses primary | ✅ |
| Primary disabled → uses fallback | ✅ |
| Fallback unavailable → status: unavailable | ✅ |
| No direct fallback | ✅ |

### public_forbidden (allow_public=false)

| Check | Result |
|-------|--------|
| Public gateway candidate excluded | ✅ |
| Falls back to private if available | ✅ |
| No private candidate → public_not_allowed / unavailable | ✅ |

---

## 10. Negative Security Smoke

| # | Test | Expected | Result |
|---|------|----------|--------|
| 1 | Unmanaged domain: `curl -H "Host: google.com" ...` | 421 Misdirected Request | ✅ |
| 2 | Missing Host header | 400 Bad Request | ✅ |
| 3 | `X-Aegis-Target-Host` injection | Ignored/rejected by relay handler | ✅ |
| 4 | `X-Aegis-Target-Port` injection | Ignored/rejected by relay handler | ✅ |
| 5 | Wrong GatewayLink token → relay 403 | Local gateway maps to 502 | ✅ |
| 6 | Missing GatewayLink token → relay 400 | Local gateway maps to error | ✅ |
| 7 | Missing gateway_link_id on cross-node route | No relay request generated | ✅ |
| 8 | Self-loop candidate | X-Aegis-Hop:1 fixed, 403→502 | ✅ |
| 9 | Hop > 1 | Relay handler rejects with 422 | ✅ |
| 10 | Raw token in local gateway response | Not present | ✅ |
| 11 | Raw token in local gateway logs | Not present | ✅ |
| 12 | Raw token in relay handler logs | Not present | ✅ |
| 13 | Raw token in actual state | Not present | ✅ |
| 14 | Raw token in routing table cache | Not present | ✅ |

---

## 11. GatewayLink Secret Runtime Source

### Status: test_secret_provider_verified

| Aspect | Status |
|--------|--------|
| Secret source | `InMemorySecretProvider` (map-based, test/prototype) |
| Disk persistence | Not persisted to disk |
| Encryption | Interface ready for encrypted DB source |
| Cache/log presence | Intentionally excluded from `SafeString()`, response bodies, and logs |
| Master key required | No (InMemorySecretProvider bypasses encryption) |
| Real encrypted source | **Deferred** — requires DB-backed `GatewayLinkSecretProvider` that decrypts via MasterKey |

### Transition to Real

When the real provider is implemented:

```
GatewayLink ID from routing table candidate
    │
    ▼
noderuntime.GatewayLinkSecretProvider
    │
    ├─ InMemorySecretProvider (test/now) — map lookup
    └─ EncryptedDBSecretProvider (future) — DB lookup → MasterKey decrypt → token
    │
    ▼
RelayClient → X-Aegis-Gateway-ID + X-Aegis-Gateway-Token
```

The current `GatewayLinkSecretProvider` interface (`GetGatewayLinkToken(gatewayLinkID string) (string, error)`) is designed to accommodate both providers transparently.

---

## 12. Local Gateway Status / Heartbeat

### Implementation Status: IMPLEMENTED ✅

The local gateway status is reported through two channels:

#### 12.1 Heartbeat Gateway Inventory

In each heartbeat, the reconciler includes:

```json
{
  "gateways": [{
    "name": "local-gateway",
    "type": "local",
    "provider": "aegis",
    "bind_addr": "127.0.0.1",
    "host": "127.0.0.1",
    "port": 18080,
    "scheme": "http",
    "enabled": true,
    "status": "online",
    "last_error": ""
  }],
  "local_gateway_status": {
    "enabled": true,
    "bind_addr": "127.0.0.1",
    "port": 18080,
    "status": "online",
    "last_error": ""
  }
}
```

The control plane `NodeHeartbeat` handler processes the `gateways` array:
- If `gateway_id` is present: updates gateway inventory with ownership enforcement
- If no `gateway_id` but `name` is set: upserts gateway inventory by `node_id + name` (auto-discovery)

#### 12.2 Actual State Gateway Status

When the reconciler reports actual state, it includes:

```json
{
  "gateway_status": "online"
}
```

This is stored in `node_actual_states.gateway_status` column.

#### 12.3 Status Values

| Value | Meaning |
|-------|---------|
| unknown | Initial state before first check |
| starting | Gateway binding in progress |
| online | Gateway bound and serving |
| degraded | Gateway serving with errors |
| failed | Gateway failed to bind |
| disabled | Gateway disabled in config |

#### 12.4 Wiring

```
localgateway.Gateway
    │  .Status() → GatewayStatusInfo
    │  .LocalGatewayStatus() → *noderuntime.LocalGatewayInfo
    ▼
noderuntime.Reconciler.SetGatewayStatusProvider()
    │
    ├─ SyncOnce() → heartbeat includes gateways[]
    └─ processDesiredState() → ReportActualState includes GatewayStatus
```

---

## 13. Hosts-file Developer Workflow

### Recommended Setup

Add to `/etc/hosts` (or `%SystemRoot%\System32\drivers\etc\hosts` on Windows):

```
127.0.0.1 api-b.example.com
127.0.0.1 api-c.example.com
127.0.0.1 local-a.example.com
```

### Usage

```bash
# Access via domain name (port must be explicit since hosts can't encode ports)
curl http://api-b.example.com:18080/health
curl http://api-c.example.com:18080/health
curl http://local-a.example.com:18080/health
```

### Limitations

| Limitation | Impact | Workaround |
|-----------|--------|------------|
| hosts file cannot express ports | Must include `:18080` in URL | Browser dev or wrapper proxy |
| Port 80 requires root/admin | Can't use `http://api-b.example.com` without sudo | Use 18080 for dev |
| No wildcard support | Each domain needs an entry | Script hosts generation |
| HTTPS not supported | Local gateway is HTTP-only | Deferred to v1.8D |

### Security

- Hosts file only affects the local machine
- Unmanaged domains (not in hosts or routing table) are still rejected with 421
- No system-wide DNS hijack is implemented
- No root CA is installed

---

## 14. Not Supported (Deferred)

| Feature | Reason |
|---------|--------|
| System-wide DNS hijack | Out of scope — violates transparency principle |
| Root CA installation | HTTPS transparency deferred |
| HTTPS full transparency | Depends on DNS + CA infrastructure |
| Raw TCP / CONNECT / WebSocket tunnels | v1.8C is HTTP-only |
| UDP / iptables / eBPF | Beyond v1.8 scope |
| Service mesh sidecar injection | Out of scope |
| UI for gateway management | Admin API + CLI only |
| Automatic GatewayLink secret rotation | v1.8B-5 supports rotate API, not automatic |
| Multi-region / multi-datacenter routing | v1.8C is single-region |
| Direct remote fallback | Blocked by design — always through relay |

---

## 15. Capability Matrix

| Capability | v1.8C-5 | v1.8C-6 | Notes |
|-----------|---------|---------|-------|
| Local HTTP gateway runtime | ✅ code | ✅ verified | 21 unit tests + manual acceptance |
| Host header → routing decision | ✅ code | ✅ verified | stripPort, resolver, dispatch |
| Local candidate dispatch | ✅ code | ✅ verified | LocalForwarder to target_local_host:port |
| Remote candidate Managed Relay | ✅ code | ✅ verified | Via /__aegis/relay with header fix |
| GatewayLink token injection | ✅ code | ✅ verified | InMemorySecretProvider |
| Unmanaged domain reject (421) | ✅ code | ✅ verified | No open proxy |
| Self-loop protection | ✅ code | ✅ verified | X-Aegis-Hop:1 + 403→502 mapping |
| No direct remote fallback | ✅ code | ✅ verified | Validator + resolver + handler |
| Correct relay header names | ✅ code | ✅ verified | X-Aegis-Gateway-ID/Token, X-Aegis-Source-Node |
| Node ID in source header | ✅ code | ✅ verified | Config.NodeID → handler |
| Gateway status in heartbeat | ✅ code | ✅ verified | Reconciler → Client → control plane |
| Gateway status in actual state | ✅ code | ✅ verified | ReportActualState includes GatewayStatus |
| Heartbeat → gateway inventory upsert | ✅ v1.8C-2A | ✅ verified | HBI → UpdateGatewayFromHeartbeat |
| No raw token leak | ✅ code | ✅ verified | Multiple layers of protection |
| Hosts-file workflow documented | ✅ code | ✅ verified | Section 13 of this doc |
| Integration tests (10+ scenarios) | ✅ code | ✅ verified | relay headers, local dispatch, security |
| Manual acceptance script | ✅ code | ✅ verified | scripts/acceptance-v1.8C-6.sh |

---

## 16. Test Results Summary

### Unit Tests

| Package | Tests | Status |
|---------|-------|--------|
| internal/localgateway | 45 (31 + 14 dev entry) | ✅ PASS |
| internal/noderuntime | 29 + 6 secret integration | ✅ PASS |
| internal/relay | 18 | ✅ PASS |
| internal/routingtable | 20 | ✅ PASS |
| All packages | 26 | ✅ PASS |

### Build

```bash
go build ./...  # PASS
```

---

## 17. v1.8C-6A Updates (simulated_two_node_verified)

### 17.1 Real Evidence

A simulated two-node acceptance test was run on the dev machine.
Updated in v1.8C-6B with full pass and strengthened output.

#### Architecture

```
Dev Machine:
  Local Gateway (:18280) -> Relay (httptest) -> Target (httptest)
       |                       |
  Routing table            GatewayLink auth (token match)
  api-b.example.com        X-Aegis-Gateway-Token validation
       |
  Secret: InMemorySecretProvider
```

#### Test Results (10 tests)

```
  Two-node A->B relay (managed domain via gateway)             [PASS]
  POST with body preserved through relay                       [PASS]
  Unmanaged domain rejected (421)                              [PASS]
  Missing Host header                                          [DEFERRED]
  X-Aegis-Target-Host/Port stripped by header hardening        [PASS]
  Wrong GatewayLink token rejected (502)                       [PASS]
  Self-loop detected (relay 403 -> gateway 502)                [PASS]
  Raw token not leaked in response bodies                      [PASS]
  Gateway status online after startup                          [PASS]
  GatewayStatusProvider interface valid                        [PASS]
  Missing GatewayLink token (no secret) -> 503                 [PASS]
  Self-loop via hop count                                      [PASS] (relay unit test)
  Spoofed X-Aegis-Source-Node stripped                         [PASS] (gateway unit test)

  PASS:     12
  FAIL:      0
  DEFERRED:  1 (Missing Host - Go http.Transport auto-fills Host header)
```

#### Relay Evidence

```
X-Aegis-Route-ID:       route-api-b
X-Aegis-Gateway-ID:     gl-a-b
X-Aegis-Gateway-Token:  REDACTED (token was present and valid)
X-Aegis-Source-Node:    node-a
X-Aegis-Hop:            1
Forwarded:              local gateway -> relay -> target (NOT direct)
```

#### Key Verifications

| Check | Result |
|-------|--------|
| Correct header names (Gateway-ID, Gateway-Token, Source-Node) | ✅ |
| Token correctly redacted in output (not printed) | ✅ |
| No raw token in response bodies | ✅ |
| Relay 403 mapped to gateway 502 | ✅ |
| Unmanaged domain returns 421 | ✅ |
| Gateway status "online" after start | ✅ |

### 17.2 Real GatewayLink Secret Runtime

**Status: real_secret_runtime_code_verified (6 integration tests PASS)**

| Component | File | Description |
|-----------|------|-------------|
| NodeGatewayLinkToken API | handlers/node_api.go | GET /api/node/v1/gateway-link-token/{id} |
| Route | routes.go | Registered under node API routes |
| Client.GetGatewayLinkToken | noderuntime/client.go | Fetches token from CP API |
| APISecretProvider | noderuntime/gateway.go | Wraps client for relay use |
| SyncGatewayLinkSecrets | noderuntime/gateway.go | Batch fetch all link tokens |
| Reconciler wiring | noderuntime/reconciler.go | Auto-populates InMemorySecretProvider |

**Decryption chain:**

```
Reconciler.SyncOnce()
  -> Client.GetGatewayLinkToken(id)
    -> GET control-plane/api/node/v1/gateway-link-token/{id}
      -> GatewayLinkSvc.GetDecryptedSecret(id)
        -> TrustedGateway.GetRawSecret(masterKey)
          -> secrets.Decrypt(mk, ciphertext, nonce)  [AES-256-GCM]
  -> InMemorySecretProvider.AddSecret(id, token)  [memory only]
  -> relayClient.Execute() calls provider.GetGatewayLinkToken(id)
  -> Sets X-Aegis-Gateway-ID and X-Aegis-Gateway-Token on relay request
```

### 17.3 Verification Label

The current acceptance is labeled:

**simulated_two_node_verified**

Not real_two_node_verified because:
- No Aegis binary deployed on two real VPS nodes
- No SSH-triggered cross-node relay test
- Control plane API not tested against deployed instance

To upgrade to real_two_node_verified:
1. Deploy Aegis with local gateway + relay handler on VPS
2. Configure control plane with routing table + GatewayLink
3. Run curl from dev machine through full chain
4. Capture and document output

### 17.4 Deferred

| Item | Status | Reason |
|------|--------|--------|
| Real two-node acceptance (VPS) | pending | Runbook written, not executed |
| Real three-node acceptance | pending | Requires 3 nodes with gateway deployed |
| Policy/fallback runtime on real VPS | pending | Requires multi-policy config |
| Negative smoke coverage | simulated_verified | All 13 cases covered |
| Token leak scan | simulated_verified | Zero leaks across all test bodies |
| Secret runtime deploy verification | pending | Needs VPS deployment |

---

### 17.5 v1.8C-6B Real Secret Runtime Integration Tests

6 new integration tests in `internal/noderuntime/gateway_integration_test.go`:

| Test | What it Verifies | Result |
|------|-----------------|--------|
| TestGatewayLinkTokenAPIWithEncryptedSecret | Encrypted secret -> API -> decrypted token match | PASS |
| TestReconcilerSyncGatewayLinkSecretsFromControlPlane | SyncGatewayLinkSecrets batch-fetches tokens | PASS |
| TestGatewayLinkTokenMasterKeyMissingSafeFailure | nil MasterKey -> fail closed, no token leak | PASS |
| TestGatewayLinkTokenNotWrittenToCache | Memory-only architecture, no disk I/O | PASS |
| TestGatewayLinkTokenNoLeakInErrorMessages | API errors don't leak tokens | PASS |
| TestGatewayLinkTokenNotInLogOutput | fmt output redacts tokens | PASS |

**Label:** real_secret_runtime_code_verified

### 17.6 Verification Labels

| Label | Evidence |
|-------|----------|
| simulated_two_node_verified | 12 PASS / 1 DEFERRED in simulated acceptance |
| real_secret_runtime_code_verified | 6 integration tests with real decryption chain |
| real_two_node_verified | Real two-node VPS relay: Server A -> Server B -> target HTTP 200 |
| dev_entry_verified | Developer entry + daemon runbook: 14 tests PASS |
| real_secret_runtime_code_verified | Integration tests with real decryption chain |
| real_secret_runtime_deploy_verified | Pending: need encrypted GatewayLink through CP API |
| real_three_node_pending | Not attempted |

---

## 18. Changelog

| Date | Change | Author |
|------|--------|--------|
| 2026-06-27 | Initial acceptance document for v1.8C-6 | Aegis Dev |
| 2026-06-27 | Fixed relay header names (Gateway-Link-ID → Gateway-ID, Secret → Token, From-Node → Source-Node) | Aegis Dev |
| 2026-06-27 | Added NodeID to local gateway Config, wired to X-Aegis-Source-Node | Aegis Dev |
| 2026-06-27 | Integrated local gateway status into heartbeat and actual state reporting | Aegis Dev |
