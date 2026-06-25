# Aegis Minimal Multi-Node Smoke Test — v1.7U

## Overview

This document defines the **minimal multi-node validation** for Aegis cluster operations. It verifies basic distributed coordination without complex scheduling, canary deployment, or multi-region scenarios.

## Scope

- Single leader election
- Follower registration and sync
- State version agreement
- ACK timeout detection
- Drift detection and recording
- Node event logging
- Follower write protection

## Out of Scope

- Multi-region failover
- Network partition healing (beyond detection)
- Automatic leader failover timing guarantees
- Cross-datacenter latency optimization
- Canary/staged rollout across nodes

---

## Architecture Context

```
┌──────────────────────────────────────────────────────┐
│                  Aegis Control Plane                  │
│                                                      │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐       │
│  │  Node A  │    │  Node B  │    │  Node C  │       │
│  │ (Leader) │    │(Follower)│    │(Follower)│       │
│  │ sv=100   │    │ sv=100   │    │ sv=100   │       │
│  └────┬─────┘    └────┬─────┘    └────┬─────┘       │
│       │               │               │              │
│       └───────────────┼───────────────┘              │
│                       │ SQLite (per-node)             │
│                       │ State push via HTTP           │
└──────────────────────────────────────────────────────┘
                        │
        ┌───────────────┼───────────────┐
        │               │               │
   ┌────▼────┐    ┌────▼────┐    ┌────▼────┐
   │ HAProxy │    │ Caddy   │    │ Caddy   │
   │  :443   │    │  :80    │    │  :8443  │
   └─────────┘    └─────────┘    └─────────┘
                  Data Plane (identical config)
```

**Key**: Each Aegis node manages its own SQLite database. State is pushed from leader to followers via HTTP API. Config is rendered identically on all nodes.

---

## Test Setup

### Option A: Single Machine (Simulated)

Run multiple Aegis instances on different ports with separate SQLite databases:

```bash
# Node A (leader) — port 9001, db = /tmp/aegis-node-a.db
AEGIS_DB=/tmp/aegis-node-a.db AEGIS_PORT=9001 ./aegis serve --port 9001 &
NODE_A_PID=$!

# Node B (follower) — port 9002, db = /tmp/aegis-node-b.db
AEGIS_DB=/tmp/aegis-node-b.db AEGIS_PORT=9002 ./aegis serve --port 9002 &
NODE_B_PID=$!

# Node C (follower) — port 9003, db = /tmp/aegis-node-c.db
AEGIS_DB=/tmp/aegis-node-c.db AEGIS_PORT=9003 ./aegis serve --port 9003 &
NODE_C_PID=$!
```

### Option B: Multiple VPS

Each node on separate VPS with network connectivity between them. Same binary, separate SQLite databases.

---

## Test Scenarios

### Scenario 1: Leader Node Startup

**Steps:**
1. Bootstrap + start Node A
2. Check leader election

```bash
curl -s http://localhost:9001/api/system/status | jq '{node_id, leader, state_version}'
```

**Expected:**
```json
{
  "node_id": "...",
  "leader": "<node-a-id>",
  "state_version": 1
}
```

**Acceptance:**
- Single node becomes leader automatically
- `leader` field populated with node ID
- `state_version` initialized

---

### Scenario 2: Follower Node Registration

**Steps:**
1. Start Node B (already has DB but no leader claim)
2. Node B registers with leader (Node A)

```bash
# Check Node B recognizes leader
curl -s http://localhost:9002/api/system/status | jq '{node_id, leader, state_version}'
```

**Expected:**
- Node B does NOT claim leadership
- `leader` field points to Node A's ID
- `state_version` may be 0 until first sync

```bash
# List all nodes from leader
curl -s http://localhost:9001/api/admin/v1/nodes \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[] | {node_id, is_current, state_version}'
```

**Expected:** Both Node A (is_current=true) and Node B listed.

**Acceptance:**
- Follower registers without error
- Leader can see all nodes
- Each node knows who the leader is

---

### Scenario 3: State Version Increment

**Steps:**
1. On leader (Node A), create a resource or trigger state change
2. Check state_version increments

```bash
# Before
curl -s http://localhost:9001/api/system/status | jq '.state_version'
# e.g., 1

# Create a route (or any state-changing operation)
# ...

# After
curl -s http://localhost:9001/api/system/status | jq '.state_version'
# EXPECTED: 2 (incremented)
```

**Acceptance:**
- state_version increments on leader when state changes
- state_version is monotonic (never decreases)

---

### Scenario 4: Follower Sync

**Steps:**
1. Leader state_version = N (after Scenario 3)
2. Trigger sync push to Node B
3. Verify Node B state_version matches

```bash
# Check follower state_version
curl -s http://localhost:9002/api/system/status | jq '.state_version'
# EXPECTED: matches leader's state_version
```

**Acceptance:**
- Follower state_version catches up to leader
- Sync does not produce errors

---

### Scenario 5: ACK Quorum

**Steps:**
1. With 3 nodes running (A=leader, B=follower, C=follower)
2. Trigger apply on leader
3. Check that ACKs are collected

```bash
# Check node events for ACKs
curl -s http://localhost:9001/api/admin/v1/node-events \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[] | select(.event_type | startswith("ack"))'
```

**Acceptance:**
- Leader records ACK from each follower
- Quorum is achieved (2/3 ACKs minimum)
- Missing ACK from one node is tolerated

---

### Scenario 6: ACK Timeout Detection

**Steps:**
1. Stop Node C (simulate unavailable)
2. Trigger apply on leader
3. Verify ACK timeout recorded

```bash
# Check for ACK timeout event
curl -s http://localhost:9001/api/admin/v1/node-events \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[] | select(.event_type == "ack_timeout")'
```

**Expected:**
```json
{
  "event_type": "ack_timeout",
  "node_id": "<node-c-id>",
  "severity": "warning",
  "message": "ACK timeout for node <node-c-id>"
}
```

**Acceptance:**
- ACK timeout event created for unavailable node
- Severity = "warning" (not "error" — single node timeout is not critical)
- Apply proceeds with remaining quorum

---

### Scenario 7: Drift Detection

**Steps:**
1. Ensure all nodes are synced (state_version = N)
2. Simulate drift: update state on leader without pushing to follower

```bash
# On leader: state_version is now N+1
# On follower B: state_version is still N (drift!)
```

3. Trigger reconcile or check drift detection

```bash
# Check for drift events
curl -s http://localhost:9001/api/admin/v1/node-events \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[] | select(.event_type == "drift_detected")'
```

**Expected:**
```json
{
  "event_type": "drift_detected",
  "node_id": "<node-b-id>",
  "severity": "warning",
  "message": "state_version drift: leader=2, node_b=1"
}
```

**Acceptance:**
- Drift is detected and recorded as node_event
- Drift severity reflects version gap size
- Reconcile can repair the drift

---

### Scenario 8: Reconcile Repair

**Steps:**
1. After drift detected (Scenario 7)
2. Run reconcile

```bash
curl -s -X POST http://localhost:9001/api/admin/v1/system/doctor \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq .
```

3. Check follower state_version after reconcile

```bash
curl -s http://localhost:9002/api/system/status | jq '.state_version'
# EXPECTED: matches leader
```

**Acceptance:**
- Reconcile event recorded: `reconcile_started` and `reconcile_finished`
- Drift repaired (state_versions match)
- No duplicate resources created during repair

---

### Scenario 9: Follower Write Protection

**Steps:**
1. Attempt system-level mutation on follower (Node B)

```bash
# Try to create a scope on follower (should be rejected)
curl -s -X POST http://localhost:9002/api/admin/v1/scopes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_COOKIE" \
  -d '{"name":"should-fail"}' | jq .
```

**Expected:** Request rejected or forwarded to leader.

```bash
# Verify no scope created on follower
curl -s http://localhost:9002/api/admin/v1/scopes \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[] | select(.name == "should-fail")'
# EXPECTED: no results
```

**Acceptance:**
- Follower rejects or forwards system-state writes
- Follower does not create independent state
- Leader remains single source of truth for mutations

---

### Scenario 10: Node Capability Refresh

**Steps:**
1. Query node capabilities on leader

```bash
curl -s http://localhost:9001/api/admin/v1/nodes/<node-id>/capabilities \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq .
```

**Expected:**
```json
{
  "node_id": "...",
  "capabilities": {
    "gateway_enabled": true,
    "caddy_installed": true,
    "haproxy_installed": true,
    "tls_supported": true,
    "hot_reload_supported": true,
    "edge_mux_supported": true
  }
}
```

2. Refresh capabilities

```bash
curl -s -X POST http://localhost:9001/api/admin/v1/nodes/<node-id>/refresh-capabilities \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq .
```

**Acceptance:**
- Capabilities reflect runtime state of each node
- Refresh re-detects and updates capabilities
- Capability diff recorded as node_event if changed

---

## Cleanup

```bash
kill $NODE_A_PID $NODE_B_PID $NODE_C_PID
rm -f /tmp/aegis-node-*.db
```

---

## Acceptance Criteria

| # | Criterion | Scenario | Status |
|---|-----------|----------|--------|
| 1 | Single leader elected | 1 | |
| 2 | Follower registers and visible to leader | 2 | |
| 3 | state_version increments on mutation | 3 | |
| 4 | Follower syncs to leader state_version | 4 | |
| 5 | ACK quorum achieved | 5 | |
| 6 | ACK timeout recorded for unavailable node | 6 | |
| 7 | Drift detected and recorded | 7 | |
| 8 | Reconcile repairs drift | 8 | |
| 9 | Follower rejects system-state writes | 9 | |
| 10 | Node capabilities refreshable | 10 | |

---

## Limitations

1. **Single-machine test**: Uses separate SQLite DBs on same machine; no real network latency
2. **No network partition**: Split-brain detection logic exists but full partition testing requires iptables manipulation
3. **Manual sync trigger**: Auto-sync interval is configurable; tests use manual triggers for deterministic results
4. **No persistent leader lease**: Leader election is based on DB state, not distributed consensus (Raft/Paxos)
