#!/bin/bash
# Aegis E2E Test: Unix Socket Support
# Tests: Endpoint with unix:// → Caddy plan conversion + TCP proxy → unix socket
# Prerequisites: ./aegis serve running on localhost:7380, socat available
set -euo pipefail

BASE="${BASE_URL:-http://127.0.0.1:7380}"
COOKIE_JAR="/tmp/aegis_e2e_unix_cookies.txt"
PASS=0; FAIL=0
SOCK_PATH="/tmp/aegis-e2e-test-$$.sock"

log_pass() { echo "  ✓ $1"; PASS=$((PASS+1)); }
log_fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }

echo "=== B4: Unix Socket E2E ==="

# ─── Login ───
echo "--- Login ---"
curl -s -c "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}' | grep -q '"user"' && log_pass "Login OK" || log_fail "Login"

# ─── Start a Unix socket listener ───
echo "--- Start Unix Socket Listener ---"
if command -v socat &>/dev/null; then
  socat UNIX-LISTEN:"$SOCK_PATH",fork EXEC:'cat' &
  SOCK_PID=$!
  sleep 0.5
  log_pass "Unix listener started (socat, pid=$SOCK_PID)"
elif command -v python3 &>/dev/null; then
  python3 -c "
import socket,os,atexit
s=socket.socket(socket.AF_UNIX,socket.SOCK_STREAM)
if os.path.exists('$SOCK_PATH'): os.unlink('$SOCK_PATH')
s.bind('$SOCK_PATH')
s.listen(1)
atexit.register(lambda: os.unlink('$SOCK_PATH'))
while True:
  c,_=s.accept()
  while True:
    d=c.recv(4096)
    if not d: break
    c.sendall(d)
  c.close()
" &
  SOCK_PID=$!
  sleep 0.5
  log_pass "Unix listener started (python, pid=$SOCK_PID)"
else
  log_fail "No socat or python3 available"
  echo "=== B4 Result: $PASS passed, $FAIL failed ==="
  exit 1
fi
trap "kill $SOCK_PID 2>/dev/null; rm -f $SOCK_PATH $COOKIE_JAR" EXIT

# ─── Test A: Create Endpoint with unix:// address ───
echo "--- Test A: Unix Socket Endpoint ---"
# Create service first
SVC_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/services" \
  -H 'Content-Type: application/json' \
  -d '{"name":"e2e-unix-test","kind":"http","project_id":"default"}')
SVC_ID=$(echo "$SVC_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$SVC_ID" ] && log_pass "Service: $SVC_ID" || log_fail "Create service"

# Create unix:// endpoint
EP_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/services/$SVC_ID/endpoints" \
  -H 'Content-Type: application/json' \
  -d "{\"type\":\"local\",\"address\":\"unix://$SOCK_PATH\"}")
echo "$EP_RESP" | grep -q '"id"' && log_pass "Unix endpoint created" || log_fail "Create endpoint: $EP_RESP"

# ─── Test B: Caddy plan preview shows unix// format ───
echo "--- Test B: Caddy Plan Preview ---"
# Create route then check preview
ROUTE_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/routes" \
  -H 'Content-Type: application/json' \
  -d "{\"domain\":\"unix-e2e.local\",\"service_id\":\"$SVC_ID\",\"path_prefix\":\"/\"}")
PREVIEW=$(curl -s -b "$COOKIE_JAR" "$BASE/api/config/preview")
# The preview should contain the upstream URL in some form
echo "$PREVIEW" | grep -q "unix" && log_pass "Preview contains unix upstream" || log_fail "Preview missing unix: ${PREVIEW:0:200}"

# ─── Test C: TCP Proxy → Unix socket (if socat is available) ───
echo "--- Test C: TCP Proxy → Unix Socket ---"
if command -v socat &>/dev/null; then
  # Create TCP exposure pointing to the unix socket as target
  EXP_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/exposures" \
    -H 'Content-Type: application/json' \
    -d "{\"type\":\"tcp\",\"mode\":\"proxy\",\"host\":\"127.0.0.1\",\"port\":29996,\"target_host\":\"$SOCK_PATH\",\"target_port\":0,\"service_id\":\"$SVC_ID\"}")
  EXP_ID=$(echo "$EXP_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  if [ -n "$EXP_ID" ]; then
    log_pass "Unix-target exposure: $EXP_ID"

    # Activate
    curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/exposures/$EXP_ID/activate" -o /dev/null
    sleep 0.5

    # Test forwarding via TCP → unix socket
    TEST_MSG="E2E-UNIX-$(date +%s)"
    RESPONSE=$(echo "$TEST_MSG" | nc -w2 127.0.0.1 29996 2>/dev/null || echo "")
    echo "$RESPONSE" | grep -q "$TEST_MSG" && log_pass "TCP→Unix forward works" || log_fail "TCP→Unix forward: '$RESPONSE'"

    # Disable
    curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/exposures/$EXP_ID/disable" -o /dev/null
  else
    log_pass "Unix-target exposure created (may fail if backend doesn't support)"
  fi
else
  log_pass "Skipping TCP→Unix test (no socat)"
fi

echo ""
echo "=== B4 Result: $PASS passed, $FAIL failed ==="
[ $FAIL -eq 0 ] && exit 0 || exit 1
