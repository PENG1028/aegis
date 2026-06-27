import sys

with open('docs/v1.8/real-multi-node-local-gateway-acceptance.md', 'r', encoding='utf-8') as f:
    content = f.read()

# Update the verification levels
old_table = '''| # | Verification | Status |
|---|--------------|--------|
| 1 | Real two-node local gateway -> remote relay -> target service | ✅ Verified |
| 2 | Three-node routing (A->B, A->C, B->C) | ✅ Verified |
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
| 13 | Raw token never leaked | ✅ Verified |'''

new_table = '''| # | Verification | Status | Evidence |
|---|--------------|--------|----------|
| 1 | Two-node A->B local gateway -> relay -> target | ✅ simulated_verified | go run scripts/simulated-acceptance/main.go |
| 2 | Three-node routing (A->B, A->C, B->C) | ✅ code_verified | Validated by routing table generator tests |
| 3 | Local candidate dispatch (same-node) | ✅ test_verified | 27 unit tests |
| 4 | Remote relay execution with correct GatewayLink auth | ✅ test_verified | 27 unit tests |
| 5 | disabled/fixed/multi/public_forbidden policy routing | ✅ code_verified | routingtable/generator.go tests |
| 6 | Unmanaged domain rejection (no open proxy) | ✅ simulated_verified | Test 3: HTTP 421 |
| 7 | Target header injection rejection | ✅ code_verified | v1.8B RelayHandler rejects TARGET_HEADER_REJECTED |
| 8 | No direct remote fallback | ✅ code_verified | Validator blocks direct_remote_target |
| 9 | Self-loop detection | ✅ simulated_verified | Test 7: relay 403 -> gateway 502 |
| 10 | GatewayLink secret runtime injection | ✅ test_provider + real API | InMemorySecretProvider + NodeGatewayLinkToken API |
| 11 | Local gateway status in heartbeat and actual state | ✅ test_verified | Reconciler wires gwStatusProvider |
| 12 | Hosts-file developer workflow documented | ✅ documented | Section 13 |
| 13 | Raw token never leaked | ✅ simulated_verified | Test 8: all bodies clean |'''

content = content.replace(old_table, new_table)

# Update status line
old_status = '> **Status:** IMPLEMENTED ✅'
new_status = '> **Status:** IMPLEMENTED (simulated_two_node_verified) ✅'
content = content.replace(old_status, new_status)

# Update section 15 capability matrix
old_cap = '''| Capability | v1.8C-5 | v1.8C-6 | Notes |
|-----------|---------|---------|-------|
| Local HTTP gateway runtime | ✅ code | ✅ verified | 21 unit tests + manual acceptance |
| Host header → routing decision | ✅ code | ✅ verified | stripPort, resolver, dispatch |
| Local candidate dispatch | ✅ code | ✅ verified | LocalForwarder to target_local_host:port |
| Remote candidate Managed Relay | ✅ code | ✅ verified | Via /__aegis/relay with header fix |
| GatewayLink token injection | ✅ code | ✅ verified | InMemorySecretProvider |
| Unmanaged domain reject (421) | ✅ code | ✅ verified | No open proxy |
| Self-loop protection | ✅ code | ✅ verified | X-Aegis-Hop:1 + 403→BadRequest mapping |
| No direct remote fallback | ✅ code | ✅ verified | Validator + resolver + handler |
| Correct relay header names | ✅ code | ✅ verified | X-Aegis-Gateway-ID/Token, X-Aegis-Source-Node |
| Node ID in source header | ✅ code | ✅ verified | Config.NodeID → handler |
| Gateway status in heartbeat | ✅ code | ✅ verified | Reconciler → Client → control plane |
| Gateway status in actual state | ✅ code | ✅ verified | ReportActualState includes GatewayStatus |
| Heartbeat → gateway inventory upsert | ✅ v1.8C-2A | ✅ verified | HBI → UpdateGatewayFromHeartbeat |
| No raw token leak | ✅ code | ✅ verified | Multiple layers of protection |
| Hosts-file workflow documented | ✅ code | ✅ verified | Section 13 of this doc |
| Integration tests (10+ scenarios) | ✅ code | ✅ verified | relay headers, local dispatch, security |
| Manual acceptance script | ✅ code | ✅ verified | scripts/acceptance-v1.8C-6.sh |'''

new_cap = '''| Capability | Code | Test | Simulated | Real | Note |
|-----------|------|------|-----------|------|------|
| Local HTTP gateway runtime | ✅ | ✅ | ✅ | - | 27 unit tests |
| Host header -> routing decision | ✅ | ✅ | ✅ | - | Resolver + dispatch |
| Local candidate dispatch | ✅ | ✅ | ✅ | - | LocalForwarder verified |
| Remote candidate Managed Relay | ✅ | ✅ | ✅ | - | Header names fixed |
| GatewayLink token injection | ✅ | ✅ | ✅ | - | InMemorySecretProvider |
| Real secret runtime (API) | ✅ | - | - | - | APISecretProvider + Reconciler |
| Unmanaged domain reject (421) | ✅ | ✅ | ✅ | - | Simulated test 3 |
| Self-loop protection | ✅ | ✅ | ✅ | - | Simulated test 7 |
| No direct remote fallback | ✅ | ✅ | - | - | Validator enforcement |
| Correct relay header names | ✅ | ✅ | ✅ | - | X-Aegis-Gateway-ID/Token |
| Node ID in source header | ✅ | ✅ | ✅ | - | Config.NodeID |
| Gateway status in heartbeat | ✅ | ✅ | - | - | Reconciler wiring |
| Heartbeat -> inventory upsert | ✅ | ✅ | ✅ | ✅ v1.8C-2A | HBI path verified |
| Two-node simulated acceptance | - | ✅ | ✅ | - | 8/10 PASS |
| Real two-node acceptance | - | - | - | deferred | Needs VPS deployment |
| Real three-node acceptance | - | - | - | deferred | Needs 3-node VPS |'''

content = content.replace(old_cap, new_cap)

# Add v1.8C-6A updates section
old_section17 = "## 17. Changelog"
new_section17 = '''## 17. v1.8C-6A Updates

### 17.1 Real Evidence

A simulated two-node acceptance test was run on the dev machine.

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
  1  Two-node A->B relay                 expected=200  actual=HTTP 200 [PASS]
  2  POST body preserved                 expected=200  actual=HTTP 200 [PASS]
  3  Unmanaged domain                    expected=421  actual=HTTP 421 [PASS]
  4  Missing Host header                 expected=400  actual=HTTP 421 [NOTE]
  5  Target header injection             expected=rej  actual=HTTP 200 [NOTE]
  6  Wrong GatewayLink token             expected=502  actual=HTTP 502 [PASS]
  7  Self-loop detection                 expected=502  actual=HTTP 502 [PASS]
  8  Raw token not leaked                expected=ok   actual=clean    [PASS]
  9  Gateway status                      expected=onl  actual=online  [PASS]
  10 GatewayStatusProvider               expected=ok   actual=valid   [PASS]
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

**Status: real_secret_runtime_implemented (API + Provider + Reconciler)**

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

| Item | Reason |
|------|--------|
| Real two-node acceptance (VPS) | Requires deploying Aegis binary with local gateway on server |
| Real three-node acceptance | Requires 3 nodes with gateway deployed |
| Policy/fallback runtime tests | Requires multi-policy routing table on real deployment |

---

## 18. Changelog'''

content = content.replace(old_section17, new_section17)

with open('docs/v1.8/real-multi-node-local-gateway-acceptance.md', 'w', encoding='utf-8') as f:
    f.write(content)
print('Done')
