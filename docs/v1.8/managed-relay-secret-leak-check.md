# v1.8B-4 — Managed Relay Secret Leak Check

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Status:** real_deploy_verified ✅ (v1.8B-6)
> **Date:** 2026-06-26

---

## Checks

### L1: Relay request logs do not contain raw GatewayLink token

**Code:** `handler.go` — `logRelayEvent()`
```go
msg := fmt.Sprintf("relay %s: route=%s source=%s gateway=%s detail=%s",
    eventType, routeID, sourceNode, gatewayID, detail)
```

The raw token is never passed to `logRelayEvent`. The log message includes:
- event type (relay_success, relay_rejected, etc.)
- route_id
- source_node_id
- gateway_id (the link ID, not the token value)
- detail string (status codes, reason, etc.)

**The raw `gatewayToken` variable is only used in `CheckAuth()` and then discarded.**

**Status:** ✅ PASS — no raw token in logs

---

### L2: Relay error responses do not contain raw token

**Proof:** Examine all `writeRelayError` calls in `handler.go`:

| Error | Response | Token leaked? |
|-------|----------|--------------|
| METHOD_NOT_ALLOWED | static message | ❌ No |
| MISSING_ROUTE_ID | static message | ❌ No |
| MISSING_GATEWAY_ID | static message | ❌ No |
| MISSING_GATEWAY_TOKEN | static message | ❌ No |
| MISSING_SOURCE_NODE | static message | ❌ No |
| MAX_HOPS_EXCEEDED | includes hop count, not token | ❌ No |
| ROUTE_NOT_FOUND | includes route_id, not token | ❌ No |
| INVALID_GATEWAY | includes gateway_id, not token | ❌ No |
| INVALID_GATEWAY_TOKEN | static message | ❌ No |
| UNKNOWN_SOURCE_NODE | includes source_node_id, not token | ❌ No |
| NODE_IDENTITY_ERROR | static message | ❌ No |
| NO_ENDPOINTS | includes service_id, not token | ❌ No |
| ENDPOINT_NODE_UNKNOWN | includes endpoint_id, not token | ❌ No |
| ENDPOINT_NOT_LOCAL | includes route_id/endpoint_id, not token | ❌ No |
| TARGET_HEADER_REJECTED | static message | ❌ No |
| TARGET_UNREACHABLE | includes local target address, not token | ❌ No |
| PROXY_ERROR | generic error | ❌ No |

**All error responses use static messages or non-sensitive identifiers. No raw token in any error path.**

**Status:** ✅ PASS — no raw token in errors

---

### L3: Resolver unavailable response does not contain final_local_target

**Code:** `resolver.go` — `unavailable()`:
```go
func unavailable(res *RelayResult, err, detail string) *RelayResult {
    res.Managed = true
    res.Mode = string(ModeUnavailable)
    res.DirectTargetSuppressed = true
    res.Error = err
    res.ErrorDetail = detail
    // FinalLocalTarget and GatewayURL are NOT set — remain ""
    return res
}
```

**Proof:** Unit test `TestUnavailableNoFinalLocalTargetLeak`:
```go
res := r.ResolveManagedRelay("test.nogw", "nd_a")
if res.FinalLocalTarget != "" {
    t.Errorf("unavailable must not leak final_local_target, got %s", res.FinalLocalTarget)
}
if res.GatewayURL != "" {
    t.Errorf("unavailable must not leak gateway_url, got %s", res.GatewayURL)
}
```

**Status:** ✅ PASS — no final_local_target or gateway_url in unavailable

---

### L4: External passthrough response does not contain internal IDs

**Code:** `resolver.go` — `externalPassthrough()`:
```go
func externalPassthrough(res *RelayResult, domain, reason string) *RelayResult {
    res.Managed = false
    res.Mode = string(ModeExternalPassthrough)
    res.DirectTargetSuppressed = false
    res.Recommendation = "domain is not managed by Aegis — relay not available"
    res.AddRisk("UNKNOWN_DOMAIN", "info", reason)
    return res
}
```

RouteID, ServiceID, EndpointID, FinalLocalTarget, and GatewayURL are all left at their zero values.

**Proof:** Unit test `TestExternalPassthroughNoInternalTargetLeak`:
```go
if res.FinalLocalTarget != "" { t.Error(...) }
if res.GatewayURL != "" { t.Error(...) }
if res.EndpointID != "" { t.Error(...) }
if res.RouteID != "" { t.Error(...) }
if res.ServiceID != "" { t.Error(...) }
```

**Status:** ✅ PASS — no internal IDs leaked

---

### L5: GatewayLink token stored as HMAC hash in DB

**Verification:** `gateway_link/crypto.go`:
```go
func hashSecret(secret string) string {
    h := hmac.New(sha256.New, []byte("aegis-gateway-link-v1"))
    h.Write([]byte(secret))
    return hex.EncodeToString(h.Sum(nil))
}
```

**DB stores ONLY the HMAC-SHA256 hash**, never the plaintext secret.

**Status:** ✅ PASS — raw token never persisted

---

## v1.8B-5 Update

### L5 upgraded: Token encrypted at rest (AES-256-GCM)

As of v1.8B-5, the GatewayLink token is **encrypted** using AES-256-GCM with a master key, replacing the HMAC-SHA256 hash.

**Changes:**
- `encrypted_secret` column stores AES-256-GCM ciphertext (base64)
- `secret_nonce` column stores GCM nonce (base64)
- `secret_version` column tracks rotation count
- Master key loaded from `AEGIS_SECRET_KEY` env var or `/etc/aegis/secret.key`
- Legacy HMAC hash preserved in `auth_value` for backward compatibility

**Verification:**
- Encrypt/Decrypt roundtrip tested (15 secrets tests)
- Wrong key → GCM auth fails → clear error
- Nonce uniqueness verified (100 iterations)
- Tampered ciphertext/nonce → GCM auth fails

**Added L6: Service List/Get do not expose raw token**

- `Service.List()` clears `AuthValue`, `EncryptedSecret`, `SecretNonce`, `SecretVersion`
- `Service.Get()` returns `secret_version` and `has_encrypted_secret` but never raw token
- Raw token only returned once at Create/Rotate time via HTTP response
- All encrypted fields have `json:"-"` tag in struct definition

## Summary

| Check | Mechanism | Status |
|-------|-----------|--------|
| L1: Logs no raw token | `logRelayEvent` never receives raw token | ✅ |
| L2: Errors no raw token | All error responses use static messages or IDs | ✅ |
| L3: Unavailable no leak | `unavailable()` clears FinalLocalTarget/GatewayURL | ✅ |
| L4: External passthrough no leak | Only domain and UNKNOWN_DOMAIN risk returned | ✅ |
| L5: Token encrypted at rest | AES-256-GCM with master key (v1.8B-5) | ✅ |
| L6: List/Get no raw token | All secret fields cleared or JSON-tagged "-" | ✅ |

**No secret leakage found.**
