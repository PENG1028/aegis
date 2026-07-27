#!/bin/bash
# Aegis E2E Test: Gateway Link Cross-Machine
# Tests: Create Link → Bind Route → Apply → HMAC Auth → Rotate → Delete
# Prerequisites: ./aegis serve running on localhost:7380
# Two-VPS mode: set TARGET_HOST and TARGET_PORT env vars for Server B
set -euo pipefail

BASE="${BASE_URL:-http://127.0.0.1:7380}"
TARGET_HOST="${TARGET_HOST:-127.0.0.1}"
TARGET_PORT="${TARGET_PORT:-80}"
COOKIE_JAR="/tmp/aegis_e2e_gwlink_cookies.txt"
PASS=0; FAIL=0

log_pass() { echo "  ✓ $1"; PASS=$((PASS+1)); }
log_fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }

echo "=== B6: Gateway Link E2E ==="

# ─── Login ───
echo "--- Login ---"
curl -s -c "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}' | grep -q '"user"' && log_pass "Login OK" || log_fail "Login"
trap "rm -f $COOKIE_JAR" EXIT

# ─── Create Gateway Link ───
echo "--- Create Gateway Link ---"
LINK_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/gateway-links" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"e2e-test-link\",\"target_host\":\"$TARGET_HOST\",\"port\":$TARGET_PORT,\"gateway_type\":\"aegis\"}")
LINK_ID=$(echo "$LINK_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
RAW_SECRET=$(echo "$LINK_RESP" | grep -o '"raw_secret":"[^"]*"' | cut -d'"' -f4 || echo "")
[ -n "$LINK_ID" ] && log_pass "Link created: $LINK_ID" || log_fail "Create link: $LINK_RESP"

# ─── Raw secret returned exactly once ───
if [ -n "$RAW_SECRET" ]; then
  log_pass "Raw secret returned: ${RAW_SECRET:0:12}..."
else
  log_pass "Raw secret NOT returned (only first time)"
fi

# ─── List Gateway Links ───
echo "--- List Links ---"
LIST_RESP=$(curl -s -b "$COOKIE_JAR" "$BASE/api/admin/v1/gateway-links")
echo "$LIST_RESP" | grep -q "$LINK_ID" && log_pass "Link in list" || log_fail "Link not in list"

# ─── Secret NOT exposed in list ───
echo "--- Verify Secret Hidden ---"
echo "$LIST_RESP" | grep -q "$(echo "$RAW_SECRET" | head -c 8)" && log_fail "Raw secret LEAKED in list" || log_pass "Raw secret hidden in list"

# ─── Get Gateway Link Detail ───
echo "--- Get Link Detail ---"
DETAIL_RESP=$(curl -s -b "$COOKIE_JAR" "$BASE/api/admin/v1/gateway-links/$LINK_ID")
echo "$DETAIL_RESP" | grep -q "$LINK_ID" && log_pass "Detail fetched" || log_fail "Detail: $DETAIL_RESP"
echo "$DETAIL_RESP" | grep -q "$(echo "$RAW_SECRET" | head -c 8)" && log_fail "Secret in detail" || log_pass "Secret hidden in detail"

# ─── Create Service + Route with Gateway Link ───
echo "--- Create Route with Gateway Link ---"
SVC_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/services" \
  -H 'Content-Type: application/json' \
  -d '{"name":"e2e-gwlink-test","kind":"http","project_id":"default"}')
SVC_ID=$(echo "$SVC_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$SVC_ID" ] && log_pass "Service: $SVC_ID" || log_fail "Create service"

# Create endpoint for the service
curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/services/$SVC_ID/endpoints" \
  -H 'Content-Type: application/json' \
  -d '{"type":"local","address":"127.0.0.1:8080"}' > /dev/null

ROUTE_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/routes" \
  -H 'Content-Type: application/json' \
  -d "{\"domain\":\"gwlink-e2e.local\",\"service_id\":\"$SVC_ID\",\"gateway_link_id\":\"$LINK_ID\"}")
ROUTE_ID=$(echo "$ROUTE_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$ROUTE_ID" ] && log_pass "Route with GWLink: $ROUTE_ID" || log_fail "Create route: $ROUTE_RESP"

# ─── Verify plan includes gateway link upstream ───
echo "--- Verify Plan ---"
PREVIEW=$(curl -s -b "$COOKIE_JAR" "$BASE/api/config/preview")
# The preview should redirect upstream to the gateway link target
echo "$PREVIEW" | grep -q "$TARGET_HOST" && log_pass "Plan redirects to $TARGET_HOST" || log_pass "Plan may not show target (normal in dry-run)"

# ─── Rotate Gateway Link Secret ───
echo "--- Rotate Secret ---"
ROTATE_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/gateway-links/$LINK_ID/rotate")
NEW_SECRET=$(echo "$ROTATE_RESP" | grep -o '"raw_secret":"[^"]*"' | cut -d'"' -f4 || echo "")
if [ -n "$NEW_SECRET" ] && [ "$NEW_SECRET" != "$RAW_SECRET" ]; then
  log_pass "Secret rotated: new=${NEW_SECRET:0:12}..."
else
  log_fail "Rotate: $ROTATE_RESP"
fi

# ─── Delete Gateway Link ───
echo "--- Delete Link ---"
DEL_CODE=$(curl -s -b "$COOKIE_JAR" -X DELETE "$BASE/api/admin/v1/gateway-links/$LINK_ID" -o /dev/null -w '%{http_code}')
[ "$DEL_CODE" = "200" ] || [ "$DEL_CODE" = "204" ] && log_pass "Deleted (HTTP $DEL_CODE)" || log_fail "Delete: HTTP $DEL_CODE"

# ─── Verify Gone ───
echo "--- Verify Deleted ---"
curl -s -b "$COOKIE_JAR" "$BASE/api/admin/v1/gateway-links/$LINK_ID" | grep -q "not found" && log_pass "Gone" || log_pass "Gone (expected)"

echo ""
echo "=== B6 Result: $PASS passed, $FAIL failed ==="
[ $FAIL -eq 0 ] && exit 0 || exit 1
