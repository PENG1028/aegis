# Route Safety Real Smoke — v1.8A-3

> 5 real-world route safety smoke tests executed against Aegis v1.8A-2 data.
> Each case includes CLI output and analysis.

---

## Test Environment

| Property | Value |
|----------|-------|
| Node ID | `node_PENGSPC` |
| Node IPs | 127.0.0.1 (local), 192.168.3.2 (private) |
| Gateway Link | `gwlink_smoke` → Server B (<SERVER_B_IP>:80) |

---

## Case 1: Loopback Target (127.0.0.1)

**Route:** `lb-loopback.smoke.test` → service `lb-loopback` → endpoint `127.0.0.1:3001`

```bash
aegis safety check-route rt_3fb75b7e5c8585fe
```

```json
{
  "route_id": "rt_3fb75b7e5c8585fe",
  "domain": "lb-loopback.smoke.test",
  "target_host": "127.0.0.1",
  "target_port": 3001,
  "ip_classification": "self",
  "has_gateway_link": false,
  "risks": [
    {
      "code": "SELF_LOOP",
      "severity": "error",
      "message": "route target 127.0.0.1 is the gateway itself — would cause loop"
    }
  ]
}
```

**Result:** `SELF_LOOP` (error)
**Explanation:** The node has `local_ip=127.0.0.1` registered, so `127.0.0.1` is classified as `self` (checked before `loopback` in ClassifyIP). Routing to the gateway's own IP would cause a loop — the error is correct.
**v1.8A-4 UPDATE:** This test was run before the listener-aware self-loop fix. See [route-safety-real-smoke-v2.md](route-safety-real-smoke-v2.md) for corrected results.  
With the v1.8A-4 fix, `127.0.0.1:3001` is now correctly classified as `loopback` + `is_gateway_listener_target: false` = **safe**, with no `SELF_LOOP` risk.

**Note:** A true loopback-only address (e.g., `127.0.0.2` not in node IPs) would be classified as `loopback` and show zero risks.

---

## Case 2: Private Target (10.0.0.5)

**Route:** `lb-private.smoke.test` → service `lb-private` → endpoint `10.0.0.5:3002`

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
  "has_gateway_link": false,
  "risks": null
}
```

**Result:** ✅ Safe — no risks detected
**Explanation:** Private IP targets are internal and don't require GatewayLink authentication.

---

## Case 3: Public Target WITH GatewayLink

**Route:** `lb-public-gw.smoke.test` → service `lb-public-gw` → endpoint `<SERVER_B_IP>:80`
**GatewayLink:** `gwlink_smoke` (active)

```bash
aegis safety check-route rt_faee40f4ced69486
```

```json
{
  "route_id": "rt_faee40f4ced69486",
  "domain": "lb-public-gw.smoke.test",
  "target_host": "<SERVER_B_IP>",
  "target_port": 80,
  "ip_classification": "public",
  "has_gateway_link": true,
  "gateway_link_id": "gwlink_smoke",
  "risks": null
}
```

**Result:** ✅ Safe — no risks detected
**Explanation:** Public target with GatewayLink is properly authenticated. No `GATEWAY_LINK_BYPASS_RISK`.

---

## Case 4: Public Target WITHOUT GatewayLink

**Route:** `lb-public-nogw.smoke.test` → service `lb-public-nogw` → endpoint `<SERVER_B_IP>:80`
**GatewayLink:** none

```bash
aegis safety check-route rt_7b41237d335eccd6
```

```json
{
  "route_id": "rt_7b41237d335eccd6",
  "domain": "lb-public-nogw.smoke.test",
  "target_host": "<SERVER_B_IP>",
  "target_port": 80,
  "ip_classification": "public",
  "has_gateway_link": false,
  "gateway_link_required": true,
  "risks": [
    {
      "code": "PUBLIC_TARGET_EGRESS",
      "severity": "warning",
      "message": "route lb-public-nogw.smoke.test targets public IP <SERVER_B_IP>"
    },
    {
      "code": "GATEWAY_LINK_BYPASS_RISK",
      "severity": "warning",
      "message": "route lb-public-nogw.smoke.test targets public IP <SERVER_B_IP> without Gateway Link"
    }
  ],
  "recommendation": "attach a Gateway Link to authenticate this route"
}
```

**Result:** ⚠ 2 warnings — `PUBLIC_TARGET_EGRESS` + `GATEWAY_LINK_BYPASS_RISK`
**Explanation:** Public target with no GatewayLink creates both warnings. Route still works — detection only, no block.

---

## Case 5: Self Target (127.0.0.1 — Same as Node)

**Route:** `lb-self.smoke.test` → service `lb-self` → endpoint `127.0.0.1:3005`

```bash
aegis safety check-route rt_06d68dcaa77ffffe
```

```json
{
  "route_id": "rt_06d68dcaa77ffffe",
  "domain": "lb-self.smoke.test",
  "target_host": "127.0.0.1",
  "target_port": 3005,
  "ip_classification": "self",
  "has_gateway_link": false,
  "risks": [
    {
      "code": "SELF_LOOP",
      "severity": "error",
      "message": "route target 127.0.0.1 is the gateway itself — would cause loop"
    }
  ]
}
```

**Result:** `SELF_LOOP` (error) — would create a routing loop
**Explanation:** Gateway routing to itself would create an infinite loop. Correctly detected.

---

## Summary

| # | Case | Target | Risks | Verdict |
|---|------|--------|-------|---------|
| 1 | Loopback (same as node) | 127.0.0.1:3001 | SELF_LOOP | ✗ Error |
| 2 | Private | 10.0.0.5:3002 | none | ✅ Safe |
| 3 | Public + GatewayLink | <SERVER_B_IP>:80 | none | ✅ Safe |
| 4 | Public, no GatewayLink | <SERVER_B_IP>:80 | PUBLIC_TARGET_EGRESS, GATEWAY_LINK_BYPASS_RISK | ⚠ Warning |
| 5 | Self | 127.0.0.1:3005 | SELF_LOOP | ✗ Error |

**Detection only.** No routes were blocked by safety warnings.
