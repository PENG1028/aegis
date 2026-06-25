# Hot Update Drill Result — v1.7AA

## Type 1: Config Hot Reload (Remote Target Update)

### Test: update-target to change backend port

| Step | Action | Result |
|------|--------|--------|
| 1 | Start Server B python3 :3000 | ✅ HTTP 200 |
| 2 | Bind domain to 10.3.0.11:3000 | ✅ success |
| 3 | Safe apply | ✅ "apply completed" |
| 4 | Trace domain → 10.3.0.11:3000 | ✅ final_target set |
| 5 | Target reachability | ❌ TARGET_TIMEOUT (cross-VPC blocked) |
| 6 | update-target to different port | ⏷ Blocked by TC5 |

**Note:** Config hot reload for local targets was verified in v1.7Y/1.7Z.
Remote target hot update requires cross-VPC connectivity.

### Local Hot Reload (from v1.7Y)
```
update-target → safe apply → Caddy reload → curl new path → ✅ verified
```

## Type 2: Data Plane Continuity (Restart)

| Step | Action | Result |
|------|--------|--------|
| 1 | Traffic before stop | ✅ HTTP 200 |
| 2 | Stop Aegis | ✅ Process killed |
| 3 | Traffic during downtime | ✅ HTTP 200 |
| 4 | Caddy/HAProxy processes | ✅ Both survive |
| 5 | Restart Aegis | ✅ Server starts |
| 6 | Login after restart | ✅ Success |
| 7 | Trace after restart | ✅ 8 steps, complete |
| 8 | Provider diagnose | ✅ healthy=True |
| 9 | No duplicate resources | ✅ 3 routes, 3 edge rules |

## Type 3: Binary Upgrade

See `docs/rollback-runbook.md`. Not executed in this drill.

## Summary

| Update Type | Status | Notes |
|-------------|--------|-------|
| Config hot reload (local) | ✅ single_node_real_verified | From v1.7Y/v1.7Z |
| Config hot reload (remote) | ⏷ Blocked | Needs cross-VPC network |
| Data plane continuity | ✅ single_node_real_verified | Restart drill 10/10 |
| Binary upgrade | ⚠️ Manual only | Rollback runbook exists |
