# Gateway Abstraction Boundary Closure — v1.7T

## Status: Frozen (mutation), Read-only (query)

The gateway abstraction layer (v1.7) was audited in v1.7R and found to create a second source of truth that was invisible to the apply pipeline. v1.7T finalizes the boundary.

---

## 1. What gateway_* tables are

| Table | Purpose | Current State |
|-------|---------|---------------|
| `gateway_domains` | Abstract domain registry (concept only) | **Exists but unused for config** — read queries redirected to `managed_domains` + `routes` |
| `gateway_routes` | Abstract path→service mapping (concept only) | **Exists but unused for config** — read queries redirected to `routes` |
| `gateway_listeners` | Abstract port listener (concept only) | **Exists but unused for config** — read queries redirected to `listeners` |

These tables exist as **schema artifacts** from v1.7 design, but are not the source of truth for any operational system.

---

## 2. Real source of truth

| Data | Source Table | Writes Via |
|------|-------------|-----------|
| HTTP routes | `routes` | ActionService.BindHTTPDomain, Admin Route CRUD |
| TLS SNI routing | `edge_mux_rules` | ActionService.BindTLSBackend, EdgeSvc |
| Listeners | `listeners` | ListenerService.RegisterDefaults |
| Domain verification | `managed_domains` | ManagedDomainService |
| Services | `services` | ActionService, Admin Service CRUD |
| Endpoints | `endpoints` | ActionService, Admin Endpoint CRUD |

---

## 3. API Status

### Read-only (available via Admin API)

```
GET /api/admin/v1/gateway/domains   → reads from routes + managed_domains
GET /api/admin/v1/gateway/listeners → reads from listeners
```

These return a **consolidated view** built from real source-of-truth tables, not from `gateway_*` tables.

### Frozen (returns 405 GATEWAY_MUTATION_FROZEN)

```
POST   /api/admin/v1/gateway/domains         → 405 (use /api/v1/actions/bind-http-domain)
POST   /api/admin/v1/gateway/routes           → 405 (use /api/routes directly)
DELETE /api/admin/v1/gateway/routes/{id}      → 405 (use /api/routes/{id} disable/delete)
PUT    /api/admin/v1/gateway/domains/{id}/tls → 405 (use exposure management)
```

---

## 4. If gateway mutations are ever restored

The correct integration path for any future gateway write API:

```
GatewayDomain create → ActionService.BindHTTPDomain()
  → creates service, endpoint, route
  → creates edge_mux_rule (if TLS)
  → triggers safe apply
  → writes gateway_domains as READ INDEX only

GatewayRoute attach → RouteService (or ActionService)
  → creates real route
  → triggers safe apply
  → writes gateway_routes as READ INDEX only
```

The gateway_* tables, if kept, must be **denormalized read views** populated by the action layer, not independent write targets.

---

## 5. GatewayService current state

`GatewayService` in `internal/gateway/service.go`:
- CreateDomain: **not called** (handler returns 405)
- AttachRoute: **not called** (handler returns 405)
- DetachRoute: **not called** (handler returns 405)
- UpdateTLSPolicy: **not called** (handler returns 405)
- ListDomains: **not called** (handler reads real tables directly)
- ListListeners: **not called** (handler reads real tables directly)
- ListRoutes: available but unused by API
- DisabledActionsForNode: available for UI consumption
- HealthCheck: available for health summary

---

## 6. Access Path Trace (v1.7T)

The trace system (`internal/trace/`) provides a **read-only** way to understand how a domain/SNI/route flows through the system:

```
TraceDomain("example.com")
  → 1. route_lookup: route found (tls=true)
  → 2. edge_listener: port 443 via haproxy_edge_mux
  → 3. sni_match: edge rule → 127.0.0.1:8443
  → 4. tls_termination: Caddy on 127.0.0.1:8443
  → 5. route_detail: route rt_xxx → service svc_xxx
  → 6. haproxy_diag: available
  → 7. caddy_diag: available
  → Final Target: 127.0.0.1:8443 (https)
```

This is the **authoritative path visualization** — it reads from the real source-of-truth tables.

---

## 7. Design Principle

> The gateway abstraction is a **view**, not a **source of truth**. All writes go through ActionService → real tables → safe apply. The trace system reads the real tables to show the actual access path. Gateway_* tables are schema artifacts frozen for now and must not be written to.
