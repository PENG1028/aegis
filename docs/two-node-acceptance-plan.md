# Two-Node Acceptance Plan — v1.7AA

## Topology

```
Server A (43.160.211.232)
├── Aegis leader
├── Caddy :80
└── HAProxy :443
         │
         │ (cross-VPC — requires security group or VPN)
         │
Server B (43.159.34.11)
└── Target service :3000

Dev Machine (SSH to both)
```

## Prerequisites

- [ ] Server A and Server B can reach each other on target port
  - Same VPC, OR
  - Cloud security group allows inbound from Server A IP, OR
  - VPN/WireGuard tunnel, OR
  - SSH tunnel from Server A to Server B

## Test Cases

### TC1: Remote Target Reachable
1. Start target on Server B: `python3 -m http.server 3000 --bind 0.0.0.0`
2. Verify from Server A: `curl http://<SERVER_B_IP>:3000/` → 200
3. On Server A: `POST /api/v1/actions/bind-http-domain` → target=`<SERVER_B_IP>:3000`
4. `POST /api/admin/v1/system/apply`
5. `GET /api/admin/v1/trace/domain/{domain}` → final_target reachable=true
6. `curl http://domain` → reaches Server B service

### TC2: Remote Target Failure
1. Stop target on Server B
2. `GET /api/admin/v1/trace/domain/{domain}` → TARGET_CONNECTION_REFUSED
3. Restart target on Server B
4. `GET /api/admin/v1/trace/domain/{domain}` → reachable=true

### TC3: Hot Update (update-target)
1. Start second target on Server B on different port: `:3001`
2. `PATCH /api/v1/actions/update-target` → change to port 3001
3. `POST /api/admin/v1/system/apply`
4. `GET /api/admin/v1/trace/domain/{domain}` → new port reflected
5. `curl http://domain` → reaches new port

### TC4: Aegis Restart
1. Kill Aegis on Server A
2. Verify curl still returns response (Caddy serves cached config)
3. Restart Aegis
4. `GET /api/admin/v1/trace/domain/{domain}` → still works
5. No duplicate routes/edge rules

## Current Status

Network connectivity between servers is blocked by cloud security group.
Full execution requires network fix.
