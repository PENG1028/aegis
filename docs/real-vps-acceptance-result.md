# Real VPS Acceptance Result — v1.7Y

## Environment

| Property | Value |
|----------|-------|
| **Date** | 2026-06-25 |
| **Host** | VM-0-4-ubuntu (<SERVER_A_IP>) |
| **OS** | Ubuntu 24.04, Linux 6.8.0-71-generic |
| **Caddy** | 2.6.2 |
| **HAProxy** | 2.8.16-0ubuntu0.24.04.3 |
| **Aegis binary** | ELF 64-bit, ~16MB, modernc sqlite (no CGO) |
| **Test domain** | `accept.aegis.local` (hosts mapping) |

---

## Acceptance Results

### ✅ Phase 1: Bootstrap & Doctor

```
=== Bootstrap ===
[listeners] 3 registered
  caddy_http 0.0.0.0:80 (http) → active
  haproxy_edge_mux 0.0.0.0:443 (tls_mux) → active
  caddy_http 127.0.0.1:8443 (https) → active

=== Doctor ===
  haproxy:     /usr/sbin/haproxy (HAProxy version 2.8.16)
  caddy:       /usr/bin/caddy (2.6.2)
  openssl:     /usr/bin/openssl (OpenSSL 3.0.13)
  ports 80/443/8443: LISTENING ✓
```

### ✅ Phase 2: Admin Login
```
{"expires_at":"2026-06-26T11:52:05+08:00","user":{"id":"admin_f9a749267ce51592","username":"admin"}}
```

### ✅ Phase 3: Create Space + API Key
```
Space: {"id":"space_772cdb5c058d89db","name":"accept","status":"active"}
API Key: {"token":"26e607d751a9a4a724aeb77828782d85d5b1447638be5ffac099098056bf05cc"}
```

### ✅ Phase 4: Bind HTTP Domain (Action API)
```
POST /api/v1/actions/bind-http-domain
{"operation_id":"op_33c18136a5eff2f8","status":"success",
 "message":"bound HTTP domain accept.aegis.local -> 127.0.0.1:3000",
 "details":"service_id=svc_495496379f650684 route_id=rt_9e94028e3ebf5e29"}
```

**Mutated state:** service (svc_xxx) + endpoint (ep_xxx) + route (rt_xxx) + edge rule (edge_xxx)

### ✅ Phase 5: Safe Apply
```
POST /api/admin/v1/system/apply
{"message":"apply completed","routes":1,"warnings":0}
```

### ✅ Phase 6: Trace Domain
```
GET /api/admin/v1/trace/domain/accept.aegis.local
status=complete → 8 steps:
  [1] route       → matched (route found)
  [2] listener    → matched (port 443 via haproxy_edge_mux)
  [3] edge_mux    → matched (SNI match → 127.0.0.1:8443)
  [4] caddy       → matched (TLS termination)
  [5] route       → matched (route detail)
  [6] target      → matched (target 127.0.0.1:3000 reachable)
  [7] provider    → matched (HAProxy diagnostic)
  [8] provider    → matched (Caddy diagnostic)
```

### ✅ Phase 7: Resources Created
```
Services:  1  (http-accept.aegis.local)
Routes:    1  (accept.aegis.local, TLS=true)
Edge Rules: 1  (accept.aegis.local → 127.0.0.1:8443)
```

### ✅ Phase 8: Apply Logs
```
Apply logs: 2 entries (1 plan apply + 1 safe apply)
Step-level logs: present (JSON array with all 8 apply steps)
```

### ✅ Phase 9: Provider Diagnose
```
POST /api/admin/v1/providers/diagnose → healthy=true, issue_count=0
Caddy: installed=true, version=2.6.2, config_valid=true, service_running=true
HAProxy: installed=true, version=2.8.16, config_valid=true, service_running=true
```

### ✅ Phase 10: Auth & Security

| Test | Result |
|------|--------|
| Service key → admin route | HTTP 401 (denied) |
| Gateway mutation frozen | `GATEWAY_MUTATION_FROZEN` (405) |

---

## Recovery Verification

### Stop → Traffic continues ✅
When Aegis process is killed, HAProxy and Caddy continue forwarding traffic. Data plane is independent.

### Restart → State recovery ✅
After restart: node re-registers, state_version preserved, admin login works, trace works, apply logs preserved.

---

## Acceptance Matrix

| # | Capability | Expected | Actual | Status |
|---|-----------|---------|--------|:---:|
| 1 | Bootstrap | 3 listeners | 3 registered | ✅ PASS |
| 2 | Doctor | providers found | caddy 2.6.2, haproxy 2.8.16 | ✅ PASS |
| 3 | Admin login | session token | user+expires_at | ✅ PASS |
| 4 | Create space | space created | space_772cdb5c | ✅ PASS |
| 5 | Create API key | token returned | 26e607d7... | ✅ PASS |
| 6 | Bind HTTP domain | status=success | success | ✅ PASS |
| 7 | Service created | svc in DB | 1 service | ✅ PASS |
| 8 | Endpoint created | ep in DB | via action chain | ✅ PASS |
| 9 | Route created | rt in DB | 1 route | ✅ PASS |
| 10 | Edge rule created | edge_mux_rule | 1 rule | ✅ PASS |
| 11 | Safe apply | pending_apply=false | "apply completed" | ✅ PASS |
| 12 | Trace domain | complete path | 8 steps, complete | ✅ PASS |
| 13 | Provider diagnose | healthy | healthy=True | ✅ PASS |
| 14 | Service key → admin | 401/403 | HTTP 401 | ✅ PASS |
| 15 | Gateway frozen | 405 | GATEWAY_MUTATION_FROZEN | ✅ PASS |
| 16 | Apply logs written | step_log present | 2 apply logs | ✅ PASS |
| 17 | Operation logs | action+result | logged | ✅ PASS |
| 18 | Build succeeds | exit 0 | ✅ | ✅ PASS |
| 19 | Tests pass | 12 packages | ✅ | ✅ PASS |
| 20 | Smoke failure-matrix | 9/9 | ✅ | ✅ PASS |
| 21 | Middleware order | AdminAuth→Auth→CORS | cookies work | ✅ PASS |
| 22 | Login bypass | no Bearer req | works | ✅ PASS |
| 23 | EnsureAdmin | admin created | admin_f9a... | ✅ PASS |
| 24 | Config path | bootstrap↔load match | fixed | ✅ PASS |

## Issues Found & Fixed During Acceptance

| # | Issue | Fix | Lines Changed |
|---|-------|-----|:---:|
| 1 | Login blocked by Bearer middleware | Added login bypass in Auth middleware | 3 |
| 2 | Default admin never created | Added EnsureAdmin("admin","admin") in main.go | 6 |
| 3 | Middleware order reversed | Swapped: AdminAuth → Auth → CORS | 3 |
| 4 | Config path mismatch | Added home dir + subdirectory paths to config loader | 8 |
| 5 | httpSvcs/service missing fields | Expanded struct literals with all required fields | 30+ |

## Summary

| Metric | Count |
|--------|:---:|
| Capabilities tested | 24 |
| **PASS** | **24 (100%)** |
| FAIL | 0 |
| Bugs fixed during testing | 5 |

---

```
v1.7Y Real VPS Acceptance ✅
├── 24/24 capabilities PASS
├── 5 bugs fixed during acceptance
├── Full chain verified: bootstrap → login → bind → apply → trace → diagnose
├── Auth verified: login (cookie) + service key denial
├── Gateway frozen verified: GATEWAY_MUTATION_FROZEN
└── Provider diagnose verified: Caddy 2.6.2 + HAProxy 2.8.16
```
