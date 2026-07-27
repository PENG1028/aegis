# Feature Gap Register — v1.7AA

## Gap Table

| Feature | Current Status | Needed for Personal Use | Priority | Decision |
|---------|---------------|:---:|:---:|---------|
| Public web UI | ❌ Not implemented | Low | P3 | Deferred — CLI-only for now |
| Unified log language | ❌ Not implemented | Medium | P2 | Deferred — planned for v1.8 |
| Real two-node acceptance | ⏳ In progress (v1.7AA) | High | P0 | In progress |
| Real multi-node production | ❌ Unsupported | Low | P4 | Beyond scope |
| Automatic binary upgrade | ❌ Unsupported | Low | P4 | Manual replace + rollback docs exist |
| Deployment executor | ❌ Unsupported | Low | P4 | Beyond scope |
| Canary/staged rollout | ❌ Unsupported | Low | P4 | Beyond scope |
| New provider (non-Caddy/HAProxy) | ❌ Unsupported | Low | P4 | Add when needed |
| Cloudflare integration | ❌ Unsupported | Low | P4 | Not planned |
| Database proxy protocol | ❌ Unsupported | Low | P4 | Not planned |
| Backup retention policy | ❌ Not implemented | Medium | P3 | Manual backups for now |
| Metrics/alerting | ❌ Not implemented | Low | P4 | Deferred |
| API docs / OpenAPI spec | ❌ Not implemented | Low | P4 | Deferred |
| SDK/client library | ❌ Not implemented | Low | P4 | Deferred |
| Liveness check on target | ⏳ Partial — trace has TCP connect | High | P2 | Works for trace; no periodic health check |
| Runtime verify (HAProxy) | ❌ Always true | Medium | P3 | Needs real implementation |
| Listener conflict detection | ❌ Always true | Low | P4 | Needs root for port scan |
| Edge rule ownership persistence | ⚠️ In-memory only (v1.7V finding) | Medium | P2 | Not persisted to DB |
| Follower write protection | ❌ Not implemented | Low | P4 | Not needed for single-node |

## Personal Use Priority

| Priority | Meaning | Count |
|----------|---------|:---:|
| P0 | Must have — current blocker | 1 |
| P1 | Important — next in queue | 0 |
| P2 | Nice to have — medium term | 3 |
| P3 | Low urgency | 3 |
| P4 | Future / out of scope | 13 |

## Decision Rules

- `Deferred` — acknowledged, will do later
- `Beyond scope` — not planned for Aegis at all
- `Not planned` — explicitly decided against

## Legend

- ✅ implemented
- ⏳ in progress / partial
- ❌ not implemented / unsupported
