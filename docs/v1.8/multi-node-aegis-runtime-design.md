# v1.8C-0 — Multi-node Aegis Runtime + Horizontal Sync + Transparent Managed Domain Access — Design

> **Phase:** v1.8C — Multi-node Aegis Runtime
> **Status:** DESIGN COMPLETE (no code)
> **Date:** 2026-06-27

---

## Table of Contents

1. [Architecture Shift: Point-to-Point Relay → Multi-node Runtime](#1-architecture-shift)
2. [Aegis Node Runtime Components](#2-aegis-node-runtime-components)
3. [Node Model](#3-node-model)
4. [Gateway Inventory / Multi-gateway Model](#4-gateway-inventory--multi-gateway-model)
5. [Service / Route / Endpoint / Gateway Policy Model](#5-service--route--endpoint--gateway-policy-model)
6. [Topology Matrix / GatewayLink Matrix](#6-topology-matrix--gatewaylink-matrix)
7. [Managed Domain Routing Table](#7-managed-domain-routing-table)
8. [Transparent Managed Domain Access](#8-transparent-managed-domain-access)
9. [HTTPS / TLS Strategy](#9-https--tls-strategy)
10. [From v1.8B to v1.8C: Compatibility & Migration](#10-from-v18b-to-v18c-compatibility--migration)
11. [Deferred Items](#11-deferred-items)

---

## 1. Architecture Shift

### Old Model (v1.7 — v1.8B)

```
┌────────────────────────────────────────────────────────┐
│              Old Model: Point-to-Point Relay             │
│                                                          │
│   Dev Machine         Server A (Gateway)  Server B (Verifier/Target)
│   ┌────────┐          ┌──────────────┐   ┌──────────────┐
│   │ Aegis  │ ──────── │   Aegis      │──▶│   Aegis      │
│   │ (CLI)  │ curl     │ Control Plane│   │ (Relay Target)│
│   └────────┘  /__aegis│   + Caddy    │   │   + Caddy    │
│                      └──────────────┘   └──────────────┘
│                                                          │
│   Key characteristics:                                   │
│   - One primary Aegis instance                           │
│   - Remote node = passive verifier/target only           │
│   - Developer manually constructs relay URL              │
│   - GatewayLink = static forwarding config               │
│   - Per-node independent SQLite, no sync                 │
│   - No node identity beyond DB record                    │
│   - No capability/agent abstraction                      │
└──────────────────────────────────────────────────────────┘
```

### New Model (v1.8C+)

```
┌──────────────────────────────────────────────────────────────────┐
│              New Model: Multi-node Aegis Runtime                   │
│                                                                    │
│   Dev Machine           Server A                Server B           │
│   ┌────────────────┐   ┌────────────────┐   ┌────────────────┐    │
│   │ Aegis Node     │   │ Aegis Node     │   │ Aegis Node     │    │
│   │                │   │                │   │                │    │
│   │ ┌────────────┐ │   │ ┌────────────┐ │   │ ┌────────────┐ │    │
│   │ │ Node ID:   │ │   │ │ Node ID:   │ │   │ │ Node ID:   │ │    │
│   │ │ dev-zhp    │ │   │ │ server-a   │ │   │ │ server-b   │ │    │
│   │ └────────────┘ │   │ └────────────┘ │   │ └────────────┘ │    │
│   │ ┌────────────┐ │   │ ┌────────────┐ │   │ ┌────────────┐ │    │
│   │ │ Local DNS  │ │   │ │ Local DNS  │ │   │ │ Local DNS  │ │    │
│   │ │ (managed)  │ │   │ │ (managed)  │ │   │ │ (managed)  │ │    │
│   │ └────────────┘ │   │ └────────────┘ │   │ └────────────┘ │    │
│   │ ┌────────────┐ │   │ ┌────────────┐ │   │ ┌────────────┐ │    │
│   │ │ Local HTTP │ │   │ │ Local HTTP │ │   │ │ Local HTTP │ │    │
│   │ │ Gateway    │ │   │ │ Gateway    │ │   │ │ Gateway    │ │    │
│   │ └────────────┘ │   │ └────────────┘ │   │ └────────────┘ │    │
│   │ ┌────────────┐ │   │ ┌────────────┐ │   │ ┌────────────┐ │    │
│   │ │ Routing    │ │   │ │ Routing    │ │   │ │ Routing    │ │    │
│   │ │ Table      │ │   │ │ Table      │ │   │ │ Table      │ │    │
│   │ └────────────┘ │   │ └────────────┘ │   │ └────────────┘ │    │
│   │ ┌────────────┐ │   │ ┌────────────┐ │   │ ┌────────────┐ │    │
│   │ │ Heartbeat  │ │   │ │ Heartbeat  │ │   │ │ Heartbeat  │ │    │
│   │ │ Reporter   │ │   │ │ Reporter   │ │   │ │ Reporter   │ │    │
│   │ └────────────┘ │   │ └────────────┘ │   │ └────────────┘ │    │
│   └────────────────┘   └────────────────┘   └────────────────┘    │
│          │                    │                     │               │
│          └────────────────────┼─────────────────────┘               │
│                               │                                     │
│                    ┌──────────▼──────────┐                          │
│                    │   Control Plane      │                          │
│                    │   (Server A, primary)│                          │
│                    │                      │                          │
│                    │  ┌────────────────┐  │                          │
│                    │  │ Desired State  │  │    Source of Truth       │
│                    │  │ (full snapshot)│  │    Per-node:             │
│                    │  └────────────────┘  │    gateways, routing,    │
│                    │  ┌────────────────┐  │    policies, secrets     │
│                    │  │ Actual State   │  │                          │
│                    │  │ (per node)     │  │    Node pulls desired    │
│                    │  └────────────────┘  │    Node reports actual   │
│                    └──────────────────────┘                          │
│                                                                    │
│   Key characteristics:                                             │
│   - Every machine runs a complete Aegis Node                       │
│   - All nodes have same binary, different roles/config             │
│   - Control plane = source of truth (desired state)                │
│   - Node pulls its own configuration (not SSH push)                │
│   - Node reconciles local state (not remote SSH)                   │
│   - Node reports actual state (heartbeat + applied revision)       │
│   - Developer accesses domain, not machine:port                    │
│   - Aegis selects path: local → private gateway → public relay     │
│   - Remote target ports never exposed to caller                    │
│   - GatewayLink = node-to-node authorization edge                  │
└────────────────────────────────────────────────────────────────────┘
```

### Why Every Machine Must Be a Full Aegis Node

1. **Self-describing deployment**: Each node can answer "what am I running?" without consulting a central authority.

2. **Local reconciliation**: When control plane is unreachable, the node continues operating with its last desired state. No remote SSH or push script needed.

3. **Gradient of capabilities**: A node can be a gateway, worker, relay, or dev machine — same binary, different role configuration. No separate binaries per role.

4. **Transparent access works locally**: The developer's machine runs Aegis Node too. The local DNS resolver intercepts managed domains and routes them through the local HTTP gateway. No `curl http://<server>:<port>/__aegis/relay` manual construction.

5. **Future-proof**: Adding a capability (local DNS, diagnostics, agent hooks) is just a config change on the node, not a new binary deployment.

6. **Single-binary architecture**: One `aegis` binary for control plane, node agent, CLI diagnostics. Install once, configure differently.

---

## 2. Aegis Node Runtime Components

Each Aegis Node contains the following runtime components. Not all components are active on every node — activation depends on role configuration.

### 2.1 Component Map

```
┌──────────────────────────────────────────────────────────────┐
│                   Aegis Node Runtime                          │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ 1. Node Identity                                       │  │
│  │    - node_id (e.g. "server-a", "dev-zhp")             │  │
│  │    - node_secret (long-term credential)                │  │
│  │    - role (control_plane, gateway, worker, relay, dev) │  │
│  │    - public_ip, private_ip, hostname, agent_version    │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ 2. Registry / Join Client                              │  │
│  │    - POST /api/node/v1/join (with join token)          │  │
│  │    - Receives node_id + node_secret                    │  │
│  │    - subsequent calls use node_secret for auth          │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ 3. Heartbeat Reporter                                  │  │
│  │    - POST /api/node/v1/heartbeat                       │  │
│  │    - sends: node_id, applied_revision, status, error   │  │
│  │    - receives: latest_revision (to check for updates)  │  │
│  │    - configurable interval (default: 30s)              │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ 4. Desired State Puller                                │  │
│  │    - GET /api/node/v1/desired-state                    │  │
│  │    - triggered when heartbeat response shows new rev   │  │
│  │    - validates state_hash before applying               │  │
│  │    - stores desired state JSON locally                 │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ 5. Local Reconcile Engine                              │  │
│  │    - compares desired state vs actual local state       │  │
│  │    - calls provider adapters to apply changes           │  │
│  │    - no-op if desired == actual (revision matches)      │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ 6. Provider Adapter Layer                              │  │
│  │    - Caddy provider (existing, reused)                 │  │
│  │    - HAProxy provider (existing, reused)               │  │
│  │    - Aegis relay provider (local gateway config)       │  │
│  │    - Local DNS provider (managed domain -> local IP)   │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ 7. Local Gateway                                       │  │
│  │    - receives HTTP on managed domain host              │  │
│  │    - looks up routing table                            │  │
│  │    - if local: forward to local target                 │  │
│  │    - if remote: call Managed Relay path                │  │
│  │    - if unmanaged: passthrough to upstream DNS         │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ 8. Relay Handler (existing)                            │  │
│  │    - POST /__aegis/relay (existing, reused)            │  │
│  │    - GatewayLink auth (existing, reused)               │  │
│  │    - hop limit + open proxy prevention (existing)      │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ 9. Local Routing Table Cache                           │  │
│  │    - snapshot of routes relevant to this node          │  │
│  │    - revision + hash for consistency check             │  │
│  │    - used by local gateway for dispatch                 │  │
│  │    - invalidated when desired state revision advances  │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ 10. Diagnostics Runner (existing)                      │  │
│  │     - provider diagnostics (existing, reused)          │  │
│  │     - node health check                                │  │
│  │     - relay path verification                          │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ 11. Secret Runtime (existing)                          │  │
│  │     - master key loaded from /etc/aegis/secret.key     │  │
│  │     - GatewayLink secret decryption (existing)         │  │
│  │     - node_secret for control plane auth               │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ 12. Future Extension Hooks                             │  │
│  │     - Agent execution (future v1.8D)                   │  │
│  │     - DNS server (future)                              │  │
│  │     - Metrics exporter (future)                        │  │
│  │     - Event collector (future)                         │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

### 2.2 Component Activation by Role

| Component | control_plane | gateway | worker | relay | dev |
|-----------|:---:|:---:|:---:|:---:|:---:|
| Node Identity | ✅ | ✅ | ✅ | ✅ | ✅ |
| Registry/Join | ✅ | ✅ | ✅ | ✅ | ✅ |
| Heartbeat | ✅ | ✅ | ✅ | ✅ | ✅ |
| Desired State Pull | ✅ | ✅ | ✅ | ✅ | ✅ |
| Local Reconcile | ✅ | ✅ | ✅ | ✅ | ✅ |
| Provider Adapter | ✅ | ✅ | partial | ❌ | partial |
| Local Gateway | optional | ✅ | ❌ | ❌ | ✅ |
| Relay Handler | ✅ | ✅ | ❌ | ✅ | ❌ |
| Routing Table Cache | ✅ | ✅ | ❌ | partial | ✅ |
| Diagnostics Runner | ✅ | ✅ | ✅ | ✅ | ✅ |
| Secret Runtime | ✅ | ✅ | ✅ | ✅ | ✅ |

### 2.3 Node Modes

A node can operate in one of the following modes for each capability:

**Public Gateway:**
- `public_reachable: true` — node has a public IP and open ports
- `public_gateway_enabled: true` — node is configured to act as a public gateway
- Cloud security group must allow inbound 80/443

**Private Gateway:**
- `private_reachable: true` — node has a private network interface
- `private_gateway_enabled: true` — node is configured to act as a private gateway
- Reachable from other nodes on the same private network

**Non-routable worker:**
- No `public_reachable` or `private_reachable`
- No inbound gateway capabilities
- Can still pull desired state, run diagnostics, and host services

**Dev:**
- Local DNS resolver + local HTTP gateway for transparent access
- No public gateway capabilities
- Connects to control plane for desired state
- May host services for local testing

---

## 3. Node Model

### 3.1 Node Record (expanded from v1.7)

```
node_id:           string       // unique identifier, e.g. "server-a", "dev-zhp"
node_name:         string       // human-readable name
role:              string       // control_plane | gateway | worker | relay | dev
public_ip:         string       // external IP (may be empty for dev nodes)
private_ip:        string       // private network IP
region:            string       // datacenter/region (optional)
network_id:        string       // private network group (optional)
agent_version:     string       // aegis binary version
os:                string       // OS type (linux, darwin, windows)
arch:              string       // amd64, arm64
hostname:          string       // OS hostname

status:            string       // online | offline | degraded | unknown
capabilities:      JSON object  // node capabilities (v1.7 model, expanded)

last_heartbeat_at: timestamp
last_error:        string       // last error message (if in degraded/offline state)
created_at:        timestamp
updated_at:        timestamp
```

### 3.2 Capabilities (v1.7 model, expanded)

New capabilities to add:

```
relay_supported        bool   // node can receive relay requests
local_gateway_enabled  bool   // node runs local HTTP gateway
local_dns_enabled      bool   // node runs local DNS resolver
diagnostics_enabled    bool   // node runs diagnostics
agent_enabled          bool   // (future) node runs agent hooks
```

The capability detection mechanism from v1.7 is reused. New capabilities are detected by:
- Binary features (does this build include relay handler? — always yes)
- Config file (is local_gateway_enabled: true in node.yaml?)
- Runtime check (is port 53 available for DNS? is port 8080 available for local gateway?)

---

## 4. Gateway Inventory / Multi-gateway Model

### 4.1 Gateway Record

```
gateway_id:          string       // unique identifier
node_id:             string       // which node this gateway belongs to
name:                string       // human-readable name
type:                string       // local | private | public
provider:            string       // caddy | haproxy | aegis
bind_addr:           string       // IP to bind to (0.0.0.0 or specific)
host:                string       // public hostname or IP for routing
port:                int          // listener port (80, 443, 8080, etc.)
scheme:              string       // http | https

public_accessible:   bool         // accessible from public internet
private_accessible:  bool         // accessible from private network
enabled:             bool         // admin toggle
priority:            int          // selection priority (lower = preferred)
status:              string       // active | disabled | error
last_verified_at:    timestamp    // last health check timestamp
```

### 4.2 Three-layer Gateway Distinction

```
Node Capability Layer:
  node.public_reachable        = true   // node has public IP + open ports
  node.public_gateway_enabled  = true   // node is configured as public entry
  
  node.private_reachable       = true   // node has private network interface
  node.private_gateway_enabled = true   // node is configured as private entry

Gateway Layer:
  gateway.public_accessible    = true   // this specific gateway is public-facing
  
  gateway.private_accessible   = true   // this specific gateway is private-facing

Service/Route Policy Layer:
  route.public_allowed         = true   // this route MAY be served through public gateways
  route.private_allowed        = true   // this route MAY be served through private gateways
```

**Key rule:** `node.public_reachable` does NOT mean `route.public_allowed`. Even if a node
is on the public internet, individual services or routes can be restricted to private
access only.

### 4.3 Gateway Discovery

At startup, each node discovers its available gateways and reports them via heartbeat.
The control plane maintains the full gateway inventory across all nodes.

```
Discovery flow:
  1. Node reads its node.yaml (or command-line flags)
  2. Node discovers: public_ip, private_ip, available_providers (Caddy, HAProxy)
  3. Node checks: port availability (80, 443, or custom ports)
  4. Node builds gateway list from config + discovery
  5. Node includes gateways in heartbeat payload
  6. Control plane reconciles gateway inventory
```

---

## 5. Service / Route / Endpoint / Gateway Policy Model

### 5.1 Policy Record

```
policy_id:           string       // unique identifier
service_id:          string       // optional — applies to service (and all its routes)
route_id:            string       // optional — applies to specific route only

mode:                string       // auto | fixed | multi | disabled

primary_gateway_id:  string       // for fixed mode: which gateway to use
fallback_gateway_ids: []string    // for multi mode: ordered fallback list

allow_local:         bool         // allow access via local gateway
allow_private:       bool         // allow access via private gateway
allow_public:        bool         // allow access via public gateway

require_gateway_link: bool        // cross-node relay must use GatewayLink
require_relay:       bool         // always use managed relay (never direct)

preserve_host:       bool         // preserve original Host header
tls_mode:            string       // http_only | terminate_local | passthrough_deferred

priority:            int          // policy selection priority
updated_at:          timestamp
```

### 5.2 Policy Mode Semantics

**auto (default):**
- Aegis selects the path based on topology and gateway availability
- For same-node: local gateway (no GatewayLink needed)
- For cross-node private: private gateway + GatewayLink
- For cross-node public: public gateway + GatewayLink
- If caller is a dev node: preferred path minimizes public exposure

```
Example — auto mode behavior:
  Dev → api.example.com (managed)
    ├── endpoint on same node? → local gateway → 127.0.0.1:<port>
    ├── endpoint on private network? → private gateway + GatewayLink
    ├── endpoint on public network? → public gateway + GatewayLink
    └── endpoint unreachable? → unavailable (no fallback direct)
```

**fixed:**
- Always routes through the specified primary_gateway_id
- Ignores topology optimization
- Useful for compliance or network segmentation requirements

**multi:**
- Uses primary_gateway_id first
- Falls back to fallback_gateway_ids in order
- Only falls back on gateway unavailability, not on request failure
- All gateways in the list must be valid for this route's allow_X settings

**disabled:**
- The route/service cannot be accessed through any gateway
- Local gateway returns 403 "route disabled"
- Does not affect internal relay dispatch

### 5.3 Policy Inheritance

```
Route-level policy overrides Service-level policy
Service-level policy overrides default (auto)

Resolution:
  1. Look for policy with matching route_id
  2. If not found, look for policy with matching service_id
  3. If not found, use default (mode: auto)
```

### 5.4 Fixed Rules (enforced at runtime)

1. **Managed domains never connect to remote target ports directly.** All cross-node traffic goes through gateway proxy (port 80/443).

2. **Cross-node HTTP relay must have a valid GatewayLink.** No GatewayLink → unavailable mode.

3. **Unavailable does not fallback to remote target_host:target_port.** The `direct_target_suppressed` flag is always true for managed relay.

4. **Gateway selection is determined by policy + topology + gateway inventory**, not by IP inference alone.

5. **Temporary IP inference (from node private/public IP matching endpoint address) is a fallback mechanism only.** The primary path uses node_id from desired state.

### 5.5 Implementation Status (v1.8C-3)

The policy model described in sections 5.1-5.4 is now implemented:

- `service_gateway_policies` and `route_gateway_policies` tables created (migration 032)
- Full CRUD for both service-level and route-level policies in `internal/routingpolicy/`
- Policy precedence: route > service > default (with disabled policies falling through)
- All four modes (auto, fixed, multi, disabled) fully implemented
- 17 tests covering creation, resolution, fields round-trip, listing, defaults, validation
- Admin API: `GET/PUT /api/admin/v1/services/{id}/gateway-policy` and `GET/PUT /api/admin/v1/routes/{id}/gateway-policy`

---

## 6. Topology Matrix / GatewayLink Matrix

### 6.1 Topology Edge Record

```
from_node_id:        string       // source node
to_node_id:          string       // destination node

private_reachable:   bool         // nodes are on the same private network
public_reachable:    bool         // nodes can reach each other via public IP

preferred_gateway_id: string      // which gateway to use for this edge
gateway_link_id:     string       // which GatewayLink authorizes this edge

status:              string       // verified | missing_link | unreachable | degraded | unknown
last_verified_at:    timestamp
last_error:          string
```

### 6.2 Topology Matrix Endpoints

```
GET /api/admin/v1/topology/matrix
  Returns a 2D matrix of all node-to-node edges with status

GET /api/admin/v1/topology/path?from=<node>&to=<node>
  Returns the optimal path between two nodes with gateway info
```

### 6.3 GatewayLink Matrix Endpoints

```
GET /api/admin/v1/gateway-links/matrix
  Returns a 2D matrix showing GatewayLink status per node pair:

  source_node_id      | target_node_id      | link_id | version | status
  ────────────────────┼─────────────────────┼─────────┼─────────┼───────
  server-a            | server-b            | gl_abc  | 2       | verified
  server-a            | server-c            | —       | —       | missing_link
```

### 6.4 GatewayLink Semantics in v1.8C

GatewayLink is refined from "static forwarding config" to "node-to-node authorization edge":

```
v1.8B semantics:
  GatewayLink = a config entry that says "trust this gateway"
  Route references GatewayLink for relay dispatch
  Two-node: upstream creates GatewayLink → downstream creates GatewayLink
  No node-aware verification

v1.8C semantics:
  GatewayLink = an authorization edge between two nodes
  Each link has source_node_id and target_node_id
  The link authorizes the source node to relay to the target node
  Topology matrix shows all links between all node pairs
  Missing link = route through target node unavailable
```

**Key distinction:** GatewayLink is not a route source of truth. Route/Endpoint determines
the final target. GatewayLink determines whether cross-node access is authorized.

---

## 7. Managed Domain Routing Table

### 7.1 Routing Table Record

```
domain:              string       // fully qualified domain name
route_id:            string       // Aegis route ID
service_id:          string       // Aegis service ID
endpoint_id:         string       // target endpoint ID
target_node_id:      string       // which node hosts this endpoint
target_local_host:   string       // 127.0.0.1 (always local to target node)
target_local_port:   int          // port on target node

protocol:            string       // http (only http in v1.8C)

gateway_policy:      object       // resolved policy for this route

candidates:          []candidate  // ordered list of gateway options
  └── mode:          string       // local_gateway | private_gateway | public_gateway
  └── gateway_id:    string       // specific gateway to use
  └── gateway_url:   string       // URL to reach this gateway
  └── priority:      int          // selection priority
  └── requires_gateway_link: bool
  └── gateway_link_id: string     // if requires_gateway_link

preserve_host:       bool
tls_mode:            string       // http_only | terminate_local | passthrough_deferred
version:             int          // routing table version (monotonic)
updated_at:          timestamp
```

### 7.2 Per-node Scope

Each node only receives routing table entries for:

1. Routes whose endpoint is hosted on this node (for local dispatch)
2. Routes the node is authorized to access (for gateway/relay)
3. Routes in the same space as the node (space isolation)

This ensures:
- Cross-scope visibility is prevented
- Internal target addresses are not leaked to unauthorized nodes
- Each node's routing table is minimal

### 7.3 Revision Tracking

```
routing_table_revision:  int          // monotonic revision number
routing_table_hash:      string       // sha256 of full table content
routing_table_etag:      string       // for HTTP caching (ETag header)

Node polling:
  1. Node has revision=42, hash="abc123"
  2. Heartbeat response: "latest_routing_table_revision=43"
  3. Node GET /api/node/v1/routing-table with If-None-Match: "abc123"
  4. If no change → 304 Not Modified
  5. If changed → 200 with new table, new revision, new hash
```

### 7.4 Implementation Status (v1.8C-3)

The routing table described in sections 7.1-7.3 is now partially implemented:

- Routing table model with entries, candidates, policy info, and status in `internal/routingtable/`
- Routing table generator supporting auto/fixed/multi/disabled modes with local/private/public candidate selection
- GatewayLink authorization integrated into candidate selection and final status assessment
- Routing table validator with 10 rules (no direct remote, cross-node requires link, disabled→disabled, etc.)
- Admin API: `GET /api/admin/v1/nodes/{id}/routing-table`, `POST .../generate`, `GET /api/admin/v1/routing/preview`, `GET /api/admin/v1/routing/validate`
- Routing table can be persisted as a desired state revision (embedded in `local_routing_table`)
- 17 tests covering all candidate modes, validation, multi-route, policy integration

**Not yet implemented (deferred from v1.8C-3):**
- Revision tracking via hash/ETag (routing table uses desired state revision indirectly)
- Per-node scoped routing (generator currently considers all routes)
- Node-side routing table cache (node agent not yet implemented)

---

## 8. Transparent Managed Domain Access

### 8.1 Developer Experience Goal

```
Before v1.8C:
  $ curl http://<SERVER_A_IP>:80/__aegis/relay \
      -H "X-Aegis-Route-ID: rt_xxx" \
      -H "X-Aegis-Gateway-ID: gl_xxx" \
      -H "X-Aegis-Gateway-Token: ..." \
      -H "X-Aegis-Source-Node: dev-zhp" \
      -H "Host: api.example.com"

After v1.8C:
  $ curl http://api.example.com/
  # Aegis handles: DNS → gateway selection → relay auth → dispatch
  # Developer thinks in domains, not in infrastructure
```

### 8.2 Local DNS Resolver

**Purpose:** Resolve managed domain names to the local Aegis gateway IP, so HTTP requests
land on the local gateway instead of going directly to the remote target.

```
Request flow:
  1. Application does DNS lookup for "api.example.com"
  2. Aegis local DNS resolver intercepts (if configured)
  3. Managed domain? → return 127.0.0.1 (or gateway IP)
  4. Unmanaged domain? → forward to upstream DNS (8.8.8.8 or system DNS)
  5. Cache TTL respected per routing table revision
```

**Design constraints:**
- NOT a full DNS server implementation
- Option 1: Edit `/etc/hosts` via `aegis dns apply` (simplest)
- Option 2: Local DNS proxy on port 53 that resolves managed domains
- Option 3: Use systemd-resolved `ManagedDNS` integration
- First version: Option 1 (hosts file edit)
- Future: Option 2 (local DNS proxy) for dynamic TTL/cache

**Cache invalidation:**
- Routing table revision change → clear managed domain cache
- TTL per domain entry (configurable, default 60s)
- Manual flush: `aegis dns flush`

### 8.3 Local HTTP Gateway

**Purpose:** Receive HTTP requests on managed domains and route them via Aegis relay.

```
Request flow:
  1. Local HTTP gateway receives request with Host: api.example.com
  2. Looks up routing table by domain
  3. If unmanaged → 404 or passthrough (configurable)
  4. If managed:
     a. Check routing table for this domain
     b. Evaluate gateway candidates in priority order
     c. Select best candidate (local > private > public)
     d. If local gateway: forward directly to 127.0.0.1:<port>
     e. If private/public: use existing Managed Relay POST flow
     f. Never expose remote target port in response
```

**Gateway types:**

| Gateway | Listener | Relay auth | Use case |
|---------|----------|------------|----------|
| local | 127.0.0.1:8080 | none | Dev machine, same-node access |
| private | <private_ip>:80 | GatewayLink | Cross-node private network |
| public | <public_ip>:443 | GatewayLink | Cross-node public internet |

**Architecture note:** The local gateway on the dev machine is a lightweight HTTP forwarder
in the Aegis binary. It does NOT require Caddy or HAProxy. On gateway nodes (server-a,
server-b), Caddy/HAProxy serve as the gateway with full proxy capabilities.

### 8.4 Transparency Without System-Level Hijacking

**Important:** The transparent access system is NOT system-level mandatory interception.
It is:

1. **Optional:** Only active if the node's `node.yaml` has `local_gateway_enabled: true`
2. **Scoped:** Only intercepts domains listed in the node's routing table
3. **Non-intercepting:** Unmanaged domains are passed through to upstream DNS
4. **Configurable:** Admin can disable transparent access at any time

The system is designed for developer convenience, not network policy enforcement.

---

## 9. HTTPS / TLS Strategy

### 9.1 First Version: HTTP-only Transparent Access

For v1.8C-1, transparent managed domain access is HTTP-only:

```
Dev → http://api.example.com → local HTTP gateway (127.0.0.1:8080)
  → if managed: route through Aegis relay → final target (HTTP)
  → if unmanaged: 404 (explicit, not silent proxy)
```

HTTPS requests (`https://api.example.com`) are not intercepted. The TLS handshake
happens between the client and the upstream TLS endpoint (when applicable).

### 9.2 Future TLS Options (documented, not implemented)

**Option A: Local TLS Termination**
```
Client → https://api.example.com → Aegis local gateway (with TLS cert)
  → Aegis terminates TLS → plain HTTP to relay → target (HTTP or HTTPS)
```
- Requires local TLS certificate
- Can use managed/wildcard cert for *.example.com
- Internal dev CA can issue certs for dev machines
- Not suitable for production internet-facing gateways (breaks end-to-end TLS)

**Option B: TLS Passthrough / SNI Routing**
```
Client → https://api.example.com → Aegis local gateway
  → Aegis reads SNI → routes based on domain
  → TCP tunnel (raw bytes) to target's HTTPS port
```
- Requires raw TCP relay or CONNECT tunnel (deferred)
- Maintains end-to-end encryption
- Cannot inspect or route based on HTTP semantics
- Deferred to future version

### 9.3 HTTPS Rules

1. **HTTP-only first.** v1.8C-1 does not intercept HTTPS.
2. **No MITM of external HTTPS sites.** Aegis will never silently intercept `https://google.com`.
3. **No root CA installation.** Aegis will not install system root CAs for MITM.
4. **If HTTPS is needed for managed domains**, deploy Caddy on the gateway node
   with Let's Encrypt (existing v1.x capability). The dev machine access remains HTTP.
5. **Wildcard/internal CA is optional**, not required for v1.8C-1.

---

## 10. From v1.8B to v1.8C: Compatibility & Migration

### 10.1 Retained v1.8B Capabilities

| Capability | Status in v1.8C |
|------------|----------------|
| Managed HTTP Relay (POST /__aegis/relay) | **RETAINED** — unchanged dispatch handler |
| RelayHandler (net/http reverse proxy) | **RETAINED** — unchanged forwarding logic |
| GatewayLink auth (X-Aegis-Gateway-ID + X-Aegis-Gateway-Token) | **RETAINED** — unchanged header validation |
| GatewayLink encrypted secret (AES-256-GCM) | **RETAINED** — unchanged crypto |
| endpoint.node_id enforcement | **RETAINED** — unchanged validation |
| trusted_gateway.target_node_id | **RETAINED** — unchanged field |
| Route safety / self-loop detection | **RETAINED** — unchanged |
| No fallback direct (direct_target_suppressed) | **RETAINED** — unchanged rule |
| Open proxy prevention (target-host/port rejection) | **RETAINED** — unchanged |

### 10.2 Upgraded v1.8B Capabilities

| Pre-v1.8C | v1.8C |
|-----------|-------|
| "two-node" wording | "point-to-point relay verified" — generalized to N nodes |
| RelayResolver → developer-facing API | Routing table generator + internal dispatch helper |
| GatewayLink = forwarding config | GatewayLink = topology authorization edge |
| Manual /__aegis/relay as primary usage | ./__aegis/relay remains as API but developer uses domain access |
| IP-only gateway selection | Policy + topology + gateway inventory based selection |

### 10.3 Deprecated or Downgraded Semantics

| Old Semantic | v1.8C Status |
|-------------|-------------|
| "Remote node as passive verifier" wording | **REMOVED** — all nodes are full Aegis Nodes |
| "Developer manually constructs relay URL" as primary workflow | **DEPRECATED** — local gateway + routing table is primary |
| "IP-only gateway selection" as final rule | **DOWNGRADED** — fallback only, policy takes precedence |

---

## 11. Deferred Items

These capabilities are explicitly deferred from v1.8C-0 and may be addressed in future phases:

| Capability | Rationale |
|-----------|-----------|
| **Raw TCP relay** | Requires TCP proxy mode, not just HTTP. Deferred to v1.8D+ |
| **CONNECT tunnel** | Requires HTTP CONNECT support in relay handler |
| **WebSocket tunnel** | Requires WebSocket upgrade support |
| **UDP / DNS relay** | Different protocol, different forwarding model |
| **SSH relay** | Not a managed domain use case |
| **Database protocol relay** | Not a managed domain use case |
| **iptables / nftables integration** | Requires kernel-level interception |
| **eBPF** | Requires kernel-level interception |
| **Service mesh sidecar** | Aegis is a control plane, not a mesh |
| **Full DNS server implementation** | Phase 1 uses `/etc/hosts` or simple proxy |
| **HTTPS interception / TLS termination** | HTTP-only first; HTTPS deferred |
| **Root CA installation / MITM** | Not in Aegis scope |
| **Multi-primary control plane / Raft** | Single control plane in v1.8C |
| **P2P gossip** | Pull-based desired state is the designed pattern |
| **Control plane SSH push** | Node-pull model only |
| **Agent hooks / plugin system** | Future extension |
| **Web UI / Dashboard** | CLI + API only |

---

## Marker

```
v1.8C-0 Multi-node Aegis Runtime Design:    COMPLETE ✅
v1.8C-3 Gateway Policy + Routing Table:      COMPLETE ✅
  - service_gateway_policies + route_gateway_policies (migration 032)
  - Policy model, CRUD, precedence, 4 modes
  - Routing table generator + validator (10 rules)
  - Admin policy CRUD APIs (4 endpoints)
  - Admin routing table APIs (4 endpoints)
  - 34 tests total (17 policy + 17 routing)
v1.8C-0 Design Output:
  - multi-node-aegis-runtime-design.md  (this document)
  - node-bootstrap-design.md
  - horizontal-sync-desired-state-design.md
  - multi-node-runtime-data-gap.md
```
