# v1.8C-0 — Node Bootstrap Design

> **Phase:** v1.8C — Multi-node Aegis Runtime
> **Status:** DESIGN COMPLETE (no code)
> **Date:** 2026-06-27

---

## Table of Contents

1. [Bootstrap Flow Overview](#1-bootstrap-flow-overview)
2. [Install Script](#2-install-script)
3. [Join Token Design](#3-join-token-design)
4. [Node Credential Design](#4-node-credential-design)
5. [Registration Sequence](#5-registration-sequence)
6. [Service Installation](#6-service-installation)
7. [Systemd Unit](#7-systemd-unit)
8. [Node Configuration File](#8-node-configuration-file)
9. [Post-registration Lifecycle](#9-post-registration-lifecycle)
10. [Security Considerations](#10-security-considerations)

---

## 1. Bootstrap Flow Overview

```
Target Experience:

  curl -fsSL https://<control-plane>/install.sh | sh -s -- \
    --control https://aegis.example.com \
    --join-token jt_abc123 \
    --node-name server-c \
    --role gateway,relay

Full Flow:

  ┌─────────────┐     ┌──────────────────┐     ┌──────────────────┐
  │ New Machine  │     │   Control Plane   │     │   Existing Nodes  │
  │ (server-c)   │     │   (server-a)      │     │   (server-b)      │
  └──────┬──────┘     └────────┬─────────┘     └────────┬─────────┘
         │                     │                        │
   1.    │──── curl install ──▶│                        │
         │   script.sh         │                        │
         │                     │                        │
   2.    │──── join ──────────▶│                        │
         │   POST /api/node/v1/join                     │
         │   {join_token, node_name, roles}             │
         │                     │                        │
         │◀─── join response ──│                        │
         │   {node_id, node_secret,                     │
         │    latest_revision: 0}                       │
         │                     │                        │
   3.    │   Write node.yaml   │                        │
         │   Save node_secret  │                        │
         │   locally           │                        │
         │                     │                        │
   4.    │   Install systemd   │                        │
         │   service + start   │                        │
         │                     │                        │
   5.    │──── heartbeat ─────▶│                        │
         │   POST /api/node/v1/heartbeat                │
         │   {node_id, status: online,                  │
         │    applied_revision: 0}                      │
         │                     │                        │
         │◀─── heartbeat resp─│                        │
         │   {latest_revision: 0,                       │
         │    desired_state_available: false}           │
         │                     │                        │
   6.    │   Node online.      │                        │
         │   Waiting for       │                        │
         │   desired state.    │                        │
         │                     │                        │
   7.    │ (Admin configures)  │                        │
         │                     │                        │
   8.    │──── pull desired ──▶│                        │
         │   GET /api/node/v1/desired-state             │
         │                     │                        │
         │◀─── desired state ─│                        │
         │   {revision: 1,     │                        │
         │    state_json: ...} │                        │
         │                     │                        │
   9.    │   Apply local       │                        │
         │   reconciliation    │                        │
         │                     │                        │
   10.   │──── report actual ─▶│                        │
         │   {applied_revision: 1,                      │
         │    status: online}                           │
         │                     │                        │
         │   Node fully        │                        │
         │   operational.      │                        │
         │                     │                        │
         │   Heartbeat loop    │                        │
         │   continues every   │                        │
         │   30s.              │                        │
```

---

## 2. Install Script

### 2.1 Design

The install script (`install.sh`) is a self-contained shell script hosted on the control
plane. It is the entry point for all new node deployments.

### 2.2 Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `--control` | ✅ | — | Control plane URL (e.g. `https://aegis.example.com`) |
| `--join-token` | ✅ | — | One-time join token |
| `--node-name` | ❌ | auto-detect hostname | Human-readable node name |
| `--role` | ❌ | `worker` | Comma-separated roles: `gateway,relay,worker,dev` |
| `--version` | ❌ | latest | Specific aegis version to install |
| `--insecure` | ❌ | false | Skip TLS verification (for test/self-signed) |

### 2.3 Script Flow

```bash
#!/bin/sh
# install.sh — Aegis Node Bootstrap

set -e

# 1. Parse arguments
# 2. Detect OS + arch (linux/amd64, linux/arm64, darwin/amd64)
# 3. Download aegis binary from control plane or release URL
#    curl -fsSL "$CONTROL/api/node/v1/download?version=$VERSION&os=$OS&arch=$ARCH"
# 4. Verify binary checksum (SHA256 from control plane response)
# 5. Install binary to /usr/local/bin/aegis
# 6. Create /etc/aegis/ directory
# 7. Register node via join API → get node_id + node_secret
#    POST "$CONTROL/api/node/v1/join"
# 8. Write /etc/aegis/node.yaml with node_id, control_plane URL, roles
# 9. Write /etc/aegis/node.secret with node_secret (0600 permissions)
# 10. Install systemd service: /etc/systemd/system/aegis-node.service
# 11. Enable + start aegis-node service
# 12. Print success message with node_id
```

### 2.4 Download Endpoint

```yaml
GET /api/node/v1/download?version=latest&os=linux&arch=amd64
  Response: binary stream
  Header: X-Checksum-SHA256: <hex>

GET /api/node/v1/download?version=v1.8C&os=darwin&arch=arm64
  Response: binary stream (for dev machines)
```

The control plane serves the aegis binary for download. In a production deployment,
this could be proxies to a release artifact store. For initial v1.8C, the binary
is served directly by the control plane's HTTP server.

### 2.5 Safety Checks

- Script refuses to run as root unless `--role gateway` or `--role relay` is set
  (dev/worker nodes should run as a dedicated `aegis` user)
- Script verifies SHA256 checksum before installing
- Script does not overwrite existing `/etc/aegis/node.yaml` without `--force`
- Join token is used once — if registration fails, a new join token must be created

---

## 3. Join Token Design

### 3.1 Purpose

A join token is a **short-lived, single-use credential** that authorizes a new machine
to register as an Aegis node. It is **not** a long-term node credential.

### 3.2 Token Record

```
join_token_id:       string       // unique identifier (e.g. "jt_abc123")
token:               string       // the actual token value (random 32-byte hex)
description:         string       // human-readable description of why this token exists

bound_roles:         []string     // optional — restrict to specific roles ("gateway,relay")
bound_node_name:     string       // optional — restrict to specific node name
bound_source_ip:     string       // optional — restrict to specific source IP CIDR

expires_at:          timestamp    // token expiry time
used_at:             timestamp    // null until first use = not yet used
used_by_node_id:     string       // which node registered with this token

created_at:          timestamp
created_by:          string       // admin who created this token
```

### 3.3 Token Lifecycle

```
1. Admin creates join token:
   POST /api/admin/v1/join-tokens
   {description: "server-c deployment", roles: "gateway,relay", expires_in: "24h"}
   → Response: {join_token_id: "jt_abc123", token: "a1b2c3...", expires_at: "..."}

2. Token is used in install script:
   curl ... | sh -s -- --join-token a1b2c3...

3. Control plane validates:
   - Token exists? ✅
   - Token expired? ❌ if expired → reject
   - Token already used? ❌ if used → reject
   - Role matches? ✅ optional check
   - Source IP matches? ✅ optional check

4. On successful registration:
   - token.used_at = now
   - token.used_by_node_id = <new_node_id>
   - Token is permanently invalidated

5. Failed registration does NOT invalidate the token
   (network error, timeout, etc. — retry allowed)
```

### 3.4 Admin API

```
POST   /api/admin/v1/join-tokens          → Create join token
GET    /api/admin/v1/join-tokens           → List join tokens (value redacted)
GET    /api/admin/v1/join-tokens/{id}      → Get token metadata (value redacted)
DELETE /api/admin/v1/join-tokens/{id}      → Revoke join token early
```

### 3.5 Security Properties

- Token value is random 32-byte hex (256 bits of entropy)
- Token value is only returned once at creation (on subsequent GET, value is redacted)
- Token stored in DB as SHA-256 hash (never plaintext)
- Token can be explicitly revoked by admin
- Token expiry is mandatory (no permanent join tokens)
- Default expiry: 24 hours. Max: 7 days.
- Join token cannot be used for any purpose other than node registration

---

## 4. Node Credential Design

### 4.1 Purpose

A node credential is a **long-term secret** used by a registered node for all subsequent
communication with the control plane (heartbeat, desired state pull, actual state report).

### 4.2 Credential Record

```
node_id:             string       // assigned by control plane during registration
node_secret:         string       // random 64-byte hex (512 bits) — stored as hash in DB
node_secret_hash:    string       // SHA-256 hash of node_secret (what DB stores)

valid_from:          timestamp    // credential activation time
expires_at:          timestamp    // credential expiry (null = no expiry)
rotated_at:          timestamp    // last rotation time
```

### 4.3 Credential Lifecycle

```
1. On registration:
   Control plane generates node_secret (64 random bytes → hex)
   Returns node_secret in join response (ONCE)
   Stores SHA-256 hash in node_credentials table

2. Node writes node_secret to /etc/aegis/node.secret (0600)

3. All subsequent API calls use node_secret:
   - Heartbeat: POST /api/node/v1/heartbeat
     Header: X-Aegis-Node-ID: <node_id>
     Header: X-Aegis-Node-Token: <node_secret>
   
4. Token validation:
   Control plane looks up node_id
   Fetches stored hash
   SHA-256(provided_secret) == stored_hash?
   If match → authorized
   If not match → 401 Unauthorized

5. Rotation (future):
   Admin POST /api/admin/v1/nodes/{id}/rotate-secret
   Node receives new secret in heartbeat response
   Node updates local /etc/aegis/node.secret
```

### 4.4 Join Token ≠ Node Credential

| Property | Join Token | Node Credential |
|----------|-----------|----------------|
| **Purpose** | Authorize registration | Authorize node ↔ control plane communication |
| **Lifetime** | Short (hours/days) | Long (months/years) |
| **Reusable** | No (single-use) | Yes (every request) |
| **Role binding** | Optional | Implied (from node record) |
| **Returned** | Once at creation | Once at registration |
| **DB storage** | SHA-256 hash | SHA-256 hash |
| **Rotation** | Not applicable | Supported (future) |
| **Can be revoked** | ✅ | ✅ |

---

## 5. Registration Sequence

### 5.1 Join API

```
POST /api/node/v1/join
  Request:
  {
    "join_token": "a1b2c3...",
    "node_name": "server-c",
    "roles": ["gateway", "relay"],
    "public_ip": "43.160.211.233",
    "private_ip": "10.0.0.3",
    "agent_version": "v1.8C",
    "os": "linux",
    "arch": "amd64",
    "hostname": "server-c"
  }

  Response 200:
  {
    "node_id": "nd_c",
    "node_secret": "a1b2c3d4e5...",
    "node_secret_redacted": false,
    "status": "registered"
  }

  Response 401:
  { "error": "invalid or expired join token" }

  Response 403:
  { "error": "join token does not allow role 'gateway'" }
  { "error": "join token bound to different node name" }
```

### 5.2 Registration Processing

```
On server (control plane):

  1. Validate join token
     a. Look up by SHA-256(token)
     b. Check expires_at
     c. Check used_at (must be null)
     d. Check role binding (if set)
     e. Check source IP binding (if set)

  2. Mark token as used
     UPDATE join_tokens SET used_at=NOW(), used_by_node_id=<new_id>

  3. Create node record
     INSERT INTO nodes (node_id, node_name, hostname, public_ip, private_ip,
                         agent_version, os, arch, role, status, capabilities...)
     VALUES (...)

  4. Create node credential
     node_secret = randomHex(64)
     INSERT INTO node_credentials (node_id, node_secret_hash, ...)
     VALUES (node_id, SHA256(node_secret), ...)

  5. Create initial desired state entry (empty)
     INSERT INTO node_desired_states (node_id, revision, state_json, ...)
     VALUES (node_id, 0, '{"gateways":[],"routes":[],"policies":[]}', ...)

  6. Return node_id + node_secret (once)
```

---

## 6. Service Installation

### 6.1 Systemd Service (Linux)

```ini
[Unit]
Description=Aegis Node Runtime
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/aegis node
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/etc/aegis
ReadWritePaths=/var/lib/aegis
PrivateTmp=true
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

# If gateway/relay role:
# AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
```

### 6.2 Launchd Service (macOS — dev nodes)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.aegis.node</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/aegis</string>
        <string>node</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/aegis-node.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/aegis-node.log</string>
</dict>
</plist>
```

### 6.3 Node Runtime Directory Layout

```
/etc/aegis/
  ├── node.yaml          # Node configuration
  ├── node.secret        # Node credential (0600, root:root)
  └── secret.key         # Master key (0600, root:root, optional — existing)

/var/lib/aegis/
  ├── node.db            # Local SQLite (desired state cache, local state)
  └── routing-table.json # Cached routing table (optional)

/var/log/aegis-node.log  # Node runtime logs
```

---

## 7. Node Configuration File

### 7.1 node.yaml

```yaml
# /etc/aegis/node.yaml — Aegis Node Configuration

# Node identity (assigned during registration)
node_id: nd_c
node_name: server-c

# Control plane connection
control_plane:
  url: https://aegis.example.com
  heartbeat_interval: 30s
  heartbeat_timeout: 10s

# Node roles (comma-separated)
roles:
  - gateway
  - relay

# Network configuration
network:
  public_ip: 43.160.211.233       # detected or configured
  public_reachable: true           # node has open ports on public IP
  public_gateway_enabled: true     # node acts as public gateway
  private_ip: 10.0.0.3            # optional
  private_reachable: true          # node accessible on private network
  private_gateway_enabled: true    # node acts as private gateway

# Local gateway (dev nodes, or gateway nodes)
local_gateway:
  enabled: true
  bind_addr: 127.0.0.1
  port: 8080

# Local DNS (dev nodes only)
local_dns:
  enabled: false                  # default: disabled
  bind_addr: 127.0.0.1
  port: 53
  upstream: 8.8.8.8

# Diagnostics
diagnostics:
  enabled: true
  interval: 300s                   # run full diagnostics every 5 minutes

# Provider configuration
providers:
  caddy:
    enabled: true
    config_path: /etc/caddy/Caddyfile
  haproxy:
    enabled: false

# Feature flags
features:
  relay_handler: true              # enable /__aegis/relay (gateway/relay nodes)
  local_gateway: true              # enable local HTTP gateway
  transparent_dns: false           # enable local DNS resolver

# Logging
logging:
  level: info                      # debug | info | warn | error
  format: json                     # json | text
```

---

## 8. Post-registration Lifecycle

After registration and service installation, the node enters its runtime loop:

```
┌────────────────────────────────────────────────────────────────┐
│                   Node Runtime Loop                              │
│                                                                  │
│   ┌─────────────┐     ┌──────────────────┐     ┌────────────┐   │
│   │  Heartbeat   │────▶│  Check Revision  │◀────│  Timer     │   │
│   │  (30s)       │     │  (heartbeat resp)│     │  (every    │   │
│   └──────┬───────┘     └────────┬─────────┘     │   30s)    │   │
│          │                      │                └────────────┘   │
│          │                      ▼                                  │
│          │              ┌──────────────────┐                       │
│          │              │ Revision behind? │                       │
│          │              └───────┬───┬──────┘                       │
│          │                      │   │                               │
│          │                 Yes  │   │  No                           │
│          │                      ▼   ▼                               │
│          │              ┌──────────────────┐                       │
│          │              │     Sleep        │                       │
│          │              │  (next heartbeat)│                       │
│          │              └──────────────────┘                       │
│          │                      │                                   │
│          │                      ▼  (desired state pull)            │
│          │              ┌──────────────────┐                       │
│          │              │ Pull Desired     │                       │
│          │              │ State            │                       │
│          │              │ (GET /desired)   │                       │
│          │              └───────┬──────────┘                       │
│          │                      │                                   │
│          │                      ▼                                   │
│          │              ┌──────────────────┐                       │
│          │              │ Validate +       │                       │
│          │              │ Reconcile Local  │                       │
│          │              └───────┬──────────┘                       │
│          │                      │                                   │
│          │                      ▼                                   │
│          │              ┌──────────────────┐                       │
│          │              │ Report Actual    │                       │
│          │              │ State            │                       │
│          │    ┌────────▶│ (via next        │                       │
│          │    │         │  heartbeat)      │                       │
│          │    │         └──────────────────┘                       │
│          │    │                                                    │
│          └────┴─────── loop continues until shutdown              │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### 8.1 Startup Sequence

```
aegis node start:
  1. Load /etc/aegis/node.yaml
  2. Load /etc/aegis/node.secret
  3. Validate configuration
  4. Load master key (if available)
  5. Register signal handlers (SIGTERM, SIGINT)
  6. Start heartbeat loop
  7. Start desired state poll loop
  8. Start local gateway (if enabled)
  9. Start local DNS (if enabled)
  10. Start diagnostics runner (if enabled)
  11. Mark node status as "online"
  12. Block on signal
```

### 8.2 Shutdown Sequence

```
aegis node stop:
  1. Receive SIGTERM
  2. Mark node status as "offline" via heartbeat
  3. Stop local gateway
  4. Stop local DNS
  5. Stop diagnostics runner
  6. Stop heartbeat loop
  7. Flush logs
  8. Exit (0)
```

---

## 9. Security Considerations

### 9.1 Secret Files

| File | Permissions | Owner | Content |
|------|-------------|-------|---------|
| `/etc/aegis/node.secret` | 0600 | root:root | Node credential (64-byte hex) |
| `/etc/aegis/secret.key` | 0600 | root:root | Master key (existing, unchanged) |
| `/etc/aegis/node.yaml` | 0644 | root:root | Node config (contains no secrets) |

### 9.2 Node Credential Protection

- Node secret is **returned once** at registration
- If the secret is lost, admin can issue a credential rotation
- Node secret is hashed in control plane DB (SHA-256)
- Node secret is NOT logged by either the control plane or the node
- All node API calls use HMAC-SHA256 signing with timestamp replay protection
  (same pattern as GatewayLink auth)

### 9.3 Node API Authentication

```
Every node → control plane API call includes:

  X-Aegis-Node-ID:    nd_c
  X-Aegis-Node-Token: <node_secret>
  X-Aegis-Timestamp:  2026-06-27T10:00:00Z  (RFC3339, UTC)
  X-Aegis-Signature:  HMAC-SHA256(node_id + timestamp + body, node_secret)

Control plane validates:
  1. Timestamp within 5-minute window (replay protection)
  2. Node ID exists and status != offline
  3. Node secret hash matches (SHA-256 comparison)
  4. HMAC signature matches
```

### 9.4 Join Token Protection

- Join tokens have mandatory expiry (default 24h, max 7d)
- Join tokens are single-use
- Join token value stored as SHA-256 hash in DB
- Join token value redacted in list/get API responses
- Source IP binding is optional but recommended for production deployments

---

## Marker

```
v1.8C-0 Node Bootstrap Design:    COMPLETE ✅
```
