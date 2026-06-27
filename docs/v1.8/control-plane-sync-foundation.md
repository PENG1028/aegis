# v1.8C-2 — Control Plane Sync Foundation

> **Phase:** v1.8C — Multi-node Aegis Runtime
> **Status:** IMPLEMENTED ✅
> **Date:** 2026-06-27

---

## 1. Implementation Scope

This phase implements the control plane sync foundation for multi-node operation:

### Implemented

| Component | Migration | Status |
|-----------|-----------|--------|
| node_desired_states | 029 | ✅ implemented |
| node_actual_states | 029 | ✅ implemented |
| gateways (inventory) | 030 | ✅ implemented |
| topology_edges | 031 | ✅ implemented |
| Desired state CRUD service | — | ✅ implemented |
| Actual state report service | — | ✅ implemented |
| Sync status (6 states) | — | ✅ implemented |
| Heartbeat revision hint | — | ✅ implemented |
| Node pull desired-state | — | ✅ implemented |
| Node report actual-state | — | ✅ implemented |
| Admin desired/actual APIs | — | ✅ implemented |
| Admin gateway inventory APIs | — | ✅ implemented |
| Admin topology matrix/path APIs | — | ✅ implemented |

### Implemented in v1.8C-2A

| Component | Migration | Status |
|-----------|-----------|--------|
| Heartbeat gateway status reporting | — | ✅ implemented |
| Gateway update by ID with ownership check | — | ✅ implemented |
| Gateway upsert by name (heartbeat auto-discovery) | — | ✅ implemented |
| Gateway degraded on last_error | — | ✅ implemented |
| Node desired-state/actual-state auth smoke | — | ✅ tested (6 tests) |
| Admin sync/gateway/topology route registration proof | — | ✅ tested (13 routes) |
| Admin API auth structural proof | — | ✅ documented |

### Not Implemented (deferred)

- Multi-gateway runtime selection
- Local DNS / Local HTTP gateway
- Transparent managed domain access
- Provider reconcile/apply from desired state
- Automatic topology probing/verification
- CLI commands

### Implemented in v1.8C-3

| Component | Migration | Status |
|-----------|-----------|--------|
| service_gateway_policies table | 032 | ✅ implemented |
| route_gateway_policies table | 032 | ✅ implemented |
| Policy model and CRUD (internal/routingpolicy/) | — | ✅ implemented |
| Policy precedence: route > service > default | — | ✅ implemented |
| auto/fixed/multi/disabled modes | — | ✅ implemented |
| Routing table generator (internal/routingtable/) | — | ✅ implemented |
| Local/private/public candidate selection | — | ✅ implemented |
| GatewayLink authorization integration | — | ✅ implemented |
| Routing table validator (10 rules) | — | ✅ implemented |
| Admin policy CRUD APIs (4 endpoints) | — | ✅ implemented |
| Admin routing table APIs (4 endpoints) | — | ✅ implemented |
| Routing table persist as desired state | — | ✅ implemented |
| 17 routingpolicy tests | — | ✅ implemented |
| 17 routingtable tests | — | ✅ implemented |

### Implemented in v1.8C-4

| Component | Package | Status |
|-----------|---------|--------|
| Node runtime config (node.yaml) | noderuntime/config.go | ✅ implemented |
| Control plane HTTP client | noderuntime/client.go | ✅ implemented |
| Atomic local cache (4 files) | noderuntime/cache.go | ✅ implemented |
| Node-side routing table validator (10 rules) | noderuntime/validator.go | ✅ implemented |
| Candidate resolver | noderuntime/resolver.go | ✅ implemented |
| Dry-run reconcile loop | noderuntime/reconciler.go | ✅ implemented |
| Relay request plan builder | noderuntime/relay_request.go | ✅ implemented |
| JSON helpers | noderuntime/helpers.go | ✅ implemented |

### Implemented in v1.8C-5

| Component | Package | Status |
|-----------|---------|--------|
| Local HTTP gateway config | localgateway/config.go | ✅ implemented |
| Domain resolver interface | localgateway/resolver.go | ✅ implemented |
| HTTP handler (managed/unmanaged dispatch) | localgateway/handler.go | ✅ implemented |
| Local forwarder (same-node dispatch) | localgateway/local_dispatch.go | ✅ implemented |
| Managed relay client (cross-node via GatewayLink) | localgateway/relay_client.go | ✅ implemented |
| Gateway lifecycle (start/stop/status) | localgateway/server.go | ✅ implemented |
| Gateway status tracking | localgateway/status.go | ✅ implemented |
| GatewayLink secret provider interface | noderuntime/secret_provider.go | ✅ implemented |
| Relay client with runtime token injection | localgateway/relay_client.go | ✅ implemented |

---

## 2. Migrations

### Migration 029: node_desired_states + node_actual_states

```sql
CREATE TABLE IF NOT EXISTS node_desired_states (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL,
    revision INTEGER NOT NULL DEFAULT 0,
    state_hash TEXT NOT NULL DEFAULT '',
    state_json TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'active',    -- active | superseded | failed
    reason TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    superseded_at TEXT DEFAULT '',
    UNIQUE(node_id, revision)
);

CREATE TABLE IF NOT EXISTS node_actual_states (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL UNIQUE,
    applied_revision INTEGER NOT NULL DEFAULT 0,
    state_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'unknown',  -- unknown | applying | applied | failed | degraded
    last_apply_at TEXT DEFAULT '',
    last_success_at TEXT DEFAULT '',
    last_error TEXT DEFAULT '',
    provider_status TEXT DEFAULT '{}',
    relay_status TEXT DEFAULT '{}',
    gateway_status TEXT DEFAULT '{}',
    diagnostics_status TEXT DEFAULT '{}',
    reported_at TEXT DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

### Migration 030: gateways

```sql
CREATE TABLE IF NOT EXISTS gateways (
    gateway_id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT 'local',     -- local | private | public
    provider TEXT NOT NULL DEFAULT 'aegis', -- caddy | haproxy | aegis
    bind_addr TEXT NOT NULL DEFAULT '0.0.0.0',
    host TEXT NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 80,
    scheme TEXT NOT NULL DEFAULT 'http',    -- http | https
    public_accessible INTEGER NOT NULL DEFAULT 0,
    private_accessible INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 100,
    status TEXT NOT NULL DEFAULT 'unknown', -- unknown | online | offline | degraded
    last_verified_at TEXT DEFAULT '',
    last_error TEXT DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

### Migration 031: topology_edges

```sql
CREATE TABLE IF NOT EXISTS topology_edges (
    id TEXT PRIMARY KEY,
    from_node_id TEXT NOT NULL,
    to_node_id TEXT NOT NULL,
    private_reachable INTEGER NOT NULL DEFAULT 0,
    public_reachable INTEGER NOT NULL DEFAULT 0,
    preferred_gateway_id TEXT DEFAULT '',
    gateway_link_id TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'unknown',
    last_verified_at TEXT DEFAULT '',
    last_error TEXT DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(from_node_id, to_node_id)
);
```

---

## 3. Desired State Schema v1

```json
{
  "version": 1,
  "node_id": "nd_c",
  "revision": 1,
  "generated_at": "2026-06-27T10:00:00Z",
  "gateways": [],
  "listeners": [],
  "provider_configs": [],
  "relay_dispatch_routes": [],
  "gateway_links": [],
  "local_routing_table": [],
  "secrets": [],
  "diagnostics": {
    "enabled": true
  }
}
```

In v1.8C-2, the state_json can contain empty arrays. All fields are placeholders for future phases. The schema validates:
- Must be valid JSON
- No raw tokens allowed (secrets must be empty array or absent)

---

## 4. Actual State Schema

```json
{
  "node_id": "nd_c",
  "applied_revision": 3,
  "state_hash": "sha256...",
  "status": "applied",
  "provider_status": "{}",
  "relay_status": "{}",
  "gateway_status": "{}",
  "diagnostics_status": "{}",
  "last_error": ""
}
```

---

## 5. Gateway Inventory Schema

```json
{
  "gateway_id": "gw_abc",
  "node_id": "nd_c",
  "name": "public-gateway",
  "type": "public",
  "provider": "caddy",
  "bind_addr": "0.0.0.0",
  "host": "<SERVER_A_IP>",
  "port": 443,
  "scheme": "https",
  "public_accessible": true,
  "private_accessible": false,
  "enabled": true,
  "priority": 100,
  "status": "online"
}
```

---

## 6. Topology Edge Schema

```json
{
  "id": "te_abc",
  "from_node_id": "nd_a",
  "to_node_id": "nd_b",
  "private_reachable": true,
  "public_reachable": false,
  "preferred_gateway_id": "",
  "gateway_link_id": "gl_abc",
  "status": "unknown"
}
```

---

## 7. Heartbeat Revision Hint

The heartbeat response now includes:

```json
{
  "node_id": "nd_c",
  "status": "accepted",
  "latest_revision": 3,
  "desired_state_available": true,
  "node_is_outdated": true
}
```

- `latest_revision`: highest revision of desired state for this node
- `desired_state_available`: true if any desired state exists
- `node_is_outdated`: true if node's applied_revision < latest_revision

---

## 8. Node APIs

### GET /api/node/v1/desired-state

Auth: Node credential (Bearer token)

Returns the latest desired state for the authenticated node. If `?revision=N` is specified, returns that specific revision. If no desired state exists, returns `{"node_id":"...","revision":0,"status":"no_desired_state"}`.

Node can only pull its own desired state.

### POST /api/node/v1/actual-state

Auth: Node credential (Bearer token)

```json
{
  "node_id": "nd_c",
  "applied_revision": 3,
  "state_hash": "sha256...",
  "status": "applied",
  "provider_status": "{}",
  "last_error": ""
}
```

Node can only report its own actual state. Node A cannot report for node B.

---

## 9. Admin APIs

### Desired / Actual State

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/admin/v1/nodes/{id}/desired-state` | Get latest desired state |
| POST | `/api/admin/v1/nodes/{id}/desired-state` | Create desired state |
| GET | `/api/admin/v1/nodes/{id}/actual-state` | Get latest actual state |
| GET | `/api/admin/v1/nodes/{id}/sync-status` | Get sync status (in_sync/outdated/etc.) |

### Gateway Inventory

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/admin/v1/gateways` | List all gateways (?node_id= to filter) |
| POST | `/api/admin/v1/gateways` | Create gateway |
| GET | `/api/admin/v1/gateways/{id}` | Get gateway detail |
| PATCH | `/api/admin/v1/gateways/{id}` | Update gateway |
| GET | `/api/admin/v1/nodes/{id}/gateways` | List gateways for a node |

### Topology

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/admin/v1/topology/matrix` | Get all topology edges |
| GET | `/api/admin/v1/topology/path?from=X&to=Y` | Get path between two nodes |
| POST | `/api/admin/v1/topology/edges` | Create/update topology edge |
| PATCH | `/api/admin/v1/topology/edges/{id}` | Update edge (by id) |

---

## 10. Sync Status Semantics

| Status | Meaning |
|--------|---------|
| `in_sync` | Node's applied_revision == latest desired revision AND state_hash matches |
| `outdated` | Node's applied_revision < latest desired revision |
| `no_desired_state` | No desired state exists for this node |
| `no_actual_state` | Desired state exists but node has never reported actual |
| `failed` | Node's actual state status = failed |
| `degraded` | Node's actual state status = degraded |

---

## 11. Security Boundaries

| Rule | Status |
|------|--------|
| Node token can only pull its own desired state | ✅ Enforced in handler |
| Node token can only report its own actual state | ✅ Enforced in handler |
| Node token cannot access admin APIs | ✅ `/api/admin/v1/` prefix by AdminAuthMiddleware |
| Service key cannot access admin node/gateway/topology APIs | ✅ `isSystemRoute()` blocks all `/api/admin/v1/` |
| Desired state without raw GatewayLink token | ✅ Enforced by design (no sensitive fields in state) |
| Malformed state_json rejected | ✅ Hash computation fails → create rejected |
| Topology edge cannot be self-edge | ✅ Service rejects `from == to` |
| Gateway must belong to valid node | ✅ Service validates node existence |

---

## 12. Test Coverage

| Package | Tests | Coverage |
|---------|-------|----------|
| nodestate | 14 | revision, hash, sync status (6 states), actual state, compare, no-leak, JSON normalization |
| gateway | 7 | create, list, update, upsert, reject missing node, list all |
| topology | 9 | create, self-edge reject, matrix, path (known + unknown), update status, list, required fields |
| httpapi (existing auth) | 16 | admin routes, join, heartbeat, auth, no-leak |
| nodeauth (existing) | 17 | join token lifecycle, credential auth, hash |

---

## 13. v1.8C-3 Entry Criteria

- [x] v1.8C-2 implemented and tested (22 packages pass)
- [x] Migrations 029, 030, 031 applied
- [x] Desired state CRUD available
- [x] Actual state report available
- [x] Gateway inventory available
- [x] Topology edges available
- [x] Sync status (6 states) available
- [x] Heartbeat revision hint available
- [x] Node can pull own desired state
- [x] Node cannot pull other node's desired state
- [x] Node can report own actual state
- [x] Node cannot report other node's actual state
- [x] Admin APIs for desired/actual/gateway/topology
- [x] All tests pass (30 new tests + 14 v1.8C-2A tests, all existing)

### v1.8C-2A Completed
- [x] Heartbeat gateway status reporting
- [x] Gateway update by ID with node ownership enforcement
- [x] Gateway upsert by name (heartbeat auto-discovery)
- [x] Gateway degraded on last_error
- [x] Node desired-state/actual-state auth smoke (6 tests)
- [x] Admin route registration proof (13 routes)
- [x] Admin auth structural proof (all under /api/admin/v1/)
- [x] Token leak verification

### Suggested v1.8C-3 Work Items (historical — all completed)

- ~~Service gateway policy model + API~~ ✅ (v1.8C-3)
- ~~Routing table generation from desired state~~ ✅ (v1.8C-3)
- Multi-gateway runtime selection (deferred)
- Provider reconcile from desired state (Caddy/HAProxy) (deferred)
- CLI commands for sync/gateway/topology (deferred)

### v1.8C-3 Completed
- [x] service_gateway_policies table (migration 032)
- [x] route_gateway_policies table (migration 032)
- [x] Policy model and CRUD (internal/routingpolicy/)
- [x] Policy precedence: route > service > default
- [x] auto/fixed/multi/disabled modes
- [x] Routing table generator (internal/routingtable/)
- [x] Local/private/public candidate selection
- [x] GatewayLink authorization integration
- [x] Routing table validator (10 rules)
- [x] Admin policy CRUD APIs (4 endpoints)
- [x] Admin routing table APIs (4 endpoints)
- [x] Routing table persist as desired state
- [x] 17 routingpolicy tests
- [x] 17 routingtable tests

---

## 14. v1.8C-2A: Gateway Heartbeat Status & Sync Auth Closure

### Implemented

| Feature | Status |
|---------|--------|
| Heartbeat gateway status reporting | ✅ implemented |
| Gateway status semantics (online/degraded/offline/unknown) | ✅ implemented |
| Gateway update by gateway_id with node ownership check | ✅ implemented |
| Gateway upsert by node_id + name (heartbeat auto-discovery) | ✅ implemented |
| Node cannot update/report other node's gateway | ✅ enforced |
| Deferred -> degraded when last_error present | ✅ implemented |
| Node desired-state/actual-state auth smoke tests | ✅ implemented |
| Admin sync/gateway/topology route registration proof | ✅ implemented |
| Admin auth structural proof (prefix under /api/admin/v1/) | ✅ documented |
| Gateway heartbeat response no token leak | ✅ verified |

### Gateway Status Semantics

When a gateway entry is received via heartbeat:

| Condition | Status |
|-----------|--------|
| `last_error` is non-empty | `degraded` (regardless of reported `status`) |
| `last_error` empty, `status` reported | Uses reported `status` |
| `last_error` empty, no `status` | `online` (upsert) |
| Gateway created via admin API before first heartbeat | `unknown` |

### Heartbeat Gateway Processing Rules

1. **gateway_id present**: Lookup by ID, enforce node ownership, update mutable fields (host, port, scheme, bind_addr, accessibility, enabled). If node_id mismatch, error is logged but heartbeat still returns 200 (gateway status is advisory).
2. **gateway_id empty, name present**: Upsert by node_id + name. Creates new gateway or updates existing one for this node.
3. **gateway_id empty, name empty**: Skipped silently.

### Heartbeat Request Extension

```json
{
  "node_id": "nd_c",
  "status": "online",
  "gateways": [
    {
      "gateway_id": "gw_xxx",      // optional — update existing by ID
      "name": "public-http",        // required if gateway_id not set
      "type": "public",
      "provider": "caddy",
      "host": "43.x.x.x",
      "bind_addr": "0.0.0.0",
      "port": 80,
      "scheme": "http",
      "public_accessible": true,
      "private_accessible": false,
      "enabled": true,
      "status": "online",
      "last_error": ""
    }
  ]
}
```

### Auth Enforcement Proof

#### Admin APIs (all under `/api/admin/v1/` → AdminAuthMiddleware)

| API | Method | Path | Auth Proof |
|-----|--------|------|------------|
| Desired state | GET/POST | `/api/admin/v1/nodes/{id}/desired-state` | structural: prefix check |
| Actual state | GET | `/api/admin/v1/nodes/{id}/actual-state` | structural: prefix check |
| Sync status | GET | `/api/admin/v1/nodes/{id}/sync-status` | structural: prefix check |
| List gateways | GET | `/api/admin/v1/gateways` | structural: prefix check |
| Create gateway | POST | `/api/admin/v1/gateways` | structural: prefix check |
| Get gateway | GET | `/api/admin/v1/gateways/{id}` | structural: prefix check |
| Update gateway | PATCH | `/api/admin/v1/gateways/{id}` | structural: prefix check |
| Node gateways | GET | `/api/admin/v1/nodes/{id}/gateways` | structural: prefix check |
| Topology matrix | GET | `/api/admin/v1/topology/matrix` | structural: prefix check |
| Topology path | GET | `/api/admin/v1/topology/path` | structural: prefix check |
| Topology edges | POST | `/api/admin/v1/topology/edges` | structural: prefix check |
| Update edge | PATCH | `/api/admin/v1/topology/edges/{id}` | structural: prefix check |

All confirmed under `/api/admin/v1/` prefix → structurally covered by `AdminAuthMiddleware`. Service API keys blocked by `isSystemRoute()`.

#### Node APIs (Bearer token credential auth)

| API | Method | Path | Auth Test |
|-----|--------|------|-----------|
| Desired state | GET | `/api/node/v1/desired-state` | 401 missing token ✅, 401 wrong token ✅ |
| Actual state | POST | `/api/node/v1/actual-state` | 401 missing token ✅, 401 wrong token ✅, 403 other node ✅ |
| Heartbeat | POST | `/api/node/v1/heartbeat` | 401 missing token ✅, 401 wrong token ✅, 403 wrong node_id ✅, 401 revoked token ✅ |

#### Gateway Enforcement

| Rule | Test |
|------|------|
| Node A cannot update Node B gateway via heartbeat | ✅ gateway_id ownership enforced, status unchanged on B's gateway |
| Node A cannot report actual state for Node B | ✅ 403 returned |
| Heartbeat response contains no raw node token | ✅ verified |

### Test Coverage (v1.8C-2A)

| Test | Type | What it proves |
|------|------|----------------|
| `TestNodeHeartbeatUpdatesExistingGateway` | heartbeat gateway | gateway_id + ownership update |
| `TestNodeHeartbeatUpsertsGatewayByName` | heartbeat gateway | upsert by name, idempotent |
| `TestNodeHeartbeatRejectsOtherNodeGateway` | heartbeat gateway | node B cannot update node A gateway |
| `TestNodeHeartbeatDegradedOnError` | heartbeat gateway | last_error → degraded |
| `TestNodeDesiredStateNoAuth` | node auth | 401 on missing token |
| `TestNodeDesiredStateWrongAuth` | node auth | 401 on wrong token |
| `TestNodeDesiredStateCannotPullOtherNode` | node auth | 403 on cross-node report |
| `TestNodeActualStateNoAuth` | node auth | 401 on missing token |
| `TestNodeActualStateWrongAuth` | node auth | 401 on wrong token |
| `TestNodeActualStateCannotReportOtherNode` | node auth | 403 on cross-node report |
| `TestNodeHeartbeatGatewayResponseNoTokenLeak` | node auth | raw token not in response |
| `TestAdminSyncRoutesRegistered` | admin auth | 13 routes registered |
| `TestAdminSyncPathsUnderAdminPrefix` | admin auth | all paths under /api/admin/v1/ |
| `TestNodeHeartbeatRevokedToken` | existing | revoked token rejected |
| `TestNodeHeartbeatNoAuth` | existing | missing token → 401 |

### Not Implemented (still deferred)

- Background stale gateway monitor (offline detection)
- Automatic topology probing

---

## Marker

```
v1.8C-2  Control Plane Sync Foundation:    COMPLETE ✅
v1.8C-2A Gateway HB Status & Auth Closure: COMPLETE ✅
v1.8C-3  Gateway Policy + Routing Table:    COMPLETE ✅
Build:                                      PASS
Tests (httpapi, v1.8C-2A new):             14/14 PASS
Tests (routingpolicy + routingtable):       34/34 PASS
All packages:                               24 PASS
All existing tests:                         PASS
```
