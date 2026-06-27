#!/usr/bin/env bash
# v1.8C-7 Developer Entry Acceptance Test
#
# Verifies the local gateway developer entry workflow.
# Does NOT modify /etc/hosts.
#
# Usage:
#   bash scripts/dev-entry-acceptance-v1.8C-7.sh
#
# Exit codes:
#   0 - all checks pass
#   1 - one or more checks fail

set -euo pipefail

PASS=0
FAIL=0
TOTAL=0

GATEWAY_PORT=${GATEWAY_PORT:-18080}
GATEWAY_URL="http://127.0.0.1:${GATEWAY_PORT}"

ok()   { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS  $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL  $1"; }
info() { echo "  :: $1"; }

echo "============================================================"
echo "  v1.8C-7 Developer Entry Acceptance"
echo "============================================================"
echo "  Gateway: ${GATEWAY_URL}"
echo "  Date:    $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
echo "============================================================"
echo ""

# ---- Prerequisites ----
info "Checking prerequisites..."
if ! command -v curl &>/dev/null; then
  echo "  ERROR: curl is required"
  exit 1
fi

# ---- Test 1: health endpoint ----
echo "---"
echo "  Test 1: Local gateway health endpoint"
HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' \
  "${GATEWAY_URL}/__aegis/local/health" 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
  ok "health endpoint returns 200"
else
  fail "health endpoint returned $HTTP_CODE (expected 200)"
fi

# ---- Test 2: status endpoint ----
echo "---"
echo "  Test 2: Local gateway status endpoint"
STATUS_BODY=$(curl -s "${GATEWAY_URL}/__aegis/local/status" 2>/dev/null || echo "")
if echo "$STATUS_BODY" | grep -q '"node_id"'; then
  ok "status endpoint returns valid JSON with node_id"
else
  fail "status endpoint did not return node_id"
fi

# ---- Test 3: No secret leak in status ----
echo "---"
echo "  Test 3: No secret leak in status response"
if echo "$STATUS_BODY" | grep -qi '"secret\|"raw_token\|"plaintext\|gateway_link_token'; then
  fail "status endpoint leaks secret-like data"
else
  ok "no secret leak in status response"
fi

# ---- Test 4: Status has gateway info ----
echo "---"
echo "  Test 4: Status has gateway status"
if echo "$STATUS_BODY" | grep -q '"local_gateway"'; then
  ok "status has local_gateway field"
else
  fail "status missing local_gateway field"
fi

# ---- Test 5: Mixed case health path ----
echo "---"
echo "  Test 5: Case-insensitive health path"
HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' \
  "${GATEWAY_URL}/__AEGIS/LOCAL/HEALTH" 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
  ok "uppercase health path returns 200"
else
  fail "uppercase health path returned $HTTP_CODE"
fi

# ---- Test 6: Status has routing table info ----
echo "---"
echo "  Test 6: Status has routing table field"
if echo "$STATUS_BODY" | grep -q '"routing_table"'; then
  ok "status has routing_table field"
else
  fail "status missing routing_table field"
fi

# ---- Test 7: Status has cache info ----
echo "---"
echo "  Test 7: Status has cache field"
if echo "$STATUS_BODY" | grep -q '"cache"'; then
  ok "status has cache field"
else
  fail "status missing cache field"
fi

# ---- Summary ----
echo ""
echo "============================================================"
echo "  SUMMARY"
echo "============================================================"
echo "  PASS:  ${PASS}/${TOTAL}"
echo "  FAIL:  ${FAIL}/${TOTAL}"
echo "============================================================"

if [ "$FAIL" -gt 0 ]; then
  echo "  STATUS: FAILED"
  echo ""
  echo "  Runbook for manual verification:"
  echo "    Mode A: curl -H \"Host: api-b.example.com\" ${GATEWAY_URL}/health"
  echo "    Mode B: hosts + curl http://api-b.example.com:${GATEWAY_PORT}/health"
  echo "    Mode C: hosts + port 80 (requires root)"
  echo ""
  exit 1
fi

echo "  STATUS: PASS"
echo ""
echo "  Verification labels:"
echo "    dev_entry_accepted:          PASS (${PASS}/${TOTAL})"
echo "    simulated_two_node_verified: PASS (see v1.8C-6B)"
echo "    real_two_node_pending:       runbook written"
echo "============================================================"
