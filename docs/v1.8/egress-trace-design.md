# Egress Trace Design — v1.8A

## Concept

Trace where a domain's traffic actually goes when leaving the Aegis gateway.
Not interception — detection and diagnosis only.

## API

```
GET /api/admin/v1/trace/egress?domain=<domain>&from_node=<node_id>
```

### Response

```json
{
  "domain": "example.com",
  "resolved_ips": ["93.184.216.34"],
  "ip_classification": "public",
  "is_aegis_managed_domain": false,
  "matched_route_id": "",
  "gateway_node": "node_VM-0-4-ubuntu",
  "current_node": "node_VM-0-4-ubuntu",
  "target_host": "",
  "target_port": 0,
  "internal_target_available": false,
  "gateway_link_required": false,
  "risks": [
    {"code": "PUBLIC_BOUNCE", "severity": "warning",
     "message": "domain resolves to public IP, no route configured"}
  ],
  "recommendation": "bind this domain to an internal target using bind-http-domain"
}
```

### CLI

```
aegis trace egress <domain> [--from-node <node_id>]
```

## Data Source

| Field | Source |
|-------|--------|
| resolved_ips | `net.LookupHost()` |
| ip_classification | `net.ParseIP()` + private range check |
| is_aegis_managed_domain | `manageddomain.Repo.FindByDomain()` |
| matched_route_id | `route.Repo.FindByDomain()` |
| gateway_node | `node.Repo.FindCurrent()` |
| target_host/port | route's resolved endpoint address |
| internal_target_available | endpoint resolver check |

## Risk Codes

| Code | Severity | Meaning |
|------|----------|---------|
| `PUBLIC_BOUNCE` | warning | Domain resolves to public IP, no Aegis route exists. Traffic leaves the gateway uncontrolled. |
| `SELF_LOOP` | error | Domain resolves to this gateway's own IP. Would cause infinite loop. |
| `MANAGED_DOMAIN_EGRESS` | info | Domain is registered as managed domain; egress is expected. |
| `INTERNAL_TARGET_AVAILABLE` | info | Route points to internal IP; no egress issue. |
| `GATEWAY_LINK_BYPASS_RISK` | warning | Route points to external IP without Gateway Link auth. |
| `DOMAIN_RESOLVES_TO_SELF` | error | DNS resolves to this gateway. |
| `UNKNOWN_DOMAIN` | info | No route or managed domain found. |

## Static Detection (No DNS)

Some checks don't need DNS:

- Route exists with internal target → no egress
- Route exists with external target → check GatewayLink
- Route exists with GatewayLink → expected egress
- No route → check managed domains
- No route, no managed domain → UNKNOWN_DOMAIN

## Boundary

v1.8A does NOT:
- Intercept or block traffic
- Modify DNS resolution
- Inject iptables/nftables rules
- Deploy eBPF programs
- Proxy system traffic
