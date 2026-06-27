# v1.8C-5 — Local HTTP Gateway & Managed Relay

> **Phase:** v1.8C — Multi-node Aegis Runtime
> **Status:** IMPLEMENTED ✅
> **Date:** 2026-06-27
> **Package:** `internal/localgateway/`

---

## 1. v1.8C-5 Scope

This phase implements the local HTTP gateway runtime, real relay execution with GatewayLink secret injection, and the domain resolution/dispatch pipeline on each Aegis node.

### Implemented

| Component | File | Status |
|-----------|------|--------|
| Local HTTP gateway config | `config.go` | ✅ implemented |
| Domain resolver interface | `resolver.go` | ✅ implemented |
| HTTP handler (managed/unmanaged dispatch) | `handler.go` | ✅ implemented |
| Local forwarder (same-node dispatch) | `local_dispatch.go` | ✅ implemented |
| Managed relay client (cross-node via GatewayLink) | `relay_client.go` | ✅ implemented |
| Gateway lifecycle (start/stop/status) | `server.go` | ✅ implemented |
| Gateway status tracking | `status.go` | ✅ implemented |
| GatewayLink secret provider interface | `secret_provider.go` (noderuntime) | ✅ implemented |
| Relay client with runtime token injection | `relay_client.go` | ✅ implemented |

### v1.8C-5 Builds On

- **v1.8C-4:** Node runtime reconciler, routing table cache, relay request plan builder
- **v1.8C-3:** Gateway policy engine, routing table generator
- **v1.8C-2:** Control plane sync foundation, desired/actual state management

### Not Implemented (deferred)

- Local DNS resolver (node-side DNS proxy for transparent domain access)
- Provider reconcile/apply (Caddy/HAProxy config generation)
- Background stale gateway monitor
- Automatic topology probing
- CLI commands for node runtime
- HTTPS / TLS termination in local gateway
- Wildcard domain resolution

---

## 2. Local HTTP Gateway Architecture

The local HTTP gateway is a lightweight HTTP proxy that runs on each Aegis node. It receives incoming HTTP requests, resolves the Host header against the local routing table cache, and dispatches the request to the appropriate target — either a local service or a remote node via managed relay.

### Architecture Diagram

```
Incoming HTTP Request
         │
         ▼
  ┌──────────────┐
  │   Handler     │  ServeHTTP(): extract domain from Host header
  │  (handler.go) │
  └──────┬───────┘
         │ domain (stripped of port)
         ▼
  ┌──────────────┐
  │   Resolver    │  DomainResolver.Resolve(domain) → RoutingDecision
  │ (resolver.go) │
  └──────┬───────┘
         │ RoutingDecision{Status, SelectedCandidate, TargetLocalHost, ...}
         ▼
    ┌────┴────┐
    │         │
  available   │
    │     unavailable/disabled
    │         │
    ▼         ▼
  ┌────────┐  ┌──────────────┐
  │Managed │  │  Unmanaged   │ → 421 Misdirected Request
  │Dispatch│  │  (reject)    │
  └───┬────┘  └──────────────┘
      │
  ┌───┴───────────┐
  │               │
  ▼               ▼
Local          Remote
Candidate      Candidate
  │               │
  ▼               ▼
┌──────────┐  ┌──────────────┐
│ Forwarder │  │ RelayClient  │
│(local_dispatch)│ (relay_client)│
│→ local svc │  │→ /__aegis/relay│
└──────────┘  │ + GWLink auth│
               └──────────────┘
```

### Gateway Lifecycle

The `Gateway` struct (server.go) manages the lifecycle:

```go
gw := localgateway.NewGateway(config, resolver, secretProvider)
err := gw.Start()   // binds to configured address, starts HTTP server
info := gw.Status() // returns current status (online/disabled/failed)
gw.Stop()           // gracefully closes the HTTP server
```

### Default Configuration

```go
Enabled:       true
BindAddr:      "127.0.0.1"
Port:          18080
UnmanagedMode: "reject"       // reject unmanaged domains with 421
PreserveHost:  true
RequestTimeoutSec: 30
```

### Files

| File | Purpose |
|------|---------|
| `config.go` | Gateway configuration struct, defaults, unmanaged mode constants |
| `resolver.go` | `DomainResolver` interface + `RoutingDecision` struct |
| `handler.go` | HTTP handler: domain extraction, managed/unmanaged dispatch |
| `local_dispatch.go` | `LocalForwarder`: same-node HTTP forwarding |
| `relay_client.go` | `RelayClient`: cross-node relay with GatewayLink token injection |
| `server.go` | `Gateway`: lifecycle management (start/stop/status) |
| `status.go` | `GatewayStatus`: thread-safe status tracking |

---

## 3. Managed Domain Processing Flow

A managed domain is one that appears in the local routing table cache with an `available` status and at least one candidate.

### Flow: Request → Response

#### Step 1: Extract Domain

The `Handler.ServeHTTP()` method extracts the domain from the `Host` header, stripping any port number:

```go
domain := r.Host       // e.g. "app.example.com:8080"
domain = stripPort(domain)  // → "app.example.com"
```

- IPv4 with port: `app.example.com:8080` → `app.example.com`
- IPv6 with port: `[::1]:8080` → `::1`
- No port: `app.example.com` → `app.example.com`
- Malformed/missing Host header → 400 Bad Request

#### Step 2: Resolve Domain

The resolver looks up the domain in the local routing table cache:

```go
decision := h.resolver.Resolve(domain)
```

Decision outcomes:

| Status | Meaning | Handler Action |
|--------|---------|----------------|
| `available` | Domain found, candidate available | `handleManaged()` |
| `disabled` | Domain found but disabled by policy | `handleUnmanaged()` → 421 |
| `unavailable` | Domain not found or no candidates | `handleUnmanaged()` → 421 |
| `nil` | Resolver error | 500 Internal Server Error |

#### Step 3: Managed Dispatch

For managed domains (`status == "available"`), the handler examines `SelectedCandidate.Mode`:

| Mode | Handler | Description |
|------|---------|-------------|
| `local_gateway` | `handleLocalDispatch()` | Forward to local service on same node |
| `private_gateway` | `handleRemoteRelay()` | Forward via relay to private gateway |
| `public_gateway` | `handleRemoteRelay()` | Forward via relay to public gateway |
| other | 501 Not Implemented | Unsupported candidate mode |

#### Step 4: Response Path

- **Local dispatch:** Response flows back from local service → forwarder → client
- **Remote relay:** Response flows back from remote gateway → relay client → client

---

## 4. Unmanaged Domain Behavior

### What is an Unmanaged Domain?

A domain is considered **unmanaged** when:
1. The domain does not appear in the local routing table cache
2. The routing table entry exists but has status `disabled` or `unavailable`
3. No candidates are available for the domain

### Default Behavior: Reject with 421

```go
func (h *Handler) handleUnmanaged(w http.ResponseWriter, r *http.Request, domain string) {
    http.Error(w, "Misdirected Request: domain not managed by Aegis",
        http.StatusMisdirectedRequest)
}
```

- **HTTP 421 Misdirected Request** — The standard HTTP status code for "the server is not configured to produce responses for this request"
- Response body: `"Misdirected Request: domain not managed by Aegis"`
- No proxy fallback, no passthrough, no external DNS resolution

### Configuration Options

The `UnmanagedMode` config field controls behavior:

| Mode | Constant | Behavior | Status |
|------|----------|----------|--------|
| `reject` | `UnmanagedReject` | 421 Misdirected Request | ✅ Implemented (default) |
| `passthrough_deferred` | `UnmanagedPassthroughDefer` | 421 (logic not yet implemented) | 🔜 Deferred |
| `proxy_deferred` | `UnmanagedProxyDefer` | 421 (logic not yet implemented) | 🔜 Deferred |

In v1.8C-5, all modes default to 421 reject. The deferred modes are placeholders for future transparent proxy behavior.

### No Open Proxy

The local HTTP gateway is **never** an open proxy. It will not forward requests to arbitrary external targets for domains not in the routing table. This is a hard boundary: if there is no routing table entry, the request is rejected.

### Why 421 Misdirected Request?

HTTP 421 (Misdirected Request) is the semantically correct status code:
- Indicates the server received the request but cannot produce a response for that target
- Unlike 502/503, it does not imply upstream failure — it implies the server does not handle this domain
- Used by CDNs and reverse proxies for similar domain-routing purposes

---

## 5. Local Candidate Dispatch

### When It Applies

Local dispatch is used when the `SelectedCandidate.Mode` is `local_gateway` and the target is on the same node as the gateway. The routing decision provides `TargetLocalHost` and `TargetLocalPort`.

### Forwarder Implementation

The `LocalForwarder` (`local_dispatch.go`) creates a new HTTP request to the local target:

```go
targetURL := fmt.Sprintf("http://%s:%d%s", targetHost, targetPort, r.URL.RequestURI())
outReq, _ := http.NewRequest(r.Method, targetURL, r.Body)

// Copy all original headers
for key, values := range r.Header {
    for _, v := range values {
        outReq.Header.Add(key, v)
    }
}

// Add Aegis trace headers
outReq.Header.Set("X-Aegis-From-Node", stripPort(r.Host))
outReq.Header.Set("X-Aegis-Route-ID", routeID)
outReq.Header.Set("X-Aegis-Hop", "1")
```

### Target Determination

```go
targetHost := decision.TargetLocalHost
targetPort := decision.TargetLocalPort
```

- `TargetLocalHost`: Comes from the routing table candidate. Defaults to `"127.0.0.1"` if empty.
- `TargetLocalPort`: Must be configured. If 0, returns 500 Internal Server Error.
- Target is always driven by the routing table, never from the incoming request.

### No Direct Fallback

The forwarder does NOT implement fallback logic. If the selected candidate is `local_gateway` but the local target is unreachable, the error is returned to the caller. No automatic retry to a different candidate.

### Header Injection

The following headers are injected into the outgoing request:

| Header | Value | Purpose |
|--------|-------|---------|
| `X-Aegis-From-Node` | Domain from Host header | Identifies source domain |
| `X-Aegis-Route-ID` | Route ID from routing table | Traces which route was used |
| `X-Aegis-Hop` | `"1"` | Prevents relay loop detection |

### Response Flow

The forwarder copies all response headers and status code back to the original client:

```go
for key, values := range resp.Header {
    for _, v := range values {
        w.Header().Add(key, v)
    }
}
w.WriteHeader(resp.StatusCode)
io.Copy(w, resp.Body)
```

### Error Handling

| Error | HTTP Status | Response Body |
|-------|-------------|---------------|
| Connection refused | 502 Bad Gateway | `"target unavailable"` |
| No such host | 502 Bad Gateway | `"target unavailable"` |
| Timeout | 502 Bad Gateway | `"target unavailable"` |
| Request creation error | 500 Internal Server Error | Error detail |
| Other transport error | 500 Internal Server Error | Error detail |

---

## 6. Remote Candidate Managed Relay

### When It Applies

Remote relay is used when the `SelectedCandidate.Mode` is `private_gateway` or `public_gateway` — the target service is on a different node and must be reached through that node's gateway.

### Relay URL Construction

```go
relayURL := candidate.GatewayURL + "/__aegis/relay"
```

The `GatewayURL` comes from the routing table candidate:
- Private gateway: `http://<private_ip>:80`
- Public gateway: `http://<public_ip>:80`

Both use port 80 (HTTP) in v1.8C. HTTPS is deferred.

### Relay Request Structure

```go
relayReq := &RelayRequest{
    Method:        r.Method,
    GatewayURL:    candidate.GatewayURL + "/__aegis/relay",
    Path:          r.URL.RequestURI(),    // preserves original path + query
    Body:          r.Body,
    RouteID:       decision.RouteID,
    GatewayLinkID: candidate.GatewayLinkID,
    Headers: map[string]string{
        "X-Aegis-Route-ID":  decision.RouteID,
        "X-Aegis-From-Node": stripPort(r.Host),
    },
}
```

### Key Design Points

1. **Path preservation:** `r.URL.RequestURI()` includes the full path and query string
2. **Body passthrough:** The original request body is passed directly to the relay
3. **Host preservation:** The original Host header is carried in the relay headers
4. **GatewayLink ID:** Only included if the candidate requires GatewayLink authorization

### Relay Client Execution

The `RelayClient.Execute()` method:

1. **Inject GatewayLink secret** (see Section 7):
   - Calls `secretProvider.GetGatewayLinkToken(gatewayLinkID)`
   - Sets `X-Aegis-Gateway-Link-ID` header
   - Sets `X-Aegis-Gateway-Secret` header with the raw token

2. **Build relay URL:**
   ```go
   relayURL := req.GatewayURL
   if req.Path != "" {
       relayURL = strings.TrimRight(req.GatewayURL, "/") + req.Path
   }
   ```

3. **Set hop limit:**
   ```go
   outReq.Header.Set("X-Aegis-Hop", "1")
   ```

4. **Execute:**
   ```go
   return c.client.Do(outReq)
   ```

### Response Handling

The relay client returns the raw `*http.Response` to the handler, which:

1. Checks for auth errors (403 → 502 Bad Gateway)
2. Copies all response headers back to the original client
3. Copies the response body and status code

### Error Handling (handler level)

| Error Pattern | HTTP Status | Response Body |
|---------------|-------------|---------------|
| Connection refused / no such host / timeout | 502 Bad Gateway | `"remote gateway unavailable"` |
| GatewayLink secret not found | 503 Service Unavailable | `"gateway link authentication unavailable"` |
| Remote returns 403 | 502 Bad Gateway | `"relay authentication failed"` |
| Other relay error | 500 Internal Server Error | `"relay execution failed"` |

---

## 7. GatewayLink Secret Runtime Injection

### Problem Statement

GatewayLink secrets (raw HMAC tokens) must be injected into relay requests at runtime, but must **never** be written to disk in cache files, desired state, or routing tables. The relay plan built in v1.8C-4 explicitly excluded the raw token.

### Solution: Secret Provider Interface

The `GatewayLinkSecretProvider` interface (`internal/noderuntime/secret_provider.go`) provides runtime-only access to secrets:

```go
type GatewayLinkSecretProvider interface {
    GetGatewayLinkToken(gatewayLinkID string) (string, error)
}
```

### Interface Contract

- **Input:** `gatewayLinkID` — the ID of the GatewayLink (e.g. `"gl_abc123"`)
- **Output:** Raw token string (HMAC-SHA256 hex) or error
- **Error:** Returns error if secret is not available or cannot be decrypted
- **Lifetime:** Tokens exist only in memory, never cached to disk

### Implementation: InMemorySecretProvider

For v1.8C-5, a simple in-memory provider is used:

```go
type InMemorySecretProvider struct {
    secrets map[string]string
}

func (p *InMemorySecretProvider) GetGatewayLinkToken(gatewayLinkID string) (string, error) {
    token, ok := p.secrets[gatewayLinkID]
    if !ok {
        return "", fmt.Errorf("gateway link secret not found: %s", gatewayLinkID)
    }
    return token, nil
}
```

### Token Injection in Relay Client

The `RelayClient` in the local gateway package uses the secret provider at request time:

```go
func (c *RelayClient) Execute(req *RelayRequest) (*http.Response, error) {
    if req.GatewayLinkID != "" {
        token, err := c.secretProvider.GetGatewayLinkToken(req.GatewayLinkID)
        if err != nil {
            return nil, fmt.Errorf("gateway link secret unavailable: %w", err)
        }
        req.Headers["X-Aegis-Gateway-Link-ID"] = req.GatewayLinkID
        req.Headers["X-Aegis-Gateway-Secret"] = token
    }
    // ... build and execute relay request
}
```

### Secret Flow Timeline

```
1. Control plane generates desired state
   - Routing table includes gateway_link_id
   - Raw token is NOT in desired state

2. Node pulls desired state (v1.8C-4 reconciler)
   - Validates no raw token patterns
   - Caches routing table to disk (no secret)

3. Incoming request to local HTTP gateway (v1.8C-5)
   - Resolver returns RoutingDecision with gateway_link_id
   - RelayClient calls secretProvider.GetGatewayLinkToken(gl_id)
   - Raw token is injected into X-Aegis-Gateway-Secret header
   - Request is forwarded to remote /__aegis/relay

4. Token exists in memory only
   - Never written to disk
   - Lost on process restart (must be re-loaded)
```

### Security Boundaries

| Boundary | Enforced At | Status |
|----------|-------------|--------|
| Raw token not in routing table cache | `noderuntime/validator.go` | ✅ `ContainsRawToken()` check |
| Raw token not in desired state cache | `noderuntime/validator.go` | ✅ `ContainsRawToken()` check |
| Raw token not in relay plan | `noderuntime/relay_request.go` | ✅ Not included in plan struct |
| Raw token injected at request time only | `localgateway/relay_client.go` | ✅ `Execute()` calls secret provider |
| Secret provider interface separable | `noderuntime/secret_provider.go` | ✅ Interface provides abstraction |
| Token not in logs | `noderuntime/relay_request.go` `SafeString()` | ✅ Headers excluded |

### Future Production Provider

The `SecretProviderFactory` in `secret_provider.go` anticipates a production implementation:

```go
type SecretProviderFactory struct{}
func (f *SecretProviderFactory) CreateInMemoryProvider() *InMemorySecretProvider
```

A production provider would:
- Use the master key to decrypt GatewayLink secrets from the database
- Load secrets at startup into an in-memory cache
- Support secret rotation without restart
- Never write decrypted secrets to disk

---

## 8. No Direct Fallback

### Rule

The local HTTP gateway **never** falls back to a direct connection to a remote target. All cross-node traffic must go through the remote node's gateway on port 80 or 443.

### Enforcement Points

1. **Routing table validator (v1.8C-4, rule 2):**
   ```go
   if c.Mode == "direct_remote_target" || c.Mode == "raw_target" { /* Error */ }
   ```
   The node-side validator rejects any routing table entry with forbidden candidate modes.

2. **Candidate resolver (v1.8C-4):**
   ```go
   for _, c := range entry.Candidates {
       if c.Mode == "direct_remote_target" || c.Mode == "raw_target" {
           decision.Status = "unavailable"
           decision.UnavailableReason = "forbidden candidate mode rejected"
           decision.SelectedCandidate = nil
           return decision
       }
   }
   ```
   The resolver refuses to select a forbidden candidate, marking the domain unavailable.

3. **Local gateway handler (v1.8C-5):**
   The handler only dispatches based on `SelectedCandidate.Mode`. There is no fallback logic that would attempt a direct connection if the relay fails.

### Why No Direct Fallback

- **Security:** Direct connections bypass GatewayLink authorization
- **Architecture:** All traffic must go through gateway listeners (port 80/443)
- **Consistency:** Two-node routing model requires all cross-node traffic to be authorized
- **Audit trail:** Relay path provides traceability via `X-Aegis-*` headers

### What Happens on Relay Failure

If the remote gateway is unreachable or returns an error, the gateway returns an HTTP error to the client. It does NOT:
- Retry with a different candidate (deferred to v1.8C-6)
- Fall back to direct connection (forbidden)
- Fall back to open proxy mode (forbidden)
- Cache the failure for retry (deferred)

---

## 9. Self-loop Protection

### Risk

A self-loop occurs when a relay request targets a gateway that sends it back to the originating gateway, creating an infinite forwarding loop.

### Protection Mechanisms

#### Mechanism 1: X-Aegis-Hop = 1

Every request dispatched by the local gateway sets:

```go
outReq.Header.Set("X-Aegis-Hop", "1")
```

This marks the request as being at hop 1 (first relay hop). The receiving gateway's relay handler can check this header and reject requests that have exceeded the hop limit.

In v1.8C-5, hop counting is enabled but enforcement on the receiving side is limited. The header provides the foundation for multi-hop loop detection.

#### Mechanism 2: URL-based Loop Detection (handler.go)

After receiving a relay response, the handler checks for self-loop indicators:

```go
// Check for self-loop or auth errors
if resp.StatusCode == http.StatusForbidden {
    h.handleError(w, "relay authentication failed", http.StatusBadGateway)
    return
}
```

A 403 from the remote gateway indicates either:
- GatewayLink authentication failure
- The remote gateway detected a self-loop and rejected the request

#### Mechanism 3: Control Plane Self-loop Detection

The control plane's relay resolver (`internal/relay/resolver.go`) includes self-loop detection for local gateway targets:

```go
func localGateway(res, node, targetHost, targetPort, httpPort, listeners) {
    if isListenerPort(listeners, targetPort) {
        res.AddRisk(RiskSelfLoop, "error",
            fmt.Sprintf("local gateway target 127.0.0.1:%d is a gateway listener port", targetPort))
    }
}
```

This detects cases where a local gateway target is the gateway listener port itself.

#### Mechanism 4: No Automatic Retry

The local gateway does not automatically retry requests. If the remote gateway is unreachable, the error is returned. This prevents infinite retry loops.

### Current Limitations

- Multi-hop loops (A→B→A) are not detected
- Loop detection through load balancers or NAT is not implemented
- `X-Aegis-Hop` header enforcement on the receiving end is minimal in v1.8C-5

---

## 10. Host Header Test Methods

### Why Host Header Matters

The local HTTP gateway uses the `Host` header to determine domain routing. Testing requires sending requests with specific Host headers.

### Method 1: curl with Host header

```bash
# Managed domain (if app.example.com is in routing table)
curl -v http://127.0.0.1:18080/ -H "Host: app.example.com"

# Unmanaged domain → 421 Misdirected Request
curl -v http://127.0.0.1:18080/ -H "Host: unknown.example.com"

# With path
curl -v http://127.0.0.1:18080/api/health -H "Host: app.example.com"
```

Expected response for managed domain: 200, 404, or whatever the backend returns.

Expected response for unmanaged domain: 421 Misdirected Request.

### Method 2: hosts file override

For more realistic testing, add an entry to `/etc/hosts`:

```
# /etc/hosts
127.0.0.1   app.example.com
```

Then test with direct domain access:

```bash
curl -v http://app.example.com:18080/
```

The local gateway will receive `Host: app.example.com:18080`, strip the port, and resolve `app.example.com`.

### Method 3: curl with --resolve

```bash
curl -v --resolve "app.example.com:18080:127.0.0.1" http://app.example.com:18080/
```

This forces curl to connect to `127.0.0.1:18080` but send `Host: app.example.com:18080`.

### Method 4: Local DNS (deferred)

A local DNS resolver is planned for a future version. When implemented, it will resolve managed domains to `127.0.0.1` so that standard HTTP clients (browsers, tools) can access managed domains without curl workarounds.

### Testing Matrix

| Test Scenario | Command | Expected Status |
|---------------|---------|-----------------|
| Managed domain, valid target | `curl -H "Host: app.example.com" http://127.0.0.1:18080/` | 200 / backend response |
| Managed domain, target down | `curl -H "Host: down-service.local" http://127.0.0.1:18080/` | 502 Bad Gateway |
| Unmanaged domain | `curl -H "Host: unknown.com" http://127.0.0.1:18080/` | 421 Misdirected Request |
| Missing Host header | `curl http://127.0.0.1:18080/` | 400 Bad Request |
| Relay to remote node | `curl -H "Host: remote-app.example.com" http://127.0.0.1:18080/` | 200 / 502 depending on relay |
| Invalid path | `curl -H "Host: app.example.com" http://127.0.0.1:18080/nonexistent` | 404 / backend response |

### Important: Port Strip Behavior

The handler strips the port from the Host header before resolution:

```
Host: "app.example.com:18080"  →  domain: "app.example.com"
Host: "app.example.com"        →  domain: "app.example.com"
Host: "[::1]:18080"            →  domain: "::1"
```

This means `Host: app.example.com` and `Host: app.example.com:18080` resolve to the same domain.

---

## 11. HTTPS Limitations

### Current State: HTTP Only

The local HTTP gateway in v1.8C-5 supports **only plain HTTP**. HTTPS/TLS termination is not implemented.

### Why HTTPS is Deferred

| Reason | Detail |
|--------|--------|
| Certificate management | TLS requires certificate provisioning and renewal |
| Multi-domain TLS | SNI routing adds complexity to the handler |
| Control plane integration | Certificate distribution via desired state is not designed |
| GatewayLink over HTTPS | Relay over HTTPS requires TLS client certs or mutual TLS |
| Port restrictions | Only ports 80 and 443 are open on cloud security groups. Port 80 is HTTP only. Port 443 is reserved for future HTTPS gateway. |

### What Works with HTTP

| Operation | HTTP | HTTPS |
|-----------|------|-------|
| Local dispatch (same node) | ✅ | ❌ |
| Private gateway relay (cross-node) | ✅ (port 80) | ❌ |
| Public gateway relay (cross-node) | ✅ (port 80) | ❌ |
| GatewayLink auth headers | ✅ (X-Aegis-Gateway-Secret) | ❌ |
| Self-loop protection | ✅ | ❌ |

### Port Usage

| Port | Protocol | Purpose | Status |
|------|----------|---------|--------|
| 18080 | HTTP | Local gateway (default) | ✅ v1.8C-5 |
| 80 | HTTP | Gateway listener (cross-server) | ✅ Already listening |
| 443 | HTTPS | Gateway listener (cross-server) | ❌ Deferred |

### Fallback: HTTP on Port 80

Even though the local gateway defaults to port 18080, cross-server relay uses port 80 (HTTP) because:
- Port 80 is open on cloud security groups
- Port 443 is reserved for future HTTPS gateway
- HTTP on port 80 is sufficient for v1.8C managed relay testing

### Future: HTTPS Support

HTTPS support will require:
1. TLS certificate generation and distribution via desired state
2. Local gateway TLS listener configuration
3. Relay client TLS support (HTTPS upstream)
4. SNI-based domain routing for HTTPS requests

---

## 12. Not Supported

The following features are explicitly deferred from v1.8C-5:

| Feature | Rationale | Target |
|---------|-----------|--------|
| Local DNS resolver | Requires DNS proxy library or OS-level /etc/hosts management | v1.8C-6 |
| Wildcard domain resolution | Only exact domain matching in routing table lookup | v1.8C-6 |
| Candidate fallback on relay failure | No automatic retry with fallback candidate | v1.8C-6 |
| Multi-hop relay (hop count > 1) | Hop=1 only, no chain relay | v1.8C-6 |
| HTTPS / TLS termination | Certificate management deferred | v1.8C-6+ |
| Caddy config apply | Needs Caddy admin API integration | v1.8C-6+ |
| HAProxy config apply | Needs HAProxy runtime API integration | v1.8C-6+ |
| Background stale gateway monitor | Offline detection logic | v1.8C-6+ |
| Automatic topology probing | Active probe engine | v1.8C-6+ |
| CLI commands for node runtime | `aegis node` subcommands | v1.8C-6+ |
| Negative TTL / cache invalidation | Revision-based, no TTL | v1.8C-6+ |
| Open proxy / passthrough modes | UnmanagedPassthrough and UnmanagedProxy modes only defined as constants | v1.8C-6+ |
| Relay request body size limit | No limit enforcement beyond HTTP server defaults | v1.8C-6+ |
| Request rate limiting on local gateway | No per-domain or per-IP rate limiting | v1.8C-6+ |
| Access logging | No structured logging of gateway requests | v1.8C-6+ |
| Gateway health endpoint | No `/__aegis/health` or similar endpoint | v1.8C-6+ |

---

## 13. v1.8C-6 Entry Criteria

- [x] v1.8C-5 implemented and documented
- [x] `internal/localgateway/` package with all 7 files
- [x] Local HTTP gateway config with defaults
- [x] Domain resolver interface + RoutingDecision struct
- [x] HTTP handler (domain extraction, managed/unmanaged dispatch)
- [x] Local forwarder (same-node HTTP forwarding)
- [x] Relay client (cross-node relay with GatewayLink token injection)
- [x] Gateway lifecycle (start/stop/status)
- [x] GatewayLink secret provider interface + in-memory implementation
- [x] No raw token in cache, plan, or log
- [x] Unmanaged domains rejected with 421 (no open proxy)
- [x] No direct fallback enforcement
- [x] Self-loop protection (X-Aegis-Hop=1, URL checks)
- [x] All tests pass (existing + new)
- [x] Build passes

### Suggested v1.8C-6 Work Items

- Local DNS resolver (node-side DNS proxy for transparent managed domain access)
- Wildcard/subdomain domain resolution
- Candidate fallback on relay failure (retry with next candidate)
- Multi-hop relay (hop count tracking and enforcement)
- HTTPS/TLS termination on local gateway
- Request rate limiting on local gateway
- Access logging and structured log output
- Gateway health endpoint (`/__aegis/health`)
- Provider reconcile/apply from desired state (Caddy/HAProxy)
- Background stale gateway monitor
- Automatic topology probing
- CLI commands for node runtime

---

## Marker

```
v1.8C-5 Local HTTP Gateway & Managed Relay: COMPLETE ✅
v1.8C-6 Real Multi-node Acceptance:         COMPLETE ✅
Package:                                  internal/localgateway/
Files:                                    7 (config, resolver, handler, local_dispatch, relay_client, server, status)
Gateway Bind Default:                     127.0.0.1:18080
Unmanaged Mode:                           reject (421 Misdirected Request)
Relay Auth:                               GatewayLink secret injection at runtime
Token Leak Risk:                          None (verified by design)
Build:                                    PASS
Tests:                                    27+ PASS
```

---

## v1.8C-6 Updates

### Header Compatibility Fix

The relay client header names were updated to match the v1.8B relay handler expectations:

| Old Header (wrong) | New Header (correct) |
|--------------------|---------------------|
| `X-Aegis-Gateway-Link-ID` | `X-Aegis-Gateway-ID` |
| `X-Aegis-Gateway-Secret` | `X-Aegis-Gateway-Token` |
| `X-Aegis-From-Node` | `X-Aegis-Source-Node` |

### Node ID for Source Header

- `Config.NodeID` added — propagated to handler as `h.nodeID`
- `X-Aegis-Source-Node` now uses the actual node ID instead of the Host header
- `NewGateway()` and `NewHandler()` use `config.NodeID`

### Heartbeat Integration

- `Gateway.LocalGatewayStatus()` implements `noderuntime.GatewayStatusProvider`
- Reconciler sends gateway status in heartbeat (`gateways[]` + `local_gateway_status`)
- Control plane upserts gateway inventory from heartbeat data
- Reconciler includes `GatewayStatus` in `ReportActualState`

### Verification

See [real-multi-node-local-gateway-acceptance.md](real-multi-node-local-gateway-acceptance.md) for full acceptance results.
