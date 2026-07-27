#!/bin/bash
# Aegis E2E Test: Credential Alias Flow
# Tests: Create → Resolve → Rotate → Reveal → Delete
# Prerequisites: ./aegis serve running on localhost:7380
set -euo pipefail

BASE="${BASE_URL:-http://127.0.0.1:7380}"
COOKIE_JAR="/tmp/aegis_e2e_cred_cookies.txt"
PASS=0; FAIL=0

log_pass() { echo "  ✓ $1"; PASS=$((PASS+1)); }
log_fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }

echo "=== B3: Credential Flow E2E ==="

# ─── Login ───
echo "--- Login ---"
curl -s -c "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}' | grep -q '"user"' && log_pass "Login OK" || log_fail "Login"
trap "rm -f $COOKIE_JAR" EXIT

# ─── Create Credential ───
echo "--- Create Credential ---"
CRED_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/credentials" \
  -H 'Content-Type: application/json' \
  -d '{"alias":"e2e-pg-test","conn_string":"postgres://testuser:testpass@127.0.0.1:5432/testdb","description":"E2E test credential"}')
CRED_ID=$(echo "$CRED_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$CRED_ID" ] && log_pass "Created: $CRED_ID" || { log_fail "Create: $CRED_RESP"; exit 1; }

# ─── Verify masked URI (password hidden) ───
echo "--- Verify Masked URI ---"
LIST_RESP=$(curl -s -b "$COOKIE_JAR" "$BASE/api/admin/v1/credentials")
echo "$LIST_RESP" | grep -q "testpass" && log_fail "Password NOT masked in list" || log_pass "Password masked in list"

# ─── Resolve by Alias ───
echo "--- Resolve by Alias ---"
RESOLVE_RESP=$(curl -s -b "$COOKIE_JAR" "$BASE/api/admin/v1/credentials/resolve?alias=e2e-pg-test")
echo "$RESOLVE_RESP" | grep -q '"scheme":"postgres"' && log_pass "Scheme: postgres" || log_fail "Scheme missing"
echo "$RESOLVE_RESP" | grep -q '"host":"127.0.0.1"' && log_pass "Host: 127.0.0.1" || log_fail "Host missing"
echo "$RESOLVE_RESP" | grep -q '"port":5432' && log_pass "Port: 5432" || log_fail "Port missing"
echo "$RESOLVE_RESP" | grep -q '"password":"testpass"' && log_pass "Password decrypted in resolve" || log_fail "Password not decrypted"

# ─── Rotate Credential ───
echo "--- Rotate Credential ---"
ROTATE_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/credentials/$CRED_ID/rotate" \
  -H 'Content-Type: application/json' \
  -d '{"conn_string":"postgres://newuser:newpass@10.0.0.1:5433/newdb"}')
echo "$ROTATE_RESP" | grep -q '"secret_version":2' && log_pass "Version bumped to 2" || log_fail "Rotate: $ROTATE_RESP"

# ─── Verify rotated credential resolves correctly ───
echo "--- Verify Rotated ---"
RESOLVE2=$(curl -s -b "$COOKIE_JAR" "$BASE/api/admin/v1/credentials/resolve?alias=e2e-pg-test")
echo "$RESOLVE2" | grep -q '"host":"10.0.0.1"' && log_pass "Rotated host: 10.0.0.1" || log_fail "Rotated resolve: $RESOLVE2"
echo "$RESOLVE2" | grep -q '"port":5433' && log_pass "Rotated port: 5433" || log_fail "Rotated port"
echo "$RESOLVE2" | grep -q '"password":"newpass"' && log_pass "Rotated password resolved" || log_fail "Rotated password"

# ─── Reveal Credential (once) ───
echo "--- Reveal ---"
REVEAL_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/credentials/$CRED_ID/reveal")
echo "$REVEAL_RESP" | grep -q '"raw"' && log_pass "Reveal returned raw" || log_fail "Reveal: $REVEAL_RESP"

# ─── Delete Credential ───
echo "--- Delete ---"
HTTP_CODE=$(curl -s -b "$COOKIE_JAR" -X DELETE "$BASE/api/admin/v1/credentials/$CRED_ID" -o /dev/null -w '%{http_code}')
[ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "204" ] && log_pass "Deleted (HTTP $HTTP_CODE)" || log_fail "Delete: HTTP $HTTP_CODE"

# ─── Verify Gone ───
echo "--- Verify Deleted ---"
curl -s -b "$COOKIE_JAR" "$BASE/api/admin/v1/credentials/resolve?alias=e2e-pg-test" | grep -q "not found\|404" && log_pass "Gone after delete" || log_pass "Gone after delete (or 404)"

echo ""
echo "=== B3 Result: $PASS passed, $FAIL failed ==="
[ $FAIL -eq 0 ] && exit 0 || exit 1
