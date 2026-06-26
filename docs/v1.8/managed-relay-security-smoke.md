# v1.8B-2 — Managed Relay Security Smoke

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Sub-phase:** v1.8B-2 — Managed Relay Real Acceptance & Auth Tightening
> **Status:** SECURITY VERIFICATION PLAN

---

## Test Cases

### S1: Missing GatewayLink token → 401

Send relay request without `X-Aegis-Gateway-Token`:

```bash
curl -v -X POST http://127.0.0.1:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_abc" \
  -H "X-Aegis-Gateway-ID: gw_xyz" \
  -H "X-Aegis-Source-Node: nd_a" \
  -H "X-Aegis-Hop: 1"
```

**Expected:** `400 MISSING_GATEWAY_TOKEN`

---

### S2: Wrong GatewayLink token → 403

Send relay request with invalid token:

```bash
curl -v -X POST http://127.0.0.1:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_abc" \
  -H "X-Aegis-Gateway-ID: gw_xyz" \
  -H "X-Aegis-Gateway-Token: wrong-token" \
  -H "X-Aegis-Source-Node: nd_a" \
  -H "X-Aegis-Hop: 1"
```

**Expected:** `403 INVALID_GATEWAY_TOKEN`

---

### S3: X-Aegis-Target-Host present → reject

```bash
curl -v -X POST http://127.0.0.1:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_abc" \
  -H "X-Aegis-Gateway-ID: gw_xyz" \
  -H "X-Aegis-Gateway-Token: test" \
  -H "X-Aegis-Source-Node: nd_a" \
  -H "X-Aegis-Target-Host: evil.com"
```

**Expected:** `400 TARGET_HEADER_REJECTED`

---

### S4: X-Aegis-Target-Port present → reject

```bash
curl -v -X POST http://127.0.0.1:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_abc" \
  -H "X-Aegis-Gateway-ID: gw_xyz" \
  -H "X-Aegis-Gateway-Token: test" \
  -H "X-Aegis-Source-Node: nd_a" \
  -H "X-Aegis-Target-Port: 9999"
```

**Expected:** `400 TARGET_HEADER_REJECTED`

---

### S5: hop > 1 → 508

```bash
curl -v -X POST http://127.0.0.1:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_abc" \
  -H "X-Aegis-Gateway-ID: gw_xyz" \
  -H "X-Aegis-Gateway-Token: test" \
  -H "X-Aegis-Source-Node: nd_a" \
  -H "X-Aegis-Hop: 2"
```

**Expected:** `508 MAX_HOPS_EXCEEDED`

---

### S6: endpoint.node_id empty → 409

If the endpoint has an empty `node_id` field:

```bash
curl -v -X POST http://127.0.0.1:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_empty_node" \
  -H "X-Aegis-Gateway-ID: gw_xyz" \
  -H "X-Aegis-Gateway-Token: test" \
  -H "X-Aegis-Source-Node: nd_a" \
  -H "X-Aegis-Hop: 1"
```

**Expected:** `409 ENDPOINT_NODE_UNKNOWN`

---

### S7: endpoint.node_id != current_node → 409

If endpoint belongs to a different node:

```bash
curl -v -X POST ...
```

**Expected:** `409 ENDPOINT_NOT_LOCAL`

---

### S8: route not found → 404

```bash
curl -v -X POST http://127.0.0.1:80/__aegis/relay \
  -H "X-Aegis-Route-ID: rt_nonexistent" \
  -H "X-Aegis-Gateway-ID: gw_xyz" \
  -H "X-Aegis-Gateway-Token: test" \
  -H "X-Aegis-Source-Node: nd_a" \
  -H "X-Aegis-Hop: 1"
```

**Expected:** `404 ROUTE_NOT_FOUND`

---

### S9: local target down → 502

If the endpoint's backend service is not running:

- Configure endpoint → route → relay request
- Stop the backend service on the target port
- Send relay request

**Expected:** `502 TARGET_UNREACHABLE`

---

### S10: unavailable must not fallback to direct remote target

When resolver returns `unavailable`, the relay dispatch must not attempt to forward to the original `target_host:target_port`. This is enforced by:

- resolver does not return a `gateway_url` or `final_local_target` when mode=unavailable
- handler only proxies to `127.0.0.1:<port>`, never to remote IPs
- handler rejects endpoints that point to non-local targets

---

## Results Table

| # | Scenario | Expected | Actual | Status |
|---|----------|----------|--------|--------|
| S1 | Missing GatewayLink token | 400 | | ⏳ |
| S2 | Wrong GatewayLink token | 403 | | ⏳ |
| S3 | Target-Host header | 400 | | ⏳ |
| S4 | Target-Port header | 400 | | ⏳ |
| S5 | hop > 1 | 508 | | ⏳ |
| S6 | endpoint.node_id empty | 409 | | ⏳ |
| S7 | endpoint.node_id mismatch | 409 | | ⏳ |
| S8 | route not found | 404 | | ⏳ |
| S9 | local target down | 502 | | ⏳ |
| S10 | unavailable no fallback | no remote target | | ⏳ |
