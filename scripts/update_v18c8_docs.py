"""Update docs for v1.8C-8 references."""
import os

docs_dir = 'docs/v1.8'

# 1. Update acceptance doc
path = os.path.join(docs_dir, 'real-multi-node-local-gateway-acceptance.md')
with open(path, 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace(
    '> **Status:** v1.8C-7 IMPLEMENTED (dev_entry_verified + real_secret_runtime_code_verified) ✅',
    '> **Status:** v1.8C-8 IMPLEMENTED (real_two_node_verified + dev_entry_verified) ✅'
)

# Update labels
old_labels = '''| dev_entry_verified | Developer entry + daemon runbook: 14 tests PASS |
| real_two_node_pending | VPS runbook written, not executed |
| real_three_node_pending | Not attempted |'''

new_labels = '''| real_two_node_verified | Real two-node VPS relay: Server A -> Server B -> target HTTP 200 |
| dev_entry_verified | Developer entry + daemon runbook: 14 tests PASS |
| real_secret_runtime_code_verified | Integration tests with real decryption chain |
| real_secret_runtime_deploy_verified | Pending: need encrypted GatewayLink through CP API |
| real_three_node_pending | Not attempted |'''

content = content.replace(old_labels, new_labels)

with open(path, 'w', encoding='utf-8') as f:
    f.write(content)
print('acceptance doc updated')

# 2. Update data gap doc
path = os.path.join(docs_dir, 'multi-node-runtime-data-gap.md')
with open(path, 'r', encoding='utf-8') as f:
    content = f.read()

old = 'v1.8C-7 Gaps Filled:          developer entry + daemon runbook'
new = 'v1.8C-7 Gaps Filled:          developer entry + daemon runbook\nv1.8C-8 Gaps Filled:          real two-node VPS relay acceptance (cross-server HTTP 200) ✅'
content = content.replace(old, new)

with open(path, 'w', encoding='utf-8') as f:
    f.write(content)
print('data-gap doc updated')
