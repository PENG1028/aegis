#!/bin/bash
# Aegis v1.7Y Real VPS Acceptance Script
# Run on Ubuntu 22.04/24.04 with caddy + haproxy installed
# Usage: chmod +x vps-acceptance.sh && ./vps-acceptance.sh

set -e

# === CONFIGURATION ===
AEGIS_PORT=9000
TEST_DOMAIN="${TEST_DOMAIN:-acceptance-test.aegis.local}"
TLS_DOMAIN="${TLS_DOMAIN:-tls-acceptance-test.aegis.local}"
BACKEND_PORT=3000
RESULTS_DIR="/tmp/aegis-acceptance-results"
AEGIS_BIN="${AEGIS_BIN:-./aegis}"

mkdir -p "$RESULTS_DIR"
echo "Aegis v1.7Y Acceptance Test — $(date)" | tee "$RESULTS_DIR/summary.txt"
echo "Domain: $TEST_DOMAIN" | tee -a "$RESULTS_DIR/summary.txt"
echo "TLS Domain: $TLS_DOMAIN" | tee -a "$RESULTS_DIR/summary.txt"
echo "" | tee -a "$RESULTS_DIR/summary.txt"

# === HELPER ===
check() {
    local name="$1"; shift
    local expected="$1"; shift
    echo -n "[....] $name ... "
    if "$@" > "$RESULTS_DIR/${name// /_}.txt" 2>&1; then
        echo "PASS"
        echo "[PASS] $name" >> "$RESULTS_DIR/summary.txt"
    else
        echo "FAIL (see $RESULTS_DIR/${name// /_}.txt)"
        echo "[FAIL] $name — expected: $expected" >> "$RESULTS_DIR/summary.txt"
    fi
}

cleanup() {
    echo ""
    echo "=== Cleanup ==="
    kill %1 2>/dev/null || true  # backend
    kill %2 2>/dev/null || true  # aegis
    echo "Backend and Aegis stopped."
}

trap cleanup EXIT

# ============================================
# PHASE 1: Bootstrap & Doctor
# ============================================
echo "=== Phase 1: Bootstrap & Doctor ==="

echo "[1.1] Bootstrap"
$AEGIS_BIN bootstrap 2>&1 | tee "$RESULTS_DIR/01_bootstrap.txt"

echo "[1.2] Doctor"
$AEGIS_BIN doctor 2>&1 | tee "$RESULTS_DIR/02_doctor.txt"

echo "[1.3] Edge check"
$AEGIS_BIN edge check 2>&1 | tee "$RESULTS_DIR/03_edge_check.txt"

# ============================================
# PHASE 2: Start Server & Provider Diagnose
# ============================================
echo ""
echo "=== Phase 2: Server & Provider Diagnose ==="

echo "[2.1] Starting Aegis server on port $AEGIS_PORT"
$AEGIS_BIN serve --port $AEGIS_PORT &
sleep 2
echo "Server PID: $!"

echo "[2.2] Provider list"
curl -s "http://localhost:$AEGIS_PORT/api/admin/v1/providers" \
  -H "Authorization: Bearer test-token" 2>&1 | tee "$RESULTS_DIR/04_providers.json"

# ============================================
# PHASE 3: Admin Setup
# ============================================
echo ""
echo "=== Phase 3: Admin Setup ==="

echo "[3.1] Admin login"
LOGIN=$(curl -s -X POST "http://localhost:$AEGIS_PORT/api/admin/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}')
echo "$LOGIN" | tee "$RESULTS_DIR/05_login.json"
ADMIN_COOKIE=$(echo "$LOGIN" | jq -r '.token // empty')
if [ -z "$ADMIN_COOKIE" ]; then
    echo "WARNING: Could not extract admin cookie from login response"
    echo "Login response: $LOGIN"
fi

echo "[3.2] Create space"
SPACE=$(curl -s -X POST "http://localhost:$AEGIS_PORT/api/admin/v1/scopes" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_COOKIE" \
  -d '{"name":"vps-acceptance-test"}')
echo "$SPACE" | tee "$RESULTS_DIR/06_space.json"
SPACE_ID=$(echo "$SPACE" | jq -r '.id // empty')

echo "[3.3] Create API key"
if [ -n "$SPACE_ID" ]; then
    KEY_RESP=$(curl -s -X POST "http://localhost:$AEGIS_PORT/api/admin/v1/scopes/${SPACE_ID}/api-keys" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $ADMIN_COOKIE" \
      -d '{"name":"test-key"}')
    echo "$KEY_RESP" | tee "$RESULTS_DIR/07_apikey.json"
    API_KEY=$(echo "$KEY_RESP" | jq -r '.key // empty')
    echo "API Key: $API_KEY"
else
    echo "SKIP: No space ID — cannot create API key"
    API_KEY=""
fi

# ============================================
# PHASE 4: HTTP Domain Bind
# ============================================
echo ""
echo "=== Phase 4: HTTP Domain Bind ==="

echo "[4.1] Start test backend on port $BACKEND_PORT"
python3 -m http.server $BACKEND_PORT --bind 127.0.0.1 &
sleep 1
echo "Backend PID: $!"

echo "[4.2] Bind HTTP domain"
if [ -n "$API_KEY" ]; then
    BIND=$(curl -s -X POST "http://localhost:$AEGIS_PORT/api/v1/actions/bind-http-domain" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $API_KEY" \
      -d "{\"domain\":\"$TEST_DOMAIN\",\"target_host\":\"127.0.0.1\",\"target_port\":$BACKEND_PORT}")
    echo "$BIND" | tee "$RESULTS_DIR/08_bind_http.json"
else
    echo "SKIP: No API key"
fi

echo "[4.3] Check system status"
curl -s "http://localhost:$AEGIS_PORT/api/system/status" | tee "$RESULTS_DIR/09_system_status.json"

echo "[4.4] List routes"
curl -s "http://localhost:$AEGIS_PORT/api/admin/v1/routes" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | tee "$RESULTS_DIR/10_routes.json"

echo "[4.5] List edge rules"
curl -s "http://localhost:$AEGIS_PORT/api/admin/v1/edge-rules" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | tee "$RESULTS_DIR/11_edge_rules.json"

# ============================================
# PHASE 5: Trace & Real Traffic
# ============================================
echo ""
echo "=== Phase 5: Trace & Traffic ==="

echo "[5.1] Trace domain"
curl -s "http://localhost:$AEGIS_PORT/api/admin/v1/trace/domain/$TEST_DOMAIN" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | tee "$RESULTS_DIR/12_trace_domain.json"

echo "[5.2] CLI trace domain"
$AEGIS_BIN trace domain "$TEST_DOMAIN" 2>&1 | tee "$RESULTS_DIR/13_trace_domain_cli.txt"

echo "[5.3] Real HTTP test"
curl -s -o /dev/null -w "HTTP %{http_code}\n" -H "Host: $TEST_DOMAIN" "http://127.0.0.1:80/" 2>&1 | tee "$RESULTS_DIR/14_http_test.txt"

echo "[5.4] Provider diagnose"
curl -s -X POST "http://localhost:$AEGIS_PORT/api/admin/v1/providers/diagnose" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | tee "$RESULTS_DIR/15_provider_diagnose.json"

# ============================================
# PHASE 6: TLS Backend Bind
# ============================================
echo ""
echo "=== Phase 6: TLS Backend ==="

if [ -n "$API_KEY" ]; then
    echo "[6.1] Bind TLS backend"
    TLS_BIND=$(curl -s -X POST "http://localhost:$AEGIS_PORT/api/v1/actions/bind-tls-backend" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $API_KEY" \
      -d "{\"sni_host\":\"$TLS_DOMAIN\",\"target_host\":\"127.0.0.1\",\"target_port\":8443}")
    echo "$TLS_BIND" | tee "$RESULTS_DIR/16_bind_tls.json"
fi

echo "[6.2] Trace SNI"
curl -s "http://localhost:$AEGIS_PORT/api/admin/v1/trace/sni/$TLS_DOMAIN" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | tee "$RESULTS_DIR/17_trace_sni.json"

echo "[6.3] CLI trace SNI"
$AEGIS_BIN trace sni "$TLS_DOMAIN" 2>&1 | tee "$RESULTS_DIR/18_trace_sni_cli.txt"

# ============================================
# PHASE 7: Log Verification
# ============================================
echo ""
echo "=== Phase 7: Log Verification ==="

echo "[7.1] Operation logs"
curl -s "http://localhost:$AEGIS_PORT/api/admin/v1/operations" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | tee "$RESULTS_DIR/19_operation_logs.json"

echo "[7.2] Apply logs"
curl -s "http://localhost:$AEGIS_PORT/api/admin/v1/apply-logs" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | tee "$RESULTS_DIR/20_apply_logs.json"

echo "[7.3] Audit logs"
curl -s "http://localhost:$AEGIS_PORT/api/admin/v1/audit-logs" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | tee "$RESULTS_DIR/21_audit_logs.json"

# ============================================
# PHASE 8: Restart Safety
# ============================================
echo ""
echo "=== Phase 8: Restart Safety ==="

echo "[8.1] Traffic before stop"
curl -s -o /dev/null -w "BEFORE: HTTP %{http_code}\n" -H "Host: $TEST_DOMAIN" "http://127.0.0.1:80/" 2>&1 | tee "$RESULTS_DIR/22_restart_before.txt"

echo "[8.2] Stop Aegis"
AEGIS_PID=$(pgrep -f "aegis serve" | head -1)
if [ -n "$AEGIS_PID" ]; then
    kill "$AEGIS_PID"
    sleep 2
    echo "Aegis (PID $AEGIS_PID) stopped"
else
    echo "WARNING: Could not find aegis PID"
fi

echo "[8.3] Traffic during Aegis downtime"
curl -s -o /dev/null -w "DURING: HTTP %{http_code}\n" -H "Host: $TEST_DOMAIN" "http://127.0.0.1:80/" 2>&1 | tee "$RESULTS_DIR/23_restart_during.txt"

echo "[8.4] Provider processes"
pgrep -a caddy 2>&1 | tee "$RESULTS_DIR/24_caddy_process.txt" || echo "Caddy not running" | tee "$RESULTS_DIR/24_caddy_process.txt"
pgrep -a haproxy 2>&1 | tee "$RESULTS_DIR/25_haproxy_process.txt" || echo "HAProxy not running" | tee "$RESULTS_DIR/25_haproxy_process.txt"

echo "[8.5] Restart Aegis"
$AEGIS_BIN serve --port $AEGIS_PORT &
sleep 2

echo "[8.6] Post-restart system status"
curl -s "http://localhost:$AEGIS_PORT/api/system/status" | tee "$RESULTS_DIR/26_post_restart_status.json"

echo "[8.7] Post-restart trace"
curl -s "http://localhost:$AEGIS_PORT/api/admin/v1/trace/domain/$TEST_DOMAIN" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | tee "$RESULTS_DIR/27_post_restart_trace.json"

# ============================================
# PHASE 9: Failure Injection
# ============================================
echo ""
echo "=== Phase 9: Failure Injection ==="

echo "[9.1] Target port closed (stop backend)"
kill %1 2>/dev/null || true
sleep 1
curl -s "http://localhost:$AEGIS_PORT/api/admin/v1/trace/domain/$TEST_DOMAIN" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.final_target' | tee "$RESULTS_DIR/28_target_closed.json"

echo "[9.2] Service key → admin route"
curl -s -o /dev/null -w "HTTP %{http_code}\n" "http://localhost:$AEGIS_PORT/api/admin/v1/routes" \
  -H "Authorization: Bearer $API_KEY" 2>&1 | tee "$RESULTS_DIR/29_service_key_admin.txt"

echo "[9.3] Gateway mutation frozen"
curl -s -X POST "http://localhost:$AEGIS_PORT/api/admin/v1/gateway/domains" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_COOKIE" \
  -d '{"domain":"should-fail.example.com"}' | tee "$RESULTS_DIR/30_gateway_frozen.json"

echo "[9.4] Apply locked (concurrent)"
curl -s -X POST "http://localhost:$AEGIS_PORT/api/admin/v1/system/apply" \
  -H "Authorization: Bearer $ADMIN_COOKIE" > "$RESULTS_DIR/31_apply_a.txt" 2>&1 &
curl -s -X POST "http://localhost:$AEGIS_PORT/api/admin/v1/system/apply" \
  -H "Authorization: Bearer $ADMIN_COOKIE" > "$RESULTS_DIR/31_apply_b.txt" 2>&1 &
wait
echo "Apply A: $(cat $RESULTS_DIR/31_apply_a.txt)"
echo "Apply B: $(cat $RESULTS_DIR/31_apply_b.txt)"

# ============================================
# SUMMARY
# ============================================
echo ""
echo "============================================"
echo "=== ACCEPTANCE COMPLETE ==="
echo "Results saved to: $RESULTS_DIR"
echo "Summary: $RESULTS_DIR/summary.txt"
echo "============================================"
cat "$RESULTS_DIR/summary.txt"
