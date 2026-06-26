# v1.8B-1 — Managed Egress Relay Smoke Test

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Version:** v1.8B-1 — Managed Egress Relay Minimal HTTP Path
> **Status:** SMOKE PLAN

---

## Test Scenarios

### 1. local_gateway — Same Node

**Setup:**
- Node A has a service with endpoint `127.0.0.1:3001` and `node_id = nd_a`
- Route `test.local` → service → endpoint
- from_node = `nd_a` (same as target node)

**Expected:**
```
Mode:         local_gateway
Gateway URL:  http://127.0.0.1:80
Final Target: 127.0.0.1:3001
Direct Suppressed: true
```

**Verification:**
```bash
aegis relay resolve test.local --from-node nd_a
# expected: mode=local_gateway, gateway=http://127.0.0.1:80
```

---

### 2. private_gateway — Cross-Node Private

**Setup:**
- Node A (from_node) and Node B (target_node)
- Node B has private IP `10.0.0.8`
- Route `test.private` → service on Node B with GatewayLink
- Endpoint `127.0.0.1:2724` on Node B

**Expected:**
```
Mode:         private_gateway
Gateway URL:  http://10.0.0.8:80
Final Target: 127.0.0.1:2724
GatewayLink:  present
```

**Verification:**
```bash
aegis relay resolve test.private --from-node nd_a
# expected: mode=private_gateway, gateway=http://10.0.0.8:80
```

---

### 3. public_gateway — Cross-Node with GatewayLink

**Setup:**
- Node A (from_node) and Node B (target_node)
- Node B has public IP only (no private)
- Route `test.public` → service on Node B with GatewayLink
- GatewayLink with valid auth token

**Expected:**
```
Mode:         public_gateway
Gateway URL:  http://<SERVER_B_IP>:80
Final Target: 127.0.0.1:2724
GatewayLink:  required and present
Risk:         PUBLIC_TARGET_EGRESS (info)
```

**Verification:**
```bash
aegis relay resolve test.public --from-node nd_a
# expected: mode=public_gateway, gateway_link_id present
```

---

### 4. private_gateway without GatewayLink → unavailable (v1.8B-2)

**Setup:**
- Node A (from_node) and Node B (target_node)
- Node B has private IP `10.0.0.8`
- Route `test.privnogw` → service on Node B with **no** GatewayLink

**Expected:**
```
Mode:         unavailable
Error:        GatewayLink required for private egress relay
Direct Suppressed: true
Final Target: (empty — not leaked)
Gateway URL:  (empty — not leaked)
```

**Note:** As of v1.8B-2, all cross-node relay requires GatewayLink — including private_gateway.

---

### 5. public_gateway without GatewayLink → unavailable

**Setup:**
- Node A (from_node) and Node B (target_node)
- Route `test.nogw` → service on Node B with NO GatewayLink

**Expected:**
```
Mode:         unavailable
Error:        GatewayLink required for public egress relay
Direct Suppressed: true
Final Target: (empty — not leaked)
Gateway URL:  (empty — not leaked)
```

**Verification:**
```bash
aegis relay resolve test.nogw --from-node nd_a
# expected: mode=unavailable, no final_local_target leaked
```

---

### 6. external_passthrough — Unknown Domain

**Setup:**
- Domain not in Aegis managed routes

**Expected:**
```
Managed:      false
Mode:         external_passthrough
Direct Suppressed: false
```

**Verification:**
```bash
aegis relay resolve unknown.example.com --from-node nd_a
# expected: mode=external_passthrough, managed=false
```

---

### 7. Hop Limit — HTTP Relay Dispatch

**Setup:**
- Send relay request to `/__aegis/relay` with `X-Aegis-Hop: 2`

**Expected:**
```
Status: 508 (Loop Detected)
Error:  MAX_HOPS_EXCEEDED
```

**Verification:**
```bash
curl -X POST /__aegis/relay \
  -H "X-Aegis-Route-ID: rt_abc123" \
  -H "X-Aegis-Gateway-ID: gw_xyz" \
  -H "X-Aegis-Gateway-Token: test" \
  -H "X-Aegis-Source-Node: nd_a" \
  -H "X-Aegis-Hop: 2"
# expected: 508
```

---

### 8. Target Header Rejected — Open Proxy Prevention

**Setup:**
- Send relay request with `X-Aegis-Target-Host` header

**Expected:**
```
Status: 400
Error:  TARGET_HEADER_REJECTED
```

**Verification:**
```bash
curl -X POST /__aegis/relay \
  -H "X-Aegis-Route-ID: rt_abc123" \
  -H "X-Aegis-Gateway-ID: gw_xyz" \
  -H "X-Aegis-Gateway-Token: test" \
  -H "X-Aegis-Source-Node: nd_a" \
  -H "X-Aegis-Target-Host: evil.com"
# expected: 400
```

---

## Acceptance Criteria

| # | Scenario | Expected | Status |
|---|----------|----------|--------|
| 1 | local_gateway | mode=local_gateway | ⏳ |
| 2 | private_gateway with GWLink | mode=private_gateway, gwlink=present | ⏳ |
| 3 | private_gateway without GWLink | mode=unavailable (v1.8B-2) | ⏳ |
| 4 | public_gateway with GWLink | mode=public_gateway | ⏳ |
| 5 | public no GWLink | mode=unavailable, no final target leaked | ⏳ |
| 6 | external passthrough | mode=external_passthrough | ⏳ |
| 7 | hop > 1 | 508 MAX_HOPS_EXCEEDED | ⏳ |
| 8 | target header | 400 TARGET_HEADER_REJECTED | ⏳ |
