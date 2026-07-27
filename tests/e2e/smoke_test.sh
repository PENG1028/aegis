#!/bin/bash
# Aegis E2E Smoke Test — Master Script
# Runs all E2E tests and reports summary.
#
# Prerequisites:
#   1. ./aegis serve running on localhost:7380
#   2. Admin credentials: admin / admin (or set ADMIN_PASSWORD env var)
#
# Usage:
#   bash tests/e2e/smoke_test.sh                    # Run all tests
#   bash tests/e2e/smoke_test.sh tcp                # Run TCP test only
#   bash tests/e2e/smoke_test.sh tcp udp cred       # Run specific tests
#   TEST_SERVICE_CONTROL=1 bash tests/e2e/smoke_test.sh  # Include service stop/start
#
# Environment variables:
#   BASE_URL        — Aegis base URL (default http://127.0.0.1:7380)
#   TARGET_HOST     — Gateway Link target (default 127.0.0.1)
#   TARGET_PORT     — Gateway Link target port (default 80)
#   ADMIN_PASSWORD  — Admin password (default admin)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export BASE_URL="${BASE_URL:-http://127.0.0.1:7380}"
export TARGET_HOST="${TARGET_HOST:-127.0.0.1}"
export TARGET_PORT="${TARGET_PORT:-80}"

TOTAL=0; PASSED=0; FAILED=0

run_test() {
  local name="$1"
  local script="$SCRIPT_DIR/${name}_test.sh"

  if [ ! -f "$script" ]; then
    echo "  ⚠ $name: script not found ($script)"
    return
  fi

  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  TOTAL=$((TOTAL+1))

  if bash "$script" 2>&1; then
    PASSED=$((PASSED+1))
    echo "  ✅ $name PASSED"
  else
    FAILED=$((FAILED+1))
    echo "  ❌ $name FAILED (exit code $?)"
  fi
}

# ─── Pre-flight check ───
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Aegis E2E Smoke Test"
echo "  Target: $BASE_URL"
echo "  Time:   $(date '+%Y-%m-%d %H:%M:%S')"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check if server is reachable
echo "--- Pre-flight: checking server ---"
if curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 "$BASE_URL/api/system/status" 2>/dev/null | grep -q '200\|401\|403'; then
  echo "  ✓ Server reachable at $BASE_URL"
else
  echo "  ✗ Cannot reach $BASE_URL — is 'aegis serve' running?"
  exit 1
fi

# ─── Check required tools ───
MISSING_TOOLS=""
for tool in curl nc; do
  if ! command -v "$tool" &>/dev/null; then
    MISSING_TOOLS="$MISSING_TOOLS $tool"
  fi
done
if [ -n "$MISSING_TOOLS" ]; then
  echo "  ⚠ Missing tools:$MISSING_TOOLS — some tests will use fallbacks"
fi

# ─── Run tests ───
if [ $# -eq 0 ]; then
  # Run all tests
  run_test "tcp_exposure"
  run_test "udp_exposure"
  run_test "credential_flow"
  run_test "unix_socket"
  run_test "middleware_lifecycle"
  run_test "gateway_link_cross_node"
else
  for name in "$@"; do
    # Support both "tcp" and "tcp_exposure" naming
    case "$name" in
      tcp|tcp_exposure) run_test "tcp_exposure" ;;
      udp|udp_exposure) run_test "udp_exposure" ;;
      cred|credential*) run_test "credential_flow" ;;
      unix|unix_socket) run_test "unix_socket" ;;
      mw|middleware*)   run_test "middleware_lifecycle" ;;
      gwlink|gateway*)  run_test "gateway_link_cross_node" ;;
      *) echo "  ⚠ Unknown test: $name (valid: tcp, udp, credential, unix, middleware, gateway_link)" ;;
    esac
  done
fi

# ─── Summary ───
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  E2E Smoke Test Summary"
echo "  Total:  $TOTAL"
echo "  Passed: $PASSED"
echo "  Failed: $FAILED"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ $FAILED -gt 0 ]; then
  exit 1
fi
echo "✅ ALL TESTS PASSED"
exit 0
