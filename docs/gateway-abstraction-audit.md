# Gateway Abstraction Audit — v1.7R

## Executive Summary

**CRITICAL FINDING**: The v1.7 gateway abstraction (`gateway_domains`, `gateway_routes`, `gateway_listeners`) is a **completely independent second source of truth** that never synchronizes with the real state tables. Mutations to gateway_* tables create records that are invisible to the apply pipeline, provider rendering, and the actual running configuration.

---

## 1. Table Relationship Analysis

### Question 1: gateway_domain ↔ managed_domain — what's the relationship?

**Answer: None. They are completely independent.**

| Attribute | gateway_domains | managed_domains |
|-----------|----------------|-----------------|
| Purpose | Abstract gateway domain binding | Domain verification + ownership tracking |
| Fields | domain, node_id, tls_enabled, tls_provider, status | domain, verification_status, verified_at, status |
| Created by | POST /api/admin/v1/gateway/domains | POST /api/managed-domains |
| Used by apply? | **No** | Yes — planner includes verified managed domains |
| Synced? | **No bidirectional sync** | N/A |

**Problem**: Creating a gateway_domain does NOT create a managed_domain. The apply planner reads managed_domains, not gateway_domains. So a domain created via the gateway API will **never appear in rendered config**.

### Question 2: gateway_route ↔ route — what's the relationship?

**Answer: None. They are completely independent.**

| Attribute | gateway_routes | routes |
|-----------|---------------|--------|
| Purpose | Abstract path → service mapping | Real HTTP route with domain + service binding |
| Fields | domain_id, path, target_service, target_port, protocol | domain, path_prefix, service_id, tls_enabled, status, space_id |
| Created by | POST /api/admin/v1/gateway/routes | POST /api/routes or BindHTTPDomain action |
| Used by apply? | **No** | Yes — planner renders routes into Caddy config |
| Synced? | **No bidirectional sync** | N/A |

**Problem**: Creating a gateway_route does NOT create a route in the routes table. The apply planner reads routes, not gateway_routes. So a gateway route will **never appear in rendered config**.

### Question 3: gateway_listener ↔ listener — what's the relationship?

**Answer: None. They are completely independent.**

| Attribute | gateway_listeners | listeners |
|-----------|------------------|-----------|
| Purpose | Abstract port listener | Real listener with port conflict detection |
| Fields | node_id, port, tls_enabled, protocol, status | port, protocol, provider, status |
| Created by | No creation API (table exists but no write endpoint) | listenerSvc.RegisterDefaults() |
| Used by apply? | **No** | Yes — exposure selects listeners for provider |
| Synced? | **No** | N/A |

**Problem**: The gateway_listeners table has no creation endpoint and no data population path. It's an empty table with no connection to the real listeners table.

---

## 2. Source of Truth Determination

### Which table is source of truth?

**The original tables are the source of truth:**

| Domain | Source of Truth |
|--------|----------------|
| HTTP routing | `routes` + `managed_domains` |
| TLS/SNI routing | `edge_mux_rules` |
| Listeners | `listeners` |
| Services | `services` |
| Endpoints | `endpoints` |

The `gateway_*` tables are a **shadow layer** that has no effect on any real system behavior.

### Are gateway_* tables used anywhere?

Searching the codebase:
- `gateway_domains` — only used by GatewayService → gateway handlers
- `gateway_routes` — only used by GatewayService → gateway handlers
- `gateway_listeners` — only used by GatewayService → gateway handlers

**NO downstream consumer reads from gateway_* tables.** The apply planner, provider renderers, health checks — none of them know these tables exist.

---

## 3. GatewayService Method Analysis

### Does GatewayService call ActionService?

**No.** GatewayService imports: `context`, `fmt`, `time`, `aegis/internal/id`, `aegis/internal/node`.

It has zero references to: ActionService, RouteService, EdgeMuxService, ManagedDomainService, ApplyService.

### Does GatewayService call RouteService?

**No.**

### Does GatewayService call EdgeMuxService?

**No.**

### Does Gateway API trigger safe apply?

**No.** None of the gateway handlers call `h.Apply.TryApply()`.

### Can two sets of state become inconsistent?

**Yes, trivially.** The scenario:

1. Admin creates `gateway_domain` for "example.com" → success
2. Admin creates `gateway_route` path "/" → "my-service":8080 → success
3. Admin calls `POST /api/apply` → renders config from `routes` table
4. **The gateway_* records are invisible — nothing is applied**
5. The gateway abstraction says "example.com is configured" but the actual Caddy/HAProxy config has nothing for it

**This is the worst outcome: the abstraction layer lies to the admin.**

---

## 4. Bug: AttachRoute Domain Existence Check

In `internal/gateway/service.go:66-69`:

```go
func (s *GatewayService) AttachRoute(...) (*GatewayRoute, error) {
    _, err := s.domains.FindByID(domainID)
    if err != nil || err == nil {  // BUG: always true
        // domain exists check
    }
    ...
}
```

The condition `err != nil || err == nil` is always `true` (a value is always either nil or not nil). This means the domain existence check is completely non-functional. A route can be attached to a non-existent domain_id.

**Fix**: Should be `if err != nil { return nil, ... }` or `if _, err := ...; err != nil { return nil, ... }`.

---

## 5. Fix Plan

### Immediate (this audit): Freeze Gateway Mutation APIs

All gateway mutation endpoints must be **frozen to read-only** until the gateway abstraction is properly wired as a synchronization layer:

1. `POST /api/admin/v1/gateway/domains` → **DISABLED** (return 405 with explanation)
2. `POST /api/admin/v1/gateway/routes` → **DISABLED**
3. `DELETE /api/admin/v1/gateway/routes/{id}` → **DISABLED**
4. `PUT /api/admin/v1/gateway/domains/{id}/tls` → **DISABLED**

Read-only endpoints remain available:
1. `GET /api/admin/v1/gateway/domains` → shows data from existing real tables (routes + managed_domains)
2. `GET /api/admin/v1/gateway/listeners` → shows data from listeners table

### Future (post v1.7R): Gateway as Synchronized View

The correct architecture for the gateway abstraction:

```
POST /api/admin/v1/gateway/domains
  → GatewayService.CreateDomain()
    → 1. Create managed_domain (or verify domain exists)
    → 2. Call ActionService.BindHTTPDomain() ← THE KEY MISSING STEP
    → 3. ActionService triggers safe apply
    → 4. gateway_domains record created as VIEW/INDEX only
```

The gateway_* tables should be a **denormalized read view** populated by the synchronization layer, not an independent write target.

---

## 6. Conclusion

**Gateway abstraction in its current form is dangerous.** It creates a parallel state that:
- Appears to work (CRUD returns success)
- Does not affect real configuration
- Lies to the admin about system state
- Has a broken domain existence check bug

**Verdict**: Freeze all gateway mutation endpoints to read-only. The gateway abstraction needs a complete redesign where it acts as a synchronization facade over the real source-of-truth tables, not as an independent data store.
