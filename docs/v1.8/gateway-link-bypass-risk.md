# Gateway Link Bypass Risk — v1.8A

## What It Is

A Gateway Link bypass occurs when a route targets an external server without
using Gateway Link authentication. The traffic leaves the gateway without the
shared-secret header that the downstream expects.

```
Server A (gateway)                    Server B (downstream)
┌─────────────────┐                  ┌──────────────────┐
│ route → target: │──no header_up──▶│ verifier checks   │
│ <SERVER_B_IP>:80 │                  │ X-Aegis-Gateway-* │
│ (no gateway_link)                  │ missing → 401     │
└─────────────────┘                  └──────────────────┘
```

## Detection

```go
func DetectGatewayLinkBypass(route *route.Route, endpoint *endpoint.Endpoint) *Risk {
    host, _, _ := net.SplitHostPort(endpoint.Address)
    ip := net.ParseIP(host)
    if ip == nil {
        return nil // hostname, can't determine
    }
    if isPublicIP(ip) && route.GatewayLinkID == "" {
        return &Risk{
            Code:     "GATEWAY_LINK_BYPASS_RISK",
            Severity: "warning",
            Message:  fmt.Sprintf("route %s targets public IP %s without Gateway Link", route.Domain, host),
        }
    }
    return nil
}
```

## When It Fires

| Route target | Has gateway_link_id? | Risk |
|---|---|---|
| 127.0.0.1:8080 | — | No (loopback) |
| 10.3.0.11:80 | — | No (private IP) |
| <SERVER_B_IP>:80 | yes | No (authenticated) |
| <SERVER_B_IP>:80 | no | **GATEWAY_LINK_BYPASS_RISK** |

## Integration Points

- `GET /api/admin/v1/routes/{id}/safety` — per-route check
- `GET /api/admin/v1/trace/egress` — egress path check
- Planner warning — if route has public target without GatewayLink

## False Positive

A route to a public IP without GatewayLink may be intentional:
- Public API backend (not all external targets need Gateway Link)
- CDN origin pull
- Third-party service

The risk is a **warning**, not an error. Admin must decide.

## Future (v1.8B+)

- `gateway_link_required` policy field on route
- If set and missing → block at apply time
- Auto-suggest Gateway Link attachment
