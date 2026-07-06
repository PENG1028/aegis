#!/bin/bash
# ServiceAuth v2 E2E test — 4-service scenario
# Requires: Aegis running on 127.0.0.1:7380
# Run from tests/e2e/ directory

set -e
AEGIS="http://127.0.0.1:7380"
PASS=0
FAIL=0
SERVICES_PID=""

cleanup() {
    echo "=== Cleaning up ==="
    kill $SERVICES_PID 2>/dev/null || true
    wait 2>/dev/null || true
}
trap cleanup EXIT

pass() { echo "  ✓ $1"; PASS=$((PASS+1)); }
fail() { echo "  ✗ $1 (expected: $2)"; FAIL=$((FAIL+1)); }

check() {
    local actual="$1" expected="$2" desc="$3"
    if [ "$actual" = "$expected" ]; then
        pass "$desc"
    else
        fail "$desc (got=$actual expected=$expected)"
    fi
}

echo "=== ServiceAuth v2 E2E Test ==="
echo ""

# ─── Phase 1: Start test services ───
echo "--- Phase 1: Starting 4 services ---"
cd services
go run . -name=depotly -port=8081 -aegis=$AEGIS &
go run . -name=aetherion -port=8082 -aegis=$AEGIS &
go run . -name=storage-svc -port=8083 -aegis=$AEGIS &
go run . -name=monitor-svc -port=8084 -aegis=$AEGIS &
SERVICES_PID=$!
sleep 4
echo ""

# ─── Phase 2: Verify registration ───
echo "--- Phase 2: Registration ---"
COUNT=$(curl -s $AEGIS/api/admin/v1/service-auth/services | python3 -c "import sys,json; print(len(json.load(sys.stdin)['services']))" 2>/dev/null || echo "0")
check "$COUNT" "4" "4 services registered"

# Check public keys are different
KEYS=$(curl -s $AEGIS/api/admin/v1/service-auth/services)
echo "  Services: $KEYS"
echo ""

# ─── Phase 3: Health endpoints (no auth) ───
echo "--- Phase 3: Health endpoints ---"
for port in 8081 8082 8083 8084; do
    STATUS=$(curl -s http://127.0.0.1:$port/health | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null)
    check "$STATUS" "ok" "health :$port"
done
echo ""

# ─── Phase 4: Ticket verification ───
echo "--- Phase 4: Ticket auth ---"
# No ticket → 401
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:8082/ping)
check "$CODE" "401" "no ticket → 401"

# Garbage ticket → 403
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:8082/ping -H "X-Service-Ticket: garbage")
check "$CODE" "403" "garbage ticket → 403"
echo ""

# ─── Phase 5: Service groups + policies ───
echo "--- Phase 5: Groups + Policies ---"
# Create groups
curl -s -X POST $AEGIS/api/admin/v1/service-auth/groups \
  -H "Content-Type: application/json" \
  -d '{"name":"core-services","members":["depotly","aetherion"]}' > /dev/null
pass "group core-services created"

curl -s -X POST $AEGIS/api/admin/v1/service-auth/groups \
  -H "Content-Type: application/json" \
  -d '{"name":"storage-group","members":["storage-svc"]}' > /dev/null
pass "group storage-group created"

# Create policies
curl -s -X POST $AEGIS/api/admin/v1/service-auth/policies \
  -H "Content-Type: application/json" \
  -d '{"subject":"core-services","target_service":"*","effect":"allow"}' > /dev/null
pass "policy core→* allow"

curl -s -X POST $AEGIS/api/admin/v1/service-auth/policies \
  -H "Content-Type: application/json" \
  -d '{"subject":"core-services","target_service":"storage-svc","effect":"deny"}' > /dev/null
pass "policy core→storage deny"

# Verify groups exist
GCOUNT=$(curl -s $AEGIS/api/admin/v1/service-auth/groups | python3 -c "import sys,json; print(len(json.load(sys.stdin)['groups']))" 2>/dev/null)
check "$GCOUNT" "2" "2 groups stored"

PCOUNT=$(curl -s $AEGIS/api/admin/v1/service-auth/policies | python3 -c "import sys,json; print(len(json.load(sys.stdin)['policies']))" 2>/dev/null)
check "$PCOUNT" "2" "2 policies stored"
echo ""

# ─── Phase 6: Identity stability (restart) ───
echo "--- Phase 6: Identity stability ---"
# Kill and restart aetherion on different port
kill $(pgrep -f "name=aetherion") 2>/dev/null || true
sleep 1
go run . -name=aetherion -port=9092 -aegis=$AEGIS &
sleep 2
STATUS=$(curl -s http://127.0.0.1:9092/health | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null)
check "$STATUS" "ok" "aetherion restart on :9092"
# Verify it's still counted as 1 service (not 5)
sleep 2
COUNT=$(curl -s $AEGIS/api/admin/v1/service-auth/services | python3 -c "import sys,json; print(len(json.load(sys.stdin)['services']))" 2>/dev/null || echo "0")
check "$COUNT" "4" "still 4 services after restart"
echo ""

# ─── Phase 7: Rebind ───
echo "--- Phase 7: Rebind ---"
BEFORE=$(curl -s $AEGIS/api/admin/v1/service-auth/services | python3 -c "import sys,json; [print(s['name']) for s in json.load(sys.stdin)['services']]" 2>/dev/null)
echo "  Before: $BEFORE"

curl -s -X POST $AEGIS/api/admin/v1/service-auth/services/depotly/rebind \
  -H "Content-Type: application/json" \
  -d '{"new_name":"depotly-v2"}' > /dev/null
AFTER=$(curl -s $AEGIS/api/admin/v1/service-auth/services | python3 -c "import sys,json; [print(s['name']) for s in json.load(sys.stdin)['services']]" 2>/dev/null)
echo "  After:  $AFTER"

if echo "$AFTER" | grep -q "depotly-v2"; then
    pass "rebind depotly → depotly-v2"
else
    fail "rebind failed" ""
fi
echo ""

# ─── Summary ───
echo "================================="
echo "Results: $PASS passed, $FAIL failed"
echo "================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
