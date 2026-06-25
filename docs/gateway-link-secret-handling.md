# Gateway Link Secret Handling — v1.7AC

## Token Flow

```
Create GatewayLink
  → generate random token
  → store token in DB as raw value (risk: not hashed)
  → return token to caller once
  → token NOT returned in list/get

Planner/Caddy render
  → read token from DB
  → inject into Caddyfile as header_up value
  → token present in rendered config (on disk)
  → token present in config backup

No other path exposes token:
  ❌ Not in apply logs
  ❌ Not in operation logs
  ❌ Not in audit logs
  ❌ Not in trace
  ❌ Not in list/get API
```

## Risk Assessment

| Location | Token Present | Risk | Mitigation |
|----------|:---:|------|------------|
| SQLite DB | ✅ Raw token | Medium | DB file permissions |
| Caddyfile (rendered) | ✅ Raw token | Medium | Caddyfile permissions |
| Config backup | ✅ Raw token | Medium | Backup directory permissions |
| API create response | ✅ Once | Low | Single return + warning |
| API list/get | ❌ | — | — |
| Apply logs | ❌ | — | — |
| Operation logs | ❌ | — | — |
| Audit logs | ❌ | — | — |
| Trace output | ❌ | — | — |
| Provider diagnose | ❌ | — | — |

## Current Gap: Token Storage

Token is stored in the `trusted_gateways.auth_value` column as **raw plaintext**, not hashed.
This means anyone with DB read access can see the token.

**Recommended fix (deferred to v1.8):**
1. Store HMAC-SHA256 hash of token instead of plaintext
2. Caddy renderer cannot use hashed token directly → need Aegis-side proxy or Caddy module
3. Or use HMAC signing where render-time computation doesn't need raw token

## Current Mitigations

- `trusted_gateways` table is admin-only (no service key access)
- SQLite file permissions should be 600
- Config backup directory should be restricted
