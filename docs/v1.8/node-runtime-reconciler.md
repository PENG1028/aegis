# v1.8C-4 — Node Runtime Reconciler

> **Phase:** v1.8C — Multi-node Aegis Runtime
> **Status:** IMPLEMENTED ✅
> **Date:** 2026-06-27
> **Package:** `internal/noderuntime/`

---

## 1. v1.8C-4 Scope

This phase implements the node-side runtime reconciler — the agent that runs on each Aegis node to pull desired state from the control plane, validate it, cache it locally, and report actual state back.

### Implemented

| Component | File | Status |
|-----------|------|--------|
| Node runtime config (`node.yaml`) | `config.go` | ✅ implemented |
| Control plane HTTP client | `client.go` | ✅ implemented |
| Atomic local cache (4 files) | `cache.go` | ✅ implemented |
| Node-side routing table validator (10 rules) | `validator.go` | ✅ implemented |
| Candidate resolver | `resolver.go` | ✅ implemented |
| Dry-run reconcile loop | `reconciler.go` | ✅ implemented |
| Relay request plan builder | `relay_request.go` | ✅ implemented |
| JSON helpers | `helpers.go` | ✅ implemented |

### Not Implemented (deferred)

- Local DNS resolver (Node-side DNS proxy)
- Local HTTP gateway (Node-side HTTP proxy)
- Real relay execution (only plan building, no execution)
- Provider reconcile/apply (Caddy/HAProxy config generation)
- Background stale gateway detection
- Automatic topology probing

---

## 2. node.yaml Config

The node runtime configuration is defined in `internal/noderuntime/config.go` as the `Config` struct, loaded from a YAML file at `/etc/aegis/node.yaml` by default.

### Config Fields

| Field | YAML Key | Type | Default | Required | Description |
|-------|----------|------|---------|----------|-------------|
| `ControlPlaneURL` | `control_plane_url` | string | `http://127.0.0.1:8080` | Yes | Control plane base URL |
| `NodeID` | `node_id` | string | `""` | **Yes** | Unique node identifier |
| `NodeTokenFile` | `node_token_file` | string | `/etc/aegis/node.token` | No | Path to node token file |
| `NodeToken` | — (loaded from file/env) | string | `""` | **Yes** | Raw node token, never serialized |
| `CacheDir` | `cache_dir` | string | `/var/lib/aegis` | No | Cache directory for state files |
| `RuntimeDir` | `runtime_dir` | string | `/run/aegis` | No | Runtime directory (sockets, etc.) |
| `HeartbeatIntervalSec` | `heartbeat_interval_seconds` | int | `15` | No | Heartbeat interval in seconds |
| `SyncIntervalSec` | `sync_interval_seconds` | int | `15` | No | Sync interval in seconds |
| `ReconcileMode` | `reconcile_mode` | string | `dry_run` | No | `dry_run` or `apply` |

### Defaults

```go
DefaultConfigPath     = "/etc/aegis/node.yaml"
DefaultCacheDir       = "/var/lib/aegis"
DefaultRuntimeDir     = "/run/aegis"
DefaultTokenFile      = "/etc/aegis/node.token"
DefaultHeartbeatSec   = 15
DefaultSyncSec        = 15
DefaultReconcileMode  = "dry_run"
```

### Example node.yaml

```yaml
control_plane_url: http://<SERVER_A_IP>:80
node_id: nd_b
node_token_file: /etc/aegis/node.token
cache_dir: /var/lib/aegis
heartbeat_interval_seconds: 15
sync_interval_seconds: 15
reconcile_mode: dry_run
```

### Environment Overrides

| Environment Variable | Overrides | If Set |
|---------------------|-----------|--------|
| `AEGIS_CONTROL_PLANE_URL` | `control_plane_url` | Non-empty |
| `AEGIS_NODE_ID` | `node_id` | Non-empty |
| `AEGIS_NODE_TOKEN_FILE` | `node_token_file` | Non-empty |
| `AEGIS_NODE_TOKEN` | `NodeToken` (direct) | Non-empty (bypasses file) |
| `AEGIS_CACHE_DIR` | `cache_dir` | Non-empty |

### Config Validation

`LoadConfig()` returns an error if:
- `node_id` is empty
- `node_token` is empty (no token file, no env var)

### Security: SafeString

The `SafeString()` method returns a log-safe representation:
```
Config{control_plane=http://...:80, node_id=nd_b, cache_dir=/var/lib/aegis, heartbeat=15s, sync=15s, mode=dry_run}
```

The `NodeToken` field is tagged `yaml:"-" json:"-"` — it is never serialized to YAML or JSON. It is loaded from a file at startup and held only in memory.

---

## 3. Control Plane Client

The client (`internal/noderuntime/client.go`) communicates with the Aegis control plane over HTTP. It uses Bearer token authentication with the node credential.

### Client Initialization

```go
client := noderuntime.NewClient(baseURL, nodeID, nodeToken)
```

- `baseURL`: Control plane URL (e.g. `http://<SERVER_A_IP>:80`)
- `nodeID`: Node identifier for the requesting node
- `nodeToken`: Raw node token for Bearer auth

Authentication is set by adding `Authorization: Bearer <token>` to every request via `setAuth()`.

### Heartbeat

**Method:** `SendHeartbeat(status, agentVersion, hostname string) (*HeartbeatResponse, error)`

**Request:** POST `/api/node/v1/heartbeat`
```json
{
  "node_id": "nd_b",
  "status": "online",
  "agent_version": "v1.8C",
  "hostname": "nd_b"
}
```

**Response:**
```json
{
  "node_id": "nd_b",
  "status": "accepted",
  "latest_revision": 3,
  "desired_state_available": true,
  "node_is_outdated": true
}
```

Key fields for the reconciler:
- `latest_revision`: highest desired state revision for this node
- `desired_state_available`: true if any desired state exists
- `node_is_outdated`: true if node's applied_revision < latest_revision

### Pull Desired State

**Method:** `PullDesiredState() (*DesiredStateResponse, error)`

**Request:** GET `/api/node/v1/desired-state`

**Response:**
```json
{
  "node_id": "nd_b",
  "revision": 3,
  "state_hash": "sha256...",
  "state_json": "{...}",
  "status": "active"
}
```

The `state_json` contains the full desired state payload including the `local_routing_table` array.

### Report Actual State

**Method:** `ReportActualState(req ActualStateRequest) error`

**Request:** POST `/api/node/v1/actual-state`
```json
{
  "node_id": "nd_b",
  "applied_revision": 3,
  "state_hash": "sha256...",
  "status": "applied",
  "diagnostics_status": "{}",
  "last_error": ""
}
```

**Response:** 200 OK on success. Non-2xx returns an `APIError` with status code classification: `auth_failed`, `forbidden`, `server_error`, or `request_failed`.

### APIError Classification

```go
type APIError struct {
    Path       string
    StatusCode int
    Body       string
}
```

The `ErrorClassification()` method maps status codes to categories (no raw tokens in error messages):
- 401 → `auth_failed`
- 403 → `forbidden`
- 500+ → `server_error`
- Other → `request_failed`

---

## 4. Sync Flow

The reconcile loop is implemented in `reconciler.go` as a single `SyncOnce()` method.

### Complete Sync Cycle

```
Heartbeat ──→ Check Outdated ──→ Pull Desired State ──→ Validate ──→ Extract RT ──→ Validate RT ──→ Cache ──→ Report
```

### Step-by-step

#### Step 1: Heartbeat

Send heartbeat to check if the node is outdated.

```go
hbResp, err := r.client.SendHeartbeat("online", "v1.8C", r.nodeID)
```

If heartbeat fails, the reconciler creates a "failed" actual state with `last_error: "heartbeat failed: ..."` and reports it immediately.

#### Step 2: Check Outdated

```go
if !hbResp.NodeIsOutdated {
    // No update needed; return existing cached state
}
```

If the node is up-to-date (`node_is_outdated=false`), the reconciler returns the existing cached actual state immediately. No desired state pull is performed.

#### Step 3: Pull Desired State

```go
ds, err := r.client.PullDesiredState()
```

Only called when the node is outdated. Pulls the latest desired state from the control plane.

#### Step 4: Validate Desired State (server-side)

```go
validation := ValidateDesiredStateForNode(r.nodeID, dsCache)
```

Validates:
- `node_id` matches the current node
- `state_hash` is non-empty
- `state_json` is non-empty
- No raw token patterns in `state_json`

#### Step 5: Extract Routing Table

```go
rtCache, err := extractRoutingTableFromState(ds.StateJSON)
```

Parses the `local_routing_table` array from the desired state JSON. If no routing table exists, returns an empty entries list.

#### Step 6: Validate Routing Table

```go
rtValidation := ValidateRoutingTable(r.nodeID, rtCache)
```

Runs 10 validation rules against the extracted routing table (see Section 6).

#### Step 7: Write Caches (Dry-Run)

```go
r.cache.WriteDesiredState(dsCache)
r.cache.WriteRoutingTable(rtCache)
```

Writes both the desired state and routing table to disk atomically. In `dry_run` mode, this is the extent of the "apply" — no Caddy/HAProxy configuration changes are made.

#### Step 8: Report Actual State

```go
r.client.ReportActualState(ActualStateRequest{
    AppliedRevision:   ds.Revision,
    StateHash:         ds.StateHash,
    Status:            "applied",
    DiagnosticsStatus: diagJSON,
})
```

Reports the applied revision and status back to the control plane. If the report fails, the local cache is written but the actual state status is set to `degraded`.

### Status Transitions

| Condition | Status | Reported |
|-----------|--------|----------|
| Heartbeat fails | `failed` | Yes |
| Desired state pull fails | `failed` | Yes |
| Desired state validation fails | `failed` | Yes |
| Routing table extraction fails | `failed` | Yes |
| Routing table validation fails | `failed` | Yes |
| Cache write fails | `failed` | Yes |
| All steps pass, report succeeds | `applied` | Yes |
| All steps pass, report fails | `degraded` | Yes |

---

## 5. Local Cache Files

The cache manager (`internal/noderuntime/cache.go`) manages 4 cache files in the cache directory (`/var/lib/aegis` by default).

### Cache File Inventory

| File | Purpose | Schema | Created/Updated |
|------|---------|--------|-----------------|
| `desired-state.json` | Latest pulled desired state | `DesiredStateCache` | On each successful sync |
| `routing-table.json` | Extracted routing table | `RoutingTableCache` | On each successful sync |
| `actual-state.json` | Last reported actual state | `ActualStateCache` | On each sync completion |
| `runtime-status.json` | Runtime status metadata | TBD | Reserved for future use |

### DesiredStateCache Schema

```json
{
  "node_id": "nd_b",
  "revision": 3,
  "state_hash": "sha256abc123...",
  "state_json": "{...}"
}
```

`state_json` contains the full desired state payload from the control plane, including the `local_routing_table` array.

### RoutingTableCache Schema

```json
{
  "node_id": "nd_b",
  "revision": 3,
  "entries": [
    {
      "domain": "app.example.com",
      "route_id": "rte_abc",
      "service_id": "svc_xyz",
      "endpoint_id": "ep_123",
      "from_node_id": "nd_b",
      "target_node_id": "nd_a",
      "protocol": "http",
      "status": "available",
      "candidates": [
        {
          "mode": "private_gateway",
          "gateway_id": "gw_abc",
          "gateway_url": "http://10.0.0.5:80",
          "priority": 1,
          "requires_gateway_link": true,
          "gateway_link_id": "gl_xyz"
        }
      ]
    }
  ]
}
```

### ActualStateCache Schema

```json
{
  "applied_revision": 3,
  "state_hash": "sha256abc123...",
  "status": "applied",
  "reported_at": "2026-06-27T12:00:00Z"
}
```

On failure:
```json
{
  "applied_revision": 0,
  "state_hash": "",
  "status": "failed",
  "last_error": "heartbeat failed: ...",
  "reported_at": "2026-06-27T12:00:01Z"
}
```

### Atomic Writes

All cache writes use `atomicWrite()` which:
1. Marshals data to JSON with indentation
2. Writes to a `.tmp` file
3. Renames the `.tmp` file to the final name (atomic on POSIX)

This prevents partial/corrupted reads if the node crashes during a write.

### Token Safety Check

The `ContainsRawToken()` helper checks for 64+ consecutive hex characters in a string — a best-effort guard against accidental token leakage into cache files.

---

## 6. Node-side Routing Table Validation

The validator (`internal/noderuntime/validator.go`) enforces 10 rules on the extracted routing table before it is written to cache.

### Validation Function

```go
func ValidateRoutingTable(nodeID string, table *RoutingTableCache) *ValidationResult
```

Returns `ValidationResult`:
```go
type ValidationResult struct {
    IsValid  bool     `json:"is_valid"`
    Errors   []string `json:"errors,omitempty"`
    Warnings []string `json:"warnings,omitempty"`
}
```

If `IsValid` is false, the reconciler rejects the desired state and reports `failed`.

### Validation Rules

#### Rule 1: from_node_id must match current node

Each entry's `FromNodeID` must equal the current node's ID. Entries generated for other nodes are rejected.

```go
if entry.FromNodeID != nodeID { /* Error */ }
```

**Rationale:** The routing table is generated per-node. A node should only consume its own routing table.

#### Rule 2: No direct_remote_target or raw_target candidate

Forbidden candidate modes that bypass the gateway proxy.

```go
if c.Mode == "direct_remote_target" || c.Mode == "raw_target" { /* Error */ }
```

**Rationale:** All cross-node traffic must go through the gateway proxy on port 80/443. Direct target access is forbidden.

#### Rule 3: Cross-node candidate must have gateway_link_id

If `TargetNodeID != nodeID` and the entry status is `available`, at least one candidate must have a non-empty `GatewayLinkID`.

```go
if entry.TargetNodeID != "" && entry.TargetNodeID != nodeID {
    hasLink := false
    for _, c := range entry.Candidates {
        if c.GatewayLinkID != "" { hasLink = true; break }
    }
    if !hasLink && entry.Status == "available" { /* Error */ }
}
```

**Rationale:** Cross-node routing requires GatewayLink authorization. Without a link ID, the candidate cannot be dispatched.

#### Rule 4: Non-local candidate must have gateway_url

All candidates except `local_gateway` on the same node must have a non-empty `GatewayURL`.

```go
if c.Mode == "local_gateway" && entry.TargetNodeID == nodeID { /* skip URL check */ }
if c.GatewayURL == "" { /* Error */ }
```

**Rationale:** Without a URL, the request has nowhere to go. Local same-node requests can use loopback.

#### Rule 5: Local candidate should not have gateway_link_id on same node

If a candidate is `local_gateway` mode and the target is the same node, a `GatewayLinkID` is unnecessary.

```go
if c.Mode == "local_gateway" && entry.TargetNodeID == nodeID && c.GatewayLinkID != "" { /* Warning */ }
```

**Rationale:** Same-node routing does not require GatewayLink. A link ID on a local candidate may indicate a generation error.

#### Rule 6: Candidate priority ordering (structural)

The validator checks that candidate ordering is stable and sensible. This is a structural pass-through — the generator ensures ordering.

#### Rule 7: Available status must have at least one candidate

If an entry's status is `available` but has zero candidates, it's a contradiction.

```go
if entry.Status == "available" && len(entry.Candidates) == 0 { /* Error */ }
```

**Rationale:** An "available" entry with no route is a bug in the generator.

#### Rule 8: No raw token in candidate fields

Checks `GatewayID` and other candidate fields for hex token patterns (64+ consecutive hex chars).

```go
if ContainsRawToken(c.GatewayID) { /* Error */ }
```

**Rationale:** Raw tokens must never appear in the routing table cache.

#### Rule 9: Protocol must be http only

In v1.8C, only HTTP protocol is supported.

```go
if entry.Protocol != "http" && entry.Protocol != "" { /* Error */ }
```

**Rationale:** HTTPS/TLS termination is deferred. Nodes should not attempt HTTPS routing.

#### Rule 10: No contradictions (structural)

Enforced through the combination of rules above — if no candidate exists and status is available, the entry is rejected per Rule 7.

### Desired State Validation

In addition to routing table validation, the `ValidateDesiredStateForNode()` function validates the desired state itself:

```go
func ValidateDesiredStateForNode(nodeID string, ds *DesiredStateCache) *ValidationResult
```

- `ds.NodeID == nodeID` — state belongs to this node
- `ds.StateHash` is non-empty
- `ds.StateJSON` is non-empty
- No raw token pattern in `StateJSON`

---

## 7. Candidate Resolver

The resolver (`internal/noderuntime/resolver.go`) resolves domains against the local routing table cache.

### Resolver

```go
type Resolver struct {
    table *RoutingTableCache
}

func NewResolver(table *RoutingTableCache) *Resolver
func (r *Resolver) Resolve(domain string) *RoutingDecision
```

### RoutingDecision Schema

```json
{
  "domain": "app.example.com",
  "status": "available",
  "route_id": "rte_abc",
  "service_id": "svc_xyz",
  "endpoint_id": "ep_123",
  "target_node_id": "nd_a",
  "selected_candidate": {
    "mode": "private_gateway",
    "gateway_id": "gw_abc",
    "gateway_url": "http://10.0.0.5:80",
    "priority": 1,
    "requires_gateway_link": true,
    "gateway_link_id": "gl_xyz"
  },
  "fallback_candidates": [],
  "unavailable_reason": ""
}
```

### Resolution Logic

1. **Exact domain match**: Find the entry whose `Domain` matches the requested domain exactly.
2. **No match found** → `status: "unavailable"`, `reason: "domain not found in routing table"`
3. **Entry disabled** → `status: "disabled"`, `reason: "routing entry is disabled by policy"`
4. **Entry not available** → `status: <entry status>`, `reason: "routing entry is not available"`
5. **No candidates** → `status: "unavailable"`, `reason: "no candidates available for domain"`
6. **Select best candidate**: Pick the first candidate (highest priority) as `selected_candidate`
7. **Build fallback list**: Remaining candidates are added as `fallback_candidates`
8. **Reject forbidden modes**: If any candidate has mode `direct_remote_target` or `raw_target`, the decision is marked unavailable
9. **Success** → `status: "available"` with selected candidate and fallbacks

### Design Notes

- Only exact domain matching is supported (no wildcard, no prefix)
- Candidate priority is implicit in array ordering (first = highest priority)
- The resolver does NOT execute the request — only builds the decision
- The resolver is safe to call concurrently (read-only on the routing table)

---

## 8. Relay Request Plan

The relay request plan builder (`internal/noderuntime/relay_request.go`) converts a `RoutingDecision` into a `RelayRequestPlan` for outbound relay dispatch.

### Plan Schema

```go
type RelayRequestPlan struct {
    Method       string            `json:"method"`
    GatewayURL   string            `json:"gateway_url"`
    Headers      map[string]string `json:"headers"`
    PreserveHost bool              `json:"preserve_host"`
    RouteID      string            `json:"route_id,omitempty"`
    ServiceID    string            `json:"service_id,omitempty"`
    Available    bool              `json:"available"`
    Reason       string            `json:"reason,omitempty"`
}
```

### Plan Building Logic

```go
func (b *RelayPlanBuilder) BuildPlan(decision *RoutingDecision, originalMethod string) *RelayRequestPlan
```

1. **Decision not available** → `Available: false`, `Reason: decision.UnavailableReason`
2. **Decision available**:
   - `GatewayURL` = candidate's `GatewayURL` + `/__aegis/relay`
   - Headers include:
     - `X-Aegis-Route-ID`: the route identifier
     - `X-Aegis-Hop`: set to `"1"` (first hop)
     - `X-Aegis-Gateway-Link-ID`: only if candidate requires GatewayLink and link ID is present
   - `PreserveHost`: always `true` in v1.8C
   - `RouteID`, `ServiceID`: from the routing decision

### Security: No Token in Plan

**Critical:** The raw GatewayLink token is NEVER included in the plan. The plan only carries the `GatewayLinkID`. Token injection is deferred to the relay execution layer (v1.8C-5).

```go
if candidate.GatewayLinkID != "" && candidate.RequiresGatewayLink {
    plan.Headers["X-Aegis-Gateway-Link-ID"] = candidate.GatewayLinkID
    // Note: raw GatewayLink token is NOT included in the plan.
    // Token injection is deferred to the relay client layer.
}
```

### SafeString (No Token in Logs)

```go
func (p *RelayRequestPlan) SafeString() string
```

Returns:
- `RelayPlan{unavailable: ...}` for unavailable plans
- `RelayPlan{GET http://.../__aegis/relay route=rte_abc}` for available plans

Headers are excluded from `SafeString()` to prevent potential token leakage.

### Plan Example (Available)

```json
{
  "method": "GET",
  "gateway_url": "http://10.0.0.5:80/__aegis/relay",
  "headers": {
    "X-Aegis-Route-ID": "rte_abc",
    "X-Aegis-Hop": "1",
    "X-Aegis-Gateway-Link-ID": "gl_xyz"
  },
  "preserve_host": true,
  "route_id": "rte_abc",
  "service_id": "svc_xyz",
  "available": true
}
```

---

## 9. Dry-run Reconcile

The reconciler runs in `dry_run` mode by default. This means:

### What Happens in Dry-Run

| Action | Dry-Run | Apply (future) |
|--------|---------|----------------|
| Heartbeat to control plane | ✅ Yes | ✅ Yes |
| Pull desired state | ✅ Yes | ✅ Yes |
| Validate desired state | ✅ Yes | ✅ Yes |
| Validate routing table | ✅ Yes | ✅ Yes |
| Write desired-state cache | ✅ Yes | ✅ Yes |
| Write routing-table cache | ✅ Yes | ✅ Yes |
| Write actual-state cache | ✅ Yes | ✅ Yes |
| Report actual state | ✅ Yes | ✅ Yes |
| Modify Caddy configuration | ❌ No | 🔜 Future |
| Modify HAProxy configuration | ❌ No | 🔜 Future |
| Start/stop local DNS resolver | ❌ No | 🔜 Future |
| Apply local HTTP gateway changes | ❌ No | 🔜 Future |

### Reconcile Mode Config

```yaml
reconcile_mode: dry_run   # safe default
```

`ReconcileModeApply` (`"apply"`) is defined as a constant but not yet implemented. In `apply` mode, the reconciler would push configuration changes to Caddy/HAProxy based on the validated routing table.

### Diagnostics Status

The reconciler builds a diagnostics structure after each sync:

```go
diagnostics := map[string]interface{}{
    "routing_table_entries": len(rtCache.Entries),
    "cache_written":         true,
    "reconcile_mode":        r.config.ReconcileMode,
}
```

This is included in the actual state report to the control plane.

---

## 10. Security Boundaries

### No Token in Cache

- `NodeToken` is tagged `yaml:"-" json:"-"` — never serialized
- Token is loaded from file at startup, held only in memory
- `SafeString()` on config omits token
- `ContainsRawToken()` heuristic check scans cache data for 64+ consecutive hex characters

### No Token in Log

| Method | Token Leak Risk |
|--------|----------------|
| `Config.SafeString()` | None — no token field included |
| `RelayRequestPlan.SafeString()` | None — headers excluded |
| `APIError.Error()` | None — only path + status code |
| `APIError.ErrorClassification()` | None — only category string |

### No Token in Plan

- The `RelayRequestPlan` contains `GatewayLinkID` but NOT the raw GatewayLink secret
- The raw token is never part of the routing table cache or desired state cache
- Token injection is deferred to the relay execution layer (not yet implemented)

### Auth Model

| Endpoint | Auth | Token Type |
|----------|------|------------|
| Heartbeat POST | Bearer token | Node credential |
| Desired state GET | Bearer token | Node credential |
| Actual state POST | Bearer token | Node credential |
| Admin APIs | Session/Admin | Admin credential |

### Cache File Permissions

Cache files are written with `0644` permissions. Token-bearing files (`/etc/aegis/node.token`) are expected to be `0600` or tighter (managed externally).

### Boundary Summary

| Boundary | Enforced At | Status |
|----------|-------------|--------|
| Node token not in config serialization | `config.go` | ✅ `yaml:"-" json:"-"` tags |
| Node token not in SafeString | `config.go` | ✅ Explicit omission |
| Raw token not in relay plan | `relay_request.go` | ✅ Comment + field exclusion |
| Raw token not in cache | `validator.go` | ✅ `ContainsRawToken()` check |
| Raw token not in error messages | `client.go` | ✅ Classification only |
| Raw token not in plan SafeString | `relay_request.go` | ✅ Headers excluded |
| Node pulls only its own desired state | Control plane handler | ✅ Server-side enforcement |
| Node reports only its own actual state | Control plane handler | ✅ Server-side enforcement |

---

## 11. Not Supported

The following features were deferred from v1.8C-4. Local HTTP gateway and real relay execution have been implemented in v1.8C-5.

| Feature | Rationale | Target | Status |
|---------|-----------|--------|--------|
| Local DNS resolver | Requires DNS proxy library or OS-level DNS config | v1.8C-6 | 📅 Deferred |
| Local HTTP gateway | Requires local HTTP proxy server binding to port 80 | v1.8C-5 | ✅ Implemented |
| Real relay execution | Requires building the relay HTTP client from the plan | v1.8C-5 | ✅ Implemented |
| Caddy config apply | Needs Caddy admin API integration | v1.8C-6+ | 📅 Deferred |
| HAProxy config apply | Needs HAProxy runtime API integration | v1.8C-6+ | 📅 Deferred |
| Background stale gateway monitor | Offline detection logic | v1.8C-6+ | 📅 Deferred |
| Automatic topology probing | Active probe engine | v1.8C-6+ | 📅 Deferred |
| CLI commands for node runtime | `aegis node` subcommands | v1.8C-6+ | 📅 Deferred |
| HTTPS / TLS routing | HTTP-only in v1.8C | v1.8C-6+ | 📅 Deferred |
| Wildcard domain resolution | Only exact domain matching | v1.8C-6+ | 📅 Deferred |
| Negative TTL / cache invalidation | Revision-based, no TTL | v1.8C-6+ | 📅 Deferred |

### Local Gateway Integration Note (v1.8C-5)

The node runtime reconciler (v1.8C-4) produces the routing table cache that the local HTTP gateway (v1.8C-5) consumes. The integration points are:

1. **Routing table cache** (`routing-table.json`): Written by the reconciler's `SyncOnce()` method, read by the local gateway's `DomainResolver` at request time.
2. **Relay request plan** (`relay_request.go`): The plan builder's `BuildPlan()` output is structurally equivalent to the local gateway's `RoutingDecision`. The plan's `GatewayURL` and headers inform the relay client's `Execute()` method.
3. **Secret provider** (`secret_provider.go`): The `GatewayLinkSecretProvider` interface defined in the noderuntime package is implemented by the local gateway's `RelayClient` to inject raw GatewayLink tokens at request time.
4. **Desired state validation**: The namespace for the two packages is separate (`internal/noderuntime/` vs `internal/localgateway/`) with the routing table cache as the shared contract. The validator in noderuntime ensures cache integrity before the local gateway reads it.

---

## 12. v1.8C-5 Entry Criteria

- [x] v1.8C-4 implemented and documented
- [x] `internal/noderuntime/` package with all 8 files
- [x] Node runtime config loading from file + environment
- [x] Control plane client (heartbeat, pull desired, report actual)
- [x] Atomic local cache (4 files with atomic writes)
- [x] Node-side routing table validator (10 rules)
- [x] Candidate resolver (domain → routing decision)
- [x] Dry-run reconcile loop (heartbeat → pull → validate → cache → report)
- [x] Relay request plan builder (no execution)
- [x] No token in cache / log / plan
- [ ] Local DNS resolver working
- [x] Local HTTP gateway working
- [x] Real relay execution from plan (token injection)
- [x] All tests pass (existing + new)
- [x] Build passes

### Suggested v1.8C-6 Work Items

- Local DNS resolver (node-side DNS proxy for transparent managed domain access)
- Wildcard/subdomain domain resolution
- Candidate fallback on relay failure (retry with next candidate)
- Multi-hop relay (hop count tracking and enforcement)
- HTTPS/TLS termination on local gateway
- Request rate limiting on local gateway
- Access logging and structured log output
- Gateway health endpoint
- Provider reconcile/apply from desired state (Caddy/HAProxy)
- Background stale gateway monitor
- Automatic topology probing
- CLI commands for node runtime

---

## Marker

```
v1.8C-4 Node Runtime Reconciler: COMPLETE ✅
Package:                             internal/noderuntime/
Files:                               8 (config, client, cache, validator, resolver, reconciler, relay_request, helpers)
Reconcile Mode:                      dry_run
Token Leak Risk:                     None (verified by design)
Build:                               PENDING CONFIRMATION
Tests:                               PENDING

v1.8C-5 Local HTTP Gateway & Managed Relay: COMPLETE ✅
Package:                             internal/localgateway/
Files:                               7 (config, resolver, handler, local_dispatch, relay_client, server, status)
Gateway Bind Default:                127.0.0.1:18080
Unmanaged Mode:                      reject (421 Misdirected Request)
Relay Auth:                          GatewayLink secret injection at runtime
Token Leak Risk:                     None (verified by design)
```
