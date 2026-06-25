# Pilot Domain Bind Result — v1.7Z-RC

## Service: `http.server :3000`

Low-risk Python HTTP server on `127.0.0.1:3000`.

## Flow

### 1. Create Scope
```json
{"id":"space_831a01dda13fedec","name":"pilot-test","status":"active"}
```

### 2. Create API Key
```json
{"id":"tok_298d559f5032191c","name":"pilot-key",
 "token":"9fcb9da195387d93f9581b2efe03e421986bdacf26fd4d9ae378f74ab4a2cf6b",
 "token_type":"space"}
```

### 3. Bind HTTP Domain
```json
POST /api/v1/actions/bind-http-domain
{"domain":"pilot.aegis.local","target_host":"127.0.0.1","target_port":3000}

Response:
{"operation_id":"op_06c15c4d4a95dd32",
 "status":"success",
 "message":"bound HTTP domain pilot.aegis.local -> 127.0.0.1:3000",
 "details":"service_id=svc_54aaca6fde2afc93 route_id=rt_52e6b16ff8c45c49"}
```

### 4. Safe Apply
```json
POST /api/admin/v1/system/apply
{"message":"apply completed","routes":2,"warnings":0}
```

### 5. Resources Created
```
services=2  (http-accept.aegis.local, http-pilot.aegis.local)
routes=2    (accept.aegis.local, pilot.aegis.local)
edge_rules=1 (pilot.aegis.local -> 127.0.0.1:8443)
```

### 6. Trace Domain (8 steps, complete)
```
[1] route      matched  route rt_52e6b16ff8c45c49: domain=pilot.aegis.local tls=true
[2] listener   matched  port 443 via haproxy_edge_mux
[3] edge_mux   matched  edge rule -> 127.0.0.1:8443
[4] caddy      matched  TLS termination on 127.0.0.1:8443
[5] route      matched  route detail: service_id=svc_54aaca6fde2afc93
[6] target     matched  target 127.0.0.1:3000 reachable
[7] provider   matched  HAProxy: available (v2.8.16) [diagnostic attached]
[8] provider   matched  Caddy: available (v2.6.2) [diagnostic attached]
```

### 7. Provider Diagnostic
```json
HAProxy: installed=true, version=2.8.16, config_valid=true, service_running=true
Caddy: installed=true, version=2.6.2, config_valid=true, service_running=true, runtime_verify_ok=true
healthy=True, issue_count=0
```

### 8. Traffic Verification
```bash
curl -H "Host: pilot.aegis.local" http://127.0.0.1:80/
# HTTP 200 (directory listing from Python server)
```

## Verdict

Full domain bind chain verified: Action API → service → endpoint → route → edge rule → safe apply → trace → provider diagnostic → traffic.
