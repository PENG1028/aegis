# Update Boundary — v1.7AA

## Four Types of "Hot Update"

### Type 1: Provider Config Hot Reload ✅ single_node_real_verified

| Step | Status | Notes |
|------|--------|-------|
| Route/domain change in DB | ✅ | Action API or admin CRUD |
| Render new config | ✅ | Provider adapter renders Caddyfile/HAProxy config |
| Validate config | ✅ | `caddy validate` / `haproxy -c` |
| Atomic replace | ✅ | Write temp → validate → backup → replace |
| Provider reload | ✅ | `systemctl reload caddy` / `systemctl reload haproxy` |
| Runtime verify | ✅ Caddy | curl check against :80 |
| Runtime verify | ❌ HAProxy | Hardcoded true |
| Config unchanged skip | ✅ | Hash comparison skips identical configs |
| Reload failure → restore | ✅ | Restore backup, reload restored config |
| **Trace reflects change** | ✅ pending | update-target + safe apply → trace shows new target |

### Type 2: Data Plane Continuity ✅ single_node_real_verified

| Condition | Result | Evidence |
|-----------|--------|----------|
| Aegis process stopped | ✅ Traffic continues | Caddy/HAProxy independent |
| Aegis process killed -9 | ✅ Traffic continues | Data plane not affected |
| Aegis crash | ✅ Traffic continues | Same as stop |
| Aegis restarted | ✅ State recovered | v1.7Z restart drill 10/10 |

### Type 3: Aegis Process Restart ✅ single_node_real_verified

| Check | Result |
|-------|--------|
| Node re-registers | ✅ |
| Leader re-elected | ✅ (single node auto-elected) |
| State version preserved | ✅ |
| Pending apply state preserved | ✅ |
| Routes/edge rules preserved | ✅ |
| Trace works after restart | ✅ |
| Provider diagnose works | ✅ |
| No duplicate resources | ✅ |

### Type 4: Aegis Binary Upgrade ⚠️ Manual Only

| Capability | Status |
|-----------|--------|
| Manual binary replace | ✅ Documented in rollback-runbook.md |
| Binary backup before upgrade | ✅ Documented |
| SQLite snapshot before upgrade | ✅ Documented |
| Provider config backup | ✅ Documented |
| Rollback via old binary | ✅ Documented |
| Automatic binary download | ❌ unsupported |
| Canary/staged rollout | ❌ unsupported |
| Version compatibility check | ❌ Not implemented |
| Upgrade session tracking | ⏳ UpgradeSession model exists but untested |

## Key Distinction

> **Config hot reload** (Type 1) is verified and safe.
> **Binary hot upgrade** (Type 4) is manual-only, no automation.
> Do not confuse the two. Aegis can hot-reload provider configs safely.
> Aegis CANNOT hot-upgrade its own binary without manual steps.
