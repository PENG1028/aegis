# Pilot Observation Report — v1.7Z-RC

## Observation Period

| Start | 2026-06-25 12:23 CST |
|-------|---------------------|
| **End** | TBD (3–7 days) |
| **Status** | ✅ IN PROGRESS |
| **Last Updated** | 2026-06-25 |

## Pilot Configuration

| Parameter | Value |
|-----------|-------|
| Domain | `pilot.aegis.local` |
| Target | `127.0.0.1:3000` (python3 http.server) |
| Scope | `pilot-test` |
| API Key | `tok_298d559f5032191c` |
| Provider | Caddy 2.6.2, HAProxy 2.8.16 |
| Aegis Binary | v1.7Z-RC (ELF 64-bit, ~13.9 MB) |

## Observation Log

### Day 1 (2026-06-25)

| # | Time | Event | Severity | Evidence | Fix? |
|---|------|-------|----------|----------|------|
| 1 | 12:23 | Aegis serve started | ✅ info | Server log: "Aegis API server starting" | — |
| 2 | 12:23 | Scope created | ✅ info | `space_831a01dda13fedec` | — |
| 3 | 12:23 | API key created | ✅ info | `tok_298d559f5032191c` | — |
| 4 | 12:23 | Domain bound | ✅ success | `op_06c15c4d4a95dd32` | — |
| 5 | 12:23 | Safe apply completed | ✅ success | "apply completed" 2 routes | — |
| 6 | 12:23 | Trace domain | ✅ complete | 8 steps with provider diagnostic | — |
| 7 | 12:23 | Provider diagnose | ✅ healthy | caddy+haproxy both healthy | — |
| 8 | 12:23 | Traffic verification | ✅ accessible | curl HTTP 200 via Caddy | — |
| 9 | 12:23 | Restart drill (stop) | ✅ data plane OK | Traffic continues during downtime | — |
| 10 | 12:23 | Restart drill (start) | ✅ state recovered | Login, trace, diagnose all pass | — |
| 11 | 12:23 | Duplicate check | ✅ no duplicates | routes=2, edge_rules=1 | — |

---

## Summary

| Observation | Count |
|-------------|:---:|
| Total events | 11 |
| ✅ Normal | 11 |
| ⚠️ Warning | 0 |
| ❌ Failure | 0 |

## To Be Monitored

- [ ] Day 2: Check for state drift (routes, edge rules, pending_apply)
- [ ] Day 3: Check SQLite size and health
- [ ] Day 4: Verify provider diagnose still healthy
- [ ] Day 5: Check operation/apply/audit log growth
- [ ] Day 7: Final review — stop Aegis, verify data plane, restart

## Health Commands

```bash
# Check pending_apply
curl -s http://127.0.0.1:9000/api/system/status

# Check provider health
curl -s -X POST http://127.0.0.1:9000/api/admin/v1/providers/diagnose

# Check for drift
curl -s http://127.0.0.1:9000/api/admin/v1/routes | python3 -c "import sys,json;print(len(json.load(sys.stdin).get('routes',[])))"
curl -s http://127.0.0.1:9000/api/admin/v1/edge-rules | python3 -c "import sys,json;print(len(json.load(sys.stdin).get('edge_rules',[])))"

# Check recent apply logs
curl -s http://127.0.0.1:9000/api/admin/v1/apply-logs | python3 -c "import sys,json;d=json.load(sys.stdin);[print(a.get('validate_status'),a.get('reload_status')) for a in d[:3]]"
```
