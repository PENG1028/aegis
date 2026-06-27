# v1.8C-0 — Horizontal Sync / Desired State Design

> **Phase:** v1.8C — Multi-node Aegis Runtime
> **Status:** DESIGN COMPLETE (no code)
> **Date:** 2026-06-27

---

## Table of Contents

1. [Design Principles](#1-design-principles)
2. [Control Plane = Source of Truth](#2-control-plane--source-of-truth)
3. [Desired State Model](#3-desired-state-model)
4. [Actual State Model](#4-actual-state-model)
5. [Sync Flow](#5-sync-flow)
6. [Pull API](#6-pull-api)
7. [Full Snapshot + Revision + Hash](#7-full-snapshot--revision--hash)
8. [State JSON Schema](#8-state-json-schema)
9. [Admin APIs](#9-admin-apis)
10. [Failure Modes](#10-failure-modes)
11. [Why Not Alternatives](#11-why-not-alternatives)

---

## 1. Design Principles

### Core Principle

**Control plane = source of truth. Node = pull desired state + reconcile + report actual.**

This is a deliberate architectural choice. The control plane stores the desired
configuration for every node. Each node independently pulls its own configuration
and reconciles its local state. There is no SSH push from the control plane to
nodes, no multi-primary database, and no P2P gossip.

### Decision Tree

```
Why not DB multi-primary?
  - Requires Raft/Paxos consensus protocol
  - Adds operational complexity (3+ nodes, quorum maintenance)
  - SQLite is fundamentally single-writer
  - Overkill for a control plane that manages < 100 nodes

Why not SSH push?
  - Requires SSH credentials on control plane
  - Push failure handling is complex (retry, rollback, partial success)
  - Does not scale well (linear with node count)
  - Control plane becomes stateful (must track push progress per node)

Why not P2P gossip?
  - Eventual consistency with no central verification
  - Hard to audit "what was the desired state at time T"
  - Gossip convergence is unpredictable under network partition
  - Overkill for a system where control plane is already a single process

Why pull?
  - Node is in control of its own reconciliation timing
  - Control plane stays stateless (no push connections to manage)
  - Natural backpressure: if control plane is overloaded, nodes retry
  - Simple failure model: node retries until success
  - Audit-friendly: desired state is a versioned document
```

---

## 2. Control Plane = Source of Truth

### 2.1 Write Path

```
Admin API (mutation)
  │
  ▼
Validate (permission, schema, consistency)
  │
  ▼
Apply to DB (routes, services, endpoints, gateways, policies, etc.)
  │
  ▼
Generate Desired State for affected nodes
  ├── Determine which nodes are affected by this change
  ├── Generate state_json per node
  ├── Increment revision counter
  └── Store in node_desired_states table
  │
  ▼
Node (eventually) pulls new desired state via heartbeat or timer
```

### 2.2 Read Path

```
Node heartbeat:
  Node: POST /api/node/v1/heartbeat
        {node_id, applied_revision: 42}
  Control plane:
        Compare applied_revision vs latest desired revision
        Response: {latest_revision: 43}

Node pulls:
  Node: GET /api/node/v1/desired-state
        Headers: X-Aegis-Node-ID, X-Aegis-Node-Token
  Control plane:
        Look up latest state_json for this node
        Response: {revision, state_hash, state_json, ...}

Node reconciles:
  Node applies state_json locally
  Node reports on next heartbeat:
        applied_revision: 43
        status: online (or degraded with error)
```

### 2.3 Revision as Monotonic Counter

The revision is **global** across all nodes (not per-node). This allows:

- Simple comparison: `applied_revision < latest_revision` means node is behind
- Ordered view of configuration evolution
- Easy audit: "at revision 100, node X was on revision 42"

However, **state_json is per-node**. When a change affects multiple nodes,
each node gets its own state_json, but all share the same revision number.

```
Global revision:   ... 40, 41, 42, 43, 44, ...
Node A's state:          |   |       |
Node B's state:              |   |       |
Node C's state:                  |   |

- Revision 41: changes affecting Node A and Node B
  → Node A gets new state_json(41), Node B gets new state_json(41)
  → Node C's state_json(40) is unchanged but still stored as rev 41
  → Node C's desired state stays at revision 40 until it pulls

When Node C heartbeats with applied_revision=40, control plane sees
latest_revision=41, but Node C's desired state at revision 41 is the same
as its desired state at revision 40 (no relevant changes). However, since
the global revision advanced, Node C must pull to confirm there's nothing
new for it. The state_hash comparison tells the node whether the content
actually changed.

Alternative: Per-node revision (simpler, recommended for v1.8C-1):
  - Each node_desired_states row has its own revision counter
  - Heartbeat response: latest_revision = node's own revision
  - No global counter needed
  - Simpler to implement, easier to reason about
```

**v1.8C-1 recommendation: Per-node revision.** Simpler implementation, lower
complexity, sufficient for the initial node count (< 100).

---

## 3. Desired State Model

### 3.1 Table: `node_desired_states`

```
node_id              TEXT PRIMARY KEY    // which node this desired state belongs to
revision             INTEGER NOT NULL    // per-node monotonic revision
state_hash           TEXT NOT NULL        // SHA-256 of state_json (for quick comparison)
state_json           TEXT NOT NULL        // full desired state for this node (JSON)
created_at           TEXT NOT NULL        // when this desired state was created
created_by           TEXT NOT NULL        // what triggered this state (admin action, system)
reason               TEXT                 // human-readable description of what changed
```

### 3.2 SQL (migration N+1)

```sql
CREATE TABLE IF NOT EXISTS node_desired_states (
    node_id     TEXT PRIMARY KEY,
    revision    INTEGER NOT NULL DEFAULT 0,
    state_hash  TEXT NOT NULL DEFAULT '',
    state_json  TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL DEFAULT '',
    reason      TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (node_id) REFERENCES nodes(node_id)
);

CREATE INDEX IF NOT EXISTS idx_desired_states_revision
    ON node_desired_states(revision);
```

### 3.3 state_json Content

```json
{
  "revision": 42,
  "node_id": "nd_c",
  "generated_at": "2026-06-27T10:00:00Z",

  "gateways": [
    {
      "gateway_id": "gw_abc",
      "type": "public",
      "provider": "caddy",
      "bind_addr": "0.0.0.0",
      "port": 443,
      "tls_enabled": true,
      "domains": ["api.example.com"]
    }
  ],

  "listeners": [
    {
      "provider": "caddy",
      "bind_addr": "0.0.0.0",
      "port": 443,
      "protocol": "https",
      "purpose": "public_tls_mux"
    }
  ],

  "relay_dispatch_routes": [
    {
      "route_id": "rt_xxx",
      "domain": "api.example.com",
      "target_local_port": 3001,
      "gateway_link_id": "gl_abc",
      "auth_required": true
    }
  ],

  "gateway_links_relevant": [
    {
      "gateway_link_id": "gl_abc",
      "target_node_id": "nd_b",
      "encrypted_secret": "...",
      "secret_nonce": "...",
      "secret_version": 2
    }
  ],

  "routing_table": {
    "revision": 42,
    "entries": [
      {
        "domain": "api.example.com",
        "route_id": "rt_xxx",
        "service_id": "svc_yyy",
        "endpoint_id": "ep_zzz",
        "target_node_id": "nd_c",
        "target_local_port": 3001,
        "candidates": [
          {
            "mode": "local_gateway",
            "gateway_url": "http://127.0.0.1:8080",
            "priority": 0,
            "requires_gateway_link": false
          },
          {
            "mode": "private_gateway",
            "gateway_url": "http://10.0.0.3:80",
            "priority": 1,
            "requires_gateway_link": true,
            "gateway_link_id": "gl_def"
          }
        ]
      }
    ]
  },

  "service_gateway_policies": [
    {
      "policy_id": "pol_abc",
      "service_id": "svc_yyy",
      "mode": "auto",
      "allow_local": true,
      "allow_private": true,
      "allow_public": false,
      "require_gateway_link": true
    }
  ],

  "secrets_relevant": [
    {
      "name": "gateway_link_gl_abc",
      "encrypted_value": "...",
      "nonce": "...",
      "version": 2
    }
  ],

  "diagnostics_config": {
    "enabled": true,
    "interval_seconds": 300,
    "providers": ["caddy"]
  }
}
```

### 3.4 state_json Content Rules

1. **Minimal per-node scope.** Each node only receives data relevant to itself.
   - Node A does not receive Node B's gateway links
   - Node A does not receive Node B's routing table entries
   - Exception: routing table entries for domains Node A is authorized to access

2. **Self-contained.** The state_json contains all configuration the node needs.
   No additional DB queries needed after pulling desired state.

3. **Deterministic.** The same desired state inputs produce the same state_json.
   (For a given revision and node_id, state_json is identical.)

4. **Versioned.** Every state_json has a revision. The node tracks which revision
   it has last applied.

---

## 4. Actual State Model

### 4.1 Table: `node_actual_states`

```
node_id              TEXT PRIMARY KEY    // which node this actual state belongs to
applied_revision     INTEGER NOT NULL    // revision of desired state that was applied
status               TEXT NOT NULL        // online | offline | degraded | unknown
last_apply_at        TEXT                 // when the node last attempted apply
last_success_at      TEXT                 // when the node last successfully applied
last_error           TEXT                 // last error message (null if no error)

provider_status      TEXT                 // JSON summary of provider status
relay_status         TEXT                 // JSON summary of relay handler status
gateway_status       TEXT                 // JSON summary of local gateway status
diagnostics_status   TEXT                 // JSON summary of last diagnostic run
```

### 4.2 SQL (migration N+1)

```sql
CREATE TABLE IF NOT EXISTS node_actual_states (
    node_id            TEXT PRIMARY KEY,
    applied_revision   INTEGER NOT NULL DEFAULT 0,
    status             TEXT NOT NULL DEFAULT 'unknown',
    last_apply_at      TEXT,
    last_success_at    TEXT,
    last_error         TEXT,
    provider_status    TEXT DEFAULT '{}',
    relay_status       TEXT DEFAULT '{}',
    gateway_status     TEXT DEFAULT '{}',
    diagnostics_status TEXT DEFAULT '{}',
    FOREIGN KEY (node_id) REFERENCES nodes(node_id)
);
```

### 4.3 Control Plane Reconciliation Query

```sql
-- Find all nodes that are behind desired state
SELECT n.node_id, n.node_name, n.status,
       ds.revision AS desired_revision,
       COALESCE(na.applied_revision, 0) AS applied_revision
FROM nodes n
JOIN node_desired_states ds ON ds.node_id = n.node_id
LEFT JOIN node_actual_states na ON na.node_id = n.node_id
WHERE n.status IN ('online', 'degraded')
  AND COALESCE(na.applied_revision, 0) < ds.revision;

-- Find all nodes that have errors
SELECT n.node_id, n.node_name, n.status, na.last_error
FROM nodes n
JOIN node_actual_states na ON na.node_id = n.node_id
WHERE n.status = 'degraded'
   OR na.status = 'degraded';
```

---

## 5. Sync Flow

### 5.1 Detailed Sequence

```
Node                                       Control Plane
───                                         ─────────────

1. HEARTBEAT
   POST /api/node/v1/heartbeat                │
   {applied_revision: 42}                     │
                                              ▼
                                   Compare: applied(42) vs desired(42)
                                   Same → nothing to do
                                   Response: {latest_revision: 42,
                                              desired_state_changed: false}
                                              ▲
   Wait 30s before next heartbeat             │
                                              │
2. HEARTBEAT (30s later)                      │
   {applied_revision: 42}                     │
                                              ▼
                                   Compare: applied(42) vs desired(43)
                                   Behind! → state available
                                   Response: {latest_revision: 43,
                                              desired_state_changed: true}
                                              ▲
   Detected: desired state behind              │
                                              │
3. PULL DESIRED STATE                         │
   GET /api/node/v1/desired-state             │
   (no body, no If-None-Match yet)            │
                                              ▼
                                   Look up state_json for node at rev 43
                                   Response: 200
                                   {revision: 43,
                                    state_hash: "sha256abc...",
                                    state_json: {...}}
                                              ▲
                                              │
4. VALIDATE + RECONCILE                       │
   Node validates:                             │
   - SHA-256(state_json) == state_hash? ✅
   - JSON schema valid? ✅
   - Required fields present? ✅
                                              │
   Node reconciles local state:                │
   - Update Caddy config (providers)          │
   - Update HAProxy config (if any)           │
   - Update local routing table cache         │
   - Update local gateway routes              │
   - Update relay dispatch table              │
   - Reload providers if config changed       │
                                              │
   Write local applied_revision = 43          │
                                              │
5. NEXT HEARTBEAT                              │
   POST /api/node/v1/heartbeat                │
   {applied_revision: 43,                     │
    status: "online"}                         │
                                              ▼
                                   Update node_actual_states
                                   applied_revision=43, status=online,
                                   last_success_at=now
                                   Response: {latest_revision: 43,
                                              desired_state_changed: false}
```

### 5.2 Error Recovery

```
A. Desired state pull fails (network error):
   1. Node retries after heartbeat interval
   2. No change to local state (still on old revision)
   3. Heartbeat reports current applied_revision with last_error

B. Desired state validation fails (hash mismatch):
   1. Node rejects state_json, does NOT apply
   2. Heartbeat reports last_error: "state_hash mismatch"
   3. Node retries pull on next heartbeat
   4. Admin investigates: "was state_json corrupted in transit or generation?"

C. Local reconcile fails (provider error):
   1. Node records last_error with details
   2. Node remains on previous revision (does NOT advance applied_revision)
   3. Heartbeat reports last_error, status: "degraded"
   4. Node retries reconcile on next heartbeat

D. Control plane restarts:
   1. Existing desired states are persisted in SQLite
   2. On restart, control plane re-reads node_desired_states
   3. Nodes continue heartbeating and pulling as normal
   4. No data loss (desired state is SQLite-persistent)

E. Node restarts:
   1. Node reads local cached routing table + applied_revision from local DB
   2. Node starts heartbeat with last known applied_revision
   3. Control plane responds with latest_revision
   4. If behind → pull desired state and reconcile
   5. No full state re-download needed (unless local cache is lost)
```

---

## 6. Pull API

### 6.1 Node API Endpoints

```
POST   /api/node/v1/join              → Register new node (with join token)
POST   /api/node/v1/heartbeat         → Report node status, get latest revision
GET    /api/node/v1/desired-state     → Pull full desired state JSON
GET    /api/node/v1/routing-table     → Pull routing table subset (ETag support)
```

### 6.2 Heartbeat API

```
POST /api/node/v1/heartbeat

Request:
{
  "node_id": "nd_c",
  "applied_revision": 42,
  "status": "online",
  "hostname": "server-c",
  "agent_version": "v1.8C",

  "public_ip": "<SERVER_A_NODE_IP>",
  "private_ip": "10.0.0.3",

  "listeners": [
    {"port": 80, "protocol": "http", "status": "active"},
    {"port": 443, "protocol": "https", "status": "active"}
  ],

  "provider_status": {
    "caddy": {"status": "active", "pid": 1234, "uptime_seconds": 3600}
  },

  "relay_status": {
    "enabled": true,
    "requests_served": 42,
    "errors_last_hour": 0
  },

  "local_gateway_status": {
    "enabled": true,
    "uptime_seconds": 3600
  },

  "diagnostics_summary": {
    "last_run_at": "2026-06-27T09:55:00Z",
    "status": "pass",
    "errors": []
  },

  "last_error": null
}

Response 200:
{
  "latest_revision": 43,
  "desired_state_changed": true,
  "server_time": "2026-06-27T10:00:00Z",
  "heartbeat_interval_seconds": 30,
  "actions": []   // control plane to node directives (optional)
}
```

### 6.3 Desired State API

```
GET /api/node/v1/desired-state
Headers:
  X-Aegis-Node-ID: nd_c
  X-Aegis-Node-Token: <secret>
  If-None-Match: "sha256abc..."   // optional ETag

Response 200:
{
  "revision": 43,
  "state_hash": "sha256abc...",
  "state_json": {
    ...  // full state JSON (see section 3.3)
  },
  "created_at": "2026-06-27T10:00:00Z",
  "created_by": "admin",
  "reason": "added route api.example.com → server-c:3001"
}

Response 304 (Not Modified):
  (empty body — state_hash unchanged)
```

---

## 7. Full Snapshot + Revision + Hash

### 7.1 Why Full Snapshot (not incremental diff)

| Aspect | Full Snapshot | Incremental Diff |
|--------|--------------|-----------------|
| Complexity | Low | High (conflict resolution, ordering) |
| Reliability | Absolute (node has complete state) | Conditional (depends on diff chain integrity) |
| Node recovery | Pull once, reconcile | Must replay all diffs from base state |
| State hash | Single hash of full JSON | Must hash accumulated state |
| Audit | Snapshot is self-contained | Diffs require full chain to interpret |
| Bandwidth | Higher (full state per pull) | Lower (only changed fields) |
| Storage | Full copy per revision | Base + chain of diffs |

**Decision: Full snapshot for v1.8C-1.** Simplicity and reliability are more
important than bandwidth optimization at this stage. If bandwidth becomes a
concern, add `If-None-Match` / `304 Not Modified` support first, then
incremental diffs if still needed.

### 7.2 Revision + Hash Guarantees

```
Revision:
  - Monotonic integer per node
  - Incremented atomically when desired state changes
  - Node uses revision to decide "do I need to pull?"
  - Control plane uses revision to detect "is node behind?"

State Hash:
  - SHA-256 of the canonical JSON serialization of state_json
  - Node validates hash after pulling: SHA-256(received_json) == state_hash?
  - Control plane stores hash alongside state_json for quick comparison
  - ETag header value = state_hash (for HTTP caching)

Guarantee:
  If node's applied_revision == control plane's latest_revision,
  AND node's local routing table's hash matches desired state hash,
  THEN node has the complete and correct desired state.
```

### 7.3 Consistency Verification

```
Admin can verify consistency at any time:

  GET /api/admin/v1/nodes/{id}/consistency

  Response:
  {
    "node_id": "nd_c",
    "node_reported_applied_revision": 42,
    "control_plane_latest_revision": 43,
    "consistent": false,
    "behind_by": 1,
    "last_error": "provider error: caddy reload failed"
  }

  GET /api/admin/v1/nodes/consistency  (all nodes)

  Response:
  {
    "consistent_count": 5,
    "behind_count": 2,
    "error_count": 1,
    "offline_count": 0,
    "total": 8,
    "details": [
      {"node_id": "nd_c", "consistent": false, "behind_by": 1},
      {"node_id": "nd_d", "consistent": false, "behind_by": 3},
      {"node_id": "nd_e", "consistent": false, "status": "degraded",
       "last_error": "caddy config syntax error"}
    ]
  }
```

---

## 8. State JSON Schema

### 8.1 Per-section Schema Validation

Each section of `state_json` must conform to its schema. The control plane
validates the generated state_json before storing. The node validates the
received state_json before applying.

```
state_json structure validation:
  - revision:     required, integer, > 0
  - node_id:      required, string, matches node identity
  - generated_at: required, ISO 8601 timestamp
  - gateways:     optional, array, each item must have gateway_id, type, provider
  - listeners:    optional, array, each item must have provider, bind_addr, port
  - relay_dispatch_routes:  optional, array
  - gateway_links_relevant: optional, array
  - routing_table:          optional, object with entries array
  - service_gateway_policies: optional, array
  - secrets_relevant:       optional, array (encrypted only, never plaintext)
  - diagnostics_config:     optional, object

Node-side validation (pre-apply):
  1. state_json.revision == envelope.revision
  2. SHA-256(canonical_json) == envelope.state_hash
  3. All required sections present
  4. No unknown top-level keys
  5. Required fields within each section present
```

---

## 9. Admin APIs

### 9.1 Node Management

```
GET    /api/admin/v1/nodes                     → List all nodes (with status)
GET    /api/admin/v1/nodes/{id}                → Get node details
GET    /api/admin/v1/nodes/{id}/health         → Get node health (from actual state)
GET    /api/admin/v1/nodes/{id}/routes         → Get routes on this node (from desired state)
GET    /api/admin/v1/nodes/{id}/gateways       → Get gateways on this node
GET    /api/admin/v1/nodes/{id}/desired-state  → Get desired state for this node
GET    /api/admin/v1/nodes/{id}/actual-state   → Get actual state reported by this node
POST   /api/admin/v1/nodes/{id}/refresh        → Force desired state regeneration

DELETE /api/admin/v1/nodes/{id}                → Remove node (requires force flag)
```

### 9.2 Desired State Management

```
GET    /api/admin/v1/desired-states            → List all desired states (revision only)
GET    /api/admin/v1/desired-states/{node_id}  → Get full desired state for node
POST   /api/admin/v1/desired-states/regenerate → Force-regenerate all desired states
```

### 9.3 Node Event Log

```
GET    /api/admin/v1/node-events               → List all node events (existing, reused)
GET    /api/admin/v1/node-events?node_id=nd_c  → Filter by node
```

---

## 10. Failure Modes

### 10.1 Control Plane Unreachable

```
Scenario: Node cannot reach control plane for heartbeats or desired state pull.

Node behavior:
  1. Retry heartbeat with exponential backoff (30s → 60s → 120s → max 300s)
  2. Continue operating with last known desired state
  3. Local routing table + gateway config remain active
  4. Relay requests continue to work (no dependency on control plane for dispatch)
  5. After N retries without success, mark local status as "degraded"
  6. Continue retrying indefinitely

Failure window:
  - Node continues operating normally during control plane outage
  - Node cannot apply configuration changes during outage
  - Route/service/policy changes are queued on control plane but not dispatched
  - Once control plane recovers, node syncs to latest revision
```

### 10.2 Node Unreachable

```
Scenario: Node stops heartbeating or responds with errors.

Control plane behavior:
  1. After 3 missed heartbeats → mark node status as "offline"
  2. Admin is alerted (via node-events or health API)
  3. Desired state for offline node is NOT automatically purged
  4. Routes pointing to offline node's endpoints continue to exist
  5. Other nodes routing to the offline node... what happens?
     - Heartbeat tells other nodes nothing about offline node
     - Other nodes attempt relay → connection failure → 502 TARGET_UNREACHABLE
     - If multi-gateway policy with fallback → next candidate tried
     - If single-gateway or auto → unavailable

Node recovery:
  1. Node reconnects, sends heartbeat
  2. Control plane marks status as "online"
  3. Node pulls latest desired state
  4. Node reconciles
```

### 10.3 Conflicting Writes

```
Scenario: Two admin operations modify the same route/service.

Control plane behavior:
  1. First write succeeds, updates DB, generates new desired state
  2. Second write fails with "conflict" (optimistic locking)
  3. Admin must retry after first change is applied
  4. Desired state generation is idempotent: same inputs → same outputs
  5. Revision is incremented atomically per successful write

Note: This is the same optimistic locking pattern used in v1.7+.
No new conflict resolution logic is needed for v1.8C.
```

---

## 11. Why Not Alternatives

### 11.1 Multi-primary DB (Raft)

| Consideration | Raft Multi-primary | Desired State Pull |
|---------------|-------------------|-------------------|
| Consistency | Strong (Raft) | Strong (control plane SSOT) |
| Complexity | High (Raft impl, election, log) | Low (HTTP pull) |
| Write throughput | Limited by Raft commit | Single node write |
| Node count limit | 3-7 (practical) | 100+ (practical) |
| Operational burden | High (3-5 nodes) | Low (1 node + backups) |
| Recovery | Complex (snapshot + log replay) | Simple (backup restore) |

**Decision: Desired state pull.** The simplicity and lower operational burden
outweigh Raft's availability advantage for this use case.

### 11.2 SSH Push

| Consideration | SSH Push | Desired State Pull |
|---------------|----------|-------------------|
| Latency | Low (push is immediate) | Medium (30s heartbeat) |
| Control plane state | Stateful (tracks push progress) | Stateless (stores desired state) |
| Failure handling | Complex (partial push, retry) | Simple (node retries) |
| Credential management | SSH keys on control plane | Node credentials (no shared keys) |
| Audit trail | Push logs | Versioned desired state |

**Decision: Desired state pull.** The 30-second latency is acceptable for
configuration changes. The operational simplicity and failure isolation are
more important.

### 11.3 P2P Gossip (SWIM, etc.)

| Consideration | P2P Gossip | Desired State Pull |
|---------------|-----------|-------------------|
| Convergence | Eventual | Eventual (but verifiable) |
| Verification | Hard (no central hash) | Easy (state_hash per node) |
| Audit | Hard (distributed log) | Easy (central desired state) |
| Network traffic | O(N²) messages | O(N) HTTP requests |
| Partition behavior | Forked state | Node stays on last state |

**Decision: Desired state pull.** The auditability and verification properties
are critical for a control plane. Gossip's eventual consistency model makes
it hard to answer "did node X receive the expected configuration?"

---

## Marker

```
v1.8C-0 Horizontal Sync / Desired State Design:    COMPLETE ✅
```
