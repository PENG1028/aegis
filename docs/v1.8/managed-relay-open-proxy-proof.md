# v1.8B-4 — Managed Relay Open Proxy Proof

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Status:** VERIFIED ✅
> **Date:** 2026-06-26

---

## Proof Points

### 1. Client cannot specify arbitrary target_host via header

**Rule enforced:** `X-Aegis-Target-Host` header is rejected.

**Proof:** Test N4 in negative smoke:
```json
{"error":"TARGET_HEADER_REJECTED","message":"X-Aegis-Target-Host and X-Aegis-Target-Port headers are not allowed"}
```
HTTP 400 — request rejected before any forwarding occurs.

**Code:** `handler.go`:
```go
if r.Header.Get("X-Aegis-Target-Host") != "" || r.Header.Get("X-Aegis-Target-Port") != "" {
    writeRelayError(w, http.StatusBadRequest, "TARGET_HEADER_REJECTED", ...)
    return
}
```

---

### 2. Client cannot specify arbitrary target_port via header

**Rule enforced:** `X-Aegis-Target-Port` header is rejected.

**Proof:** Test N5 in negative smoke — same `TARGET_HEADER_REJECTED` error returned.

---

### 3. RelayHandler only uses DB route/endpoint for target

**Rule enforced:** Target is derived from `endpoint.address` (DB), never from request headers.

**Proof chain:**
```
Request → X-Aegis-Route-ID → routeRepo.FindByID()
       → FindEnabledByServiceID(route.ServiceID)
       → endpoint.HostPort()  ← ONLY from DB field endpoint.address
       → "127.0.0.1:<port>"  ← hardcoded to localhost
```

**No request header influences the target host or port.** The only headers used for routing are:
- `X-Aegis-Route-ID` — identifies which DB route to use
- `Host` — not used for target (only for X-Forwarded-Host)

---

### 4. Endpoint must belong to current node

**Rule enforced:** `endpoint.node_id` must be non-empty and match `currentNode.NodeID`.

**Proof:** Tests N8 (empty → 409) and N9 (mismatch → 409) in negative smoke.

**Code:** `handler.go`:
```go
if eps[i].NodeID != "" && (eps[i].NodeID == currentNode.NodeID || eps[i].NodeID == currentNode.ID) {
    localEP = &eps[i]
    break
}
// ...
if localEP == nil {
    // Check for empty node_id
    for i := range eps {
        if eps[i].NodeID == "" {
            writeRelayError(w, http.StatusConflict, "ENDPOINT_NODE_UNKNOWN", ...)
            return
        }
    }
    writeRelayError(w, http.StatusConflict, "ENDPOINT_NOT_LOCAL", ...)
    return
}
```

---

### 5. Relay does not forward to non-127.0.0.1 targets

**Rule enforced:** After extracting `targetHost, targetPort = localEP.HostPort()`, if `targetHost` is not `127.0.0.1` or `localhost`, the endpoint's `node_id` is verified to match the current node. If it doesn't match, the request is rejected.

**Code:** `handler.go`:
```go
if targetHost != "127.0.0.1" && targetHost != "localhost" {
    if localEP.NodeID != currentNode.NodeID && localEP.NodeID != currentNode.ID {
        writeRelayError(w, http.StatusConflict, "ENDPOINT_NOT_LOCAL", ...)
        return
    }
}
```

**Forwarding target is hardcoded:**
```go
targetAddr := fmt.Sprintf("127.0.0.1:%d", targetPort)
```

No mechanism exists in the code to forward to a non-localhost address.

---

### 6. Route not found does not fallback

**Rule enforced:** When a route is not found, relay returns 404. No fallback to `Host` header, no DNS resolution, no default target.

**Proof:** Test N7 in negative smoke:
```json
{"error":"ROUTE_NOT_FOUND","message":"route rt_nonexistent not found"}
```
HTTP 404 — no attempt to forward.

---

### 7. Unavailable does not fallback to direct remote target

**Rule enforced:** The resolver returns `mode=unavailable` with no `gateway_url` or `final_local_target`. The handler never receives the request because the resolver prevents the call.

**Proof:** Unit tests `TestUnavailableNoFinalLocalTargetLeak` and `TestResolveNoFallbackToDirectTarget` confirm that:
- `final_local_target` is empty when mode=unavailable
- `gateway_url` is empty when mode=unavailable
- No fallback path exists

---

## Summary

| # | Protection | Mechanism | Status |
|---|-----------|-----------|--------|
| 1 | Client cannot specify target_host | Header rejected (400) | ✅ Verifed |
| 2 | Client cannot specify target_port | Header rejected (400) | ✅ Verifed |
| 3 | Target from DB only | endpoint.Address → HostPort() | ✅ Verifed |
| 4 | endpoint.node_id must match current node | node_id check (409) | ✅ Verifed |
| 5 | Forward only to 127.0.0.1 | Hardcoded targetAddr | ✅ Verifed |
| 6 | Route not found — no fallback | 404 response | ✅ Verifed |
| 7 | Unavailable — no fallback | empty gateway_url/final_local_target | ✅ Verifed |

**Conclusion:** Relay is NOT an open proxy. All 7 protections are verified through real smoke tests or code review.
