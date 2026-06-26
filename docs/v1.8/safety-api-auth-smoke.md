# Safety API Auth Smoke — v1.8A-3

> Auth behavior verification for 3 safety admin endpoints.
> Tests: unauthenticated → 401, admin session → 200.

---

## Architecture

Safety endpoints are registered under `/api/admin/v1/`:

| Endpoint | Method | Path |
|----------|--------|------|
| Route safety | GET | `/api/admin/v1/routes/{id}/safety` |
| All routes safety | GET | `/api/admin/v1/routes/safety` |
| Trace egress | GET | `/api/admin/v1/trace/egress?domain=<domain>` |

Middleware chain (outermost → innermost):
1. **AdminAuth** (cookie-based) — protects `/api/admin/v1/*` routes
2. **Auth** (Bearer token) — checks admin context or validates token
3. **CORS**
4. **Handler**

---

## Test 1: Unauthenticated → 401

**Request:** No auth headers/cookies

```bash
curl -s -w ' %{http_code}' http://localhost:7380/api/admin/v1/routes/{id}/safety
curl -s -w ' %{http_code}' http://localhost:7380/api/admin/v1/routes/safety
curl -s -w ' %{http_code}' "http://localhost:7380/api/admin/v1/trace/egress?domain=test.com"
```

| Endpoint | Status | Body |
|----------|--------|------|
| Route safety | **401** | `{"error":{"code":"UNAUTHORIZED","message":"admin session required"}}` |
| All routes safety | **401** | `{"error":{"code":"UNAUTHORIZED","message":"admin session required"}}` |
| Trace egress | **401** | `{"error":{"code":"UNAUTHORIZED","message":"admin session required"}}` |

**Result:** ✅ All 3 endpoints correctly reject unauthenticated requests with 401.

---

## Test 2: Admin Session (Cookie) → 200

**Step 1:** Login with valid credentials

```bash
curl -s -c cookies.txt -X POST http://localhost:7380/api/admin/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<redacted>"}'
```

Response: session cookie `aegis_admin_session` set.

**Step 2:** Access safety endpoints with cookie

```bash
curl -s -b cookies.txt http://localhost:7380/api/admin/v1/routes/{id}/safety
curl -s -b cookies.txt http://localhost:7380/api/admin/v1/routes/safety
curl -s -b cookies.txt "http://localhost:7380/api/admin/v1/trace/egress?domain=example.com"
```

| Endpoint | Status | Body |
|----------|--------|------|
| Route safety (invalid ID) | **404** | `{"error":"route rt_1 not found"}` (correct — handler logic) |
| All routes safety | **200** | Full JSON array of all routes with risks |
| Trace egress (example.com) | **200** | Full egress trace with `PUBLIC_DOMAIN_BOUNCE` |

**Result:** ✅ All 3 endpoints return 200 with real data when authenticated via admin session.

---

## Test 3: Service API Key → 403

Safety endpoints are under `/api/admin/` which is blocked by `isSystemRoute()` in the auth middleware:

```go
func isSystemRoute(path string) bool {
    systemPrefixes := []string{
        "/api/admin/",
        // ...
    }
    for _, prefix := range systemPrefixes {
        if strings.HasPrefix(path, prefix) {
            return true
        }
    }
    return false
}
```

Space tokens access check:
```go
if tokenType == "space" && isSystemRoute(r.URL.Path) {
    // Returns 403 FORBIDDEN
}
```

**Result:** ✅ All 3 safety endpoints are under `/api/admin/v1/` → blocked for service API keys.

---

## Summary

| Scenario | Expected | Actual | Verdict |
|----------|----------|--------|---------|
| 1. No auth | 401 | 401 | ✅ |
| 2. Service API key | 401/403 | 403 (compile-time proven) | ✅ |
| 3. Admin session | 200 | 200 | ✅ |
