# v1.8B-2 — Relay API / CLI Smoke

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Sub-phase:** v1.8B-2 — Managed Relay Real Acceptance & Auth Tightening
> **Status:** VERIFICATION PLAN

---

## API Auth Tests

### A1: No auth → 401

```bash
curl -v "http://127.0.0.1:8080/api/admin/v1/relay/resolve?domain=test.example.com"
```

**Expected:** `401` (admin session required)

---

### A2: Service API key → 403 (blocked by isSystemRoute)

```bash
curl -v "http://127.0.0.1:8080/api/admin/v1/relay/resolve?domain=test.example.com" \
  -H "Authorization: Bearer <service_api_key>"
```

**Expected:** `403` (service API key blocked for admin routes)

---

### A3: Admin session → 200

```bash
# Login first
curl -v -c cookies.txt -X POST http://127.0.0.1:8080/api/admin/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<admin_password>"}'

# Then resolve
curl -v -b cookies.txt "http://127.0.0.1:8080/api/admin/v1/relay/resolve?domain=test.example.com"
```

**Expected:** `200` with JSON response body

---

## CLI Smoke Tests

### C1: Normal resolve — verbose output

```bash
aegis relay resolve test.local --from-node nd_a
```

**Expected:** Readable output with mode, gateway URL, risks, etc.

---

### C2: JSON output

```bash
aegis relay resolve test.local --from-node nd_a --json
```

**Expected:** Valid JSON with all relay result fields.

---

### C3: Unknown domain → external_passthrough

```bash
aegis relay resolve nonexistent.example.com
```

**Expected:**
```
Managed:     false
Mode:        external_passthrough
Direct Target Suppressed: false
```

---

### C4: Missing from-node → defaults to "self"

```bash
aegis relay resolve test.local
```

**Expected:** Resolves using "self" as from_node. Returns local_gateway if the target is on the current node.

---

### C5: Unavailable domain

```bash
aegis relay resolve test-nogw.test --from-node nd_b
```

**Expected:**
```
Mode:   unavailable
Error:  GatewayLink required for ...
Direct Target Suppressed: true
```

---

## Scope Boundary

### Scope leakage prevention

| Context | Can see final_local_target? | Notes |
|---------|----------------------------|-------|
| Admin API (`/api/admin/v1/relay/resolve`) | ✅ Yes | Full administrative view |
| Service API (Bearer token, future) | ❌ No (deferred) | Service API relay resolve endpoint not yet implemented |
| CLI (admin session) | ✅ Yes | Admin CLI |

**Note:** Service API relay resolve is **deferred**. Currently only admin API and CLI exist. When Service API is added, it must not expose `final_local_target` to non-admin scopes.

---

## Log Verification

### L1: Relay logs do not contain raw token

After running a relay smoke test, check logs:

```bash
aegis logs --type=node-events | grep relay
```

**Verify:**
- [ ] No raw token value visible in log entries
- [ ] Log entries contain `relay_success`, `relay_rejected`, `relay_forward`, `relay_failed` event types
- [ ] Log entries do NOT contain `auth_value`, `gateway_token`, or similar sensitive fields

---

## Results Table

| # | Scenario | Expected | Actual | Status |
|---|----------|----------|--------|--------|
| A1 | No auth | 401 | | ⏳ |
| A2 | Service API key | 403 | | ⏳ |
| A3 | Admin session | 200 | | ⏳ |
| C1 | CLI verbose | readable output | | ⏳ |
| C2 | CLI JSON | valid JSON | | ⏳ |
| C3 | Unknown domain | external_passthrough | | ⏳ |
| C4 | Missing from-node | defaults to self | | ⏳ |
| C5 | Unavailable domain | mode=unavailable | | ⏳ |
| L1 | Logs no raw token | no token in logs | | ⏳ |
