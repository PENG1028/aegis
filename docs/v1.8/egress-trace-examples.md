# Egress Trace Examples — v1.8A

> 6 real-world egress trace scenarios demonstrating route safety and path diagnosis.
> All examples use the CLI command `aegis safety trace-egress <domain>`.
> Updated v1.8A-4: loopback non-listener ports are safe (no SELF_LOOP).

---

## Example 1: Loopback Target (Safe)

**Scenario:** A route targets `127.0.0.1:8080` (local reverse proxy).

```bash
aegis safety trace-egress loopback.internal
```

```json
{
  "domain": "loopback.internal",
  "resolved_ips": [],
  "ip_classification": "loopback",
  "is_aegis_managed_domain": false,
  "matched_route_id": "rt_a1b2c3",
  "current_node": "nd_self",
  "target_host": "127.0.0.1",
  "target_port": 8080,
  "has_gateway_link": false,
  "risks": []
}
```

**Risks:** None  
**Recommendation:** None  
**Verdict:** ✅ Safe — local-only route.

---

## Example 2: Private Target (Safe)

**Scenario:** A route targets `10.0.0.5:3000` (internal service, no GatewayLink needed).

```bash
aegis safety trace-egress api.internal
```

```json
{
  "domain": "api.internal",
  "resolved_ips": [],
  "ip_classification": "private",
  "is_aegis_managed_domain": false,
  "matched_route_id": "rt_d4e5f6",
  "current_node": "nd_self",
  "target_host": "10.0.0.5",
  "target_port": 3000,
  "has_gateway_link": false,
  "risks": []
}
```

**Risks:** None  
**Recommendation:** None  
**Verdict:** ✅ Safe — private network route.

---

## Example 3: Public Target with GatewayLink (Expected External Route)

**Scenario:** Route `app.example.com` targets `<SERVER_B_IP>:80` downstream server B, with GatewayLink attached.

```bash
aegis safety trace-egress app.example.com
```

```json
{
  "domain": "app.example.com",
  "resolved_ips": [],
  "ip_classification": "public",
  "is_aegis_managed_domain": false,
  "matched_route_id": "rt_g7h8i9",
  "current_node": "nd_self",
  "target_host": "<SERVER_B_IP>",
  "target_port": 80,
  "has_gateway_link": true,
  "gateway_link_id": "gwlink_j1k2l3",
  "risks": []
}
```

**Risks:** None (GatewayLink authenticates the downstream)  
**Recommendation:** None  
**Verdict:** ✅ Safe — external route with GatewayLink.

---

## Example 4: Public Target without GatewayLink (Warning)

**Scenario:** Route `exposed.example.com` targets a public IP with no GatewayLink, creating a bypass risk.

```bash
aegis safety trace-egress exposed.example.com
```

```json
{
  "domain": "exposed.example.com",
  "resolved_ips": [],
  "ip_classification": "public",
  "is_aegis_managed_domain": false,
  "matched_route_id": "rt_m4n5o6",
  "current_node": "nd_self",
  "target_host": "<SERVER_B_IP>",
  "target_port": 80,
  "has_gateway_link": false,
  "risks": [
    {
      "code": "PUBLIC_TARGET_EGRESS",
      "severity": "warning",
      "message": "route exposed.example.com targets public IP <SERVER_B_IP>"
    },
    {
      "code": "GATEWAY_LINK_BYPASS_RISK",
      "severity": "warning",
      "message": "route exposed.example.com targets public IP <SERVER_B_IP> without Gateway Link"
    }
  ],
  "gateway_link_required": true,
  "recommendation": "attach a Gateway Link to authenticate this route"
}
```

**Risks:** `PUBLIC_TARGET_EGRESS` + `GATEWAY_LINK_BYPASS_RISK`  
**Recommendation:** Attach a Gateway Link.  
**Verdict:** ⚠ Warning — route works but bypasses Gateway Link authentication.

---

## Example 5: Self Target (Error)

**Scenario:** Route points to the gateway's own IP, creating a routing loop.

```bash
aegis safety trace-egress self.gateway
```

```json
{
  "domain": "self.gateway",
  "resolved_ips": [],
  "ip_classification": "self",
  "is_aegis_managed_domain": false,
  "matched_route_id": "rt_p7q8r9",
  "current_node": "nd_self",
  "target_host": "<SERVER_A_IP>",
  "target_port": 80,
  "has_gateway_link": false,
  "risks": [
    {
      "code": "SELF_LOOP",
      "severity": "error",
      "message": "route target <SERVER_A_IP> is the gateway itself — would cause loop"
    }
  ]
}
```

**Risks:** `SELF_LOOP`  
**Recommendation:** Reconfigure target to a different server.  
**Verdict:** ✗ Error — would create a routing loop.

---

## Example 6: Unknown Domain (Info)

**Scenario:** DNS lookup fails for a domain with no route and no managed domain registration.

```bash
aegis safety trace-egress nonexistent.example.com
```

```json
{
  "domain": "nonexistent.example.com",
  "resolved_ips": [],
  "ip_classification": "",
  "is_aegis_managed_domain": false,
  "matched_route_id": "",
  "current_node": "nd_self",
  "target_host": "",
  "target_port": 0,
  "has_gateway_link": false,
  "risks": [
    {
      "code": "UNKNOWN_DOMAIN",
      "severity": "info",
      "message": "domain nonexistent.example.com does not resolve"
    }
  ],
  "recommendation": "check if the domain is correct or register it as a managed domain"
}
```

**Risks:** `UNKNOWN_DOMAIN`  
**Recommendation:** Register as a managed domain or check spelling.  
**Verdict:** ℹ Info — domain not tracked by Aegis.

---

## Risk Code Summary

| Code | Severity | Scenario | Example |
|------|----------|----------|---------|
| `PUBLIC_DOMAIN_BOUNCE` | warning | No route + DNS resolves to public IP | 6 |
| `PUBLIC_TARGET_EGRESS` | warning | Route targets public IP | 4 |
| `GATEWAY_LINK_BYPASS_RISK` | warning | Public target without GatewayLink | 4 |
| `SELF_LOOP` | error | Route targets gateway itself | 5 |
| `DOMAIN_RESOLVES_TO_SELF` | error | DNS resolves to gateway IP | — |
| `INTERNAL_TARGET_AVAILABLE` | info | Private/loopback target (suggested) | 1, 2 |
| `UNKNOWN_DOMAIN` | info | No route, no managed domain, no DNS | 6 |

## Non-Overlap Guarantee

Each risk code triggers exclusively in its documented scenario:

- `PUBLIC_DOMAIN_BOUNCE` only fires when no route exists (DNS-only check)
- `PUBLIC_TARGET_EGRESS` only fires when a route targets a public IP
- `GATEWAY_LINK_BYPASS_RISK` only fires _in addition to_ PUBLIC_TARGET_EGRESS when GatewayLink is missing
- `SELF_LOOP` only fires when the resolved target IP matches the gateway's node IPs
- `DOMAIN_RESOLVES_TO_SELF` only fires in DNS-only mode when the resolved IP is the gateway
- `INTERNAL_TARGET_AVAIlABLE` is a placeholder for future internal target suggestions
- `UNKNOWN_DOMAIN` only fires when DNS returns no results and no route/managed domain matches
