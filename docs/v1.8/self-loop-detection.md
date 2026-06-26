# Self-Loop Detection — v1.8A

## What It Is

A self-loop occurs when a route's target points back to the Aegis gateway itself.
This creates an infinite forwarding loop: Caddy → itself → itself → ...

Example:
```
app.example.com → route → target: 43.160.211.232:80 (gateway's own public IP)
                  ↓
            Caddy forwards to itself
                  ↓
            loop until timeout
```

## Detection

Compare target_host against this node's known IPs:

```go
func IsSelfTarget(host string, nodeIPs []string) bool {
    for _, ip := range nodeIPs {
        if ip == host {
            return true
        }
    }
    return false
}
```

Node IP sources:
- `node.Repo.FindCurrent()` → `LocalIP`, `PrivateIP`, `PublicIP`

## Risk

| Severity | Impact |
|----------|--------|
| error | Infinite loop, connection timeout, resource exhaustion |

## API Output

```json
{
  "risks": [
    {"code": "SELF_LOOP", "severity": "error",
     "message": "route target 43.160.211.232:80 is the gateway itself — would cause loop"}
  ]
}
```

## Why Static Detection Suffices

Node IPs are known at registration time. No DNS needed. Compare target_host against the node's own IP list.

## Limitation

- Does NOT detect multi-hop loops (A→B→A)
- Does NOT detect loops through load balancers or NAT
- v1.8A scope: single-hop self-loop only
