# Public Bounce Detection — v1.8A

## What It Is

A "public bounce" happens when a domain bound to Aegis resolves to a public IP that is NOT behind this Aegis gateway. The traffic leaves the gateway uncontrolled.

Example:
```
app.example.com → route → target: 8.8.8.8:80 (public IP, no Gateway Link)
                  ↑
            Traffic exits gateway to public internet
```

## Detection (Static, No DNS Needed)

For a route with a known target_host:

```
target_host is public IP? 
  └─ yes ─→ gateway_link_id set?
              ├─ yes → expected egress (safe, authenticated)
              ├─ no  → PUBLIC_BOUNCE (warning)
```

## API Integration

Egress trace already includes this check:

```json
{
  "risks": [
    {"code": "PUBLIC_BOUNCE", "severity": "warning",
     "message": "route points to public IP 8.8.8.8:80 without Gateway Link authentication"}
  ]
}
```

## Why Static Detection Suffices

Route target_host is known at config time. The Planner has the target address.
No DNS lookup needed. If the target is public and has no Gateway Link, it's a bounce.

## Internal Target Recommendation

When a public bounce is detected, check if the same port is available on the private network:

```go
func SuggestInternalTarget(publicIP string, port int) (string, bool) {
    // Check if a private IP alternative is known for this target
    // Requires a mapping table or convention
    // v1.8A: detection only, no enforcement
}
```

This is a lookup against known infrastructure, not a real-time network scan.
