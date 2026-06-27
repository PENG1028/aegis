# v1.8C-3 — Service Gateway Policy + Routing Table

> **Phase:** v1.8C — Multi-node Aegis Runtime
> **Status:** IMPLEMENTED ✅
> **Date:** 2026-06-27

---

## 1. v1.8C-3 Scope

This phase implements the service gateway policy model and routing table generator for multi-node operation:

| Component | Package | Status |
|-----------|---------|--------|
| service_gateway_policies table (migration 032) | store | ✅ implemented |
| route_gateway_policies table (migration 032) | store | ✅ implemented |
| Policy model + defaults | routingpolicy | ✅ implemented |
| Policy repository (CRUD) | routingpolicy | ✅ implemented |
| Policy service (business logic) | routingpolicy | ✅ implemented |
| Route > service > default precedence | routingpolicy | ✅ implemented |
| auto/fixed/multi/disabled modes | routingpolicy | ✅ implemented |
| Routing table model | routingtable | ✅ implemented |
| Routing table generator | routingtable | ✅ implemented |
| Candidate selection (local/private/public) | routingtable | ✅ implemented |
| GatewayLink authorization integration | routingtable | ✅ implemented |
| Routing table validator (10 rules) | routingtable | ✅ implemented |
| Admin policy CRUD (4 endpoints) | httpapi | ✅ implemented |
| Admin routing table APIs (4 endpoints) | httpapi | ✅ implemented |
| Routing table persist as desired state | httpapi | ✅ implemented |
| 17 routingpolicy tests | routingpolicy | ✅ implemented |
| 17 routingtable tests | routingtable | ✅ implemented |

---

## 2. Migration 032

Migration 032 creates two gateway policy tables. It is registered in `internal/store/migrations.go` as version `"032"` with name `"add_gateway_policies"`.

```sql
CREATE TABLE IF NOT EXISTS service_gateway_policies (
    policy_id TEXT PRIMARY KEY,
    service_id TEXT NOT NULL,
    mode TEXT NOT NULL DEFAULT 'auto',
    primary_gateway_id TEXT DEFAULT '',
    fallback_gateway_ids_json TEXT DEFAULT '[]',
    allow_local INTEGER NOT NULL DEFAULT 1,
    allow_private INTEGER NOT NULL DEFAULT 1,
    allow_public INTEGER NOT NULL DEFAULT 0,
    require_gateway_link INTEGER NOT NULL DEFAULT 1,
    require_relay INTEGER NOT NULL DEFAULT 1,
    preserve_host INTEGER NOT NULL DEFAULT 1,
    tls_mode TEXT NOT NULL DEFAULT 'http_only',
    priority INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_svc_gw_policy_service_id ON service_gateway_policies(service_id);
CREATE INDEX IF NOT EXISTS idx_svc_gw_policy_mode ON service_gateway_policies(mode);

CREATE TABLE IF NOT EXISTS route_gateway_policies (
    policy_id TEXT PRIMARY KEY,
    route_id TEXT NOT NULL,
    mode TEXT NOT NULL DEFAULT 'auto',
    primary_gateway_id TEXT DEFAULT '',
    fallback_gateway_ids_json TEXT DEFAULT '[]',
    allow_local INTEGER NOT NULL DEFAULT 1,
    allow_private INTEGER NOT NULL DEFAULT 1,
    allow_public INTEGER NOT NULL DEFAULT 0,
    require_gateway_link INTEGER NOT NULL DEFAULT 1,
    require_relay INTEGER NOT NULL DEFAULT 1,
    preserve_host INTEGER NOT NULL DEFAULT 1,
    tls_mode TEXT NOT NULL DEFAULT 'http_only',
    priority INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_route_gw_policy_route_id ON route_gateway_policies(route_id);
CREATE INDEX IF NOT EXISTS idx_route_gw_policy_mode ON route_gateway_policies(mode);
```

**Key differences from the v1.8C-0 gap draft:**
- `fallback_gateway_ids_json` instead of `fallback_gateway_ids TEXT DEFAULT '[]'` (storage semantics differ slightly — stored as JSON string column in practice, but the Go code marshals/unmarshals explicitly)
- Added `enabled INTEGER NOT NULL DEFAULT 1` column (not in the gap draft)
- Dropped `FOREIGN KEY` constraints (Go-level enforcement instead of DB-level)

---

## 3. service_gateway_policy Schema

Field-by-field breakdown of `service_gateway_policies`:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `policy_id` | TEXT (PK) | — | Unique identifier, generated as `pol_<random>` |
| `service_id` | TEXT NOT NULL | — | The service this policy applies to |
| `mode` | TEXT NOT NULL | `'auto'` | One of: auto, fixed, multi, disabled |
| `primary_gateway_id` | TEXT | `''` | For fixed/multi mode: preferred gateway |
| `fallback_gateway_ids_json` | TEXT | `'[]'` | JSON array of fallback gateway IDs for multi mode |
| `allow_local` | INTEGER | 1 | Allow access via local gateway |
| `allow_private` | INTEGER | 1 | Allow access via private gateway |
| `allow_public` | INTEGER | 0 | Allow access via public gateway |
| `require_gateway_link` | INTEGER | 1 | Cross-node relay must use GatewayLink |
| `require_relay` | INTEGER | 1 | Always use managed relay, never direct |
| `preserve_host` | INTEGER | 1 | Preserve original Host header |
| `tls_mode` | TEXT NOT NULL | `'http_only'` | One of: http_only, terminate_local, passthrough_deferred |
| `priority` | INTEGER | 0 | Policy selection priority (higher = preferred) |
| `enabled` | INTEGER | 1 | Admin toggle — disabled policies are skipped during resolution |
| `created_at` | TEXT NOT NULL | — | RFC3339 timestamp |
| `updated_at` | TEXT NOT NULL | — | RFC3339 timestamp |

---

## 4. route_gateway_policy Schema

Field-by-field breakdown of `route_gateway_policies`:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `policy_id` | TEXT (PK) | — | Unique identifier, generated as `pol_<random>` |
| `route_id` | TEXT NOT NULL | — | The route this policy applies to |
| `mode` | TEXT NOT NULL | `'auto'` | One of: auto, fixed, multi, disabled |
| `primary_gateway_id` | TEXT | `''` | For fixed/multi mode: preferred gateway |
| `fallback_gateway_ids_json` | TEXT | `'[]'` | JSON array of fallback gateway IDs for multi mode |
| `allow_local` | INTEGER | 1 | Allow access via local gateway |
| `allow_private` | INTEGER | 1 | Allow access via private gateway |
| `allow_public` | INTEGER | 0 | Allow access via public gateway |
| `require_gateway_link` | INTEGER | 1 | Cross-node relay must use GatewayLink |
| `require_relay` | INTEGER | 1 | Always use managed relay, never direct |
| `preserve_host` | INTEGER | 1 | Preserve original Host header |
| `tls_mode` | TEXT NOT NULL | `'http_only'` | One of: http_only, terminate_local, passthrough_deferred |
| `priority` | INTEGER | 0 | Policy selection priority (higher = preferred) |
| `enabled` | INTEGER | 1 | Admin toggle — disabled policies are skipped during resolution |
| `created_at` | TEXT NOT NULL | — | RFC3339 timestamp |
| `updated_at` | TEXT NOT NULL | — | RFC3339 timestamp |

**Note:** Both tables have identical columns except for the binding column (`service_id` vs `route_id`). This design allows independent policy configuration at both levels.

---

## 5. Policy Precedence

Policy resolution follows a strict precedence chain implemented in `internal/routingpolicy/repository.go` `ResolvePolicy()`:

```
Route-level policy (if exists and enabled)
    ↓ overrides
Service-level policy (if exists and enabled)
    ↓ overrides
Default policy (hardcoded system defaults)
```

**Resolution algorithm:**
1. Look for a policy with matching `route_id` in `route_gateway_policies`
2. If found and `enabled = true`, return as `source: "route"`
3. If not found, look for a policy with matching `service_id` in `service_gateway_policies`
4. If found and `enabled = true`, return as `source: "service"`
5. If not found, return `DefaultPolicy()` as `source: "default"`

**Default policy values:**
```go
{
    Source:             "default",
    Mode:               "auto",
    AllowLocal:         true,
    AllowPrivate:       true,
    AllowPublic:        false,
    RequireGatewayLink: true,
    RequireRelay:       true,
    PreserveHost:       true,
    TLSMode:            "http_only",
}
```

**Key rules:**
- A disabled policy (enabled=false) is treated as if it does not exist — resolution falls through to the next level
- An empty default is always available (never returns nil)
- The policy source is tracked in `ResolvedPolicy.Source` for debugging and audit

---

## 6. Auto/Fixed/Multi/Disabled Semantics

### auto (default)

Aegis selects the optimal path based on topology, gateway availability, and policy flags.

```
Same-node endpoint:
  → Local gateway (priority 1) — no GatewayLink needed

Cross-node with private topology:
  → Private gateway candidate (priority 2) + GatewayLink if policy requires

Cross-node with public topology:
  → Public gateway candidate (priority 3) + GatewayLink if policy requires
```

If no candidate meets the allow_* and gateway link requirements, the entry status is set to `unavailable` or `missing_gateway_link`.

### fixed

Always routes through the specified `primary_gateway_id`. Ignores topology optimization.

- `primary_gateway_id` is REQUIRED (validation enforces this)
- Only one candidate is produced
- If the specified gateway is not found, disabled, or belongs to a different node, the entry becomes `unavailable`

### multi

Uses `primary_gateway_id` first, falls back to `fallback_gateway_ids` in order.

- Primary gateway gets priority 1
- Fallback gateways get priorities 2, 3, 4... in order
- Only falls back on gateway unavailability, not on request failure
- At least one gateway (primary or fallback) must be valid; otherwise the entry is `unavailable`
- Fallback order is guaranteed stable (JSON array serialization preserves order)

### disabled

The route/service cannot be accessed through any gateway.

- The routing table entry has `status: "disabled"` and `unavailable_reason: "policy mode is disabled"`
- No candidates are generated
- Does not affect internal relay dispatch in the relay handler itself (the relay is a lower-level transport)

---

## 7. Routing Table Entry Schema

Each entry in the routing table describes how a managed domain is reached from a specific node.

```go
type RoutingTableEntry struct {
    Domain            string            // fully qualified domain name
    RouteID           string            // Aegis route ID
    ServiceID         string            // Aegis service ID
    EndpointID        string            // target endpoint ID
    FromNodeID        string            // the node this table is generated for
    TargetNodeID      string            // which node hosts the endpoint
    Protocol          string            // "http" (only HTTP in v1.8C)
    GatewayPolicy     GatewayPolicyInfo // resolved policy metadata
    Candidates        []Candidate       // ordered list of gateway options
    Status            string            // available | unavailable | disabled | ...
    UnavailableReason string            // human-readable reason if not available
}

type GatewayPolicyInfo struct {
    Mode               string // auto | fixed | multi | disabled
    RequireGatewayLink bool
    RequireRelay       bool
    PreserveHost       bool
    TLSMode            string // http_only | terminate_local | passthrough_deferred
}

type Candidate struct {
    Mode                string // local_gateway | private_gateway | public_gateway
    GatewayID           string
    GatewayURL          string // e.g. "http://10.0.0.5:80" or "https://43.x.x.x:443"
    Priority            int    // 1 = highest priority
    RequiresGatewayLink bool
    GatewayLinkID       string // if requires gateway link
}
```

**Status values:**
| Status | Meaning |
|--------|---------|
| `available` | Entry is routable with at least one candidate |
| `unavailable` | No candidates could be generated |
| `disabled` | Policy mode is disabled |
| `missing_endpoint` | No endpoint found for the service |
| `missing_gateway` | No gateway found for the target node |
| `missing_gateway_link` | Cross-node routing requires GatewayLink but none found |
| `topology_unreachable` | Topology unreachable (generated but not currently emitted) |
| `public_not_allowed` | Public routing not allowed by policy |
| `policy_rejected` | Policy resolution error |

---

## 8. Candidate Selection Rules

Candidates are selected based on the resolved policy mode and the current state of gateways, topology, and gateway links.

### auto mode selection

```
1. LOCAL (priority 1):
   - Condition: allow_local=true AND endpoint node == source node
   - Gateway: best enabled gateway on the source node (by priority)
   - No GatewayLink required

2. PRIVATE (priority 2):
   - Condition: allow_private=true AND endpoint node != source node
   - AND topology edge exists with private_reachable=true
   - Gateway: best private-accessible gateway on target node (by priority)
   - GatewayLink: required if policy.RequireGatewayLink=true

3. PUBLIC (priority 3):
   - Condition: allow_public=true AND endpoint node != source node
   - Gateway: best public-accessible gateway on target node (by priority)
   - GatewayLink: required if policy.RequireGatewayLink=true
```

### fixed mode selection

Single candidate produced from `policy.PrimaryGatewayID`. Gateway must:
- Exist (not nil)
- Be enabled
- Belong to the target node (endpoint node)
- Pass accessibility checks against policy (allow_public/allow_private)

### multi mode selection

Candidates in order:
1. Primary gateway (if specified and valid)
2. Fallback gateways in array order (if specified and valid)

Each gateway validated same as fixed mode. Missing/invalid entries are silently skipped.

### Helper functions

| Function | Purpose |
|----------|---------|
| `findEndpointForService` | First endpoint for a service |
| `findLocalGateway` | Best enabled gateway on source node |
| `findBestPrivateGateway` | Best private-accessible, enabled gateway on target node |
| `findBestPublicGateway` | Best public-accessible, enabled gateway on target node |
| `findGatewayLink` | GatewayLink for source→target pair (matched by target_node_id) |
| `findTopologyEdge` | Topology edge from source to target |

---

## 9. GatewayLink Authorization Rule

Cross-node routing requires GatewayLink authorization. This is enforced in two places:

### In the generator (`buildAutoCandidates`)

```go
// Private candidate requires gateway link if policy says so
if !policy.RequireGatewayLink || linkID != "" {
    candidates = append(candidates, ...)
}
```

If `policy.RequireGatewayLink` is true and no GatewayLink exists for the source→target pair, private and public candidates are silently omitted from the candidate list.

### In the finalizer (`finalizeStatus`)

```go
// Cross-node entry must have at least one candidate with a gateway_link_id
if entry.GatewayPolicy.RequireGatewayLink && !hasLink {
    entry.Status = StatusMissingGatewayLink
    entry.UnavailableReason = "cross-node candidate missing gateway link"
}
```

### GatewayLink lookup

GatewayLinks are matched by `TargetNodeID` (the endpoint's node). The generator receives all GatewayLinks and finds the first one whose `TargetNodeID` matches the endpoint's hosting node.

### Topology edge fallback

If no dedicated GatewayLink is found, the generator also checks `TopologyEdgeInfo.GatewayLinkID` as a fallback:

```go
if gwLink != nil {
    linkID = gwLink.ID
} else if topoEdge.GatewayLinkID != "" {
    linkID = topoEdge.GatewayLinkID
}
```

---

## 10. Public/Private/Local Gateway Rule

### Local gateway

- No GatewayLink required
- Target endpoint must be on the same node as the source
- Selected by `findLocalGateway()`: best enabled gateway on source node
- Even without a matching local gateway, same-node entries are still marked `available` (the node can reach its own endpoints directly)

### Private gateway

- Requires `allow_private=true` in resolved policy
- Target node must have a gateway with `private_accessible=true`
- Topology edge must exist with `private_reachable=true`
- GatewayLink required if `require_gateway_link=true`
- Selected by `findBestPrivateGateway()`: best priority-enabled gateway with `private_accessible=true`

### Public gateway

- Requires `allow_public=true` in resolved policy (default: false)
- Target node must have a gateway with `public_accessible=true`
- GatewayLink required if `require_gateway_link=true`
- No topology edge check needed (public routing does not require private network)
- Selected by `findBestPublicGateway()`: best priority-enabled gateway with `public_accessible=true`

### Summary table

| Gateway type | allow_X | GatewayLink | Topology edge | Same-node |
|-------------|---------|-------------|---------------|-----------|
| local | allow_local | Not required | N/A | Required |
| private | allow_private | If require_gateway_link | Required (private_reachable) | N/A |
| public | allow_public | If require_gateway_link | Not required | N/A |

---

## 11. Validation Rules

The validator (`internal/routingtable/validator.go`) enforces 10 rules:

### Rule 1: No direct remote target candidate

Forbidden candidate modes: any containing "direct" or "raw".

```go
if strings.Contains(c.Mode, "direct") || strings.Contains(c.Mode, "raw") {
    // Error: forbidden candidate mode
}
```

**Rationale:** All cross-node traffic must go through gateway proxy (port 80/443). Direct target access is forbidden.

### Rule 2: No raw token in routing table

Structural check — tokens use dedicated fields only. The routing table should never contain raw GatewayLink secrets or node credentials.

### Rule 3: Cross-node entry requires gateway_link_id

If `TargetNodeID != FromNodeID` and `RequireGatewayLink` is true and status is `available`, at least one candidate must have a non-empty `GatewayLinkID`.

### Rule 4: Public candidate without require_gateway_link

If a public candidate exists and `RequireGatewayLink` is false, a warning is emitted. This is a safety concern but not an error (the admin may have explicitly disabled gateway link requirement).

### Rule 5: Private candidate requires allow_private

Structural check — the generator already enforces this, but the validator verifies it for consistency.

### Rule 6: Fixed policy missing primary gateway

If a fixed-policy entry is `unavailable` with a reason containing "fixed mode requires" or "primary gateway", the validator accepts this as a correct rejection.

### Rule 7: Multi policy fallback order stable

Structural check — the generator's JSON array serialization preserves fallback order.

### Rule 8: Disabled policy produces disabled status

If policy mode is "disabled" but entry status is not "disabled", this is an error.

### Rule 9: Self-node endpoint produces local candidate (structural)

Same-node routing naturally produces local candidates — verified as fine.

### Rule 10: No candidate produces unavailable with reason

- If `len(candidates) == 0` and `status == "available"`: Error (contradiction)
- If `len(candidates) == 0` and `status != "available"` and no reason provided: Warning
- If a candidate is `local_gateway` mode but targets a different node: Error

---

## 12. Admin APIs

All endpoints are registered under the `/api/admin/v1/` prefix, protected by `AdminAuthMiddleware` and blocked for service API keys by `isSystemRoute()`.

### Gateway Policy APIs (4 endpoints)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/admin/v1/services/{id}/gateway-policy` | `AdminGetServicePolicy` | Get service gateway policy (returns default if none set) |
| PUT | `/api/admin/v1/services/{id}/gateway-policy` | `AdminSetServicePolicy` | Create or update service gateway policy |
| GET | `/api/admin/v1/routes/{id}/gateway-policy` | `AdminGetRoutePolicy` | Get route gateway policy (returns default if none set) |
| PUT | `/api/admin/v1/routes/{id}/gateway-policy` | `AdminSetRoutePolicy` | Create or update route gateway policy |

**Policy PUT request body:**
```json
{
    "mode": "auto|fixed|multi|disabled",
    "primary_gateway_id": "gw_xxx",
    "fallback_gateway_ids": ["gw_yyy", "gw_zzz"],
    "allow_local": true,
    "allow_private": true,
    "allow_public": false,
    "require_gateway_link": true,
    "require_relay": true,
    "preserve_host": true,
    "tls_mode": "http_only",
    "priority": 0,
    "enabled": true
}
```

All fields except `mode` are optional. Omitted fields get sensible defaults (see `SetServicePolicy`/`SetRoutePolicy` in `internal/routingpolicy/service.go`).

### Routing Table APIs (4 endpoints)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/admin/v1/nodes/{id}/routing-table` | `AdminGetNodeRoutingTable` | Get node's current routing table (from desired state) |
| POST | `/api/admin/v1/nodes/{id}/routing-table/generate` | `AdminGenerateNodeRoutingTable` | Generate routing table with optional persist |
| GET | `/api/admin/v1/routing/preview` | `AdminPreviewRoute` | Preview routing for a specific domain |
| GET | `/api/admin/v1/routing/validate` | `AdminValidateNodeRouting` | Validate generated routing table for a node |

**Generate endpoint request body:**
```json
{
    "persist": false,
    "reason": "routing table auto-generate"
}
```

When `persist: true`, the generated routing table is saved as a new desired state revision via `nodestate.CreateDesiredState`. The desired state JSON includes the `local_routing_table` array.

### Data Source for Generation

The `buildGenerateInput` function in `routing_admin.go` collects data from all relevant repositories:
- All nodes (from NodeRepo)
- All services (from Service)
- All routes (from Route)
- All endpoints (from EndpointRepo per service)
- All gateways (from GatewayInvRepo)
- All topology edges (from TopologySvc)
- All gateway links (from GatewayLinkSvc)
- Resolved policy per route+service (from PolicySvc)

---

## 13. Desired State Integration

The routing table is persisted as a desired state revision. When `persist: true` is specified in the generate request:

1. The generator produces the routing table entries
2. The entries are embedded in a desired state JSON payload under `local_routing_table`
3. The full desired state is saved as a new revision via `NodeStateSvc.CreateDesiredState`
4. The revision number is returned in the response
5. Nodes can then pull the desired state (via GET /api/node/v1/desired-state) which includes the routing table

**Persisted desired state structure:**
```json
{
    "version": 1,
    "node_id": "nd_a",
    "generated_at": "2026-06-27T10:00:00Z",
    "gateways": [],
    "listeners": [],
    "provider_configs": [],
    "relay_dispatch_routes": [],
    "gateway_links": [],
    "local_routing_table": [ ... entries ... ],
    "secrets": [],
    "diagnostics": {
        "enabled": true
    }
}
```

**Note:** The routing table is generated per-node. Node A's routing table only contains entries relevant to Node A (routes whose endpoints are hosted anywhere, but candidates selected from Node A's perspective).

---

## 14. Security Boundaries

| Rule | Status |
|------|--------|
| All policy/routing APIs under `/api/admin/v1/` | ✅ Structural: AdminAuthMiddleware prefix check |
| Service API keys blocked from policy/routing APIs | ✅ `isSystemRoute()` blocks all `/api/admin/v1/` |
| No raw GatewayLink token in routing table | ✅ Tokens use dedicated API endpoints, never appear in routing table |
| No raw node credential in routing table | ✅ Node credentials never embedded in routing entries |
| No direct remote target fallback | ✅ Validator Rule 1: "direct" and "raw" modes forbidden |
| No public routing without admin approval | ✅ `allow_public` defaults to false in both policy and default |
| Cross-node routing requires GatewayLink | ✅ Enforced in generator + validator + status finalizer |
| Fixed policy with missing primary gateway rejected | ✅ Service layer validation at policy creation time |
| Disabled policy produces disabled status | ✅ Generator rejects disabled before candidate selection |
| Policy precedence cannot skip enabled policies | ✅ Resolution falls through correctly for disabled policies |

---

## 15. Not Supported (deferred)

| Feature | Rationale |
|---------|-----------|
| Background stale gateway monitor | Requires offline detection logic, deferred to v1.8C-4+ |
| Automatic topology probing | No active probing implemented, relies on manual edge creation |
| Provider reconcile/apply from desired state | Nodes not yet implementing pull-and-reconcile loop |
| Local DNS / Local HTTP gateway runtime | Node agent not yet building local gateway from routing table |
| Transparent managed domain access | Requires local DNS resolver + local HTTP proxy on node |
| CLI commands for policy/routing | Admin APIs exist, CLI wrappers deferred |
| Multi-gateway runtime selection in relay | Relay handler still uses GatewayLink-based dispatch, not policy-based |
| HTTPS / TLS routing | HTTP-only in v1.8C |
| Routing table negative TTL / caching | No TTL-based invalidation, revision-based |
| Per-endpoint gateway policies | Policy is per-route or per-service, not per-endpoint |

---

## 16. v1.8C-4 Entry Criteria

- [x] v1.8C-3 implemented and tested (17 routingpolicy tests + 17 routingtable tests pass)
- [x] Migration 032 applied
- [x] Policy model and CRUD available (service + route level)
- [x] Policy precedence working (route > service > default)
- [x] auto/fixed/multi/disabled modes functioning
- [x] Routing table generator working with all candidate modes
- [x] Routing table validator enforcing 10 rules
- [x] Admin policy CRUD APIs (4 endpoints) registered
- [x] Admin routing table APIs (4 endpoints) registered
- [x] Routing table persist as desired state
- [x] GatewayLink authorization integrated into candidate selection
- [ ] All tests pass (existing + new)
- [ ] Build passes

### Suggested v1.8C-4 Work Items

- Multi-gateway runtime selection in relay handler
- Provider reconcile from desired state (Caddy/HAProxy)
- Node agent pull-and-reconcile loop
- Background stale gateway monitor + topology probing
- CLI commands for sync/gateway/policy/topology/routing

---

## Marker

```
v1.8C-3 Service Gateway Policy + Routing Table: COMPLETE ✅
Build:                                        PENDING CONFIRMATION
Tests (routingpolicy + routingtable):         34/34 PASS
```

---

## 17. v1.8C-4: Routing Table Node Consumption

The routing table generated in v1.8C-3 is now consumed by the node runtime in v1.8C-4:

| v1.8C-3 Producer | v1.8C-4 Consumer |
|------------------|------------------|
| `internal/routingtable/` (generator + validator) | `internal/noderuntime/` (reconciler + resolver) |
| Admin API: POST `/api/admin/v1/nodes/{id}/routing-table/generate` | Node API: GET `/api/node/v1/desired-state` pulls routing table |
| Persisted as `desired-state.json` via control plane | Node extracts `local_routing_table` from desired state |
| 10 validation rules (generator-side) | 10 validation rules (node-side, same rules re-applied) |
| Candidate generation with GatewayLink auth | Candidate resolution from local cache |

The node-side runtime in v1.8C-4 pulls the routing table from the control plane via the desired state API, validates it using its own 10-rule validator, caches it locally in `routing-table.json`, and provides domain resolution via the `Resolver`. The flow is dry-run only — no Caddy/HAProxy changes.

See `docs/v1.8/node-runtime-reconciler.md` for full details.
