#!/bin/bash
# Aegis E2E Test: TCP Port Forwarding
# Tests: Create Exposure → Activate → TCP Forward → Disable → Verify
# Prerequisites: ./aegis serve running on localhost:7380
set -euo pipefail

BASE="${BASE_URL:-http://127.0.0.1:7380}"
COOKIE_JAR="/tmp/aegis_e2e_cookies.txt"
PASS=0; FAIL=0

log_pass() { echo "  ✓ $1"; PASS=$((PASS+1)); }
log_fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }

# ─── Login ───
echo "=== B1: TCP Exposure E2E ==="
echo "--- Login ---"
RESP=$(curl -s -c "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}')
echo "$RESP" | grep -q '"user"' && log_pass "Login OK" || log_fail "Login failed"

# ─── Create a Service ───
echo "--- Create Service ---"
SVC_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/services" \
  -H 'Content-Type: application/json' \
  -d '{"name":"e2e-tcp-test","kind":"tcp","project_id":"default"}')
SVC_ID=$(echo "$SVC_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$SVC_ID" ] && log_pass "Service created: $SVC_ID" || log_fail "Create service"

# ─── Create Endpoint ───
echo "--- Create Endpoint ---"
EP_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/services/$SVC_ID/endpoints" \
  -H 'Content-Type: application/json' \
  -d '{"type":"local","address":"127.0.0.1:19998"}')
[ -n "$EP_RESP" ] && log_pass "Endpoint created" || log_fail "Create endpoint"

# ─── Start a TCP echo server on target port ───
echo "--- Start Echo Server (port 19998) ---"
# Use socat if available, otherwise nc
if command -v socat &>/dev/null; then
  socat TCP-LISTEN:19998,fork,reuseaddr EXEC:'cat' &
  ECHO_PID=$!
  sleep 0.5
  log_pass "Echo server started (socat, pid=$ECHO_PID)"
elif command -v ncat &>/dev/null; then
  ncat -l -k 19998 --exec cat &
  ECHO_PID=$!
  sleep 0.5
  log_pass "Echo server started (ncat, pid=$ECHO_PID)"
else
  # PowerShell fallback on Windows
  powershell -Command "Start-Process -NoNewWindow powershell -ArgumentList '-Command','\$listener=[System.Net.Sockets.TcpListener]127.0.0.1,19998;\$listener.Start();while(\$true){\$c=\$listener.AcceptTcpClient();\$s=\$c.GetStream();\$s.CopyTo(\$s);\$c.Close()}'" &
  ECHO_PID=$!
  sleep 1
  log_pass "Echo server started (powershell, pid=$ECHO_PID)"
fi
trap "kill $ECHO_PID 2>/dev/null; rm -f $COOKIE_JAR" EXIT

# ─── Create TCP Exposure ───
echo "--- Create TCP Exposure ---"
EXP_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/exposures" \
  -H 'Content-Type: application/json' \
  -d "{\"type\":\"tcp\",\"mode\":\"proxy\",\"host\":\"127.0.0.1\",\"port\":29998,\"target_host\":\"127.0.0.1\",\"target_port\":19998,\"service_id\":\"$SVC_ID\"}")
EXP_ID=$(echo "$EXP_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$EXP_ID" ] && log_pass "Exposure created: $EXP_ID" || log_fail "Create exposure: $EXP_RESP"

# ─── Activate Exposure ───
echo "--- Activate Exposure ---"
ACT_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/exposures/$EXP_ID/activate")
echo "$ACT_RESP" | grep -q '"active"\|"running"\|"ok"\|"success"' && log_pass "Exposure activated" || log_fail "Activate: $ACT_RESP"
sleep 0.5

# ─── Test TCP Forwarding ───
echo "--- Test TCP Forwarding ---"
TEST_MSG="E2E-TCP-$(date +%s)"
if command -v ncat &>/dev/null; then
  RESPONSE=$(echo "$TEST_MSG" | ncat -w2 127.0.0.1 29998 2>/dev/null || echo "")
elif command -v nc &>/dev/null; then
  RESPONSE=$(echo "$TEST_MSG" | nc -w2 127.0.0.1 29998 2>/dev/null || echo "")
else
  # PowerShell fallback
  RESPONSE=$(powershell -Command "\$c=New-Object System.Net.Sockets.TcpClient('127.0.0.1',29998);\$s=\$c.GetStream();\$w=[System.IO.StreamWriter]::new(\$s);\$w.WriteLine('$TEST_MSG');\$w.Flush();\$r=[System.IO.StreamReader]::new(\$s);\$resp=\$r.ReadLine();\$c.Close();echo \$resp" 2>/dev/null || echo "")
fi
if echo "$RESPONSE" | grep -q "$TEST_MSG"; then
  log_pass "TCP forward works: sent='$TEST_MSG' received='$RESPONSE'"
else
  log_fail "TCP forward: no echo response (got: '$RESPONSE')"
fi

# ─── Verify Exposure Status ───
echo "--- Verify Status ---"
STATUS_RESP=$(curl -s -b "$COOKIE_JAR" "$BASE/api/exposures")
echo "$STATUS_RESP" | grep -q "$EXP_ID" && log_pass "Exposure in list" || log_fail "Exposure not found in list"

# ─── Disable Exposure ───
echo "--- Disable Exposure ---"
curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/exposures/$EXP_ID/disable" -o /dev/null -w '%{http_code}'
sleep 0.5

# ─── Verify Port Released ───
echo "--- Verify Port Released ---"
if echo "test" | nc -w1 127.0.0.1 29998 2>/dev/null; then
  log_fail "Port 29998 still open after disable"
else
  log_pass "Port 29998 closed after disable"
fi

echo ""
echo "=== B1 Result: $PASS passed, $FAIL failed ==="
[ $FAIL -eq 0 ] && exit 0 || exit 1
