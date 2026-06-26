# Self-Loop Detection — v1.8A (updated v1.8A-4)

## What It Is

A self-loop occurs when a route's target points back to a **gateway listener port**
on the Aegis gateway itself. This creates an infinite forwarding loop.

**Not all node-IP targets are self-loops.** Only targets that match a registered
gateway listener port:

```
app.example.com → route → target: 127.0.0.1:80 (gateway listener port)
                  ↓
            Caddy forwards to itself
                  ↓
            loop until timeout
```

Safe loopback/non-listener targets (no self-loop):
```
api.internal → route → target: 127.0.0.1:3001 (service port, not a listener)
                  ↓
            Caddy forwards to local service
                  ↓
            normal proxying — no loop
```

## Detection (v1.8A-4)

Two-step detection:

1. **Is the host a loopback or node IP?** → `ClassifyIP()` returns `loopback`, or
   `IsCurrentNodeAddress()` returns true
2. **Is the port a registered gateway listener?** → `isGatewayListenerPort()` checks:
   - Actual `listeners` table (active/planned records)
   - Fallback: 80, 443, 8443 (standard gateway ports)

Only when BOTH conditions are met → `SELF_LOOP`.

```go
func (s *Service) isGatewayListenerTarget(host string, port int) bool {
    if port <= 0 { return false }
    class := ClassifyIP(host, nodeIPs)
    isNodeAddr := IsCurrentNodeAddress(host, nodeIPs)
    if class != IPLoopback && !isNodeAddr { return false }
    return s.isGatewayListenerPort(port)
}
```

## IP Classification Priority (v1.8A-4)

`ClassifyIP` no longer returns `"self"` as a classification. Priority:

1. invalid / hostname
2. **loopback** — 127.x.x.x (highest, even if also a node IP)
3. private — RFC1918
4. public — global unicast

Use `IsCurrentNodeAddress()` separately to check if an IP belongs to the node.

## Scenarios

| Target | Classification | Is Node Addr? | Listener Port? | Verdict |
|--------|---------------|---------------|----------------|---------|
| 127.0.0.1:3001 | loopback | true | false | ✅ Safe |
| 127.0.0.1:80 | loopback | true | true | ✗ SELF_LOOP |
| node_private:3005 | private | true | false | ✅ Safe |
| node_public:80 | public | true | true | ✗ SELF_LOOP |
| 10.0.0.5:80 | private | false | false | ✅ Safe |

## Risk

| Severity | Impact |
|----------|--------|
| error | Infinite loop, connection timeout, resource exhaustion |

## Listener Port Sources

Priority:
1. `listener.Repository.FindAll()` — active/planned records from DB
2. Fallback: `[80, 443, 8443]` — standard gateway ports

## Limitation

- Does NOT detect multi-hop loops (A→B→A)
- Does NOT detect loops through load balancers or NAT
- v1.8A scope: single-hop self-loop only
