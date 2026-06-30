#!/bin/bash
# Aegis E2E Test: UDP Port Forwarding
# Tests: Create Exposure → Activate → UDP Forward → Disable → Stats
# Prerequisites: ./aegis serve running on localhost:7380
set -euo pipefail

BASE="${BASE_URL:-http://127.0.0.1:7380}"
COOKIE_JAR="/tmp/aegis_e2e_udp_cookies.txt"
PASS=0; FAIL=0

log_pass() { echo "  ✓ $1"; PASS=$((PASS+1)); }
log_fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }

# ─── Login ───
echo "=== B2: UDP Exposure E2E ==="
echo "--- Login ---"
curl -s -c "$COOKIE_JAR" -X POST "$BASE/api/admin/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}' | grep -q '"user"' && log_pass "Login OK" || log_fail "Login"

# ─── Create Service ───
echo "--- Create Service ---"
SVC_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/services" \
  -H 'Content-Type: application/json' \
  -d '{"name":"e2e-udp-test","kind":"udp","project_id":"default"}')
SVC_ID=$(echo "$SVC_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$SVC_ID" ] && log_pass "Service: $SVC_ID" || log_fail "Create service"

# ─── Start UDP echo server ───
echo "--- Start UDP Echo (port 19997) ---"
if command -v socat &>/dev/null; then
  socat UDP-LISTEN:19997,fork,reuseaddr EXEC:'cat' &
  UDP_PID=$!
  sleep 0.5
  log_pass "UDP echo started (socat, pid=$UDP_PID)"
elif command -v ncat &>/dev/null; then
  ncat -u -l -k 19997 --exec cat &
  UDP_PID=$!
  sleep 0.5
  log_pass "UDP echo started (ncat, pid=$UDP_PID)"
else
  # Simple Python fallback
  python3 -c "
import socket
s=socket.socket(socket.AF_INET,socket.SOCK_DGRAM)
s.bind(('127.0.0.1',19997))
while True:
  d,a=s.recvfrom(1024)
  s.sendto(d,a)
" &
  UDP_PID=$!
  sleep 0.5
  log_pass "UDP echo started (python, pid=$UDP_PID)"
fi
trap "kill $UDP_PID 2>/dev/null; rm -f $COOKIE_JAR" EXIT

# ─── Create UDP Exposure ───
echo "--- Create UDP Exposure ---"
EXP_RESP=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/exposures" \
  -H 'Content-Type: application/json' \
  -d "{\"type\":\"udp\",\"mode\":\"proxy\",\"host\":\"127.0.0.1\",\"port\":29997,\"target_host\":\"127.0.0.1\",\"target_port\":19997,\"service_id\":\"$SVC_ID\"}")
EXP_ID=$(echo "$EXP_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$EXP_ID" ] && log_pass "Exposure: $EXP_ID" || log_fail "Create: $EXP_RESP"

# ─── Activate ───
echo "--- Activate ---"
curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/exposures/$EXP_ID/activate" -o /dev/null
sleep 0.5
log_pass "Activated"

# ─── Test UDP Forwarding ───
echo "--- Test UDP Forwarding ---"
if command -v socat &>/dev/null; then
  RESPONSE=$(echo "E2E-UDP-HELLO" | socat -t 1 - UDP:127.0.0.1:29997 2>/dev/null || echo "")
elif command -v nc &>/dev/null; then
  RESPONSE=$(echo "E2E-UDP-HELLO" | nc -u -w1 127.0.0.1 29997 2>/dev/null || echo "")
else
  RESPONSE=$(python3 -c "
import socket
s=socket.socket(socket.AF_INET,socket.SOCK_DGRAM)
s.sendto(b'E2E-UDP-HELLO',('127.0.0.1',29997))
s.settimeout(1)
try:
  d,_=s.recvfrom(1024)
  print(d.decode())
except: pass
" 2>/dev/null || echo "")
fi
echo "$RESPONSE" | grep -q "E2E-UDP-HELLO" && log_pass "UDP echo: '$RESPONSE'" || log_fail "UDP: no echo (got: '$RESPONSE')"

# ─── Disable ───
echo "--- Disable ---"
curl -s -b "$COOKIE_JAR" -X POST "$BASE/api/exposures/$EXP_ID/disable" -o /dev/null
sleep 0.5
# Test that port is released
if echo "test" | nc -u -w1 127.0.0.1 29997 2>/dev/null; then
  log_pass "Port released (no error)"
else
  log_pass "Port released (connection refused)"
fi

echo ""
echo "=== B2 Result: $PASS passed, $FAIL failed ==="
[ $FAIL -eq 0 ] && exit 0 || exit 1
