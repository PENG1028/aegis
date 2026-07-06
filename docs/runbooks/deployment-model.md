# Deployment Version Model

## Overview

The deployment model provides **version tracking** for service deployments across nodes. Each deployment records what version was deployed, to which nodes, with what strategy, and the per-node application status.

## Abstraction Boundary

```
┌─────────────────────────────────────┐
│         Admin API                   │
│  create / list / rollback           │
├─────────────────────────────────────┤
│       DeploymentService             │
│  Version tracking + instance mgmt  │
├─────────────────────────────────────┤
│  Deployment / DeploymentInstance    │
│  Pure data models                  │
├─────────────────────────────────────┤
│  No scheduler / no deployment engine│
│  (future: canary/staged rollout)   │
└─────────────────────────────────────┘
```

## Models

### Deployment
| Field | Type | Description |
|---|---|---|
| id | string | Primary key |
| version | string | Deployment version label |
| service_id | string | Target service |
| target_nodes | []string | Node IDs (JSON array) |
| rollout_strategy | string | all / canary / staged |
| status | string | pending / running / success / failed / rolled_back |

### DeploymentInstance
| Field | Type | Description |
|---|---|---|
| id | string | Primary key |
| deployment_id | string | FK to deployments |
| node_id | string | Target node |
| status | string | Per-node deployment status |
| last_applied_version | string | Version actually applied on this node |
| applied_at | timestamp | When applied |
| error_message | string | Failure reason if any |

## Rollout Strategies

| Strategy | Description |
|---|---|
| `all` | Deploy to all nodes simultaneously |
| `canary` | Deploy to one node first, verify, then all |
| `staged` | Deploy node by node with verification gates |

## Operations

| Operation | Description |
|---|---|
| CreateDeployment | Create version + per-node instances |
| GetDeployment | Get deployment with instance status |
| ListDeployments | Query all deployments |
| RollbackDeployment | Mark deployment as rolled_back + all instances |

## API

```
POST /api/admin/v1/deployments              — Create deployment
GET  /api/admin/v1/deployments              — List all deployments
GET  /api/admin/v1/deployments/{id}          — Get deployment + instances
POST /api/admin/v1/deployments/{id}/rollback — Rollback deployment
```

## Design Principle

> The deployment model tracks WHAT version is where, not HOW to deploy it. The actual apply/render/reload logic remains in the existing apply pipeline. This model provides the version tracking and per-node state that the UI needs for deployment management.
