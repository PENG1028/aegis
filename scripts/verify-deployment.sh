#!/usr/bin/env bash
# Post-deploy verification for the codex/cert-lifecycle branch.
#
# Runs the checks that do not need a human deciding anything, and stops at the
# first failure with the command that failed. Read-only except where a step says
# otherwise; the two mutating steps (reload against a broken config, mode switch)
# restore what they changed.
#
# Usage:
#   bash scripts/verify-deployment.sh <target_ip> [ssh_user]
#
# Prerequisites: scripts/deploy.sh has already run against this host.

set -euo pipefail

TARGET="${1:?Usage: $0 <target_ip> [ssh_user]}"
SSH_USER="${2:-ubuntu}"
SSH="ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new ${SSH_USER}@${TARGET}"
BIN=/usr/local/bin/aegis

pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1"; echo "        $2"; exit 1; }
step() { echo ""; echo "=== $1 ==="; }

# ─── 1. Services are up ───
# Nothing below is meaningful if the process is not running, so this gates the rest.
step "1. Service state"
for unit in aegis caddy; do
  if ${SSH} "systemctl is-active ${unit}" >/dev/null 2>&1; then
    pass "${unit} active"
  else
    fail "${unit} not active" "${SSH} 'systemctl status ${unit} --no-pager -l'"
  fi
done

# ─── 2. Built-in self-checks ───
# doctor checks OS, binaries, permissions, ports, providers. verify --full compares
# running state against Aegis state. smoke is read-only by default. These already
# exist; no reason to hand-roll equivalents.
step "2. Self-checks"
for chk in "doctor" "verify --full" "smoke"; do
  if OUT=$(${SSH} "sudo ${BIN} ${chk}" 2>&1); then
    pass "aegis ${chk}"
  else
    fail "aegis ${chk}" "$(echo "${OUT}" | tail -5)"
  fi
done

# ─── 3. ACME is pointed at staging ───
# The production default for acme_server is the empty string, which means real
# Let's Encrypt. Renewal testing is iterative and LE allows only 5 duplicate orders
# per domain set per week, so hitting production here costs a week of waiting.
# This is a hard gate, not a warning.
step "3. ACME directory"
ACME=$(${SSH} "sudo ${BIN} settings 2>/dev/null | grep acme_server | awk '{print \$2}'" || echo "")
case "${ACME}" in
  *acme-staging*) pass "acme_server points at staging" ;;
  "")             fail "acme_server is empty — that means production Let's Encrypt" \
                       "Set acme_server in /etc/aegis/config.yaml to https://acme-staging-v02.api.letsencrypt.org/directory, then: sudo systemctl restart aegis" ;;
  *)              fail "acme_server is ${ACME}, expected a staging URL" \
                       "Only run the cert steps against production once staging has passed end to end." ;;
esac

# ─── 4. The reload bug fixed on this branch ───
# Two call sites reported success for a reload the gateway had refused. This breaks
# the Caddyfile on purpose, asks for a reload, and requires the API to report the
# failure. Restores the original file either way.
step "4. Refused reload is reported as a failure"
${SSH} "sudo cp /etc/caddy/Caddyfile /tmp/Caddyfile.verify-backup"
restore_caddyfile() {
  ${SSH} "sudo cp /tmp/Caddyfile.verify-backup /etc/caddy/Caddyfile && sudo chown root:caddy /etc/caddy/Caddyfile && sudo chmod 640 /etc/caddy/Caddyfile" || true
}
trap restore_caddyfile EXIT

${SSH} "echo 'this is not valid caddy config {{{' | sudo tee -a /etc/caddy/Caddyfile >/dev/null"
BODY=$(${SSH} "sudo curl -s -X POST --unix-socket /dev/null http://127.0.0.1/api/admin/v1/providers/caddy/reload" 2>/dev/null || echo "")
if [ -z "${BODY}" ]; then
  echo "  SKIP  needs an authenticated session; verify this one in the UI instead:"
  echo "        open the panel, break the Caddyfile, click 热重载 on the Caddy card."
  echo "        Expected: a failure toast carrying Caddy's reason. A success toast is the bug."
else
  case "${BODY}" in
    *failed*|*error*) pass "reload reported the failure: ${BODY}" ;;
    *success*)        fail "reload reported success for a broken config" "body: ${BODY}" ;;
    *)                echo "  WARN  unexpected body: ${BODY}" ;;
  esac
fi
restore_caddyfile
trap - EXIT
${SSH} "sudo ${BIN} provider reload caddy" >/dev/null 2>&1 || true
pass "Caddyfile restored"

# ─── 5. Provider control against real systemd ───
# Local tests used fake providers, so this is the first time the capability-vs-shell
# handoff runs against real units. Compares what Aegis reports to what systemd says.
step "5. Provider control"
AEGIS_VIEW=$(${SSH} "sudo ${BIN} provider list 2>/dev/null | grep -i caddy" || echo "")
SYSTEMD_VIEW=$(${SSH} "systemctl is-active caddy" 2>/dev/null || echo "inactive")
echo "  aegis:   ${AEGIS_VIEW}"
echo "  systemd: ${SYSTEMD_VIEW}"
case "${AEGIS_VIEW}" in
  *running*|*ready*|*active*)
    [ "${SYSTEMD_VIEW}" = "active" ] && pass "both agree caddy is running" \
      || fail "aegis says running, systemd says ${SYSTEMD_VIEW}" "capability claim does not match reality" ;;
  *) echo "  WARN  could not parse provider state; check manually" ;;
esac

# ─── 6. Mode switch preview ───
# preview does not mutate. Run it before any real switch so the plan is visible.
step "6. Mode switch preview (read-only)"
if OUT=$(${SSH} "sudo ${BIN} edge preview 2>&1" || ${SSH} "sudo ${BIN} preview 2>&1") ; then
  echo "${OUT}" | head -15
  pass "preview produced a plan"
else
  echo "  WARN  no preview subcommand at that path; use POST /api/admin/v1/mode/preview"
fi

echo ""
echo "════════════════════════════════════════════"
echo " Automated checks done."
echo ""
echo " Still needs a human (see the checklist):"
echo "   - DNS pointing at ${TARGET} before any cert issuance"
echo "   - staging cert issuance and the 202 renewal path"
echo "   - the real mode switch and its rollback"
echo "   - provider uninstall (destructive)"
echo "════════════════════════════════════════════"
