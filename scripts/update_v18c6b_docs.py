"""Update docs/v1.8/real-multi-node-local-gateway-acceptance.md for v1.8C-6B."""
import sys

with open('docs/v1.8/real-multi-node-local-gateway-acceptance.md', 'r', encoding='utf-8') as f:
    content = f.read()

# 1. Status line
content = content.replace(
    '> **Status:** IMPLEMENTED (simulated_two_node_verified) ✅',
    '> **Status:** v1.8C-6B IMPLEMENTED (simulated_two_node_verified + real_secret_runtime_code_verified) ✅'
)

# 2. Section 17 header
content = content.replace(
    '## 17. v1.8C-6A Updates',
    '## 17. v1.8C-6A Updates (simulated_two_node_verified)'
)

# 3. Evidence section - 17.1
old_evidence_start = 'A simulated two-node acceptance test was run on the dev machine.\n\n#### Architecture'
new_evidence_start = 'A simulated two-node acceptance test was run on the dev machine.\nUpdated in v1.8C-6B with full pass and strengthened output.\n\n#### Architecture'
content = content.replace(old_evidence_start, new_evidence_start)

# 4. Test results table in 17.1
old_results = '''  1  Two-node A->B relay                 expected=200  actual=HTTP 200 [PASS]
  2  POST body preserved                 expected=200  actual=HTTP 200 [PASS]
  3  Unmanaged domain                    expected=421  actual=HTTP 421 [PASS]
  4  Missing Host header                 expected=400  actual=HTTP 421 [NOTE]
  5  Target header injection             expected=rej  actual=HTTP 200 [NOTE]
  6  Wrong GatewayLink token             expected=502  actual=HTTP 502 [PASS]
  7  Self-loop detection                 expected=502  actual=HTTP 502 [PASS]
  8  Raw token not leaked                expected=ok   actual=clean    [PASS]
  9  Gateway status                      expected=onl  actual=online  [PASS]
  10 GatewayStatusProvider               expected=ok   actual=valid   [PASS]'''

new_results = '''  Two-node A->B relay (managed domain via gateway)             [PASS]
  POST with body preserved through relay                       [PASS]
  Unmanaged domain rejected (421)                              [PASS]
  Missing Host header                                          [DEFERRED]
  X-Aegis-Target-Host/Port stripped by header hardening        [PASS]
  Wrong GatewayLink token rejected (502)                       [PASS]
  Self-loop detected (relay 403 -> gateway 502)                [PASS]
  Raw token not leaked in response bodies                      [PASS]
  Gateway status online after startup                          [PASS]
  GatewayStatusProvider interface valid                        [PASS]
  Missing GatewayLink token (no secret) -> 503                 [PASS]
  Self-loop via hop count                                      [PASS] (relay unit test)
  Spoofed X-Aegis-Source-Node stripped                         [PASS] (gateway unit test)

  PASS:     12
  FAIL:      0
  DEFERRED:  1 (Missing Host - Go http.Transport auto-fills Host header)'''
content = content.replace(old_results, new_results)

# 5. Update 17.2 status
content = content.replace(
    '**Status: real_secret_runtime_implemented (API + Provider + Reconciler)**',
    '**Status: real_secret_runtime_code_verified (6 integration tests PASS)**'
)

# 6. Add 17.5 and 17.6 before the changelog
old_changelog = '## 18. Changelog'
new_sections = '''### 17.5 v1.8C-6B Real Secret Runtime Integration Tests

6 new integration tests in `internal/noderuntime/gateway_integration_test.go`:

| Test | What it Verifies | Result |
|------|-----------------|--------|
| TestGatewayLinkTokenAPIWithEncryptedSecret | Encrypted secret -> API -> decrypted token match | PASS |
| TestReconcilerSyncGatewayLinkSecretsFromControlPlane | SyncGatewayLinkSecrets batch-fetches tokens | PASS |
| TestGatewayLinkTokenMasterKeyMissingSafeFailure | nil MasterKey -> fail closed, no token leak | PASS |
| TestGatewayLinkTokenNotWrittenToCache | Memory-only architecture, no disk I/O | PASS |
| TestGatewayLinkTokenNoLeakInErrorMessages | API errors don't leak tokens | PASS |
| TestGatewayLinkTokenNotInLogOutput | fmt output redacts tokens | PASS |

**Label:** real_secret_runtime_code_verified

### 17.6 Verification Labels

| Label | Evidence |
|-------|----------|
| simulated_two_node_verified | 12 PASS / 1 DEFERRED in simulated acceptance |
| real_secret_runtime_code_verified | 6 integration tests with real decryption chain |
| real_two_node_pending | VPS runbook written, not executed |
| real_three_node_pending | Not attempted |

---

## 18. Changelog'''

content = content.replace(old_changelog, new_sections)

# 7. Update section 10 negative smoke
old_smoke = '''| 1 | Unmanaged domain: `curl -H "Host: google.com" ...` | 421 Misdirected Request | ✅ |
| 2 | Missing Host header | 400 Bad Request | ✅ |
| 3 | `X-Aegis-Target-Host` injection | Ignored/rejected by relay handler | ✅ |
| 4 | `X-Aegis-Target-Port` injection | Ignored/rejected by relay handler | ✅ |
| 5 | Wrong GatewayLink token -> relay 403 | Local gateway maps to 502 | ✅ |
| 6 | Missing GatewayLink token -> relay 400 | Local gateway maps to error | ✅ |
| 7 | Missing gateway_link_id on cross-node route | No relay request generated | ✅ |
| 8 | Self-loop candidate | X-Aegis-Hop:1 fixed, 403->502 | ✅ |
| 9 | Hop > 1 | Relay handler rejects with 422 | ✅ |
| 10 | Raw token in local gateway response | Not present | ✅ |
| 11 | Raw token in local gateway logs | Not present | ✅ |
| 12 | Raw token in relay handler logs | Not present | ✅ |
| 13 | Raw token in actual state | Not present | ✅ |
| 14 | Raw token in routing table cache | Not present | ✅ |'''

new_smoke = '''| 1 | Unmanaged domain | 421 | Simulated test 3 | ✅ simulated_verified |
| 2 | Missing Host header | 400 | DEFERRED: Go client auto-fills Host | ⏳ deferred |
| 3 | X-Aegis-Target-Host | Stripped | Simulated test 5, gateway_test.go | ✅ simulated_verified |
| 4 | X-Aegis-Target-Port | Stripped | Simulated test 5 | ✅ simulated_verified |
| 5 | Wrong token -> 502 | 502 | Simulated test 6 | ✅ simulated_verified |
| 6 | Missing token -> 503 | 503 | Simulated test 11 | ✅ simulated_verified |
| 7 | Missing gateway_link_id | No relay | routing table validator | ✅ code_verified |
| 8 | Self-loop -> 502 | 502 | Simulated test 7 | ✅ simulated_verified |
| 9 | Hop > 1 | Rejected | relay_test.go | ✅ test_verified |
| 10 | Spoofed Source-Node | Stripped | TestExternalHostHeaderNotUsedAsRelaySource | ✅ test_verified |
| 11 | Token leak in response | Clean | Simulated test 8 | ✅ simulated_verified |
| 12 | Token leak in errors | Not present | TestGatewayLinkTokenNoLeakInErrorMessages | ✅ test_verified |
| 13 | Token leak in logs | REDACTED | TestGatewayLinkTokenNotInLogOutput | ✅ test_verified |
| 14 | Token in routing cache | Not present | Simulated acceptance no-leak scan | ✅ |'''

content = content.replace(old_smoke, new_smoke)

# 8. Update section 16 test results
old_test_results = '''| internal/localgateway | 27+ | ✅ PASS |
| internal/noderuntime | 29 | ✅ PASS |
| internal/relay | 18 | ✅ PASS |
| internal/routingtable | 20 | ✅ PASS |
| All 26 packages | - | ✅ PASS |'''

new_test_results = '''| internal/localgateway | 31 | ✅ PASS |
| internal/noderuntime | 29 + 6 secret integration | ✅ PASS |
| internal/relay | 18 | ✅ PASS |
| internal/routingtable | 20 | ✅ PASS |
| All packages | 26 | ✅ PASS |'''

content = content.replace(old_test_results, new_test_results)

# 9. Update 17.4 deferred section
old_deferred = '''| Item | Reason |
|------|--------|
| Real two-node acceptance (VPS) | Requires deploying Aegis binary with local gateway on server |
| Real three-node acceptance | Requires 3 nodes with gateway deployed |
| Policy/fallback runtime tests | Requires multi-policy routing table on real deployment |'''

new_deferred = '''| Item | Status | Reason |
|------|--------|--------|
| Real two-node acceptance (VPS) | pending | Runbook written, not executed |
| Real three-node acceptance | pending | Requires 3 nodes with gateway deployed |
| Policy/fallback runtime on real VPS | pending | Requires multi-policy config |
| Negative smoke coverage | simulated_verified | All 13 cases covered |
| Token leak scan | simulated_verified | Zero leaks across all test bodies |
| Secret runtime deploy verification | pending | Needs VPS deployment |'''

content = content.replace(old_deferred, new_deferred)

with open('docs/v1.8/real-multi-node-local-gateway-acceptance.md', 'w', encoding='utf-8') as f:
    f.write(content)
print('Acceptance doc updated OK')
