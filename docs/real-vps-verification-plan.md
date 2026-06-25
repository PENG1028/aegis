# Real VPS Verification Plan — v1.7X

## 1. Environment Requirements

### Minimum Setup
- Ubuntu 22.04 or 24.04
- 1 GB RAM, 1 vCPU
- Root or sudo access (for systemctl, port 80/443)
- Public IP or test domain with DNS/hosts mapping

### Software Installation
```bash
# Install dependencies
sudo apt update
sudo apt install -y haproxy caddy curl jq openssl

# Verify
which haproxy caddy curl jq openssl
haproxy -v
caddy version
```

### Build Aegis
```bash
cd /opt/aegis
go build -o aegis ./cmd/aegis/
```

### Test Domain Setup
```bash
# Option A: Real domain pointing to VPS IP
# Option B: /etc/hosts entry
echo "127.0.0.1 test-acceptance.example.com" | sudo tee -a /etc/hosts
echo "127.0.0.1 tls-backend.example.com" | sudo tee -a /etc/hosts
```

---

## 2. Acceptance Steps

### Phase 1: Bootstrap & Verify

```bash
# Step 1: Bootstrap
./aegis bootstrap
# EXPECTED: DB created, 3 listeners registered, no errors

# Step 2: Doctor
./aegis doctor
# EXPECTED: haproxy found, caddy found, ports 80/443/8443 shown

# Step 3: Provider Diagnose
./aegis serve --port 9000 &
sleep 2
curl -s -X POST http://localhost:9000/api/admin/v1/providers/diagnose \
  -H "Authorization: Bearer $(cat /tmp/admin_token)" | jq .
# EXPECTED: healthy=true, issue_count=0
```

### Phase 2: Admin Setup

```bash
# Step 4: Admin login
LOGIN=$(curl -s -X POST http://localhost:9000/api/admin/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}')
ADMIN_COOKIE=$(echo $LOGIN | jq -r '.token')
echo "Admin cookie: $ADMIN_COOKIE"

# Step 5: Create space
SPACE=$(curl -s -X POST http://localhost:9000/api/admin/v1/scopes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_COOKIE" \
  -d '{"name":"acceptance-test"}')
SPACE_ID=$(echo $SPACE | jq -r '.id')
echo "Space: $SPACE_ID"

# Step 6: Create API key
KEY=$(curl -s -X POST "http://localhost:9000/api/admin/v1/scopes/${SPACE_ID}/api-keys" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_COOKIE" \
  -d '{"name":"test-key"}')
API_KEY=$(echo $KEY | jq -r '.key')
echo "API Key: $API_KEY"
```

### Phase 3: HTTP Domain Bind

```bash
# Step 7: Bind HTTP domain
BIND=$(curl -s -X POST http://localhost:9000/api/v1/actions/bind-http-domain \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"domain":"test-acceptance.example.com","target_host":"127.0.0.1","target_port":8080}')
echo $BIND | jq .
# EXPECTED: status=success, operation_id present

# Step 8: Start a test backend
python3 -m http.server 8080 &
BACKEND_PID=$!

# Step 9: Safe apply (via admin)
curl -s -X POST http://localhost:9000/api/admin/v1/system/apply \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq .
# EXPECTED: success

# Step 10: Verify traffic
curl -s -H "Host: test-acceptance.example.com" http://127.0.0.1:80/ | head -5
# EXPECTED: Directory listing or HTTP response from python server

# Step 11: Trace domain
curl -s "http://localhost:9000/api/admin/v1/trace/domain/test-acceptance.example.com" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq .
# EXPECTED: trace_status=complete, final_target.reachable=true, caddy_diag present
```

### Phase 4: TLS Backend Bind

```bash
# Step 12: Bind TLS backend
TLS=$(curl -s -X POST http://localhost:9000/api/v1/actions/bind-tls-backend \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"sni_host":"tls-backend.example.com","target_host":"127.0.0.1","target_port":8443}')
echo $TLS | jq .

# Step 13: Apply
curl -s -X POST http://localhost:9000/api/admin/v1/system/apply \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq .

# Step 14: Verify TLS
openssl s_client -servername tls-backend.example.com -connect 127.0.0.1:443 </dev/null 2>&1 | head -10
# EXPECTED: CONNECTED

# Step 15: Trace SNI
curl -s "http://localhost:9000/api/admin/v1/trace/sni/tls-backend.example.com" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq .
```

### Phase 5: Restart Safety

```bash
# Step 16: Record state before
curl -s http://localhost:9000/api/system/status | jq '{state_version, pending_apply}' > /tmp/before.json
BEFORE_SV=$(jq -r '.state_version' /tmp/before.json)

# Step 17: Test traffic BEFORE stopping
curl -s -o /dev/null -w "%{http_code}" -H "Host: test-acceptance.example.com" http://127.0.0.1:80/
# EXPECTED: 200

# Step 18: Stop Aegis
kill $(pgrep aegis)
sleep 2
curl -s http://localhost:9000/api/health && echo "ERROR: Aegis still running" || echo "Aegis stopped (expected)"

# Step 19: Verify data plane works during Aegis downtime
curl -s -o /dev/null -w "%{http_code}" -H "Host: test-acceptance.example.com" http://127.0.0.1:80/
# EXPECTED: 200 (Caddy still serving)

# Step 20: Restart Aegis
./aegis serve --port 9000 &
sleep 2

# Step 21: Verify state recovery
curl -s http://localhost:9000/api/system/status | jq '{state_version, pending_apply}'
# EXPECTED: state_version >= BEFORE_SV, pending_apply=false

# Step 22: Verify trace still works
curl -s "http://localhost:9000/api/admin/v1/trace/domain/test-acceptance.example.com" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '{trace_status, final_target}'
```

### Phase 6: Log Verification

```bash
# Step 23: Operation logs
curl -s http://localhost:9000/api/admin/v1/operations \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[0] | {action, result, message}'

# Step 24: Apply logs
curl -s http://localhost:9000/api/admin/v1/apply-logs \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[0] | {validate_status, reload_status, step_log}'

# Step 25: Audit logs
curl -s http://localhost:9000/api/admin/v1/audit-logs \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.[0] | {event_type, actor_type, result}'
```

---

## 3. Failure Injection

### F1: Stop Caddy
```bash
sudo systemctl stop caddy
curl -s -X POST http://localhost:9000/api/admin/v1/providers/diagnose \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.caddy'
# EXPECTED: service_running=false, last_error_code=SERVICE_NOT_RUNNING
sudo systemctl start caddy
```

### F2: Invalid Config
```bash
echo "invalid { syntax }" | sudo tee /etc/caddy/Caddyfile.broken
# Validate should fail
caddy validate --config /etc/caddy/Caddyfile.broken 2>&1
# EXPECTED: syntax error
sudo rm /etc/caddy/Caddyfile.broken
```

### F3: Target Port Closed
```bash
kill $BACKEND_PID
curl -s "http://localhost:9000/api/admin/v1/trace/domain/test-acceptance.example.com" \
  -H "Authorization: Bearer $ADMIN_COOKIE" | jq '.final_target'
# EXPECTED: reachable=false, error_code=TARGET_CONNECTION_REFUSED
python3 -m http.server 8080 &
```

### F4: Duplicate Domain
```bash
# Create second space + key
SPACE2=$(curl -s -X POST http://localhost:9000/api/admin/v1/scopes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_COOKIE" \
  -d '{"name":"space-2"}')
SPACE2_ID=$(echo $SPACE2 | jq -r '.id')
KEY2=$(curl -s -X POST "http://localhost:9000/api/admin/v1/scopes/${SPACE2_ID}/api-keys" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_COOKIE" \
  -d '{"name":"key2"}' | jq -r '.key')

# Try to bind same domain
curl -s -X POST http://localhost:9000/api/v1/actions/bind-http-domain \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $KEY2" \
  -d '{"domain":"test-acceptance.example.com","target_host":"127.0.0.1","target_port":9090}' | jq .
# EXPECTED: status=409, error.code=DOMAIN_ALREADY_OWNED
```

### F5: Service Key → Admin Route
```bash
curl -s http://localhost:9000/api/admin/v1/scopes \
  -H "Authorization: Bearer $API_KEY" | jq .
# EXPECTED: 401 (no admin cookie) or 403 (service key denied)
```

### F6: Unauthenticated Admin Mutation
```bash
curl -s -X POST http://localhost:9000/api/admin/v1/system/apply | jq .
# EXPECTED: 401 UNAUTHORIZED
```

---

## 4. Evidence Collection Checklist

For each step, collect:

| Step | Command | stdout | HTTP Status | operation_log | apply_log | audit_log | trace |
|------|---------|--------|:---:|:---:|:---:|:---:|:---:|
| Bootstrap | `./aegis bootstrap` | □ | N/A | □ | N/A | N/A | N/A |
| Doctor | `./aegis doctor` | □ | N/A | N/A | N/A | N/A | N/A |
| Provider diagnose | POST /providers/diagnose | □ | □ | □ | N/A | N/A | N/A |
| Admin login | POST /auth/login | □ | □ | N/A | N/A | □ | N/A |
| Create scope | POST /scopes | □ | □ | □ | N/A | □ | N/A |
| Create API key | POST /api-keys | □ | □ | □ | N/A | □ | N/A |
| Bind HTTP domain | POST /actions/bind-http-domain | □ | □ | □ | □ | □ | N/A |
| Safe apply | POST /system/apply | □ | □ | □ | □ | N/A | N/A |
| curl traffic | curl -H Host:... | □ | N/A | N/A | N/A | N/A | N/A |
| Trace domain | GET /trace/domain/... | □ | □ | N/A | N/A | N/A | □ |
| Stop Aegis | kill | N/A | N/A | N/A | N/A | N/A | N/A |
| Traffic during downtime | curl | □ | N/A | N/A | N/A | N/A | N/A |
| Restart Aegis | ./aegis serve | N/A | N/A | N/A | N/A | N/A | N/A |
| State recovery | GET /system/status | □ | □ | N/A | N/A | N/A | N/A |
| Trace after restart | GET /trace/domain/... | □ | □ | N/A | N/A | N/A | □ |
| Operation logs | GET /operations | □ | □ | N/A | N/A | N/A | N/A |
| Apply logs | GET /apply-logs | □ | □ | N/A | N/A | N/A | N/A |
| Audit logs | GET /audit-logs | □ | □ | N/A | N/A | N/A | N/A |

---

## 5. Cleanup

```bash
kill $(pgrep aegis)
kill $BACKEND_PID
rm -rf ~/.aegis
sudo sed -i '/test-acceptance.example.com/d' /etc/hosts
sudo sed -i '/tls-backend.example.com/d' /etc/hosts
```
