# Control Plane UX Rules

## Overview

This document defines how the Aegis control plane should behave from a UI/UX perspective. The UI does NOT exist yet — these rules define the contract that a future UI would follow.

## Core UX Principle

> **Gray out, don't hide.** When an action is unavailable, show it as disabled with an explanation of why. Never hide controls based on state — the user should always understand what's possible and why something isn't.

## Capability-Driven UI Rules

### Gateway Actions

| Action | Enabled When | Disabled Reason |
|---|---|---|
| Create domain | node.capabilities.gateway_enabled = true | "Gateway not installed on this node" |
| Enable TLS | node.capabilities.tls_supported = true | "TLS not supported on this node" |
| Hot reload | node.capabilities.hot_reload_supported = true | "Hot reload not supported on this node" |
| Bind domain | node.capabilities.dns_control_available = true | "DNS control not available on this node" |

### Node Actions

| Action | Enabled When | Disabled Reason |
|---|---|---|
| All gateway actions | node.status = online | "Node is offline" |
| Deploy to node | node in deployment.target_nodes | "Node not in deployment target list" |

## Control Flow

### Domain Binding Flow
```
1. User enters domain + target
2. UI checks: node.capabilities.gateway_enabled?
   ├─ NO  → "Create Domain" button grayed + reason shown
   └─ YES → button enabled
3. User clicks "Create Domain"
4. POST /api/admin/v1/gateway/domains
5. System validates capability server-side
6. Success → domain appears in list
   Failure → error message with CODE + explanation
```

### Deployment Flow
```
1. User selects service + target nodes + strategy
2. UI shows per-node capability summary
3. User clicks "Deploy"
4. POST /api/admin/v1/deployments
5. System creates Deployment + per-node DeploymentInstances
6. Status: pending → running → success/failed per node
7. Failed nodes show error_message
8. Rollback available via POST /deployments/{id}/rollback
```

## System Control Logic

### What MUST go through abstraction
- Gateway domain creation, route attachment, TLS policy changes
- Deployment creation and version tracking
- Node capability queries before any gateway operation

### What MUST NOT be exposed to UI
- Provider names (caddy_http, haproxy_edge_mux)
- Config file paths
- Raw rendered configs
- Reload commands
- Internal state versions

## Error Communication

All errors returned to the UI must include:
- `code`: machine-readable error code (e.g., `GATEWAY_NOT_ENABLED`)
- `message`: human-readable explanation
- `details` (optional): additional context

Example:
```json
{
  "error": {
    "code": "GATEWAY_NOT_ENABLED",
    "message": "Cannot create domain: gateway not installed on node desk-01",
    "details": "missing capabilities: [gateway_enabled]"
  }
}
```

## Multi-Node Awareness

The UI must show:
- Which node is the leader
- Which nodes are online/offline
- Per-node capability summary
- Per-node deployment status
- Version drift between nodes

## Design Principle

> The control plane provides structured, queryable state. The UI consumes this state to make decisions. The system never makes assumptions about what the UI will do — it provides the data and enforces the rules server-side.
