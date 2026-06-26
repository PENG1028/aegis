# v1.8B-2 — Managed Relay Real Acceptance

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Sub-phase:** v1.8B-2 — Managed Relay Real Acceptance & Auth Tightening
> **Status:** ACCEPTANCE PLAN (requires real two-node execution)

---

## Environment

| Property | Value |
|----------|-------|
| Server A | Gateway node (from_node) |
| Server B | Target node (target_node) |
| Server B local target | 127.0.0.1:2724 (example HTTP service) |
| Gateway Link | `gwlink_smoke` — Server A → Server B auth |
| Route | `rt_smoke_relay` — domain `relay-smoke.test` → Server B service |
| Endpoint | `ep_smoke_relay` — `127.0.0.1:2724`, `node_id` = Server B node_id |
| Listener | Server B gateway listener port 80 (HTTP) |

---

## Test 1: Relay Resolve (Server A → Server B)

**Command:**
```bash
aegis relay resolve relay-smoke.test --from-node <ServerA_node_id> --json
```

**Expected output:**
```json
{
  "domain": "relay-smoke.test",
  "managed": true,
  "mode": "private_gateway",
  "from_node_id": "<ServerA_node_id>",
  "from_node_hostname": "server-a",
  "target_node_id": "<ServerB_node_id>",
  "target_node_hostname": "server-b",
  "gateway_url": "http://<ServerB_private_ip>:80",
  "gateway_port": 80,
  "route_id": "rt_smoke_relay",
  "service_id": "<svc_id>",
  "endpoint_id": "ep_smoke_relay",
  "gateway_link_id": "gwlink_smoke",
  "direct_target_suppressed": true,
  "final_local_target": "127.0.0.1:2724",
  "risks": [],
  "recommendation": "send request to private gateway ... with GatewayLink auth"
}
```

**Checklist:**
- [ ] `mode` = `private_gateway` or `public_gateway`
- [ ] `direct_target_suppressed` = true
- [ ] `final_local_target` = `127.0.0.1:2724` (admin only)
- [ ] `gateway_link_id` = non-empty
- [ ] `risks` empty (no GATEWAY_LINK_MISSING)
- [ ] `managed` = true

---

## Test 2: Server A → Server B Gateway → localhost target

**Setup:** Start a test HTTP server on Server B:
```bash
# On Server B:
echo "relay-ok" > /tmp/relay-test.html
cd /tmp && python3 -m http.server 2724 &
# Or: nc -l -p 2724 -e /bin/echo -e "HTTP/1.1 200 OK\r\n\r\nrelay-ok"
```

**Relay request from Server A:**
```bash
curl -v -X POST http://<ServerB_gateway_ip>:80/__aegis/relay \
  -H "Host: relay-smoke.test" \
  -H "X-Aegis-Route-ID: rt_smoke_relay" \
  -H "X-Aegis-Gateway-ID: gwlink_smoke" \
  -H "X-Aegis-Gateway-Token: <secret>" \
  -H "X-Aegis-Source-Node: <ServerA_node_id>" \
  -H "X-Aegis-Hop: 1" \
  -H "X-Aegis-Request-ID: smoke-001"
```

**Expected:**
```
HTTP/1.1 200 OK
...
relay-ok
```

**Checklist:**
- [ ] Server B returns 200
- [ ] Response body matches the target service output
- [ ] Server B Aegis logs show `relay_success`

---

## Test 3: Direct access blocked (conceptual)

The design prohibits direct access to `ServerB:2724`. In a hardened environment:

```bash
# This should NOT be used:
curl http://<ServerB_ip>:2724/  # direct target access
```

If the cloud security group only opens port 80/443, this connection will be blocked by the firewall — which is the desired behavior.

**Checklist:**
- [ ] Direct `ServerB:2724` is either blocked by firewall or documented as prohibited
- [ ] All managed traffic goes through gateway listener port (80/443)

---

## Test 4: Missing GatewayLink → unavailable

If route has no `gateway_link_id`:

```bash
aegis relay resolve relay-nogw.test --from-node <ServerA_node_id> --json
```

**Expected:**
```json
{
  "domain": "relay-nogw.test",
  "managed": true,
  "mode": "unavailable",
  "error": "GatewayLink required for private egress relay",
  "direct_target_suppressed": true,
  "final_local_target": "",
  "gateway_url": ""
}
```

**Checklist:**
- [ ] `mode` = `unavailable`
- [ ] `error` mentions GatewayLink required
- [ ] `final_local_target` = empty (not leaked)
- [ ] `gateway_url` = empty (not leaked)

---

## Results Table

| # | Scenario | Expected | Actual | Status |
|---|----------|----------|--------|--------|
| 1 | Relay resolve (private/public) | mode=private_gateway, gwlink present | | ⏳ |
| 2 | Relay request → gateway → localhost | 200, body matches | | ⏳ |
| 3 | No direct access to target port | blocked / not used | | ⏳ |
| 4 | Missing GatewayLink → unavailable | mode=unavailable, no leak | | ⏳ |
