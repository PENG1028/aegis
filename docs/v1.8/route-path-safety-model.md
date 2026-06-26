# Route Path Safety Model — v1.8A

## Concept

Model the safety attributes of each route. Detection only — no enforcement in v1.8A.

## Route Safety Fields

```go
type RouteSafety struct {
    // Target classification
    target_host         string  // resolved endpoint address
    target_classification string // "loopback" | "private" | "gateway_link" | "public_unauthenticated"
    ip_classification   string  // "public" | "private" | "loopback" | "self"
    
    // Gateway link
    gateway_link_id     string
    has_gateway_link    bool
    gateway_link_required bool // detected: target is external and no link
    
    // Internal access policy (detected, not enforced)
    internal_target_available bool // same-server private IP reachable
    
    // Detected risks
    risks               []Risk
    safety_score        string // "safe" | "warning" | "risk"
}
```

## Classification Rules

| Condition | Classification | Safety |
|-----------|---------------|--------|
| target_host is 127.0.0.1 | loopback | safe |
| target_host is private IP (10.x, 172.16.x, 192.168.x) | private | safe |
| target_host is self (gateway's own IP) | self | risk (SELF_LOOP) |
| target_host is public IP with gateway_link_id | gateway_link | safe (authenticated) |
| target_host is public IP without gateway_link_id | public_unauthenticated | warning (PUBLIC_BOUNCE or GATEWAY_LINK_BYPASS_RISK) |

## Risk Detection (No DNS)

| Risk | Detection Method | When |
|------|-----------------|------|
| SELF_LOOP | Compare target_host with node's own public/private IPs | On trace egress, on route safety check |
| PUBLIC_BOUNCE | target is public IP, no route, no managed domain | On trace egress |
| GATEWAY_LINK_BYPASS | target is public IP, route exists, no gateway_link_id | On route safety check |
| INTERNAL_TARGET_AVAILABLE | Suggest private IP alternative for public target | On route safety check |

## Integration

Route safety is embedded into:
- `GET /api/admin/v1/routes/{id}/safety` — check single route
- `GET /api/admin/v1/routes/safety` — check all routes
- `GET /api/admin/v1/trace/egress` — check domain egress path

## Future (v1.8B / v2)

- `internal_access_policy` field on route model
- `gateway_link_required` toggle
- Enforced at apply/render time (warn if unsafe)
- Block on apply if policy violated
