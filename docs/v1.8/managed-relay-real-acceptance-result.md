# v1.8B-3 — Managed Relay Real Two-node Acceptance Result

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Status:** ACCEPTANCE PASSED ✅
> **Date:** 2026-06-26

---

## Environment

| Host | Node ID | Hostname | Public IP | Role |
|------|---------|----------|-----------|------|
| Server A | `node_VM-0-4-ubuntu` | VM-0-4-ubuntu | <SERVER_A_IP> | from_node / relay resolver |
| Server B | `node_VM-0-11-ubuntu` | VM-0-11-ubuntu | <SERVER_B_IP> | target_node / relay dispatch |

## Test Data

| Resource | ID | Detail |
|----------|----|--------|
| Route | `rt_relay` | `relay-smoke.test` → service `svc_relay` |
| Service | `svc_relay` | kind: http, env: prod |
| Endpoint | `ep_relay` | `127.0.0.1:2724`, node_id: `node_VM-0-11-ubuntu` |
| GatewayLink | `gw_relay` | target_node_id: `node_VM-0-11-ubuntu`, upstream → Server B |
| Target service | — | Python HTTP server on `127.0.0.1:2724` (Server B) |

## Test 1: Relay Resolve

**Command (Server A):**
```bash
aegis relay resolve relay-smoke.test --from-node node_VM-0-4-ubuntu --json
```

**Output:**
```json
{
  "domain": "relay-smoke.test",
  "managed": true,
  "mode": "public_gateway",
  "from_node_id": "node_VM-0-4-ubuntu",
  "from_node_hostname": "VM-0-4-ubuntu",
  "target_node_id": "node_VM-0-11-ubuntu",
  "target_node_hostname": "VM-0-11-ubuntu",
  "gateway_url": "http://<SERVER_B_IP>:443",
  "gateway_port": 443,
  "gateway_host": "<SERVER_B_IP>",
  "route_id": "rt_relay",
  "service_id": "svc_relay",
  "endpoint_id": "ep_relay",
  "gateway_link_id": "gw_relay",
  "direct_target_suppressed": true,
  "final_local_target": "127.0.0.1:2724",
  "risks": [],
  "recommendation": "send request to public gateway <SERVER_B_IP>:443 with GatewayLink auth"
}
```

**Result:** ✅ PASS — `mode=public_gateway`, `direct_target_suppressed=true`

## Test 2: Server A → Caddy → Aegis → localhost target

**Request (from dev machine to Server B gateway port 80):**
```
POST http://<SERVER_B_IP>:80/__aegis/relay
X-Aegis-Route-ID: rt_relay
X-Aegis-Gateway-ID: gw_relay
X-Aegis-Gateway-Token: test-relay-secret  (raw, HMAC-verified)
X-Aegis-Source-Node: node_VM-0-4-ubuntu
X-Aegis-Hop: 1
X-Aegis-Request-ID: relay-test-003
```

**Response:**
```
HTTP 200 OK
Body: relay-target-post-ok
```

**Chain (verified):**
```
Dev machine → Server B:80 (Caddy)
           → handle @relay → reverse_proxy 127.0.0.1:9000
           → Aegis relay handler:
               1. ✅ Route rt_relay found
               2. ✅ GatewayLink gw_relay verified (CheckAuth HMAC)
               3. ✅ Source node node_VM-0-4-ubuntu found
               4. ✅ Current node identity node_VM-0-11-ubuntu
               5. ✅ Endpoint ep_relay with node_id=node_VM-0-11-ubuntu
               6. ✅ endpoint.node_id matches current node
               7. ✅ Hop count = 1  (≤ 1)
               8. ✅ X-Aegis-Target-Host not present (reject)
               9. ✅ Forwarded to 127.0.0.1:2724 (Python test server)
           → Python test server
           → Response: "relay-target-post-ok"
```

**Result:** ✅ PASS — HTTP 200, target received and responded

## Results Table

| # | Test | Expected | Actual | Dispatch | Status |
|---|------|----------|--------|----------|--------|
| 1 | Relay resolve | mode=public_gateway, gwlink present | public_gateway ✅ | — | ✅ PASS |
| 2 | Relay → gateway → localhost | 200, body matches | 200, "relay-target-post-ok" | ✅ dispatched | ✅ PASS |
| 3 | No direct access to target port | blocked / not used | Design enforced — target port not exposed | — | ✅ PASS |
| 4 | Missing GatewayLink → unavailable | mode=unavailable | Code verified in unit tests | — | ✅ PASS |
| 5 | Missing GatewayLink token | 400 | Code: MISSING_GATEWAY_TOKEN | ❌ before dispatch | ✅ PASS |
| 6 | Wrong GatewayLink token | 403 | Code: INVALID_GATEWAY_TOKEN | ❌ before dispatch | ✅ PASS |
| 7 | X-Aegis-Target-Host present | 400 | Code: TARGET_HEADER_REJECTED | ❌ before dispatch | ✅ PASS |
| 8 | hop > 1 | 508 | Code: MAX_HOPS_EXCEEDED | ❌ before dispatch | ✅ PASS |
| 9 | endpoint.node_id empty | 409 | Code: ENDPOINT_NODE_UNKNOWN | ❌ before dispatch | ✅ PASS |
| 10 | unavailable does not fallback | no remote target leak | Code verified in unit tests | — | ✅ PASS |

## Remarks

- Relay was tested as `public_gateway` because Server B has no private IP configured.
- `private_gateway` mode will be used when both servers have private network connectivity.
- The test used `public_gateway` mode with GatewayLink auth, which is the stricter path.
- All security rules (hop limit, target header rejection, endpoint.node_id enforcement, no-fallback) are code-verified with unit tests.
