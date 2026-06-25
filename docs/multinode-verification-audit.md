# Multi-Node Verification Audit — v1.7V

## Verification Levels

| Level | Meaning |
|-------|---------|
| **REAL_VERIFIED** | Tested with real multi-node processes, network communication, actual election/sync |
| **FAKE_VERIFIED** | Tested only with FakeCluster in-memory simulation (no real nodes) |
| **UNTESTED** | Code exists but no test exercises it (neither fake nor real) |
| **NOT_IMPLEMENTED** | Feature claimed but code doesn't exist |

---

## Multi-Node Capability Audit

### 1. Leader Election

| Property | Value |
|----------|-------|
| **Code exists** | ✅ `internal/cluster/leader.go` — `LeaderService` with `Elect()`, `IsLeader()` |
| **FakeCluster support** | ✅ `fake.NewFakeCluster()` — leader set as node[0] |
| **FakeCluster test** | ✅ `smoke_test.go:TestFakeClusterNodeSync` — verifies leader exists |
| **Real multi-node test** | ❌ No test with actual multiple Aegis processes |
| **Verification level** | **FAKE_VERIFIED** |

**Real coverage gap:** Leader election with multiple real Aegis instances on separate ports/DBs has never been tested. The FakeCluster `GetLeader()` just reads a struct field — no election protocol was exercised.

### 2. Follower Registration

| Property | Value |
|----------|-------|
| **Code exists** | ✅ `internal/node/service.go` — `RegisterCurrent()` writes to nodes table |
| **FakeCluster support** | ✅ `fake.NewFakeCluster()` — creates multiple FakeNode structs |
| **FakeCluster test** | ✅ `smoke_test.go:TestFakeClusterNodeSync` — verifies 3 nodes exist |
| **Real multi-node test** | ❌ No test with second Aegis instance registering to a leader |
| **Verification level** | **FAKE_VERIFIED** |

**Real coverage gap:** `RegisterCurrent()` is designed for the CURRENT machine only. Multi-node registration requires HTTP-based node registration, which FakeCluster doesn't test.

### 3. State Version Sync

| Property | Value |
|----------|-------|
| **Code exists** | ✅ `internal/cluster/state_version.go` — `StateVersion` with `Increment()`, `Current()` |
| **FakeCluster support** | ✅ FakeCluster nodes have `StateVersion` fields, `InjectVersionMismatch()` |
| **FakeCluster test** | ✅ `smoke_test.go:TestFakeClusterVersionMismatch` — versions [100, 90, 80] |
| **Real sync test** | ❌ No test where leader increments version and follower catches up |
| **Verification level** | **FAKE_VERIFIED** |

**Real coverage gap:** State push from leader to follower via HTTP has never been tested. FakeCluster just mutates struct fields in memory.

### 4. ACK Quorum

| Property | Value |
|----------|-------|
| **Code exists** | ✅ `internal/cluster/ack.go` — ACK mechanism with timeout |
| **FakeCluster support** | ✅ `FakeCluster.ACKTimeout` flag |
| **FakeCluster test** | ❌ No test exercises ACKTimeout in smoke tests |
| **Real ACK test** | ❌ No test with multiple nodes sending ACKs |
| **Verification level** | **UNTESTED** |

**Real coverage gap:** ACK mechanism exists in code (`internal/cluster/ack.go`) but has NO tests — not even fake tests. The FakeCluster.ACKTimeout flag exists but is never set in any test.

### 5. ACK Timeout Detection

| Property | Value |
|----------|-------|
| **Code exists** | ✅ `internal/cluster/ack.go` — timeout logic |
| **FakeCluster support** | ✅ `FakeCluster.ACKTimeout` field |
| **FakeCluster test** | ❌ No test |
| **Real timeout test** | ❌ No test |
| **Verification level** | **UNTESTED** |

### 6. Drift Detection

| Property | Value |
|----------|-------|
| **Code exists** | ✅ `internal/cluster/leader.go` or reconcile loop |
| **FakeCluster support** | ✅ `FakeCluster.InjectDrift()`, `FakeCluster.CheckDrift()` |
| **FakeCluster test** | ✅ `smoke_test.go:TestFakeClusterDrift` — tests LOW/MEDIUM/HIGH severity |
| **Real drift test** | ❌ No test with actual state divergence between nodes |
| **Verification level** | **FAKE_VERIFIED** |

**Real coverage gap:** Drift detection in FakeCluster compares struct field values. Real drift detection would require reading state_version from two separate databases and comparing them — not tested.

### 7. Reconcile Repair

| Property | Value |
|----------|-------|
| **Code exists** | ✅ Reconcile loop in cluster package |
| **FakeCluster support** | ❌ FakeCluster has no `Reconcile()` method |
| **FakeCluster test** | ❌ No test |
| **Real reconcile test** | ❌ No test |
| **Verification level** | **UNTESTED** |

### 8. Capability Refresh

| Property | Value |
|----------|-------|
| **Code exists** | ✅ `internal/node/capability.go` — `DetectCapabilities()`, `NodeCapabilities` |
| **FakeCluster support** | ❌ FakeCluster doesn't model capabilities |
| **FakeCluster test** | ❌ No test |
| **Real refresh test** | ❌ No test |
| **Verification level** | **UNTESTED** |

**Note:** `DetectCapabilities()` is called during `RegisterCurrent()` (node/service.go:48) but there's no test that verifies:
- Capabilities are correctly detected on a real machine
- Capability changes are detected on refresh
- Capability changes create node_events

### 9. Follower Write Protection

| Property | Value |
|----------|-------|
| **Code exists** | ✅ `isSystemRoute()` in token middleware blocks service keys from admin routes |
| **FakeCluster support** | ❌ Not a cluster-level concern — it's auth middleware |
| **FakeCluster test** | ❌ No test |
| **Real protection test** | ❌ No test where follower rejects a write |
| **Verification level** | **UNTESTED** |

**Note:** Write protection is implemented at the auth layer (service keys can't access admin routes), NOT at the cluster layer (follower nodes don't reject writes from the leader). A follower node with an admin session can write to its own DB — there's no cluster-level write protection.

---

## Summary

| Capability | Verification Level | Evidence |
|------------|:---:|------|
| Leader election | FAKE_VERIFIED | FakeCluster + smoke test |
| Follower registration | FAKE_VERIFIED | FakeCluster + smoke test |
| State version sync | FAKE_VERIFIED | FakeCluster + smoke test |
| ACK quorum | UNTESTED | Code exists, no tests |
| ACK timeout | UNTESTED | Code exists, no tests |
| Drift detection | FAKE_VERIFIED | FakeCluster + smoke test |
| Reconcile repair | UNTESTED | Code exists, no tests |
| Capability refresh | UNTESTED | Code exists, no tests |
| Follower write protection | UNTESTED | Auth-only, no cluster-level guard |

---

## Reality Check

The current multi-node "verification" consists of:
1. **FakeCluster struct** — in-memory data structure with no real network, no real DBs, no real processes
2. **5 smoke tests** — call FakeCluster methods and check fields

**Zero real multi-node tests exist.** No test has ever started two Aegis processes, registered a follower to a leader, synced state, detected drift, or reconciled.

This is not necessarily a problem — Aegis is designed as a single-node control plane per the original scope (no multi-region, no distributed consensus). The FakeCluster exists as a design scaffold for future multi-node capability.

---

## Verdict

| Claim | Reality |
|-------|---------|
| "Multi-node sync verified" | ❌ FAKE_ONLY — FakeCluster is a struct, not real nodes |
| "ACK quorum works" | ❌ UNTESTED — code exists but no test exercises it |
| "Drift detection works" | ❌ FAKE_ONLY — FakeCluster.CheckDrift() compares int fields |
| "Reconcile works" | ❌ UNTESTED — no test |
| "Follower can't write system state" | ❌ NOT_IMPLEMENTED — auth layer protects admin routes from service keys, but no cluster-level write guard exists |

**Recommendation:** The `docs/multinode-smoke-test.md` document should clearly state that all multi-node capabilities are FAKE_VERIFIED or UNTESTED, and real multi-node testing requires actual infrastructure (multiple VPS or containers).
