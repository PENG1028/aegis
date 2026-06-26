# v1.8 Implementation Plan

## Phases

### v1.8A: Egress Trace & Path Diagnosis

**Scope**: Detection only. No enforcement, no interception.

| # | Item | Effort | Dependencies |
|---|------|--------|-------------|
| 1 | Egress trace API + CLI | Medium | RouteRepo, ManagedDomainRepo, NodeRepo |
| 2 | IP classification utility | Small | net package |
| 3 | Self-loop detection | Small | Node IPs from registration |
| 4 | Public bounce detection | Small | Route endpoint + IP check |
| 5 | Gateway Link bypass detection | Small | Route.gateway_link_id check |
| 6 | Internal target suggestion | Medium | Needs IP mapping data |
| 7 | Route safety API endpoints | Medium | Route model + safety check |
| 8 | Planner egress warning | Small | Additional Planner warning |
| 9 | Tests | Medium | Model + API + detection tests |

**Total effort**: ~2-3 weeks

### v1.8B: Route Path Safety

**Scope**: Strengthen route model with safety fields. Hardening, not new features.

| # | Item | Effort |
|---|------|--------|
| 1 | GatewayLink secret hashing (HMAC instead of plaintext) | Medium |
| 2 | Route attach/detach gateway_link admin endpoints | Small |
| 3 | Route validation: warn if public IP without GatewayLink | Small |
| 4 | Route safety policy fields (model only) | Small |
| 5 | Docs: v1.8B safety model | Small |

### v1.8C: Observability

**Scope**: Make issues findable. No log system rewrite.

| # | Item | Effort |
|---|------|--------|
| 1 | JSON log format option | Medium |
| 2 | Log level config | Small |
| 3 | Periodic provider health with events | Medium |
| 4 | Target connectivity drift monitoring | Medium |

### v1.8D: Operational Reliability

**Scope**: Safe operation.

| # | Item | Effort |
|---|------|--------|
| 1 | Startup self-check | Medium |
| 2 | Pre-apply safety check | Medium |
| 3 | Apply rollback retry | Medium |
| 4 | Graceful shutdown | Small |
| 5 | SQLite maintenance (WAL checkpoint) | Small |

## Deferred to v2

| Item | Reason |
|------|--------|
| Transparent egress interception | Requires agent/sidecar |
| iptables/nftables integration | Out of scope |
| eBPF | Out of scope |
| DNS hijack | Out of scope |
| Service mesh | Out of scope |
| Multi-node control plane | real_env_required |
| HMAC Gateway Link | Caddy module dependency |

## Decision Log

| Date | Decision | Rationale |
|:----:|----------|-----------|
| 2026-06-26 | v1.8A detection only, no enforcement | Avoid complex network changes in first phase |
| 2026-06-26 | IP classification uses static checks, not DNS | Route targets are known, no DNS overhead |
| 2026-06-26 | Self-loop detection compares against node IPs | Node IPs registered at bootstrap |
| 2026-06-26 | Gateway Link bypass is warning, not error | Admin discretion, not enforcement |
