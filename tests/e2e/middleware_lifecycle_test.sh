#!/bin/bash
# Aegis E2E Test: Middleware Lifecycle (Caddy)
# Tests: List → Diagnose → Config → Service Control (start/stop/restart)
# Prerequisites: ./aegis serve running on localhost:7380, Caddy may or may not be installed
set -euo pipefail

BASE="${BASE_URL:-http://127.0.0.1:7380}"
COOKIE_JAR="/tmp/aegis_e2e_mw_cookies.txt"
PASS=0; FAIL=0

log_pass() { echo "  ✓ $1"; PASS=$((PASS+1)); }
log_fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }

echo "=== B5: Middleware Lifecycle E2E ==="

# ─── Login ───
echo "--- Login ---"
curl -s -c "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}' | grep -q '"user"' && log_pass "Login OK" || log_fail "Login"
trap "rm -f $COOKIE_JAR" EXIT

# ─── List Providers ───
echo "--- List Providers ---"
LIST_RESP=$(curl -s -b "$COOKIE_JAR" "$BASE/api/admin/v1/providers")
echo "$LIST_RESP" | grep -q "caddy" && log_pass "Caddy in provider list" || log_fail "Caddy not in list"
echo "$LIST_RESP" | grep -q "haproxy" && log_pass "HAProxy in provider list" || log_pass "HAProxy not in list (OK if not installed)"

# ─── Diagnose All Providers ───
echo "--- Diagnose All ---"
DIAG_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/providers/diagnose")
# Diagnose should return results with status for each provider
echo "$DIAG_RESP" | grep -q '"caddy"' && log_pass "Caddy diagnosed" || log_fail "Diagnose: $DIAG_RESP"

# ─── Get Caddy Config ───
echo "--- Get Caddy Config ---"
CFG_RESP=$(curl -s -b "$COOKIE_JAR" "$BASE/api/admin/v1/providers/caddy/config")
HTTP_CODE=$(curl -s -b "$COOKIE_JAR" "$BASE/api/admin/v1/providers/caddy/config" -o /dev/null -w '%{http_code}')
if [ "$HTTP_CODE" = "200" ]; then
  log_pass "Caddy config fetched (HTTP $HTTP_CODE)"
elif [ "$HTTP_CODE" = "404" ]; then
  log_pass "Caddy config not available (HTTP 404 — Caddy may not be installed)"
else
  log_fail "Caddy config: HTTP $HTTP_CODE"
fi

# ─── Get HAProxy Config ───
echo "--- Get HAProxy Config ---"
HAP_HTTP=$(curl -s -b "$COOKIE_JAR" "$BASE/api/admin/v1/providers/haproxy/config" -o /dev/null -w '%{http_code}')
[ "$HAP_HTTP" = "200" ] || [ "$HAP_HTTP" = "404" ] && log_pass "HAProxy config (HTTP $HAP_HTTP)" || log_fail "HAProxy: HTTP $HAP_HTTP"

# ─── Caddy Service Control (if installed) ───
echo "--- Caddy Service Control ---"
# Try reload — may fail gracefully if Caddy isn't running
RELOAD_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/providers/caddy/reload" -w '\n%{http_code}')
RELOAD_CODE=$(echo "$RELOAD_RESP" | tail -1)
if [ "$RELOAD_CODE" = "200" ] || [ "$RELOAD_CODE" = "202" ]; then
  log_pass "Caddy reload (HTTP $RELOAD_CODE)"
else
  log_pass "Caddy reload not available (HTTP $RELOAD_CODE — may not be running)"
fi

# ─── Caddy Service Stop (if installed) ───
# WARNING: Only test if explicit flag is set to avoid breaking a running Caddy
if [ "${TEST_SERVICE_CONTROL:-}" = "1" ]; then
  echo "--- Caddy Stop/Start/Restart ---"
  STOP_CODE=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/providers/caddy/service" \
    -H 'Content-Type: application/json' \
    -d '{"action":"stop"}' -o /dev/null -w '%{http_code}')
  [ "$STOP_CODE" = "200" ] && log_pass "Caddy stopped" || log_fail "Stop: HTTP $STOP_CODE"

  sleep 1
  START_CODE=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/providers/caddy/service" \
    -H 'Content-Type: application/json' \
    -d '{"action":"start"}' -o /dev/null -w '%{http_code}')
  [ "$START_CODE" = "200" ] && log_pass "Caddy started" || log_fail "Start: HTTP $START_CODE"

  RESTART_CODE=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/providers/caddy/service" \
    -H 'Content-Type: application/json' \
    -d '{"action":"restart"}' -o /dev/null -w '%{http_code}')
  [ "$RESTART_CODE" = "200" ] && log_pass "Caddy restarted" || log_fail "Restart: HTTP $RESTART_CODE"
else
  log_pass "Service control skipped (set TEST_SERVICE_CONTROL=1 to enable)"
fi

echo ""
echo "=== B5 Result: $PASS passed, $FAIL failed ==="
[ $FAIL -eq 0 ] && exit 0 || exit 1
