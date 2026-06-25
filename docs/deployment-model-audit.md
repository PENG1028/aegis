# Deployment Model Freeze Audit — v1.7R

## Executive Summary

The deployment model is **tracking-only as designed**. The `DeploymentService` does not execute binary upgrades, does not touch provider configs, does not invoke the apply pipeline, and does not interact with upgrade sessions or state versions. Rollback is purely a status marker with no config impact. **No freeze action required** — the model already respects the v1.7 boundary.

---

## 1. Question-by-Question Audit

### Q1: Does deployment actually modify nodes?

**No.** `CreateDeployment()`:
1. Generates a version label
2. Creates a `Deployment` row in SQLite
3. Creates `DeploymentInstance` rows (one per target node) — status = "pending"

No SSH, no remote exec, no file transfer, no binary execution, no provider interaction.

```go
// deployment/service.go:23-56
func (s *Service) CreateDeployment(...) (*Deployment, error) {
    d := &Deployment{...}  // pure data
    s.repo.Create(d)       // SQL INSERT only
    for _, nodeID := range targetNodes {
        inst := &DeploymentInstance{...}  // pure data
        s.instRepo.Create(inst)           // SQL INSERT only
    }
    return d, nil
}
```

**Verdict: ✅ SAFE — tracking only, zero node interaction**

### Q2: Is rollback just a status marker?

**Yes.** `RollbackDeployment()`:
1. Finds the deployment by ID
2. Sets `d.Status = StatusRolledBack`
3. Calls `s.repo.Update(d)` — SQL UPDATE only
4. Sets each instance status to `StatusRolledBack`
5. Calls `s.instRepo.Update(&inst)` — SQL UPDATE only

No config restore, no backup restoration, no provider reload, no apply pipeline interaction.

```go
// deployment/service.go:80-97
func (s *Service) RollbackDeployment(ctx context.Context, id string) error {
    d, err := s.repo.FindByID(id)
    d.Status = StatusRolledBack
    s.repo.Update(d)
    for _, inst := range instances {
        inst.Status = StatusRolledBack
        s.instRepo.Update(&inst)
    }
    return nil
}
```

**Verdict: ✅ SAFE — status marker only, no config impact**

### Q3: Does it affect provider config?

**No.** The deployment service:
- Has zero imports of `provider`, `proxy`, `apply`, `caddy`, or `haproxy`
- Has no knowledge of config files, renderers, or reload commands
- Only interacts with its own `deployments` and `deployment_instances` tables

**Verdict: ✅ SAFE — no provider config interaction**

### Q4: Does it bypass upgrade sessions?

**No.** The deployment model has no relationship with the upgrade session system:
- No reference to `upgrade_sessions` table
- No reference to `UpgradeSession` model
- No `upgrade` package import
- Deployment and UpgradeSession are completely orthogonal concepts in this codebase

**Verdict: ✅ SAFE — no upgrade session interaction**

### Q5: Does it bypass state_version?

**No.** The deployment model:
- Does not read `state_version` from nodes
- Does not write `state_version` to nodes
- Does not import the `cluster` package
- Has no awareness of state versioning

**Verdict: ✅ SAFE — no state_version interaction**

---

## 2. What DeploymentService Actually Does

```
CreateDeployment(version, serviceID, targetNodes, strategy)
  → INSERT INTO deployments
  → INSERT INTO deployment_instances (one per node)

GetDeployment(id)
  → SELECT FROM deployments WHERE id = ?
  → SELECT FROM deployment_instances WHERE deployment_id = ?

ListDeployments()
  → SELECT FROM deployments ORDER BY created_at DESC

RollbackDeployment(id)
  → UPDATE deployments SET status = 'rolled_back' WHERE id = ?
  → UPDATE deployment_instances SET status = 'rolled_back' WHERE deployment_id = ?

GetInstanceStatus(deploymentID)
  → SELECT FROM deployment_instances WHERE deployment_id = ?
```

**All operations are pure SQL CRUD on two tables.** No side effects.

---

## 3. Deployment Rollout Strategies — Not Implemented

The model defines three strategies but only the names exist:
- `StrategyAll` = "all" — default, but no "deploy to all" logic
- `StrategyCanary` = "canary" — defined as constant, zero execution logic
- `StrategyStaged` = "staged" — defined as constant, zero execution logic

This is the correct v1.7 boundary — strategies are labels only. No scheduler or executor exists.

---

## 4. Deployment Instance State Machine

Current states (status constants):
```
pending → running → success
                  → failed
pending → running → rolled_back
```

No state transition enforcement exists — status is set directly. No validation prevents:
- Marking a pending deployment as success without running
- Marking a rolled_back deployment as success
- Concurrent status updates

**This is acceptable for a tracking model.** State machine enforcement can be added later if deployment execution is implemented.

---

## 5. Gap Analysis

### What it does well
- ✅ Pure tracking
- ✅ No node interaction
- ✅ No config mutation
- ✅ No provider dependency
- ✅ Separate from upgrade sessions

### What it's missing (acceptable gaps for tracking-only model)
- No operation logging (deployment create/rollback doesn't write to logs)
- No audit logging
- No event emission
- No state transition validation
- No `DiffVersions` real implementation (returns a format string, not actual diff)

### The DiffVersions stub
```go
func DiffVersions(from, to string) string {
    return fmt.Sprintf("diff from %s to %s", from, to)
}
```
This is a placeholder — it doesn't compute actual version differences. Acceptable for v1.7 tracking model.

---

## 6. Fix Plan

### Immediate
1. **Add operation logging** to `CreateDeployment` and `RollbackDeployment`
2. **Add audit logging** for deployment creation and rollback
3. No code changes needed to freeze — model already respects boundary

### Explicitly Prohibited (for all 1.x)
- ❌ Binary upgrade execution
- ❌ Canary executor
- ❌ Staged rollout executor
- ❌ Deployment scheduler
- ❌ Remote deploy agent
- ❌ Config mutation from deployment
- ❌ Provider interaction from deployment
- ❌ State version mutation from deployment

---

## 7. Conclusion

**The deployment model is correctly scoped as a tracking-only abstraction.** It does not execute, does not mutate config, and does not interact with any existing system (apply, upgrade, state_version, provider). No freeze action is needed — the code already respects the v1.7 boundary.

The only needed change is adding operation/audit logging to make deployment actions traceable through the log system.
