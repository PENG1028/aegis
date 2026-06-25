# Gateway Link Rotation — v1.7AC

## Flow

```
1. POST /api/admin/v1/gateway-links/{id}/rotate
2. Aegis generates new random token
3. Updates auth_value in trusted_gateways table
4. Increments token_version
5. MarkPending("gateway link secret rotated")
6. Returns new token once

Manual steps:
7. Copy new token to Server B verifier config
8. Server B restart or reload with new token
9. POST /api/admin/v1/system/apply
10. Caddy reloads with new header_up token
11. Old token rejected by Server B, new token accepted
```

## Token Version

The token_version field tracks rotation count. It appears in:
- GatewayLink API response (GET /gateway-links/{id})
- Trace output (token_version)
- NOT exposed in logs

## Behaviour

| State | Server A | Server B | Result |
|-------|----------|----------|--------|
| Before rotate | Token v1 in Caddy | Verifier expects v1 | ✅ 200 |
| Rotate → pre-apply | Token v2 in DB, v1 in Caddy | Verifier expects v1 | ✅ 200 (stale config) |
| Apply | Token v2 in Caddy | Verifier expects v1 | ❌ 403 |
| Update verifier | Token v2 in Caddy | Verifier expects v2 | ✅ 200 |

## Risk: Window Between Rotate and Apply

During rotation, there's a window where Caddy still has the old token:
- Safe: old token still works → no downtime
- Unsafe: if token was compromised, rotate doesn't take effect until apply

## API

```
POST /api/admin/v1/gateway-links/{id}/rotate
→ 200 {"id": "...", "secret": "...", "warning": "..."}
→ 404 "gateway link not found"
```

## Tests

- rotate increments token_version
- rotate marks pending_apply
- apply clears pending_apply
- trace shows new token_version
- old token not returned by API
- raw token not logged
