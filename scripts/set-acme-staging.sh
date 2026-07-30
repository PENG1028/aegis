#!/usr/bin/env bash
# Point acme_server at the Let's Encrypt staging directory, and prove it took.
#
# Why this is a script and not a one-line sed: the production default for
# acme_server is the empty string, which means real Let's Encrypt. A sed that
# silently matches nothing leaves you testing renewal against production, where
# the limit is 5 duplicate orders per domain set per week. The failure is silent
# and costs a week, so the value is read back through `aegis settings` and the
# script exits non-zero if it did not change.
#
# Usage:
#   bash scripts/set-acme-staging.sh <target_ip> [ssh_user]
#   bash scripts/set-acme-staging.sh <target_ip> [ssh_user] --production   # switch back

set -euo pipefail

TARGET="${1:?Usage: $0 <target_ip> [ssh_user] [--production]}"
SSH_USER="${2:-ubuntu}"
MODE="${3:-staging}"
SSH="ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new ${SSH_USER}@${TARGET}"
CFG=/etc/aegis/config.yaml
BIN=/usr/local/bin/aegis

if [ "${MODE}" = "--production" ]; then
  WANT=""
  LABEL="production Let's Encrypt"
else
  WANT="https://acme-staging-v02.api.letsencrypt.org/directory"
  LABEL="staging"
fi

echo "Target: ${TARGET}   ->   ${LABEL}"
echo ""
echo "Before:"
${SSH} "sudo ${BIN} settings 2>/dev/null | grep -i acme_server" || echo "  (acme_server not reported)"

${SSH} "sudo cp ${CFG} ${CFG}.bak-acme"
echo ""
echo "Backup: ${CFG}.bak-acme"

# Rewrite in Python rather than sed: the key sits under proxy: at whatever
# indentation the writer used, and it may be absent entirely in a config written
# before acme_server was added to the struct. Substitution alone would miss both.
${SSH} "sudo python3 - <<'PY'
import re, sys
path = '${CFG}'
want = '${WANT}'
src = open(path).read()
lines = src.split('\n')
out, in_proxy, done, indent = [], False, False, '    '
for line in lines:
    if re.match(r'^proxy:\s*\$', line):
        in_proxy, out = True, out + [line]
        continue
    if in_proxy and re.match(r'^\S', line):          # left the proxy block
        if not done:
            out.append(f'{indent}acme_server: \"{want}\"')
            done = True
        in_proxy = False
    if in_proxy:
        m = re.match(r'^(\s+)acme_server:', line)
        if m:
            out.append(f'{m.group(1)}acme_server: \"{want}\"')
            done = True
            continue
        m2 = re.match(r'^(\s+)\S', line)
        if m2:
            indent = m2.group(1)
    out.append(line)
if not done:
    print('ERROR: no proxy: block found', file=sys.stderr)
    sys.exit(1)
open(path, 'w').write('\n'.join(out))
print('rewrote acme_server')
PY"

${SSH} "sudo systemctl restart aegis"
sleep 3

echo ""
echo "After:"
GOT=$(${SSH} "sudo ${BIN} settings 2>/dev/null | grep -i acme_server | awk '{print \$2}'" || echo "")

if [ "${MODE}" = "--production" ]; then
  if [ -z "${GOT}" ]; then
    echo "  acme_server is empty -> production Let's Encrypt. Quota now counts for real."
  else
    echo "  FAIL: expected empty, got '${GOT}'"; exit 1
  fi
else
  case "${GOT}" in
    *acme-staging*) echo "  acme_server = ${GOT}"; echo ""; echo "OK. Staging is active; cert steps are safe to iterate." ;;
    *) echo "  FAIL: expected a staging URL, got '${GOT}'"
       echo "  Restore with: ${SSH} 'sudo cp ${CFG}.bak-acme ${CFG} && sudo systemctl restart aegis'"
       exit 1 ;;
  esac
fi

${SSH} "systemctl is-active aegis" >/dev/null 2>&1 \
  && echo "aegis is active." \
  || { echo "WARNING: aegis is not active after restart."; ${SSH} "sudo journalctl -u aegis -n 20 --no-pager"; exit 1; }
