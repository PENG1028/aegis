# v1.8B-4 — Managed Relay Negative Real Smoke

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Status:** ACCEPTANCE PASSED ✅
> **Date:** 2026-06-26

---

## Environment

| Property | Value |
|----------|-------|
| Server B | <SERVER_B_IP> (target_node, relay dispatch) |
| Server B Gateway | Port 80 via Caddy → Aegis RelayHandler (port 9000) |
| Local target | 127.0.0.1:2724 (Python test server) |
| Valid route | `rt_relay` → service `svc_relay` → endpoint `ep_relay` (127.0.0.1:2724, node_id=node_VM-0-11-ubuntu) |
| Valid GatewayLink | `gw_relay` (HMAC hashed auth) |
| Valid source node | `node_VM-0-4-ubuntu` |

---

## Test Results

### N1: Missing GatewayLink token

**Request:**
```bash
curl -X POST http://<SERVER_B_IP>:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_relay" \
  -H "X-Aegis-Gateway-ID: gw_relay" \
  -H "X-Aegis-Source-Node: node_VM-0-4-ubuntu" \
  -H "X-Aegis-Hop: 1"
```
*(no X-Aegis-Gateway-Token header)*

**Expected:** 400 (MISSING_GATEWAY_TOKEN)  
**Actual:** `{"error":"MISSING_GATEWAY_TOKEN","message":"X-Aegis-Gateway-Token is required"}` — HTTP 400  
**Target received:** ❌ No (rejected before dispatch)  
**Status:** ✅ PASS

---

### N2: Wrong GatewayLink token

**Request:**
```bash
curl -X POST http://<SERVER_B_IP>:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_relay" \
  -H "X-Aegis-Gateway-ID: gw_relay" \
  -H "X-Aegis-Gateway-Token: wrong-token-value" \
  -H "X-Aegis-Source-Node: node_VM-0-4-ubuntu" \
  -H "X-Aegis-Hop: 1"
```

**Expected:** 403 (INVALID_GATEWAY_TOKEN)  
**Actual:** `{"error":"INVALID_GATEWAY_TOKEN","message":"gateway token verification failed"}` — HTTP 403  
**Target received:** ❌ No (rejected before dispatch)  
**Status:** ✅ PASS

---

### N3: Missing GatewayLink ID

**Request:**
```bash
curl -X POST http://<SERVER_B_IP>:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_relay" \
  -H "X-Aegis-Gateway-Token: test-relay-secret" \
  -H "X-Aegis-Source-Node: node_VM-0-4-ubuntu" \
  -H "X-Aegis-Hop: 1"
```
*(no X-Aegis-Gateway-ID header)*

**Expected:** 400 (MISSING_GATEWAY_ID)  
**Actual:** `{"error":"MISSING_GATEWAY_ID","message":"X-Aegis-Gateway-ID is required"}` — HTTP 400  
**Target received:** ❌ No (rejected before dispatch)  
**Status:** ✅ PASS

---

### N4: X-Aegis-Target-Host present

**Request:**
```bash
curl -X POST http://<SERVER_B_IP>:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_relay" \
  -H "X-Aegis-Gateway-ID: gw_relay" \
  -H "X-Aegis-Gateway-Token: test-relay-secret" \
  -H "X-Aegis-Source-Node: node_VM-0-4-ubuntu" \
  -H "X-Aegis-Hop: 1" \
  -H "X-Aegis-Target-Host: evil.com" \
  -H "X-Aegis-Target-Port: 9999"
```

**Expected:** 400 (TARGET_HEADER_REJECTED)  
**Actual:** `{"error":"TARGET_HEADER_REJECTED","message":"X-Aegis-Target-Host and X-Aegis-Target-Port headers are not allowed"}` — HTTP 400  
**Target received:** ❌ No (rejected before dispatch)  
**Status:** ✅ PASS

---

### N5: X-Aegis-Target-Port present (solo)

**Request:**
```bash
curl -X POST http://<SERVER_B_IP>:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_relay" \
  -H "X-Aegis-Gateway-ID: gw_relay" \
  -H "X-Aegis-Gateway-Token: test-relay-secret" \
  -H "X-Aegis-Source-Node: node_VM-0-4-ubuntu" \
  -H "X-Aegis-Hop: 1" \
  -H "X-Aegis-Target-Port: 9999"
```

**Expected:** 400 (TARGET_HEADER_REJECTED)  
**Actual:** `{"error":"TARGET_HEADER_REJECTED","message":"X-Aegis-Target-Host and X-Aegis-Target-Port headers are not allowed"}` — HTTP 400  
**Target received:** ❌ No (rejected before dispatch)  
**Status:** ✅ PASS

---

### N6: Hop > 1

**Request:**
```bash
curl -X POST http://<SERVER_B_IP>:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_relay" \
  -H "X-Aegis-Gateway-ID: gw_relay" \
  -H "X-Aegis-Gateway-Token: test-relay-secret" \
  -H "X-Aegis-Source-Node: node_VM-0-4-ubuntu" \
  -H "X-Aegis-Hop: 2"
```

**Expected:** 508 (MAX_HOPS_EXCEEDED)  
**Actual:** `{"error":"MAX_HOPS_EXCEEDED","message":"hop count 2 exceeds limit of 1"}` — HTTP 508  
**Target received:** ❌ No (rejected before dispatch)  
**Status:** ✅ PASS

---

### N7: Unknown route

**Request:**
```bash
curl -X POST http://<SERVER_B_IP>:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_nonexistent" \
  -H "X-Aegis-Gateway-ID: gw_relay" \
  -H "X-Aegis-Gateway-Token: test-relay-secret" \
  -H "X-Aegis-Source-Node: node_VM-0-4-ubuntu" \
  -H "X-Aegis-Hop: 1"
```

**Expected:** 404 (ROUTE_NOT_FOUND)  
**Actual:** `{"error":"ROUTE_NOT_FOUND","message":"route rt_nonexistent not found"}` — HTTP 404  
**Target received:** ❌ No (rejected before dispatch)  
**Status:** ✅ PASS

---

### N8: endpoint.node_id empty

**Setup:** Route `rt_empty` → service → endpoint `ep_empty` with `node_id=""`.

**Request:**
```bash
curl -X POST http://<SERVER_B_IP>:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_empty" \
  -H "X-Aegis-Gateway-ID: gw_relay" \
  -H "X-Aegis-Gateway-Token: test-relay-secret" \
  -H "X-Aegis-Source-Node: node_VM-0-4-ubuntu" \
  -H "X-Aegis-Hop: 1"
```

**Expected:** 409 (ENDPOINT_NODE_UNKNOWN)  
**Actual:** `{"error":"ENDPOINT_NODE_UNKNOWN","message":"endpoint ep_empty has empty node_id — must be set to node_VM-0-11-ubuntu for relay"}` — HTTP 409  
**Target received:** ❌ No (rejected before dispatch)  
**Status:** ✅ PASS

---

### N9: endpoint.node_id mismatch

**Setup:** Route `rt_mismatch` → service → endpoint `ep_mismatch` with `node_id="node_nonexistent"`.

**Request:**
```bash
curl -X POST http://<SERVER_B_IP>:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_mismatch" \
  -H "X-Aegis-Gateway-ID: gw_relay" \
  -H "X-Aegis-Gateway-Token: test-relay-secret" \
  -H "X-Aegis-Source-Node: node_VM-0-4-ubuntu" \
  -H "X-Aegis-Hop: 1"
```

**Expected:** 409 (ENDPOINT_NOT_LOCAL)  
**Actual:** `{"error":"ENDPOINT_NOT_LOCAL","message":"no endpoint for route rt_mismatch belongs to this node"}` — HTTP 409  
**Target received:** ❌ No (rejected before dispatch)  
**Status:** ✅ PASS

---

### N10: Local target down

**Setup:** Stop the Python test server on 127.0.0.1:2724.

**Request:** (same valid relay request with target down)
```bash
curl -X POST http://<SERVER_B_IP>:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_relay" \
  -H "X-Aegis-Gateway-ID: gw_relay" \
  -H "X-Aegis-Gateway-Token: test-relay-secret" \
  -H "X-Aegis-Source-Node: node_VM-0-4-ubuntu" \
  -H "X-Aegis-Hop: 1"
```

**Expected:** 502 (TARGET_UNREACHABLE)  
**Actual:** `{"error":"TARGET_UNREACHABLE","message":"local target 127.0.0.1:2724 unreachable: ... connection refused"}` — HTTP 502  
**Dispatch:** ✅ Auth validated (passed N1–N9 checks), local dispatch attempted → failed (target down)  
**Note:** Error message does not leak raw token or auth info.  
**Status:** ✅ PASS

---

## Results Summary

| # | Test | Expected | Actual | Dispatch Attempt | Status |
|---|------|----------|--------|-----------------|--------|
| N1 | Missing GatewayLink token | 400 | 400 | ❌ rejected before dispatch | ✅ PASS |
| N2 | Wrong GatewayLink token | 403 | 403 | ❌ rejected before dispatch | ✅ PASS |
| N3 | Missing GatewayLink ID | 400 | 400 | ❌ rejected before dispatch | ✅ PASS |
| N4 | X-Aegis-Target-Host present | 400 | 400 | ❌ rejected before dispatch | ✅ PASS |
| N5 | X-Aegis-Target-Port present | 400 | 400 | ❌ rejected before dispatch | ✅ PASS |
| N6 | hop > 1 | 508 | 508 | ❌ rejected before dispatch | ✅ PASS |
| N7 | Unknown route | 404 | 404 | ❌ rejected before dispatch | ✅ PASS |
| N8 | endpoint.node_id empty | 409 | 409 | ❌ rejected before dispatch | ✅ PASS |
| N9 | endpoint.node_id mismatch | 409 | 409 | ❌ rejected before dispatch | ✅ PASS |
| N10 | Local target down | 502 | 502 | ✅ attempted dispatch → 502 | ✅ PASS |

**N1–N9: All rejected before dispatch. N10: Successfully validated all auth, then attempted local dispatch, returned 502 (target down). No request reached the local target when invalid.**

## Error Code Mapping

| Condition | HTTP Status | Error Code |
|-----------|-------------|-----------|
| Missing required relay auth header (token) | 400 | MISSING_GATEWAY_TOKEN |
| Missing required relay auth header (ID) | 400 | MISSING_GATEWAY_ID |
| Wrong/invalid token | 403 | INVALID_GATEWAY_TOKEN |
| Route not found | 404 | ROUTE_NOT_FOUND |
| Gateway link not found | 401 | INVALID_GATEWAY |
| Hop limit exceeded | 508 | MAX_HOPS_EXCEEDED |
| Target header injection attempt | 400 | TARGET_HEADER_REJECTED |
| Endpoint has empty node_id | 409 | ENDPOINT_NODE_UNKNOWN |
| Endpoint belongs to different node | 409 | ENDPOINT_NOT_LOCAL |
| Local target unreachable | 502 | TARGET_UNREACHABLE |
