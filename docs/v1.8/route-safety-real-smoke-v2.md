# Route Safety Real Smoke v2 — v1.8A-4

> 5 real-world route safety smoke tests with listener-aware self-loop semantics.
> All cases executed against Aegis v1.8A-4.

---

## Test Environment

| Property | Value |
|----------|-------|
| Node ID | `node_PENGSPC` |
| Node IPs | 127.0.0.1 (local), 192.168.3.2 (private) |
| Gateway Link | `gwlink_smoke` → Server B (43.159.34.11:80) |
| Default listeners | 80 (HTTP), 443 (TLS), 8443 (internal HTTPS) |

---

## Case 1: Loopback Non-Listener Port — 127.0.0.1:3001 ✅ Safe

```bash
aegis safety check-route rt_3fb75b7e5c8585fe
```

```json
{
  "route_id": "rt_3fb75b7e5c8585fe",
  "domain": "lb-loopback.smoke.test",
  "target_host": "127.0.0.1",
  "target_port": 3001,
  "ip_classification": "loopback",
  "is_current_node_address": true,
  "is_gateway_listener_target": false,
  "has_gateway_link": false,
  "risks": null
}
```

**Risks:** None  
**Verdict:** ✅ Safe — loopback non-listener port is a normal service target.  
**v1.8A-4 fix:** Previously returned `SELF_LOOP`. Now correctly: `loopback` + `is_gateway_listener_target: false` = safe.

---

## Case 2: Private Target — 10.0.0.5:3002 ✅ Safe

```bash
aegis safety check-route rt_4c5efff0f66c18ca
```

```json
{
  "route_id": "rt_4c5efff0f66c18ca",
  "domain": "lb-private.smoke.test",
  "target_host": "10.0.0.5",
  "target_port": 3002,
  "ip_classification": "private",
  "is_current_node_address": false,
  "is_gateway_listener_target": false,
  "has_gateway_link": false,
  "risks": null
}
```

**Risks:** None  
**Verdict:** ✅ Safe — private network target.

---

## Case 3: Public Target with GatewayLink — 43.159.34.11:80 ✅ Safe

```bash
aegis safety check-route rt_faee40f4ced69486
```

```json
{
  "route_id": "rt_faee40f4ced69486",
  "domain": "lb-public-gw.smoke.test",
  "target_host": "43.159.34.11",
  "target_port": 80,
  "ip_classification": "public",
  "is_current_node_address": false,
  "is_gateway_listener_target": false,
  "has_gateway_link": true,
  "gateway_link_id": "gwlink_smoke",
  "risks": null
}
```

**Risks:** None  
**Verdict:** ✅ Safe — public target with GatewayLink authenticates the downstream.

---

## Case 4: Public Target without GatewayLink — 43.159.34.11:80 ⚠ Warning

```bash
aegis safety check-route rt_7b41237d335eccd6
```

```json
{
  "route_id": "rt_7b41237d335eccd6",
  "domain": "lb-public-nogw.smoke.test",
  "target_host": "43.159.34.11",
  "target_port": 80,
  "ip_classification": "public",
  "is_current_node_address": false,
  "is_gateway_listener_target": false,
  "has_gateway_link": false,
  "gateway_link_required": true,
  "risks": [
    {"code": "PUBLIC_TARGET_EGRESS", "severity": "warning",
     "message": "route lb-public-nogw.smoke.test targets public IP 43.159.34.11"},
    {"code": "GATEWAY_LINK_BYPASS_RISK", "severity": "warning",
     "message": "route lb-public-nogw.smoke.test targets public IP 43.159.34.11 without Gateway Link"}
  ],
  "recommendation": "attach a Gateway Link to authenticate this route"
}
```

**Risks:** `PUBLIC_TARGET_EGRESS` + `GATEWAY_LINK_BYPASS_RISK`  
**Verdict:** ⚠ Warning — route works but bypasses Gateway Link authentication.

---

## Case 5: Loopback Listener Port — 127.0.0.1:80 ✗ SELF_LOOP

```bash
aegis safety check-route rt_selflistener
```

```json
{
  "route_id": "rt_selflistener",
  "domain": "lb-selflistener.smoke.test",
  "target_host": "127.0.0.1",
  "target_port": 80,
  "ip_classification": "loopback",
  "is_current_node_address": true,
  "is_gateway_listener_target": true,
  "has_gateway_link": false,
  "risks": [
    {"code": "SELF_LOOP", "severity": "error",
     "message": "route target 127.0.0.1:80 matches a gateway listener — would cause self-loop"}
  ]
}
```

**Risks:** `SELF_LOOP`  
**Verdict:** ✗ Error — routing to a gateway listener port would create a self-loop.  
**v1.8A-4 key fix:** Only ports that are registered gateway listeners (80, 443, 8443) trigger SELF_LOOP.

---

## Summary

| # | Case | Target | Classification | Listener Target | Risks | Verdict |
|---|------|--------|---------------|----------------|-------|---------|
| 1 | Loopback non-listener | 127.0.0.1:3001 | loopback | false | none | ✅ Safe |
| 2 | Private | 10.0.0.5:3002 | private | false | none | ✅ Safe |
| 3 | Public + GatewayLink | 43.159.34.11:80 | public | false | none | ✅ Safe |
| 4 | Public, no GWLink | 43.159.34.11:80 | public | false | PUBLIC_TARGET_EGRESS, GATEWAY_LINK_BYPASS_RISK | ⚠ Warning |
| 5 | Loopback listener port | 127.0.0.1:80 | loopback | **true** | SELF_LOOP | ✗ Error |

## Semantic Rules (v1.8A-4)

| Condition | Classification | Self-Loop? | Example |
|-----------|---------------|------------|---------|
| loopback + non-listener port | loopback | No | 127.0.0.1:3001 |
| loopback + listener port | loopback | **Yes** | 127.0.0.1:80 |
| private + non-node IP | private | No | 10.0.0.5:3002 |
| node IP + non-listener port | as classified | No | node_private:3005 |
| node IP + listener port | as classified | **Yes** | node_pub:80 |
| public + GatewayLink | public | No | 43.159.34.11:80 |
| public, no GatewayLink | public | No | 43.159.34.11:80 |
