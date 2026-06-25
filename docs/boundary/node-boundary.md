# Node Boundary — v1.7AA

## Capability Status

| Capability | Status | Evidence |
|-----------|--------|----------|
| Node registration | ✅ single_node_real_verified | Server A registers on bootstrap |
| Leader election (single) | ✅ single_node_real_verified | Auto-elected on bootstrap |
| State version tracking | ✅ single_node_real_verified | Cluster_state table |
| Node capability detection | ✅ verified | `exec.LookPath` binary checks |
| Capability refresh API | ✅ verified | `POST /nodes/{id}/refresh-capabilities` |
| Node events logging | ✅ verified | Node drifts, leader changes |
| Reachable via admin API | ✅ verified | `GET /api/admin/v1/nodes` |
| Follower registration | ⏳ two_node_pending | To be tested in v1.7AA |
| State version sync | ⏳ two_node_pending | HTTP-based push from leader |
| ACK quorum | ❌ fake_only | FakeCluster only |
| ACK timeout detection | ❌ fake_only | FakeCluster only |
| Drift detection | ❌ fake_only | FakeCluster.CheckDrift() |
| Reconcile repair | ❌ fake_only | ReconcileLoop exists but untested |
| Follower write protection | ❌ unsupported | Auth layer only; no cluster-level guard |
| Multi-node production | ❌ unsupported | Beyond current scope |

## Single-Node Verified (Server A)

The current Aegis deployment is a single-node control plane:
- Server A runs Aegis + Caddy + HAProxy all on one machine
- There is NO second Aegis instance as a follower
- Node registration creates one record for the local machine

## Two-Node Topology (v1.7AA)

```
Server A (43.160.211.232)           Server B (43.159.34.11)
┌─────────────────────┐            ┌──────────────────────┐
│ Aegis (leader)      │            │ python3 http.server  │
│ Caddy :80           │            │ :3000 (target)       │
│ HAProxy :443        │            │ (no Aegis - just     │
│ Route → Server B    │───────────│  remote target)      │
└─────────────────────┘            └──────────────────────┘
```

## Aegis Node Architecture Notes

- Node records are created per-machine via `nodeSvc.RegisterCurrent()`
- Only one node (the current machine) is registered in single-node mode
- Leader election uses DB state (`leader_elected_at`), not distributed consensus
- There is NO separate Aegis process on Server B in this test
- The "two-node" term here refers to **two machines in the data path**, not two control plane nodes
