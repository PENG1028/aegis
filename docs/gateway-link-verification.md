# Gateway Link Verification — v1.7AC

## Security Mode: Static Shared Token

Current v1.7AC uses **static shared token mode**.

| Mode | Status | Scope |
|------|--------|-------|
| Static shared token | ✅ Supported | Personal use, trusted A→B link |
| HMAC dynamic signing | ⏳ Deferred | Requires Caddy dynamic HMAC support |

## Headers

```
X-Aegis-Gateway-Link:  <link_id>          # identifies the gateway link
X-Aegis-Gateway-Token: <static_token>     # pre-shared secret (not a signature!)
```

## Verification Flow

```
Server A (upstream)
  Caddy reverse_proxy {
      header_up X-Aegis-Gateway-Link  "gw_abc123"
      header_up X-Aegis-Gateway-Token "abcd1234..."
  }
        │
        │ HTTP/HTTPS
        ▼
Server B (downstream)
  Verifier/App checks:
  1. X-Aegis-Gateway-Link exists and matches expected link_id
  2. X-Aegis-Gateway-Token matches expected token
  3. Both correct → 200 OK
  4. Missing header → 401
  5. Wrong token → 403
```

## Security Boundary

| Property | Value |
|----------|-------|
| Token storage in DB | ❌ Raw token (risk documented) |
| Token in Caddy config | ✅ Yes (rendered in header_up) |
| Token in config backup | ✅ Yes |
| Token in apply logs | ❌ Blocked |
| Token in operation logs | ❌ Blocked |
| Token in trace | ❌ Not included |
| Token in API list/get | ❌ Not returned (create/rotate only) |
| Suitable for public multi-tenant | ❌ No |

## HMAC Dynamic Signing

Deferred. Requirements for implementation:
1. Caddy can compute HMAC at request time (via caddy-ext or custom module)
2. Or Aegis generates short-lived tokens and injects them via render-time computation
3. Current Caddy 2.6.x does not support dynamic HMAC in `header_up` without custom module

## Verifier Example

See `examples/gateway-link-verifier/` for a reference implementation.
