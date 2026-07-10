# Real Two-node VPS Acceptance Runbook

> **Status:** v1.8C-7 | planned (not executed)
> **Verification label:** `real_two_node_pending`
> **Target:** Execute this runbook to upgrade from `simulated_two_node_verified` to `real_two_node_verified`.

---

## 1. Topology

```
┌─────────────────────────────┐          ┌─────────────────────────────┐
│      Node A (dev machine)   │          │   Node B (Server B VPS)     │
│                             │          │                             │
│  control plane (:8443)      │  ──────  │  Aegis relay handler (:443) │
│  local gateway (:18080)     │  HTTP(s) │  target service (:8080)     │
│  node runtime               │          │  public/private gateway     │
│                             │          │                             │
│  routing table cache:       │          │  GatewayLink gl-a-b:        │
│    api-b.example.com ───────┼──────────┤  encrypted secret           │
│    -> node-b (relay)        │          │                             │
└─────────────────────────────┘          └─────────────────────────────┘
```

**Prerequisites:**

- Developer entry workflow verified (see developer-entry-local-gateway.md)
- Local gateway health/status endpoints available
- Startup diagnostics pass
- Node configuration in standard node.yaml format

- Two Linux VPS with port 80/443 open in security group
- Aegis binary built (`go build -o aegis ./cmd/aegis/`)
- SSH access to both machines
- `~/.ssh/config` configured with `server-a` and `server-b` aliases

---

## 2. Control Plane Setup (Node A)

### 2.1 Generate Master Key

```bash
# Generate a 64-hex-char AES-256 key
./aegis init keygen
# Output: AEGIS_SECRET_KEY=abc123... (64 hex chars)

# Write to file with secure permissions
echo 'abc123...' > /etc/aegis/secret.key
chmod 0600 /etc/aegis/secret.key
```

### 2.2 Start Control Plane

```bash
# Initialize DB
./aegis init db

# Start control plane (listens on :8443)
AEGIS_SECRET_KEY=abc123... ./aegis serve \
  --bind 0.0.0.0:8443 \
  --data-dir /var/lib/aegis \
  --dev-mode
```

Verify:
```bash
curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8443/health
# Expected: 200
```

### 2.3 Create Admin Token

```bash
# Generate admin token
ADMIN_TOKEN=$(./aegis admin token generate --description "runbook")
echo "Admin token: $ADMIN_TOKEN"

# Verify admin access
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://127.0.0.1:8443/api/admin/v1/settings | jq .
```

---

## 3. Node Registration

### 3.1 Create Join Tokens

```bash
# Create join token for node-a
JOIN_TOKEN_A=$(curl -s -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"description":"node-a join"}' \
  http://127.0.0.1:8443/api/admin/v1/join-tokens | jq -r '.join_token')

# Create join token for node-b
JOIN_TOKEN_B=$(curl -s -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"description":"node-b join"}' \
  http://127.0.0.1:8443/api/admin/v1/join-tokens | jq -r '.join_token')

echo "Token A: $JOIN_TOKEN_A"
echo "Token B: $JOIN_TOKEN_B"
```

### 3.2 Node A Joins

```bash
# On Node A (dev machine):
NODE_A_RESP=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d "{
    \"join_token\": \"$JOIN_TOKEN_A\",
    \"node_name\": \"node-a\",
    \"hostname\": \"$(hostname)\",
    \"public_ip\": \"$(curl -s ifconfig.me)\",
    \"agent_version\": \"$(./aegis version 2>/dev/null || echo 'dev')\"
  }" \
  http://127.0.0.1:8443/api/node/v1/join)

NODE_A_ID=$(echo $NODE_A_RESP | jq -r '.node_id')
NODE_A_TOKEN=$(echo $NODE_A_RESP | jq -r '.node_token')
echo "Node A: ID=$NODE_A_ID Token=$NODE_A_TOKEN"

# Save node-a token to /etc/aegis/node-token
echo "$NODE_A_TOKEN" > /etc/aegis/node-token
chmod 0600 /etc/aegis/node-token
```

### 3.3 Node B Joins

```bash
# On Node B (Server B, SSH):
NODE_B_RESP=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d "{
    \"join_token\": \"$JOIN_TOKEN_B\",
    \"node_name\": \"node-b\",
    \"hostname\": \"$(hostname)\",
    \"public_ip\": \"$(curl -s ifconfig.me)\",
    \"agent_version\": \"$(./aegis version 2>/dev/null || echo 'dev')\"
  }" \
  http://<control-plane-ip>:8443/api/node/v1/join)

NODE_B_ID=$(echo $NODE_B_RESP | jq -r '.node_id')
NODE_B_TOKEN=$(echo $NODE_B_RESP | jq -r '.node_token')
echo "Node B: ID=$NODE_B_ID Token=$NODE_B_TOKEN"

# Save node-b token
echo "$NODE_B_TOKEN" > /etc/aegis/node-token
chmod 0600 /etc/aegis/node-token
```

---

## 4. Gateway Inventory

### 4.1 Register Node-A Local Gateway

```bash
# This is auto-reported by heartbeat, but can be pre-registered:
curl -s -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "node_id": "'$NODE_A_ID'",
    "name": "local-gateway",
    "type": "local",
    "host": "127.0.0.1",
    "port": 18080,
    "scheme": "http",
    "enabled": true
  }' \
  http://127.0.0.1:8443/api/admin/v1/gateway-inventory | jq .
```

### 4.2 Register Node-B Gateway

```bash
# Node-B relay handler (on port 443):
curl -s -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "node_id": "'$NODE_B_ID'",
    "name": "relay-gateway",
    "type": "private",
    "host": "'$(ssh server-b "curl -s ifconfig.me")'",
    "port": 443,
    "scheme": "https",
    "enabled": true,
    "public_accessible": true
  }' \
  http://127.0.0.1:8443/api/admin/v1/gateway-inventory | jq .
```

---

## 5. GatewayLink

### 5.1 Create Encrypted GatewayLink

```bash
# Create GatewayLink gl-a-b between node-a and node-b
# The secret will be encrypted with the MasterKey (AES-256-GCM)
GL_RESP=$(curl -s -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "gl-a-b",
    "gateway_type": "upstream",
    "source_node_id": "'$NODE_A_ID'",
    "target_node_id": "'$NODE_B_ID'",
    "host": "'$(ssh server-b "curl -s ifconfig.me")'",
    "public_ip": "'$(ssh server-b "curl -s ifconfig.me")'",
    "port": 443,
    "auth_type": "shared_secret",
    "auto_route": true
  }' \
  http://127.0.0.1:8443/api/admin/v1/gateway-links)

echo "$GL_RESP" | jq .
GL_ID=$(echo $GL_RESP | jq -r '.id')
GL_SECRET=$(echo $GL_RESP | jq -r '.raw_secret // .secret')

echo "GatewayLink ID: $GL_ID"
echo "Secret (save once): $GL_SECRET"
```

### 5.2 Verify Encryption

```bash
# List should NOT expose raw token
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://127.0.0.1:8443/api/admin/v1/gateway-links | jq '.[] | {id, name, encrypted_secret_present: (.encrypted_secret != ""), secret_visible: (.secret != null)}'

# Expected: encrypted_secret_present=true, secret_visible=false
```

---

## 6. Service / Route / Endpoint

### 6.1 Create the Target Service

```bash
SVC_RESP=$(curl -s -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api-b",
    "description": "API service on node-b"
  }' \
  http://127.0.0.1:8443/api/admin/v1/services)

SVC_ID=$(echo $SVC_RESP | jq -r '.id')
echo "Service ID: $SVC_ID"
```

### 6.2 Create Endpoint

```bash
# Endpoint on node-b, port 8080
EP_RESP=$(curl -s -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "service_id": "'$SVC_ID'",
    "node_id": "'$NODE_B_ID'",
    "host": "127.0.0.1",
    "port": 8080,
    "protocol": "http",
    "weight": 100
  }' \
  http://127.0.0.1:8443/api/admin/v1/endpoints)

EP_ID=$(echo $EP_RESP | jq -r '.id')
echo "Endpoint ID: $EP_ID"
```

### 6.3 Create Route

```bash
# Route that maps api-b.example.com to the service
ROUTE_RESP=$(curl -s -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "service_id": "'$SVC_ID'",
    "domain": "api-b.example.com",
    "path": "/",
    "protocol": "http"
  }' \
  http://127.0.0.1:8443/api/admin/v1/routes)

ROUTE_ID=$(echo $ROUTE_RESP | jq -r '.id')
echo "Route ID: $ROUTE_ID"
```

---

## 7. Gateway Policy

### 7.1 Create Route Gateway Policy

```bash
# Allow private_gateway relay via gl-a-b
POLICY_RESP=$(curl -s -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "route_id": "'$ROUTE_ID'",
    "mode": "private_gateway",
    "gateway_link_id": "'$GL_ID'",
    "priority": 10
  }' \
  http://127.0.0.1:8443/api/admin/v1/route-gateway-policies)

POLICY_ID=$(echo $POLICY_RESP | jq -r '.id')
echo "Policy ID: $POLICY_ID"
```

---

## 8. Routing Table

### 8.1 Generate Node-A Routing Table

```bash
# Generate routing table for node-a
RT_RESP=$(curl -s -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"node_id": "'$NODE_A_ID'"}' \
  http://127.0.0.1:8443/api/admin/v1/routing-table/generate)

echo "$RT_RESP" | jq '.entries[] | {domain, candidates}'
```

### 8.2 Push Desired State

```bash
# Push desired state to node-a
curl -s -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "node_id": "'$NODE_A_ID'",
    "force": true
  }' \
  http://127.0.0.1:8443/api/admin/v1/desired-state/push | jq .
```

---

## 9. Node-A Runtime

### 9.1 Start Node Aegis

```bash
# On Node A (dev machine):
./aegis node \
  --control-plane http://127.0.0.1:8443 \
  --node-id $NODE_A_ID \
  --node-token-file /etc/aegis/node-token \
  --local-gateway-port 18080 \
  --node-id-string "node-a"
```

The node runtime will:
1. Pull desired state from control plane
2. Write routing table cache
3. Sync GatewayLink secrets via `GetGatewayLinkToken` API
4. Start local HTTP gateway on `127.0.0.1:18080`
5. Send heartbeat with gateway status

---

## 10. Node-B Runtime

### 10.1 Start Target Service

```bash
# On Node B (SSH):
# Simple health endpoint for testing
python3 -m http.server 8080 &
# OR a proper target:
# ./target-service --port 8080
```

### 10.2 Start Aegis with Relay Handler

```bash
# On Node B (SSH):
./aegis serve \
  --bind 0.0.0.0:443 \
  --data-dir /var/lib/aegis \
  --relay-enabled \
  --node-id $NODE_B_ID \
  --node-token-file /etc/aegis/node-token
```

Alternatively, for a minimal relay-only deployment:
```bash
# Run just the relay handler
./aegis relay --bind 0.0.0.0:443 \
  --control-plane http://<control-plane-ip>:8443 \
  --node-id $NODE_B_ID \
  --node-token-file /etc/aegis/node-token
```

---

## 11. Verify End-to-End

### 11.1 Pre-flight: Port Connectivity

```bash
# From dev machine, check Node-B relay port is reachable
ssh ubuntu@<control-plane> "curl -s -o /dev/null -w '%{http_code}' \
  --connect-timeout 3 --max-time 5 \
  http://$(ssh server-b 'curl -s ifconfig.me'):443/"
# Expected: 4xx (relay returns 400 for direct requests without relay headers)
```

### 11.2 Execute Relay Request

```bash
# From dev machine, through local gateway:
curl -i -H "Host: api-b.example.com" http://127.0.0.1:18080/health

# Expected output:
# HTTP/1.1 200 OK
# ...
# {"service":"node-b-target", "path":"/health", ...}

# Verify relay headers (check logs):
# X-Aegis-Route-ID: <route-id>
# X-Aegis-Gateway-ID: gl-a-b
# X-Aegis-Gateway-Token: REDACTED
# X-Aegis-Source-Node: node-a
# X-Aegis-Hop: 1
```

### 11.3 Verify POST Body Preservation

```bash
curl -i -X POST \
  -H "Host: api-b.example.com" \
  -H "Content-Type: application/json" \
  -d '{"key":"value"}' \
  http://127.0.0.1:18080/submit

# Expected: 200, body includes {"method":"POST"}
```

---

## 12. Negative Security Tests

### 12.1 421 — Unmanaged Domain

```bash
curl -i -H "Host: google.com" http://127.0.0.1:18080/anything
# Expected: 421 Misdirected Request
```

### 12.2 502 — Wrong GatewayLink Token

```bash
# After providing a wrong token upstream (simulate mismatch),
# or when the relay handler rejects:
curl -i -H "Host: api-b.example.com" http://127.0.0.1:18080/health
# If token wrong: 502 relay authentication failed
```

### 12.3 502 — Self-loop

```bash
# If routing table points to self:
# Expected: 502 relay authentication failed
```

### 12.4 Header Injection Prevention

```bash
# Attacker tries to inject headers:
curl -i -H "Host: api-b.example.com" \
  -H "X-Aegis-Target-Host: 1.2.3.4" \
  -H "X-Aegis-Target-Port: 9999" \
  http://127.0.0.1:18080/health

# Expected: 200 (headers stripped by stripAegisHeaders)
# Target receives NO X-Aegis-* headers
```

### 12.5 Token Leak Check

```bash
# Scan all response bodies for raw token pattern
# The GatewayLink token must NOT appear anywhere:
curl -s -H "Host: api-b.example.com" http://127.0.0.1:18080/health | grep -i "secret\|token"
# Expected: no output (token not in body)

# Check server logs:
ssh server-b "grep -i 'gl-a-b\|secret\|gateway.*token' /var/log/aegis/relay.log"
# Expected: no raw token in logs (only REDACTED or header names)
```

---

## 13. Troubleshooting

### Symptom | Likely Cause | Check | Fix
---|---|---|---
401 Unauthorized | Node token expired or wrong | `curl -s -w '%{http_code}' -H "Authorization: Bearer $(cat /etc/aegis/node-token)" http://cp:8443/api/node/v1/heartbeat` | Re-register node, update token
403 Forbidden | GatewayLink token mismatch | Check relay handler logs for `INVALID_GATEWAY_TOKEN` | Rotate GatewayLink secret, update routing table
400 Missing headers | Direct request to relay (not via local gateway) | Check request has X-Aegis-* headers | Route through local gateway :18080
502 Bad Gateway | Target service down | `curl http://127.0.0.1:8080/health` on Node B | Start target service
502 relay auth failed | Self-loop detected | Check routing table doesn't point back to same gateway | Fix routing table entry
503 Service Unavailable | Secret not found | `curl -s -H "Authorization: Bearer $TOKEN" http://cp:8443/api/node/v1/gateway-link-token/$GL_ID` | Check MasterKey is loaded, GatewayLink exists
421 Misdirected | Unmanaged domain | Check routing table has the domain | Create route + policy, regenerate routing table
Connection refused | Local gateway not running | `curl http://127.0.0.1:18080/health` | Check node-runtime logs
Secret decrypt failed | MasterKey missing or wrong | Check `AEGIS_SECRET_KEY` env or `/etc/aegis/secret.key` | Set correct master key
No route to host | Security group blocking port | `ssh ubuntu@source "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://target_ip:443/"` | Open port 443 on security group
Gateway link token unavailable | GatewayLink ID not in routing table | `curl -s -H "Authorization: Bearer $ADMIN_TOKEN" http://cp:8443/api/admin/v1/gateway-links/$GL_ID` | Regenerate routing table, push desired state
Local gateway port bind failed | Port already in use | `netstat -tlnp \| grep 18080` | Kill process using port, change port

---

## 14. Rollback Steps

```bash
# 1. Push empty desired state
curl -s -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"node_id": "'$NODE_A_ID'", "entries": [], "force": true}' \
  http://127.0.0.1:8443/api/admin/v1/desired-state/push

# 2. Stop node runtime (Ctrl+C)

# 3. Optionally delete GatewayLink
curl -s -X DELETE \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://127.0.0.1:8443/api/admin/v1/gateway-links/$GL_ID

# 4. Stop control plane (Ctrl+C)
```

---

## 15. Acceptance Criteria Checklist

| # | Check | Expected | Result |
|---|-------|----------|--------|
| 1 | Port connectivity A→B:443 | 4xx (relay active) | |
| 2 | Managed domain relay (200) | HTTP 200, target response | |
| 3 | POST body preserved | method: POST | |
| 4 | Unmanaged domain rejected | HTTP 421 | |
| 5 | Wrong token rejected | HTTP 502 | |
| 6 | Self-loop rejected | HTTP 502 | |
| 7 | Target header injection rejected | HTTP 200 (stripped) | |
| 8 | Missing gateway token → 503 | HTTP 503 | |
| 9 | Token not leaked in response | scan clean | |
| 10 | Token not leaked in logs | log scan clean | |
| 11 | Gateway status online | heartbeat accepted | |
| 12 | GatewayStatusProvider valid | LocalGatewayStatus() ok | |

**Verification label:** `real_two_node_verified` (all 12 checks pass)

---

## 16. Cleanup

```bash
# Remove node credentials
rm -f /etc/aegis/node-token /etc/aegis/secret.key

# Stop processes
pkill -f "aegis serve"
pkill -f "aegis node"
pkill -f "aegis relay"

# Delete database (if starting fresh)
rm -rf /var/lib/aegis/*.db
```
