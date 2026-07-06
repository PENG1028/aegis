# Node Capability System

## Overview

Each Aegis node has a set of **detected capabilities** that describe what the node can do. This system allows:

- **Runtime detection** of installed software and supported features
- **Capability querying** via the Admin API
- **Capability diff** to track changes over time
- **UI-driven action gating** — actions are grayed out with explanations when capabilities are missing

## Capability Constants

| Capability | Description | Detection Method |
|---|---|---|
| `gateway_enabled` | Gateway proxy available | Caddy or HAProxy installed |
| `caddy_installed` | Caddy binary found | `exec.LookPath("caddy")` |
| `haproxy_installed` | HAProxy binary found | `exec.LookPath("haproxy")` |
| `tls_supported` | TLS termination support | Gateway enabled (both support TLS) |
| `dns_control_available` | DNS management tools | `dig` or `nslookup` available |
| `hot_reload_supported` | Graceful reload | `systemctl` available |
| `edge_mux_supported` | HAProxy SNI passthrough | HAProxy installed |

## Abstraction Boundary

```
┌─────────────────────────────────┐
│         Admin API / UI          │
│  queries capabilities to decide │
│  which actions are available    │
├─────────────────────────────────┤
│     NodeCapabilities Map        │
│  map[string]bool (JSON in DB)   │
├─────────────────────────────────┤
│     DetectCapabilities()        │
│  Runtime binary/path checks     │
└─────────────────────────────────┘
```

**The capability system does NOT directly invoke Caddy or HAProxy.** It only checks for their presence.

## UI Grayscale Rules

Actions are **grayed out** (not hidden) with an explanation:

| Missing Capability | Disabled Action | Reason |
|---|---|---|
| `gateway_enabled` | create_gateway_domain | "Gateway not installed on this node" |
| `dns_control_available` | bind_domain | "DNS control not available on this node" |
| `hot_reload_supported` | hot_reload | "Hot reload not supported on this node" |
| `tls_supported` | enable_tls | "TLS not supported on this node" |

## API

```
GET  /api/admin/v1/nodes/{id}/capabilities     — Query capabilities + disabled actions
POST /api/admin/v1/nodes/{id}/refresh-capabilities — Re-detect and return diff
```

## Capability Change Detection

When `RegisterCurrent()` runs on node heartbeat:
1. New capabilities are detected
2. Compared against stored capabilities
3. `CapabilityDiff` tracks added/removed/changed flags
4. Significant changes can trigger node events

## Design Principle

> Capabilities describe what a node CAN do, not what it IS doing. The gateway abstraction layer checks capabilities before allowing operations. The UI queries capabilities to control action availability.
