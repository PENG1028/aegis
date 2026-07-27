# Gateway Abstraction Layer

## Overview

The gateway abstraction layer provides a **provider-agnostic** model for managing gateway domains, routes, and listeners. No Caddy or HAProxy specifics appear in the core model.

## Abstraction Boundary

```
┌──────────────────────────────────────┐
│          Admin API / Action API       │
│  create_domain / attach_route / TLS  │
├──────────────────────────────────────┤
│       GatewayService (abstract)      │
│  Capability checks + CRUD            │
├──────────────────────────────────────┤
│  GatewayDomain / Route / Listener    │
│  Pure data models (no provider dep)  │
├──────────────────────────────────────┤
│  Caddy/HAProxy Backend (existing)    │
│  Provider renders from abstract data │
└──────────────────────────────────────┘
```

## Models

### GatewayDomain
| Field | Type | Description |
|---|---|---|
| id | string | Primary key |
| domain | string | Domain name (UNIQUE) |
| node_id | string | Target node |
| tls_enabled | bool | TLS termination enabled |
| tls_provider | string | Selected TLS backend (caddy/haproxy/empty) |
| status | string | active/disabled/provisioning/failed |

### GatewayRoute
| Field | Type | Description |
|---|---|---|
| id | string | Primary key |
| domain_id | string | FK to gateway_domains |
| path | string | URL path prefix |
| target_service | string | Backend service name |
| target_port | int | Backend port |
| protocol | string | http/https/tcp |
| status | string | active/disabled |

### GatewayListener
| Field | Type | Description |
|---|---|---|
| id | string | Primary key |
| node_id | string | Target node |
| port | int | Listen port |
| tls_enabled | bool | TLS on this listener |
| protocol | string | http/https/tcp/tls_mux |
| status | string | active/disabled |

## Service Operations

| Operation | Description | Capability Required |
|---|---|---|
| CreateDomain | Bind domain to node | gateway_enabled |
| AttachRoute | Add path route to domain | gateway_enabled |
| DetachRoute | Remove path route | — |
| ListDomains | Query all domains | — |
| UpdateTLSPolicy | Enable/disable TLS | tls_supported |
| HealthCheck | Aggregate health status | — |

## API

```
POST   /api/admin/v1/gateway/domains         — Create gateway domain
GET    /api/admin/v1/gateway/domains         — List all domains
POST   /api/admin/v1/gateway/routes          — Attach route to domain
DELETE /api/admin/v1/gateway/routes/{id}      — Detach route
GET    /api/admin/v1/gateway/listeners       — List all listeners
PUT    /api/admin/v1/gateway/domains/{id}/tls — Update TLS policy
```

## Design Principle

> The gateway abstraction describes WHAT routing should exist, not HOW it is implemented. Caddy and HAProxy are backend renderers that consume this abstract model. The Admin API never exposes provider details to the caller.
