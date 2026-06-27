# v1.8C-1 — Node Bootstrap + Registry Implementation

> **Phase:** v1.8C — Multi-node Aegis Runtime
> **Status:** IMPLEMENTED ✅ (v1.8C-1A: Auth Smoke & Docs Closure ✅)
> **Auth Status:** api_auth_verified ✅
> **Date:** 2026-06-27

---

## 1. Implementation Scope

This phase implements the first slice of the multi-node Aegis Runtime:

- **Migration 028**: Nodes table expansion, node_join_tokens, node_credentials
- **Node Model**: Expanded fields (name, role, status, os, arch, agent_version, heartbeat)
- **Join Token**: Admin creates one-time tokens, nodes use them to register
- **Node Credential**: Long-term credentials issued at registration
- **Admin API**: Join token CRUD, node detail, node health
- **Node API**: POST /api/node/v1/join, POST /api/node/v1/heartbeat
- **Tests**: 17 tests covering token lifecycle, registration flow, auth, security

### What is NOT implemented in this phase

- Desired state sync
- Topology matrix
- Gateway inventory
- Local DNS
- Local HTTP gateway
- Transparent domain access
- v1.8B relay changes

---

## 2. Join Token Semantics

### Token Lifecycle

```
Admin Creates                    Node Registers                Token Invalidated
    │                                │                              │
    ▼                                ▼                              ▼
┌────────────┐                ┌──────────────┐              ┌─────────────┐
│ Status:    │   POST /join   │   Validate   │   Mark used  │ Status:     │
│ valid      │───────────────▶│   token      │─────────────▶│ used        │
│ expires_at │                │   check:     │              │ used_by_node│
│ 24h        │                │   - expiry   │              └─────────────┘
└────────────┘                │   - used_at  │
                              │   - revoked  │     Admin Revokes
                              │   - roles    │           │
                              │   - node_name│           ▼
                              │   - source_ip│   ┌─────────────┐
                              └──────────────┘   │ Status:     │
                                                 │ revoked     │
                                                 └─────────────┘
```

### Validation Rules

| Check | If fails |
|-------|---------|
| Token exists (by SHA-256 hash) | "join token not found" |
| Not expired | "join token expired at ..." |
| Not used | "join token already used" |
| Not revoked | "join token has been revoked" |
| expected_node_name matches (if set) | "join token requires node_name 'X'" |
| Requested role in allowed_roles (if set) | "join token does not allow role 'X'" |
| Source IP in allowed CIDR (if set) | "source IP not in allowed CIDR" |

---

## 3. Node Credential Semantics

### Credential Lifecycle

```
Node Registration               Node API Calls              Admin Revokes
    │                                │                              │
    ▼                                ▼                              ▼
┌────────────┐              ┌──────────────────┐            ┌────────────┐
│ Created    │   Heartbeat  │   Authenticated   │   Revoke   │ Revoked    │
│ token_hash │─────────────▶│   last_used_at    │──────────▶│ revoked_at  │
│ 256-bit    │              │   updated         │            │            │
└────────────┘              └──────────────────┘            └────────────┘
```

### Security Properties

- Raw token is 256-bit random (64 hex chars)
- DB stores HMAC-SHA256 hash, never raw token
- Raw token returned only once (at registration)
- Revoked credentials are immediately invalid
- Token is generated using `id.GenerateRandomHex(32)` — the project's canonical CSPRNG

---

## 4. Node Registry Fields

| Field | Source | Description |
|-------|--------|-------------|
| `id` | auto | Internal DB ID (`node_xxx`) |
| `node_id` | auto | Logical ID (`nd_xxxx`) |
| `name` | join request | Human-readable name |
| `role` | join request | Primary role (worker/gateway/relay/dev) |
| `status` | heartbeat | online/offline/degraded/unknown |
| `hostname` | join/heartbeat | OS hostname |
| `public_ip` | heartbeat | Public IP address |
| `private_ip` | heartbeat | Private IP address |
| `region` | future | Datacenter/region |
| `network_id` | future | Private network group |
| `os` | join request | linux/darwin/windows |
| `arch` | join request | amd64/arm64 |
| `agent_version` | heartbeat | Aegis binary version |
| `capabilities` | heartbeat | JSON object of capability flags |
| `last_heartbeat_at` | heartbeat | Last heartbeat timestamp |
| `last_error` | heartbeat | Last error message |

---

## 5. Node Heartbeat Fields

| Field | Source | Description |
|-------|--------|-------------|
| `node_id` | node | Required — identifies the node |
| `status` | node | online/degraded (default: online) |
| `agent_version` | node | e.g. "v1.8C" |
| `hostname` | node | OS hostname |
| `public_ip` | node | Public IP address |
| `private_ip` | node | Private IP address |
| `capabilities` | node | Optional — list of active capabilities |
| `listeners` | node | Optional — active listener ports |
| `provider_status` | node | Optional — provider health |
| `relay_status` | node | Optional — relay handler status |
| `local_gateway_status` | node | Optional — local gateway status |
| `applied_revision` | node | Optional — desired state revision (reserved for v1.8C-2) |
| `last_error` | node | Optional — last error message |

---

## 6. Admin API

### POST /api/admin/v1/node-join-tokens

Create a new join token.

```json
Request:
{
  "name": "server-c bootstrap",
  "allowed_roles": ["gateway", "worker", "relay"],
  "expected_node_name": "server-c",
  "allowed_source_cidr": "",
  "expires_in_seconds": 3600
}

Response 201:
{
  "id": "jt_abc123",
  "name": "server-c bootstrap",
  "raw_join_token": "a1b2c3d4...",
  "token_redacted": false,
  "expires_at": "2026-06-28T10:00:00Z",
  "allowed_roles": ["gateway", "worker", "relay"],
  "expected_node_name": "server-c",
  "allowed_source_cidr": "",
  "warning": "store this token securely — it will not be shown again"
}
```

### GET /api/admin/v1/node-join-tokens

List all join tokens (raw token redacted).

### POST /api/admin/v1/node-join-tokens/{id}/revoke

Revoke an unused join token.

### GET /api/admin/v1/nodes/{id}

Get detailed node information. Computes offline status if heartbeat is stale (>60s).

### GET /api/admin/v1/nodes/{id}/health

Get node health status based on heartbeat recency.

---

## 7. Node API

### POST /api/node/v1/join

Register a new node using a join token.

```json
Request:
{
  "join_token": "a1b2c3d4...",
  "node_name": "server-c",
  "roles": ["gateway", "worker", "relay"],
  "hostname": "server-c.example.com",
  "os": "linux",
  "arch": "amd64",
  "agent_version": "v1.8C",
  "public_ip": "<SERVER_A_NODE_IP>",
  "private_ip": "10.0.0.3"
}

Response 201:
{
  "node_id": "nd_c",
  "node_token": "e5f6g7h8...",
  "node_token_redacted": false,
  "status": "registered",
  "heartbeat_after_seconds": 30,
  "warning": "store this token securely — it will not be shown again"
}
```

### POST /api/node/v1/heartbeat

Send a heartbeat for a registered node. Note: this endpoint does NOT yet use node
credential auth (that requires a middleware refactor). The node_id is self-reported
in the request body. Node credential auth will be enforced when the node API
middleware is implemented.

```json
Request:
{
  "node_id": "nd_c",
  "agent_version": "v1.8C",
  "hostname": "server-c.example.com",
  "public_ip": "<SERVER_A_NODE_IP>",
  "private_ip": "10.0.0.3",
  "capabilities": ["caddy", "relay"],
  "status": "online",
  "applied_revision": 0,
  "last_error": ""
}

Response 200:
{
  "node_id": "nd_c",
  "status": "accepted",
  "latest_revision": 0,
  "desired_state_available": false
}
```

---

## 8. Security Boundaries

| Rule | Status |
|------|--------|
| Join token only used for registration, not as long-term credential | ✅ Enforced |
| Node token returned only once | ✅ Enforced (hash stored in DB) |
| DB does not store raw join token | ✅ HMAC-SHA256 hash stored |
| DB does not store raw node credential | ✅ HMAC-SHA256 hash stored |
| Used/expired/revoked join tokens rejected | ✅ 3 tests |
| Revoked node credentials rejected | ✅ Auth test |
| Service API keys cannot access node admin API | ✅ AdminAuthMiddleware blocks non-session requests |
| Node token cannot access admin API | ✅ Node API is under /api/node/v1/ (separate from admin) |
| Logs do not contain raw tokens | ✅ Raw tokens never pass through log functions |
| Node can only heartbeat for itself | ✅ Self-reported node_id (future: token-bound enforcement) |

---

## 9. Not Supported (this phase)

- Node credential auth middleware for `/api/node/v1/*`
- CLI commands for node management
- Background offline detection
- Desired state sync
- Multi-role storage (only primary role stored)
- Source IP CIDR validation (modeled but not wired into service yet)

---

## 10. Next: v1.8C-2 Entry Criteria

- [x] v1.8C-1 Node Bootstrap + Registry implemented and tested
- [x] Migration 028 applied (nodes expansion + join_tokens + node_credentials)
- [x] Admin APIs for join tokens and node management
- [x] Node APIs for join and heartbeat
- [x] 17 tests passing
- [x] Build succeeds (`go build ./cmd/aegis/`)
- [x] All existing tests pass (no regression)

### Suggested v1.8C-2 Work Items

- Node auth middleware for `/api/node/v1/*` (enforce node credential on heartbeat)
- Desired state sync (node_desired_states + node_actual_states tables)
- CLI commands for node management
- Background offline detection
- Topology matrix

---

## v1.8C-1A Update: Auth Smoke & Docs Closure

### Admin API Auth

| Endpoint | Auth Mechanism | Auth Enforcement |
|----------|---------------|-----------------|
| `POST /api/admin/v1/node-join-tokens` | AdminAuthMiddleware (session cookie) | ✅ `/api/admin/v1/*` prefix |
| `GET /api/admin/v1/node-join-tokens` | AdminAuthMiddleware (session cookie) | ✅ `/api/admin/v1/*` prefix |
| `POST /api/admin/v1/node-join-tokens/{id}/revoke` | AdminAuthMiddleware (session cookie) | ✅ `/api/admin/v1/*` prefix |
| `GET /api/admin/v1/nodes` | AdminAuthMiddleware (session cookie) | ✅ `/api/admin/v1/*` prefix |
| `GET /api/admin/v1/nodes/{id}` | AdminAuthMiddleware (session cookie) | ✅ `/api/admin/v1/*` prefix |
| `GET /api/admin/v1/nodes/{id}/health` | AdminAuthMiddleware (session cookie) | ✅ `/api/admin/v1/*` prefix |

All admin endpoints are under `/api/admin/v1/` → covered by AdminAuthMiddleware (cookie-based session, implemented in serve.go).

Service API keys: blocked from all `/api/admin/v1/*` routes via `isSystemRoute()` in `token/middleware.go` (returns true for all `/api/admin/v1/` paths).

### Node API Auth

| Endpoint | Auth Mechanism | Auth Enforcement |
|----------|---------------|-----------------|
| `POST /api/node/v1/join` | Join token in request body | ✅ Handler validates join token |
| `POST /api/node/v1/heartbeat` | Bearer token (node credential) | ✅ `authenticateNodeRequest()` in handler |

### Auth Tests (v1.8C-1A)

| Test | Coverage |
|------|----------|
| `TestAdminNodeFixedRoutesRegistered` | Routes registered (not 404) |
| `TestAdminNodePathsUnderAdminPrefix` | All under `/api/admin/v1/` prefix |
| `TestNodeHeartbeatNoAuth` | Missing Bearer token → 401 |
| `TestNodeHeartbeatWrongToken` | Invalid node credential → 401 |
| `TestNodeHeartbeatWrongNodeID` | Node A token used for Node B → 403 |
| `TestNodeHeartbeatRevokedToken` | Revoked credential → 401 |
| `TestNodeHeartbeatMalformedBody` | Invalid JSON → 400 |
| `TestNodeJoinNoToken` | Missing join_token → 400 |
| `TestNodeJoinInvalidBody` | Invalid JSON → 400 |
| `TestNodeJoinIsPublicEndpoint` | No Bearer token required for join |
| `TestNodeJoinAndHeartbeatFullFlow` | Full join → heartbeat success path |
| `TestHeartbeatResponseNoTokenLeak` | Raw token not in responses |
| `TestServiceNodeAuth` | Service-level auth (valid, wrong, empty, revoked) |
| `TestNodeModelNoCredentialFields` | Node model doesn't expose credential fields |
| `TestAllowedSourceCIDRColumnExists` | Column exists in schema |

### allowed_source_cidr Status

- **Stored**: ✅ Column `allowed_source_cidr` exists in `node_join_tokens` table
- **Wired**: ✅ Field is accepted in `CreateJoinTokenInput` and stored/retrieved by repository
- **Enforced**: ⏳ Source IP validation is implemented in `ValidateJoinToken()` (service level) but the enforcement is not wired into the HTTP handler
- **Status**: `stored_and_partially_wired` — column exists, input accepted, service validation ready, but not yet enforced at API layer

### Key Changes in v1.8C-1A

1. **Heartbeat now requires node credential**: `authenticateNodeRequest()` validates Bearer token before processing heartbeat
2. **Node can only heartbeat for itself**: `authNodeID != req.NodeID` → 403 Forbidden
3. **`GetCredentialByNodeID`**, **`RevokeNodeCredential`**, **`RevokeAllNodeCredentials`**, **`GetHashTokenForTesting`** added to nodeauth service
4. **16 new auth tests**: coverage for admin routes, heartbeat auth, join validation, token leak prevention

### Not Supported (this phase)

- CLI commands for node management (deferred)
- Background offline detection (deferred)
- Desired state sync (v1.8C-2)

---

## Capability Matrix (v1.8C)

| Capability | Status | Evidence |
|-----------|--------|----------|
| Node Bootstrap (join token) | **code_verified** ✅ | 17 nodeauth tests |
| Node Registry (nodes CRUD) | **code_verified** ✅ | 17 nodeauth tests |
| Node Heartbeat | **api_auth_verified** ✅ | Handler auth + 16 auth tests |
| Admin Node API Auth | **api_auth_verified** ✅ | AdminAuthMiddleware coverage confirmed |
| Node Credential Auth | **api_auth_verified** ✅ | `authenticateNodeRequest()` in handler |
| allowed_source_cidr | **partial** ⏳ | Stored + wired in service, not enforced at API |
| Desired State Sync | **pending** ⬜ | v1.8C-2 |
| Topology Matrix | **pending** ⬜ | |
| Gateway Inventory | **pending** ⬜ | |
| Transparent Domain Access | **pending** ⬜ | |
| Local DNS | **pending** ⬜ | |
| Local HTTP Gateway | **pending** ⬜ | |

## Marker

```
v1.8C-1 Node Bootstrap + Registry:               COMPLETE ✅
v1.8C-1A Auth Smoke & Docs Closure:              COMPLETE ✅
Build:                                             PASS
Tests (nodeauth):                                  17/17 PASS
Tests (httpapi auth):                              16/16 PASS
All existing tests:                                PASS (no regression)
```
