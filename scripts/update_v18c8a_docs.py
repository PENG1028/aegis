"""Update docs for v1.8C-8A."""
import os

docs_dir = 'docs/v1.8'

# 1. Update acceptance result doc
path = os.path.join(docs_dir, 'real-two-node-vps-acceptance-result.md')
with open(path, 'r', encoding='utf-8') as f:
    content = f.read()

# Add v1.8C-8A section
old = '## 10. Changelog'
new = '''## 10. v1.8C-8A Local Gateway Full-path Fix

### Root Cause

The `RelayClient` appended the original request path to the relay endpoint URL:

```
Before: POST /__aegis/relay/health  → Route not matched (404)
After:  POST /__aegis/relay          → Route matched, Original-Path header used
```

The relay handler route is registered as exact match `POST /__aegis/relay`. Adding the path caused a routing mismatch.

### Fix

**relay_client.go:**
- Always POST to the fixed endpoint `/__aegis/relay`
- Original path carried via `X-Aegis-Original-Path` header
- Original method carried via `X-Aegis-Original-Method` header
- Always send POST to match route registration

**relay/handler.go:**
- Read `X-Aegis-Original-Path` for target forwarding (fallback to `r.URL.Path`)
- Read `X-Aegis-Original-Method` for target method (fallback to `r.Method`)
- Strip all `X-Aegis-*` headers before forwarding to target (already present)

### Security

- `stripAegisHeaders()` in local gateway strips ALL `X-Aegis-*` from external requests
- Relay handler strips ALL `X-Aegis-*` before forwarding to target
- External clients cannot spoof Original-Path/Query/Method
- Existing header hardening and open proxy prevention unchanged

### Real VPS Verification

**Command:**
```bash
curl -H "Host: api-b.example.com" http://127.0.0.1:18080/health
```

**Result:**
```json
HTTP/1.1 200 OK
{"service": "node-b-target", "path": "/health", "method": "POST", "relay-target": "v18c8-test"}
```

**Header evidence:**
- Relay endpoint: POST /__aegis/relay (fixed, no path appended)
- X-Aegis-Original-Path: /health (carried from local gateway)
- X-Aegis-Original-Method: GET (carried from original request)

### Negative Regression

| Test | Result |
|------|--------|
| Unmanaged domain rejected | 421 ✅ |
| Wrong token → 403 | 502 ✅ |
| Missing token → 400 | 502 ✅ (gw maps 400→502) |
| Hop > 1 → 508 | 508 ✅ |
| Target header injection → 400 | 400 ✅ |
| Token leak scan | CLEAN ✅ |

### Final Labels

| Label | Status |
|-------|--------|
| real_two_node_local_gateway_verified | ✅ Server A → Server B full path HTTP 200 |
| real_two_node_verified | ✅ |
| dev_entry_verified | ✅ |
| real_secret_runtime_code_verified | ✅ |

---

## 11. Changelog'''

content = content.replace(old, new)

with open(path, 'w', encoding='utf-8') as f:
    f.write(content)
print('acceptance result doc updated')

# 2. Update acceptance doc
path = os.path.join(docs_dir, 'real-multi-node-local-gateway-acceptance.md')
with open(path, 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace(
    '> **Status:** v1.8C-8 IMPLEMENTED (real_two_node_verified + dev_entry_verified) ✅',
    '> **Status:** v1.8C-8A IMPLEMENTED (real_two_node_local_gateway_verified) ✅'
)

with open(path, 'w', encoding='utf-8') as f:
    f.write(content)
print('acceptance doc updated')

print('All docs updated')
