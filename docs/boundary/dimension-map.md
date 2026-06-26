# Aegis Dimension Map — v1.7AA

## Overview

This document maps all current boundary dimensions, their capabilities, and inter-dependencies.

---

## 1. Identity & Auth Dimension

### Capabilities
| Capability | Status | Details |
|-----------|--------|---------|
| Admin cookie auth | ✅ real | Single admin user, HttpOnly cookie session |
| Admin auto-creation | ✅ real | EnsureAdmin("admin","admin") on startup |
| API key (Bearer token) | ✅ real | Scope-based, SHA-256 hashed storage |
| Scope isolation | ✅ real | Space-scoped keys only see own resources |
| Ownership enforcement | ✅ real | requireOwnership() in ActionService |
| Admin → service key blocking | ✅ real | isSystemRoute() covers all admin + CRUD routes |
| Login bypass (no Bearer) | ✅ real | Auth middleware bypasses POST /auth/login |
| Gateway Link auth | 🛠️ implemented | HMAC-SHA256 shared secret between gateways |

### Depends On
- Admin auth → AdminUserRepository, AdminSessionRepository
- API key → TokenRepository, Scope definitions
- Scope → Space service
- Gateway Link → GatewayLinkRepository

---

## 2. API Surface Dimension

### Capabilities
| Capability | Status | Details |
|-----------|--------|---------|
| Action API (service actions) | ✅ real | 5 actions + 4 read-only my/* endpoints |
| Admin CRUD API | ✅ real | 30+ endpoints for routes/services/scopes/etc |
| Trace API | ✅ real | domain/sni/route trace |
| Provider diagnose API | ✅ real | Full 7-code diagnostic |
| Gateway CRUD API (frozen) | ✅ real | Returns 405 GATEWAY_MUTATION_FROZEN |
| Gateway Link API | 🛠️ implemented | Register/List/Rotate/Remove trusted gateways |
| Action API → safe apply | ✅ real | Every action triggers TryApply |
| Admin CRUD → MarkPending | ✅ real | v1.7V fix |

### Depends On
- Action API → ActionService, Apply service
- Admin CRUD → Handler dependencies (all must be non-nil)
- Trace → RouteRepo, EdgeSvc, ListenerSvc, EndpointRepo
- Diagnose → Provider binaries (Caddy/HAProxy)

---

## 3. Data Model Dimension

### Tables
| Table | Purpose | Status |
|-------|---------|--------|
| routes | Domain→service mappings | ✅ |
| services | Backend service definitions | ✅ |
| endpoints | Service endpoint addresses | ✅ |
| edge_mux_rules | TLS SNI routing rules | ✅ |
| listeners | Port listener registry | ✅ |
| managed_domains | Domain verification | ✅ |
| spaces | Scope/tenant isolation | ✅ |
| api_tokens | API keys with scopes | ✅ |
| admin_users | Admin credentials | ✅ |
| admin_sessions | Admin login sessions | ✅ |
| operation_logs | Action audit trail | ✅ |
| apply_logs | Apply step-level logs | ✅ |
| audit_logs | Security events | ✅ |
| node_events | Cluster lifecycle events | ✅ |
| nodes | Node registry + capabilities | ✅ |
| cluster_state | Key-value state (pending_apply, etc.) | ✅ |
| **trusted_gateways** | 🆕 Gateway link auth | 🛠️ implemented |

### Dependencies
- routes → services, endpoints, edge_mux_rules
- api_tokens → spaces
- admin_sessions → admin_users
- All tables → SQLite via modernc.org/sqlite

---

## 4. Provider Dimension

### Capabilities
| Provider | Manage Config | Validate | Reload | Diagnose | In Trace |
|----------|:---:|:---:|:---:|:---:|:---:|
| caddy_http | ✅ | ✅ caddy validate | ✅ systemctl | ✅ 5/7 codes | ✅ |
| haproxy_edge_mux | ✅ | ✅ haproxy -c | ✅ systemctl | ✅ 5/7 codes | ✅ |
| haproxy_tcp | ✅ | ✅ haproxy -c | ✅ systemctl | ✅ 5/7 codes | ❌ |

### Caddy Renderer Features
- Domain + reverse_proxy ✓
- Maintenance mode (503) ✓
- Encode gzip ✓
- **Extra upstream headers (header_up)** 🆕
- Path prefix routing ✓
- TLS configuration (via Caddy auto) ✓

### Dependencies
- Provider → ProxyAdapter interface
- Render → RouteConfig (UpstreamURL, Options)
- Validate → exec.Command (provider binary)
- Reload → systemctl / exec.Command

---

## 5. Apply Pipeline Dimension

### Stages
```
acquire_lock → render_config → provider_validate → 
config_hash_compare → atomic_replace → reload_provider → 
runtime_verify → release_lock
```

| Stage | Success | Failure |
|-------|---------|---------|
| acquire_lock | ✅ | APPLY_LOCKED |
| render_config | ✅ | Config error |
| provider_validate | ✅ | CONFIG_VALIDATE_FAILED + stderr preserved |
| config_hash_compare | ✅ "config unchanged" skip reload | N/A |
| atomic_replace | ✅ | File error |
| reload_provider | ✅ | Backup restore + fallback reload |
| runtime_verify | ✅ Caddy (curl) | RUNTIME_VERIFY_FAILED |
| release_lock | ✅ | N/A |

### Step-level Log Fields
operation_id, state_version, provider, step, status, error_code, error_message, stderr, created_at

### Dependencies
- Apply → Planner, Executor, Adapter, RollbackService
- Planner → RouteRepo, MDRepo, EndpointResolver
- Executor → Config paths, systemctl
- Step logs → LogService.ApplyLogRepository

---

## 6. Trace Dimension

### Trace Types
| Type | Steps | Target Connectivity | Provider Diagnostic |
|------|:---:|:---:|:---:|
| TraceDomain | 8 | ✅ | ✅ HAProxy + Caddy |
| TraceSNI | 5 | ✅ Direct + Caddy | ✅ HAProxy |
| TraceRoute | delegates to TraceDomain | ✅ | Via TraceDomain |

### Error Codes
| Code | Source |
|------|--------|
| TARGET_UNREACHABLE | net.DialTimeout general error |
| TARGET_TIMEOUT | net.Error.Timeout() = true |
| TARGET_DNS_FAILED | net.LookupHost fails |
| TARGET_CONNECTION_REFUSED | connection refused |
| TRACE_NOT_FOUND | route/edge rule not found |
| TRACE_INCOMPLETE | missing edge rule / target down |

### Dependencies
- Trace → RouteRepo, EdgeSvc, ListenerSvc, EndpointRepo
- Target connectivity → net.DialTimeout, net.LookupHost
- Provider diagnostic → DiagnoseHAProxy(), DiagnoseCaddy()

---

## 7. Gateway Link Dimension 🆕

### Capabilities
| Capability | Status | Details |
|-----------|--------|---------|
| TrustedGateway model | ✅ | id, name, host, private_ip, port, auth, type, auto_route |
| Shared secret auth | ✅ | HMAC-SHA256 with timestamp replay protection (5 min window) |
| Auth header generation | ✅ | Format: "Aegis <gateway_id>:<timestamp>:<signature>" |
| Auth header verification | ✅ | Constant-time comparison |
| Auto-routing (private→public fallback) | ✅ | ResolveHost() |
| Secret rotation | ✅ | Old secret invalidated |
| Migration 024 | ✅ | trusted_gateways table |
| ValidateTarget public IP | ✅ | Now allows any valid IP |
| Caddy header_up rendering | ✅ | ExtraHeaders in ProxyOptions |
| Wiring into Planner | ❌ | Requires GatewayLinkService reference in route planning |

### Flow
```
Register Gateway B on A:
  Aegis A → POST /api/admin/v1/gateway-links
          → generates secret, stores hash, returns raw secret

Manual step:
  Copy secret to Server B's Aegis config (or register via B's API)

Caddy rendering (when wired):
  A's Caddyfile → reverse_proxy B's IP:port
                → header_up X-Aegis-Gateway "<signed header>"
  B's Caddyfile → validates header before forwarding to local target
                → invalid/missing header → 403
```

### Dependencies
- GatewayLink → GatewayLinkRepository, crypto functions
- GatewayLink rendering → Planner → RouteConfig → ProxyOptions.ExtraHeaders
- Migration 024 → store.Initialize() (automatic)

---

## 8. Network Dimension

| Path | Protocol | Port | Security | Status |
|------|----------|:---:|----------|--------|
| User → A:80 | HTTP | 80 | Host header match | ✅ |
| User → A:443 | TLS/SNI | 443 | SNI match | ✅ |
| A Caddy → target | TCP | any | None (unless gateway link) | ✅ |
| A Caddy → B via gateway link | TCP | 80/443 | Shared secret header | 🛠️ |
| A Aegis API (local) | HTTP | 9000 | Bearer token | ✅ |
| A Aegis API (127.0.0.1) | HTTP | 7380 | Bearer token | ✅ |

---

## 9. Key Dependencies Map

```
Identity & Auth ──→ API Surface ──→ Data Model
       │                 │
       │                 ▼
       │           Action Service
       │                 │
       ▼                 ▼
     Apply Pipeline ──→ Provider (Caddy/HAProxy)
       │                 │
       ▼                 ▼
   Trace ←─────── Provider Config
       │
       ▼
  Operator (human)

Gateway Link ──→ Planner ──→ Provider Render (header_up)
```

---

## 10. Current Gaps (Not Yet Wired)

| Feature | Status | Reason |
|---------|--------|--------|
| Server B config (verify auth) | ❌ | Needs Caddy template for header validation |
| HAProxy header_up equivalent | ❌ | HAProxy renderer doesn't support custom headers yet |
| Gateway Link auto-route in planner | ⏳ | Route-level gateway link ID binding works; auto-route pending |

---

## Summary

| Dimension | Real Verified | Implemented | Planned | Not Planned |
|-----------|:---:|:---:|:---:|:---:|
| Identity & Auth | 7 | 1 | — | — |
| API Surface | 8 | 1 | — | — |
| Data Model | 16 tables | 1 table | — | — |
| Provider | 3 providers | 1 feature | — | — |
| Apply Pipeline | 8 stages | — | — | — |
| Trace | 3 types × 6 codes | — | — | — |
| Gateway Link | — | 7 capabilities | 4 wiring | — |
| Network | 6 paths | 1 path | — | — |
