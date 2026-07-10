# v1.8C-0 — Multi-node Runtime Data Gap Analysis

> **Phase:** v1.8C — Multi-node Aegis Runtime
> **Status:** DATA GAP ANALYSIS COMPLETE (v1.8C-1: 6 gaps filled ✅ | v1.8C-2A: gateway auto-discovery partial ✅ | v1.8C-3: service_gateway_policies + route_gateway_policies + routing table generator ✅ | v1.8C-4: node runtime reconciler + routing table consumption ✅ | v1.8C-5: local HTTP gateway + managed relay + GatewayLink secret injection ✅ | v1.8C-6: real multi-node local gateway acceptance ✅)
> **Date:** 2026-06-27
> **Purpose:** Identify missing data fields required for Multi-node Aegis Runtime implementation

---

## Table of Contents

1. [Current Schema Inventory](#1-current-schema-inventory)
2. [Gap Details](#2-gap-details)
3. [Gap Summary Table](#3-gap-summary-table)
4. [Migration Draft](#4-migration-draft)
5. [v1.8C-1 Blockers](#5-v18c-1-blockers)

---

## 1. Current Schema Inventory

As of v1.8B (migration 027):

### `nodes` (migration 010 + 011 + 012 + 021)

```sql
id              TEXT PRIMARY KEY,       -- DB internal ID
node_id         TEXT NOT NULL,           -- nd_a, nd_b, node_<hostname>
hostname        TEXT NOT NULL,
local_ip        TEXT DEFAULT '127.0.0.1',
private_ip      TEXT DEFAULT '',
public_ip       TEXT DEFAULT '',
is_current      INTEGER DEFAULT 0,
is_leader       INTEGER DEFAULT 0,       -- migration 011
state_version   INTEGER DEFAULT 0,       -- migration 012
ip_migrated     INTEGER DEFAULT 0,
capabilities    TEXT DEFAULT '{}',        -- migration 021
last_seen       TEXT NOT NULL,
created_at      TEXT NOT NULL,
updated_at      TEXT NOT NULL
```

**What's missing for v1.8C:**
- `node_name` — human-readable name (separate from node_id)
- `role` — control_plane | gateway | worker | relay | dev
- `region` — datacenter / region
- `network_id` — private network group
- `agent_version` — aegis binary version
- `os` / `arch` — OS type and architecture
- `last_heartbeat_at` — separate from last_seen (which is any update)
- `last_error` — last error message
- `public_reachable` / `private_reachable` — network capability flags
- `public_gateway_enabled` / `private_gateway_enabled` — gateway capability flags

### `trusted_gateways` (migration 024 + 026 + 027)

```sql
id                  TEXT PRIMARY KEY,
name                TEXT DEFAULT '',
host                TEXT DEFAULT '',
private_ip          TEXT DEFAULT '',
port                INTEGER DEFAULT 443,
auth_type           TEXT DEFAULT 'shared_secret',
auth_value          TEXT DEFAULT '',
gateway_type        TEXT DEFAULT 'upstream',
auto_route          INTEGER DEFAULT 1,
status              TEXT DEFAULT 'active',
target_node_id      TEXT DEFAULT '',        -- migration 026
encrypted_secret    TEXT DEFAULT '',        -- migration 027
secret_nonce        TEXT DEFAULT '',        -- migration 027
secret_version      INTEGER DEFAULT 0,      -- migration 027
secret_created_at   TEXT DEFAULT '',        -- migration 027
secret_rotated_at   TEXT DEFAULT '',        -- migration 027
created_at          TEXT NOT NULL,
updated_at          TEXT NOT NULL
```

**What's missing for v1.8C:**
- `source_node_id` — which node this gateway link originates from (missing)

### Other Tables

`endpoints` (migration 001 + 026): Has `node_id` (+ type, address, enabled).

`routes` (migration 001 + 025): Has `gateway_link_id`, `domain`, `service_id`,
`path_prefix`, `status`, TLS, maintenance fields.

`services` (migration 001): Has `kind`, `status`, `env`.

`gateway_domains`, `gateway_routes`, `gateway_listeners` (migration 022): Existing
abstraction tables (v1.7). These are a starting point but need significant
expansion for the multi-gateway model.

`deployments`, `deployment_instances` (migration 023): Existing deployment
version tracking tables.

`node_events` (migration 020): Existing event log table.

### Not Yet Created

The following tables do NOT exist yet:

| Table | Needed for | Blocking v1.8C-1? |
|-------|-----------|-------------------|
| `join_tokens` | Node bootstrap | 🔴 Yes |
| `node_credentials` | Node auth | 🔴 Yes |
| `node_desired_states` | Desired state | 🔴 Yes |
| `node_actual_states` | Actual state | 🔴 Yes |
| `gateways` | Gateway inventory | 🟡 Yes (extend existing gateway_domains/listeners) |
| `service_gateway_policies` | Gateway policy | 🟡 Yes |
| `topology_edges` | Topology matrix | 🟢 No (nice to have) |
| `routing_table_cache` | Local routing | 🟡 Yes (node-local, not in control plane DB) |

---

## 2. Gap Details

### Gap 1: Node Fields Expansion

**Current:** `nodes` table has basic identity fields (id, node_id, hostname, IPs).

**Need for v1.8C:**
- `node_name` TEXT — human-readable name (defaults to hostname)
- `role` TEXT — control_plane | gateway | worker | relay | dev
- `region` TEXT — optional datacenter/region
- `network_id` TEXT — optional private network group
- `agent_version` TEXT — e.g. "v1.8C"
- `os` TEXT — linux, darwin, windows
- `arch` TEXT — amd64, arm64
- `status` TEXT — online | offline | degraded | unknown
- `last_heartbeat_at` TEXT — last heartbeat timestamp
- `last_error` TEXT — last error message
- `public_reachable` INTEGER — node has public IP + open ports
- `public_gateway_enabled` INTEGER — node acts as public gateway
- `private_reachable` INTEGER — node accessible on private network
- `private_gateway_enabled` INTEGER — node acts as private gateway

**Risk if not filled:** Node cannot self-describe. Control plane cannot
distinguish gateway-capable nodes from workers. Cannot determine topology.

**Migration estimate:** 2-3 ALTER TABLE statements or new columns.

---

### Gap 2: `join_tokens` Table (MISSING)

**Current:** Does not exist.

**Need for v1.8C:**
```sql
CREATE TABLE join_tokens (
    join_token_id   TEXT PRIMARY KEY,
    token_hash      TEXT NOT NULL,       -- SHA-256 of actual token
    description     TEXT NOT NULL DEFAULT '',
    bound_roles     TEXT DEFAULT '',     -- comma-separated, empty = any
    bound_node_name TEXT DEFAULT '',
    bound_source_ip TEXT DEFAULT '',
    expires_at      TEXT NOT NULL,
    used_at         TEXT,                -- null until used
    used_by_node_id TEXT,
    created_at      TEXT NOT NULL,
    created_by      TEXT NOT NULL
);
```

**Risk if not filled:** Cannot bootstrap new nodes. No controlled registration.

**Migration estimate:** 1 new table (CREATE TABLE).

---

### Gap 3: `node_credentials` Table (MISSING)

**Current:** Does not exist. Node authentication is implicit (no credential system).

**Need for v1.8C:**
```sql
CREATE TABLE node_credentials (
    credential_id   TEXT PRIMARY KEY,
    node_id         TEXT NOT NULL,
    node_secret_hash TEXT NOT NULL,      -- SHA-256 of node_secret
    valid_from      TEXT NOT NULL,
    expires_at      TEXT,                -- null = never expires
    rotated_at      TEXT,
    created_at      TEXT NOT NULL,
    FOREIGN KEY (node_id) REFERENCES nodes(node_id)
);
```

**Risk if not filled:** No way to authenticate node → control plane communication.
Cannot secure heartbeat, desired state pull, or actual state report APIs.

**Migration estimate:** 1 new table + join API handler + node auth middleware.

---

### Gap 4: `node_desired_states` Table (MISSING)

**Current:** Does not exist.

**Need for v1.8C:**
```sql
CREATE TABLE node_desired_states (
    node_id      TEXT PRIMARY KEY,
    revision     INTEGER NOT NULL DEFAULT 0,
    state_hash   TEXT NOT NULL DEFAULT '',
    state_json   TEXT NOT NULL DEFAULT '{}',
    created_at   TEXT NOT NULL DEFAULT '',
    created_by   TEXT NOT NULL DEFAULT '',
    reason       TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (node_id) REFERENCES nodes(node_id)
);
```

**Risk if not filled:** Cannot distribute configuration to nodes. Core of the
desired state sync model.

**Migration estimate:** 1 new table + desired state generation logic + pull API.

---

### Gap 5: `node_actual_states` Table (MISSING)

**Current:** Does not exist.

**Need for v1.8C:**
```sql
CREATE TABLE node_actual_states (
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

**Risk if not filled:** Cannot track which nodes are up to date. Cannot detect
reconciliation failures.

**Migration estimate:** 1 new table + heartbeat handler update.

---

### Gap 6: `gateways` Inventory Table (IMPLEMENTED ✅ — v1.8C-2 + v1.8C-2A)

**Current:** `gateway_domains`, `gateway_routes`, `gateway_listeners` exist from
v1.7, but they model Caddy/HAProxy domains and routes, not the new gateway
inventory abstraction.

**v1.8C-2:** `gateways` table created (migration 030), full CRUD via admin API.

**v1.8C-2A:** Heartbeat gateway status reporting added. Gateway auto-discovery
via name-based upsert: partial (upsert by node_id + name works, background stale
detection deferred). Gateway status semantics: online/degraded/offline/unknown.
Node ownership enforced (gateway_id cross-node update rejected).

**Need for v1.8C:**
```sql
CREATE TABLE gateways (
    gateway_id          TEXT PRIMARY KEY,
    node_id             TEXT NOT NULL,
    name                TEXT NOT NULL DEFAULT '',
    type                TEXT NOT NULL,    -- local | private | public
    provider            TEXT NOT NULL,    -- caddy | haproxy | aegis
    bind_addr           TEXT NOT NULL DEFAULT '0.0.0.0',
    host                TEXT DEFAULT '',  -- hostname or IP for routing
    port                INTEGER NOT NULL,
    scheme              TEXT NOT NULL DEFAULT 'http',  -- http | https
    public_accessible   INTEGER DEFAULT 0,
    private_accessible  INTEGER DEFAULT 0,
    enabled             INTEGER DEFAULT 1,
    priority            INTEGER DEFAULT 100,
    status              TEXT DEFAULT 'active',
    last_verified_at    TEXT,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    FOREIGN KEY (node_id) REFERENCES nodes(node_id)
);
```

**Risk if not filled:** Cannot do multi-gateway selection. Cannot distinguish
public vs private gateways per node.

**Migration estimate:** 1 new table. The old gateway_domains etc. are separate
(they model rendered config, not gateway inventory).

---

### Gap 7: `service_gateway_policies` Table (MISSING)

**Current:** Does not exist. Gateway selection logic is implicit (hardcoded in
relay resolver).

**Need for v1.8C:**
```sql
CREATE TABLE service_gateway_policies (
    policy_id            TEXT PRIMARY KEY,
    service_id           TEXT,
    route_id             TEXT,
    mode                 TEXT NOT NULL DEFAULT 'auto',  -- auto | fixed | multi | disabled
    primary_gateway_id   TEXT,
    fallback_gateway_ids TEXT DEFAULT '[]',  -- JSON array
    allow_local          INTEGER DEFAULT 1,
    allow_private        INTEGER DEFAULT 1,
    allow_public         INTEGER DEFAULT 0,
    require_gateway_link INTEGER DEFAULT 1,
    require_relay        INTEGER DEFAULT 1,
    preserve_host        INTEGER DEFAULT 0,
    tls_mode             TEXT DEFAULT 'http_only',  -- http_only | terminate_local
    priority             INTEGER DEFAULT 0,
    updated_at           TEXT NOT NULL,
    FOREIGN KEY (service_id) REFERENCES services(id),
    FOREIGN KEY (route_id) REFERENCES routes(id)
);
```

**Risk if not filled:** No configurable gateway policy. All routes use the same
hardcoded selection logic.

**Migration estimate:** 1 new table + policy resolution in routing table generator.

---

### Gap 8: `topology_edges` Table (MISSING)

**Current:** Does not exist. Topology is implicitly derived by comparing
node IPs in the relay resolver.

**Need for v1.8C:**
```sql
CREATE TABLE topology_edges (
    edge_id              TEXT PRIMARY KEY,
    from_node_id         TEXT NOT NULL,
    to_node_id           TEXT NOT NULL,
    private_reachable    INTEGER DEFAULT 0,
    public_reachable     INTEGER DEFAULT 0,
    preferred_gateway_id TEXT,
    gateway_link_id      TEXT,
    status               TEXT DEFAULT 'unknown',  -- verified | missing_link | unreachable | degraded | unknown
    last_verified_at     TEXT,
    last_error           TEXT,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL,
    FOREIGN KEY (from_node_id) REFERENCES nodes(node_id),
    FOREIGN KEY (to_node_id) REFERENCES nodes(node_id),
    FOREIGN KEY (gateway_link_id) REFERENCES trusted_gateways(id)
);
```

**Risk if not filled:** Topology matrix cannot be displayed. Auto-detection of
reachability is not possible.

**Migration estimate:** 1 new table. Not blocking v1.8C-1 (topology matrix is
nice-to-have for first iteration).

---

### Gap 9: `trusted_gateways.source_node_id` Missing

**Current:** `trusted_gateways` has `target_node_id` (migration 026) but no
`source_node_id`.

**Need for v1.8C:** Add `source_node_id` column to `trusted_gateways` so each
link records which node it originates from. This allows the GatewayLink matrix
to display both source and target for every link.

```sql
ALTER TABLE trusted_gateways ADD COLUMN source_node_id TEXT DEFAULT '';
```

**Risk if not filled:** Cannot build GatewayLink matrix without knowing which
node each link originates from. Cross-reference would require IP matching
(ambiguous).

**Migration estimate:** 1 ALTER TABLE statement. Existing records can be
backfilled from context (e.g., current node for upstream links).

---

### Gap 10: Routing Table Cache (Node-local)

**Current:** No routing table caching mechanism. The resolver queries the DB
on every request.

**Need for v1.8C:** Node-local cache of routing table entries, keyed by domain.
Used by the local gateway to dispatch requests without round-tripping to
control plane.

This is node-local state, not a control plane DB table. It could be:
- In-memory cache (process lifetime)
- `routing-table.json` file on disk (persist across restarts)
- Local SQLite table in node-local DB

**Risk if not filled:** Local gateway must query control plane for every request.
Increased latency and control plane dependency.

**Migration estimate:** Node-local storage, not a migration.

---

### Gap 11: Node Event Types for Relay/Gateway Lifecycle

**Current:** `node_events` table exists (migration 020) but only supports basic
event types (node_online, node_offline, capability_changed, etc.).

**Need for v1.8C:** New event types:
- `node_heartbeat_missed` — node did not heartbeat within expected interval
- `node_registered` — new node joined
- `node_deregistered` — node was removed
- `desired_state_pulled` — node pulled desired state
- `desired_state_applied` — node applied desired state (success/fail)
- `gateway_link_created` — new gateway link
- `gateway_link_rotated` — gateway link rotated

**Risk if not filled:** Missing observability. Cannot audit node lifecycle events.

**Migration estimate:** No schema change (event type is just a string). Update
event logging in relevant handlers.

---

### Gap 12: Admin Credential for Node API

**Current:** Node API endpoints don't exist yet. The admin API uses session
cookie or bearer token auth. For node API, a different auth model is needed
(node credential, not admin session).

**Need for v1.8C:** A new auth middleware for `/api/node/v1/*` that validates
node credentials (node_id + node_secret) instead of admin sessions. This is
a middleware change, not a DB change.

**Risk if not filled:** Node API has no auth protection. Unauthorized nodes
could pull desired state or register.

**Migration estimate:** New middleware file + node auth check in existing
auth package. No DB migration needed.

---

## 3. Gap Summary Table

| # | Gap | Current State | v1.8C Requirement | Blocks 1.8C-1? | Est. Migration | Risk |
|---|-----|--------------|-------------------|----------------|----------------|------|
| 1 | Node fields expansion | Basic identity (id, node_id, hostname, IPs) | role, agent_version, os, arch, status, last_heartbeat_at, last_error, reachability flags | 🔴 Yes | Migration 028 | Node cannot self-describe |
| 2 | `join_tokens` table | MISSING | Token creation, validation, single-use, expiry | 🔴 Yes | Migration 029 | Cannot bootstrap nodes |
| 3 | `node_credentials` table | MISSING | Node secret storage, HMAC-SHA256 auth | 🔴 Yes | Migration 030 | No node auth |
| 4 | `node_desired_states` table | MISSING | Full desired state per node, revision, state_hash | 🔴 Yes | Migration 031 | Core sync model |
| 5 | `node_actual_states` table | MISSING | Applied revision, status, error tracking | 🔴 Yes | Migration 032 | Cannot track sync status |
| 6 | `gateways` inventory table | MISSING (v1.7 gateway_domains exist but different scope) | Gateway inventory with type, provider, accessibility flags | 🟡 Yes | Migration 033 | Cannot do multi-gateway |
| 7 | `service_gateway_policies` table | MISSING | Policy model (auto/fixed/multi/disabled), allow flags | 🟡 Yes | Migration 034 | Hardcoded selection only |
| 8 | `topology_edges` table | MISSING | Node-to-node reachability, preferred path | 🟢 No | Migration 035 | Nice-to-have |
| 9 | `trusted_gateways.source_node_id` | MISSING (has target_node_id) | source_node_id for link matrix | 🟡 Yes | Migration 028 (or 036) | Cannot build link matrix |
| 10 | Routing table cache (node-local) | MISSING | In-memory or file-based routing cache | 🟡 Yes | Node-local only | Latency on every dispatch |
| 11 | Node event types | Basic event types exist | New event types for registration, heartbeat, desired state, gateway lifecycle | 🟢 No | No schema change | Reduced observability |
| 12 | Node API auth middleware | MISSING | node_id + node_secret auth for `/api/node/v1/*` | 🔴 Yes | New middleware (no DB) | No node API security |

### Priority Summary

| Priority | Count | Gaps |
|----------|-------|------|
| 🔴 High (blocks v1.8C-1) | 6 | 1, 2, 3, 4, 5, 12 |
| 🟡 Medium (needed for v1.8C-1) | 4 | 6, 7, 9, 10 |
| 🟢 Low (nice to have) | 2 | 8, 11 |

---

## 4. Migration Draft

### Migration 028 (IMPLEMENTED v1.8C-1 ✅): Node fields + join_tokens + node_credentials

```sql
-- 1. Expand nodes table
ALTER TABLE nodes ADD COLUMN node_name TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN role TEXT DEFAULT 'worker';
ALTER TABLE nodes ADD COLUMN region TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN network_id TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN agent_version TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN os TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN arch TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN status TEXT DEFAULT 'unknown';
ALTER TABLE nodes ADD COLUMN last_heartbeat_at TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN last_error TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN public_reachable INTEGER DEFAULT 0;
ALTER TABLE nodes ADD COLUMN public_gateway_enabled INTEGER DEFAULT 0;
ALTER TABLE nodes ADD COLUMN private_reachable INTEGER DEFAULT 0;
ALTER TABLE nodes ADD COLUMN private_gateway_enabled INTEGER DEFAULT 0;

-- 2. Add source_node_id to trusted_gateways
ALTER TABLE trusted_gateways ADD COLUMN source_node_id TEXT DEFAULT '';

-- Create index for node lookups
CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
CREATE INDEX IF NOT EXISTS idx_nodes_role ON nodes(role);
CREATE INDEX IF NOT EXISTS idx_trusted_gateways_source_node ON trusted_gateways(source_node_id);
```

### Migration 029: join_tokens

```sql
CREATE TABLE IF NOT EXISTS join_tokens (
    join_token_id   TEXT PRIMARY KEY,
    token_hash      TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    bound_roles     TEXT NOT NULL DEFAULT '',
    bound_node_name TEXT NOT NULL DEFAULT '',
    bound_source_ip TEXT NOT NULL DEFAULT '',
    expires_at      TEXT NOT NULL,
    used_at         TEXT,
    used_by_node_id TEXT,
    created_at      TEXT NOT NULL,
    created_by      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_join_tokens_token_hash ON join_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_join_tokens_expires ON join_tokens(expires_at);
```

### Migration 030: node_credentials

```sql
CREATE TABLE IF NOT EXISTS node_credentials (
    credential_id    TEXT PRIMARY KEY,
    node_id          TEXT NOT NULL,
    node_secret_hash TEXT NOT NULL,
    valid_from       TEXT NOT NULL,
    expires_at       TEXT,
    rotated_at       TEXT,
    created_at       TEXT NOT NULL,
    FOREIGN KEY (node_id) REFERENCES nodes(node_id)
);

CREATE INDEX IF NOT EXISTS idx_node_credentials_node_id ON node_credentials(node_id);
```

### Migration 031: node_desired_states

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

CREATE INDEX IF NOT EXISTS idx_desired_states_revision ON node_desired_states(revision);
```

### Migration 032: node_actual_states

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

### Migration 033: gateways

```sql
CREATE TABLE IF NOT EXISTS gateways (
    gateway_id          TEXT PRIMARY KEY,
    node_id             TEXT NOT NULL,
    name                TEXT NOT NULL DEFAULT '',
    type                TEXT NOT NULL,
    provider            TEXT NOT NULL DEFAULT 'aegis',
    bind_addr           TEXT NOT NULL DEFAULT '0.0.0.0',
    host                TEXT DEFAULT '',
    port                INTEGER NOT NULL,
    scheme              TEXT NOT NULL DEFAULT 'http',
    public_accessible   INTEGER DEFAULT 0,
    private_accessible  INTEGER DEFAULT 0,
    enabled             INTEGER DEFAULT 1,
    priority            INTEGER DEFAULT 100,
    status              TEXT DEFAULT 'active',
    last_verified_at    TEXT,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    FOREIGN KEY (node_id) REFERENCES nodes(node_id)
);

CREATE INDEX IF NOT EXISTS idx_gateways_node_id ON gateways(node_id);
CREATE INDEX IF NOT EXISTS idx_gateways_type ON gateways(type);
```

### Migration 034: service_gateway_policies

```sql
CREATE TABLE IF NOT EXISTS service_gateway_policies (
    policy_id            TEXT PRIMARY KEY,
    service_id           TEXT,
    route_id             TEXT,
    mode                 TEXT NOT NULL DEFAULT 'auto',
    primary_gateway_id   TEXT,
    fallback_gateway_ids TEXT DEFAULT '[]',
    allow_local          INTEGER DEFAULT 1,
    allow_private        INTEGER DEFAULT 1,
    allow_public         INTEGER DEFAULT 0,
    require_gateway_link INTEGER DEFAULT 1,
    require_relay        INTEGER DEFAULT 1,
    preserve_host        INTEGER DEFAULT 0,
    tls_mode             TEXT DEFAULT 'http_only',
    priority             INTEGER DEFAULT 0,
    updated_at           TEXT NOT NULL,
    FOREIGN KEY (service_id) REFERENCES services(id),
    FOREIGN KEY (route_id) REFERENCES routes(id)
);

CREATE INDEX IF NOT EXISTS idx_gateway_policies_service ON service_gateway_policies(service_id);
CREATE INDEX IF NOT EXISTS idx_gateway_policies_route ON service_gateway_policies(route_id);
```

### Migration 035: topology_edges

```sql
CREATE TABLE IF NOT EXISTS topology_edges (
    edge_id              TEXT PRIMARY KEY,
    from_node_id         TEXT NOT NULL,
    to_node_id           TEXT NOT NULL,
    private_reachable    INTEGER DEFAULT 0,
    public_reachable     INTEGER DEFAULT 0,
    preferred_gateway_id TEXT,
    gateway_link_id      TEXT,
    status               TEXT DEFAULT 'unknown',
    last_verified_at     TEXT,
    last_error           TEXT,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL,
    FOREIGN KEY (from_node_id) REFERENCES nodes(node_id),
    FOREIGN KEY (to_node_id) REFERENCES nodes(node_id),
    FOREIGN KEY (gateway_link_id) REFERENCES trusted_gateways(id)
);

CREATE INDEX IF NOT EXISTS idx_topology_edges_from ON topology_edges(from_node_id);
CREATE INDEX IF NOT EXISTS idx_topology_edges_to ON topology_edges(to_node_id);
```

---

## 5. v1.8C-1 Blockers

### Must Have (🔴)

| # | Gap | Migration | New Code Required | Risk if Skip |
|---|-----|-----------|-------------------|-------------|
| 1 | Node fields expansion | 028 | Node registration handler, heartbeat handler | Cannot determine node capabilities |
| 2 | join_tokens | 029 | Join API, token management admin API | Cannot bootstrap new nodes |
| 3 | node_credentials | 030 | Node auth middleware, credential generation | No node API security |
| 4 | node_desired_states | 031 | Desired state generator, pull API (GET /desired-state) | Cannot distribute configuration |
| 5 | node_actual_states | 032 | Heartbeat handler update, actual state reconciler | Cannot track sync status |
| 12 | Node API auth | No migration | Auth middleware for `/api/node/v1/*` | Node APIs are unprotected |

### Should Have (🟡)

| # | Gap | Migration | New Code Required | Risk if Skip |
|---|-----|-----------|-------------------|-------------|
| 6 | gateways inventory | 033 | Gateway discovery, inventory API | Cannot do multi-gateway selection |
| 7 | service_gateway_policies | 034 | Policy CRUD, policy resolution in routing table | Hardcoded selection only |
| 9 | source_node_id on gateway_links | 028 (same as #1) | Update gateway link create/read | Cannot build link matrix |
| 10 | Routing table cache | Node-local | Cache population on desired state apply | Every dispatch queries control plane |

### Nice to Have (🟢)

| # | Gap | Migration | New Code Required | Risk if Skip |
|---|-----|-----------|-------------------|-------------|
| 8 | topology_edges | 035 | Topology matrix API | Cannot display topology, but routing works without it |
| 11 | Node event types | No schema | Update event logging | Reduced observability, not blocking |

### Recommended v1.8C-1 Scope

```
Phase 1 — Core node infrastructure (migrations 028-032):
  - Node fields expansion
  - join_tokens table + API
  - node_credentials table + API
  - node_desired_states table + pull API
  - node_actual_states table + heartbeat handler
  - Node API auth middleware
  - Node registration flow
  - Heartbeat loop
  - Desired state generation (initial, per-node)

Phase 2 — Gateway policies (migrations 033-034):
  - Gateway inventory table + heartbeat auto-discovery
  - Service gateway policies table + API
  - Policy resolution in routing table generator

Phase 3 — Topology (migration 035):
  - topology_edges table + matrix API
  - GatewayLink matrix enhancement

Phase 4 — Transparent access (no new migrations):
  - Local routing table cache
  - Local HTTP gateway (if not using Caddy)
  - Local DNS resolver (hosts file or proxy)
```

---

## Marker

```
v1.8C-0 Data Gap Analysis:    COMPLETE ✅
v1.8C-3 Gap 7 Filled:         service_gateway_policies + route_gateway_policies (migration 032) + routing table generator ✅
v1.8C-5 Gaps Filled:          local HTTP gateway (transparent managed domain access) + real relay execution + GatewayLink secret runtime injection ✅
v1.8C-6 Gaps Filled:          real multi-node local gateway acceptance (header fixes, heartbeat integration, security smoke, acceptance docs) ✅
v1.8C-6B Gaps Filled:        simulated acceptance full pass 12/12 + 1 deferred, real secret runtime integration tests (6), VPS runbook, docs update ✅
v1.8C-7 Gaps Filled:          developer entry + daemon runbook
v1.8C-8 Gaps Filled:          real two-node VPS relay acceptance (cross-server HTTP 200) ✅ — health/status endpoints, startup diagnostics, node.yaml config, systemd blueprints, dev acceptance script, 14 new tests ✅
```
