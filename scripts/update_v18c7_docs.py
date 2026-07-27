"""Update docs for v1.8C-7 references."""
import os

docs_dir = 'docs/v1.8'

# 1. Update real-two-node-vps-acceptance-runbook.md - add v1.8C-7 reference
runbook_path = os.path.join(docs_dir, 'real-two-node-vps-acceptance-runbook.md')
with open(runbook_path, 'r', encoding='utf-8') as f:
    content = f.read()

# Update status line
if 'v1.8C-6B' in content:
    content = content.replace(
        '> **Status:** v1.8C-6B | planned (not executed)',
        '> **Status:** v1.8C-7 | planned (not executed)'
    )

# Add v1.8C-7 reference in prerequisites
old_prereq = '**Prerequisites:**'
new_prereq = '''**Prerequisites:**

- Developer entry workflow verified (see developer-entry-local-gateway.md)
- Local gateway health/status endpoints available
- Startup diagnostics pass
- Node configuration in standard node.yaml format'''
content = content.replace(old_prereq, new_prereq)

with open(runbook_path, 'w', encoding='utf-8') as f:
    f.write(content)
print('runbook.md updated')

# 2. Update acceptance doc - add v1.8C-7 section
accept_path = os.path.join(docs_dir, 'real-multi-node-local-gateway-acceptance.md')
with open(accept_path, 'r', encoding='utf-8') as f:
    content = f.read()

# Update status line
content = content.replace(
    '> **Status:** v1.8C-6B IMPLEMENTED (simulated_two_node_verified + real_secret_runtime_code_verified) ✅',
    '> **Status:** v1.8C-7 IMPLEMENTED (dev_entry_verified + real_secret_runtime_code_verified) ✅'
)

# Add v1.8C-7 verification label reference
old_labels = '''| real_two_node_pending | VPS runbook written, not executed |
| real_three_node_pending | Not attempted |'''

new_labels = '''| dev_entry_verified | Developer entry + daemon runbook: 14 tests PASS |
| real_two_node_pending | VPS runbook written, not executed |
| real_three_node_pending | Not attempted |'''

content = content.replace(old_labels, new_labels)

# Add v1.8C-7 to the test results table
old_tests = '''| internal/localgateway | 31 | ✅ PASS |'''
new_tests = '''| internal/localgateway | 45 (31 + 14 dev entry) | ✅ PASS |'''
content = content.replace(old_tests, new_tests)

with open(accept_path, 'w', encoding='utf-8') as f:
    f.write(content)
print('acceptance.md updated')

# 3. Update data gap doc
gap_path = os.path.join(docs_dir, 'multi-node-runtime-data-gap.md')
with open(gap_path, 'r', encoding='utf-8') as f:
    content = f.read()

# Add v1.8C-7 marker
old_marker = 'v1.8C-6B Gaps Filled:        simulated acceptance full pass 12/12 + 1 deferred, real secret runtime integration tests (6), VPS runbook, docs update ✅'
new_marker = old_marker + '\nv1.8C-7 Gaps Filled:          developer entry + daemon runbook — health/status endpoints, startup diagnostics, node.yaml config, systemd blueprints, dev acceptance script, 14 new tests ✅'
content = content.replace(old_marker, new_marker)

with open(gap_path, 'w', encoding='utf-8') as f:
    f.write(content)
print('data-gap.md updated')
print('All docs updated')
