# Two-Node Acceptance Result — v1.7AA

## Topology

```
Dev Machine (Windows)
  ├── SSH → Server A (43.160.211.232) — Aegis leader + Caddy + HAProxy
  └── SSH → Server B (43.159.34.11) — Remote target (python3 :3000)
```

| Property | Server A | Server B |
|----------|----------|----------|
| **IP** | 43.160.211.232 | 43.159.34.11 |
| **Private IP** | 10.3.0.4 | 10.3.0.11 |
| **OS** | Ubuntu 24.04 | Ubuntu 24.04 |
| **Role** | Aegis leader, Caddy :80, HAProxy :443 | Remote target |
| **Caddy** | 2.6.2 | ❌ Not installed |
| **HAProxy** | 2.8.16 | ❌ Not installed |
| **Target** | — | python3 http.server :3000 |

## Results

### ✅ Completed Tests

| # | Test | Result | Evidence |
|---|------|--------|----------|
| 1 | Server B target reachable locally | ✅ | `curl http://127.0.0.1:3000/` → HTTP 200 |
| 2 | Server A bind-http-domain → Server B | ✅ | `status: success`, target=10.3.0.11:3000 |
| 3 | Safe apply succeeds | ✅ | "apply completed" |
| 4 | Route created for remote target | ✅ | route two-node.aegis.local → active |
| 5 | Edge rule created | ✅ | SNI two-node.aegis.local → 127.0.0.1:8443 |
| 6 | Trace shows remote target in path | ✅ | final_target: 10.3.0.11:3000 |
| 7 | Target unreachable detected | ✅ | TARGET_TIMEOUT with connect_error |
| 8 | Provider diagnostics healthy | ✅ | Caddy + HAProxy both healthy=True |

### ❌ Blocked by Cloud Security Group

| # | Test | Result | Reason |
|---|------|--------|--------|
| 6 | curl domain → remote target | ❌ TIMEOUT | Cross-VPC traffic blocked by cloud firewall |
| 7 | Target stopped → trace detection | ⏳ Inferred | Cannot test remote; TARGET_TIMEOUT confirmed |
| 8 | Target restored → trace recovery | ⏳ Inferred | Same network issue |
| 9 | update-target to new port | ⏳ Inferred | Action API tested locally in v1.7Y |

### Network Diagnosis

```bash
# From Server A → Server B private IP:3000 → TIMEOUT (exit 124)
# From Server A → Server B public IP:3000 → TIMEOUT (exit 124)
# Root cause: Tencent Cloud security group blocks cross-VPC traffic
# Server B has iptables YJ-FIREWALL-INPUT chain (cloud agent)
# SSH from dev machine to both servers works (separate connections)
# Server A does NOT have SSH key for Server B (can't create tunnel)
```

## Verdict

| Criterion | Status |
|-----------|--------|
| Aegis accepts remote target_host | ✅ Verified |
| Route/edge rule created for remote target | ✅ Verified |
| Safe apply succeeds with remote target | ✅ Verified |
| Trace detects remote target unreachable | ✅ Verified |
| Trace returns structured error code | ✅ Verified |
| Actual traffic reaches remote target | ❌ Blocked by cloud network |
| Remote target failure → recovery cycle | ⏷ Not testable without network fix |

## Required Fix

To complete full two-node acceptance, one of:
1. Place both servers in same VPC / subnet
2. Open cloud security group for cross-VPC traffic on target port
3. Set up VPN tunnel (WireGuard, etc.) between the two servers
4. Configure SSH tunnel from Server A to Server B with key-based auth
