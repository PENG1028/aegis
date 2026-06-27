# v1.8C-7 — Developer Entry + Daemon Runbook

> **Phase:** v1.8C — Multi-node Aegis Runtime
> **Status:** IMPLEMENTED (dev_entry_verified) ✅
> **Date:** 2026-06-27
> **Type:** Developer Workflow Documentation

---

## 1. v1.8C-7 Scope

This phase closes the developer entry experience for the local HTTP gateway.
It adds:

- Three developer access modes (Host header, hosts file + dev port, port 80)
- Local gateway health/status API endpoints (`/__aegis/local/health`, `/__aegis/local/status`)
- Startup diagnostics (pre-flight checks before gateway binding)
- Node configuration standardization (`node.yaml`)
- Systemd service blueprints
- Developer acceptance script
- Token leak safety verification

**What is NOT included (deferred):**
- System-wide DNS hijack
- HTTPS full transparency
- Root CA installation
- Raw TCP, CONNECT, WebSocket tunnels
- iptables / eBPF / service mesh / UI
- RelayHandler semantics changes
- Direct remote fallback

---

## 2. Three Developer Entry Modes

### Mode A: Host Header (recommended for development)

No system files modified. Works immediately on any OS.

```bash
# Start the local gateway (from code):
go run ./cmd/aegis/ node run --config ./node.yaml

# OR via the simulated acceptance setup:
# (This starts the gateway on port 18080)

# Access managed domains:
curl -H "Host: api-b.example.com" http://127.0.0.1:18080/health
```

**Pros:**
- No system configuration changes
- Works on Windows, macOS, Linux equally
- Multiple domains without hosts file

**Cons:**
- Some tools don't support custom Host headers (browsers)
- Not suitable for GUI tools

### Mode B: Hosts File + Dev Port

Add to `/etc/hosts` (Linux/macOS) or `%SystemRoot%\System32\drivers\etc\hosts` (Windows):

```
127.0.0.1 api-b.example.com
127.0.0.1 api-c.example.com
127.0.0.1 local-a.example.com
```

Then access:

```bash
# Port must be explicit — hosts cannot encode ports
curl http://api-b.example.com:18080/health
```

**Pros:**
- Works with tools that resolve DNS normally
- Closer to production URL (same hostname)

**Cons:**
- Port must be explicit in URL
- Requires admin/root to edit hosts file
- Each domain needs a separate entry

### Mode C: Hosts File + Port 80

Same hosts entries as Mode B, but bind the gateway to port 80:

```yaml
# node.yaml
local_gateway:
  port: 80
```

```bash
# Requires root/admin to bind port 80
sudo -E ./aegis node run --config ./node.yaml

# Now access without port:
curl http://api-b.example.com/health
```

**Risk:**
- Port 80 binding requires `sudo` or root
- Port 80 may conflict with existing HTTP servers (Apache, Nginx, Caddy)
- On Linux, `CAP_NET_BIND_SERVICE` can grant bind capability without full root
- On Windows, port 80 may be reserved by another service
- **Never expose port 80 to the LAN** — the gateway rejects unmanaged domains
  with 421, but defense-in-depth still applies

**Recommended platform:**
```bash
# Linux: grant net_bind_service capability without root
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/aegis
```

---

## 3. Node Configuration (`node.yaml`)

The standard node configuration file:

```yaml
# /etc/aegis/node.yaml — Aegis Node Configuration
# Dev path:    ./node.yaml or ~/.aegis/config.yaml
# Production:  /etc/aegis/node.yaml

control_plane_url: "http://127.0.0.1:9000"
node_id: "node-a"
node_token_file: "/etc/aegis/node.token"
cache_dir: "/var/lib/aegis"
runtime_dir: "/run/aegis"

local_gateway:
  enabled: true
  bind_addr: "127.0.0.1"
  port: 18080
  unmanaged_mode: "reject"
  preserve_host: true
  request_timeout_seconds: 30

sync:
  heartbeat_interval_seconds: 15
  sync_interval_seconds: 15
  reconcile_mode: "dry_run"
```

**Dev path vs Production path:**

| Aspect | Dev Path | Production Path |
|--------|----------|----------------|
| Config location | `./node.yaml` or `~/.aegis/config.yaml` | `/etc/aegis/node.yaml` |
| Token file | `./node.token` | `/etc/aegis/node.token` |
| Cache dir | `./.aegis/cache/` | `/var/lib/aegis/` |
| Runtime dir | `./.aegis/run/` | `/run/aegis/` |
| Port | 18080 (no root) | 443 (requires setup) |
| Sync mode | `dry_run` | `live` |

---

## 4. Token File Security

The node token file (`node_token_file`) is the credential that authenticates
the node to the control plane. Treat it like an SSH private key.

**Requirements:**
- Permissions: `0600` (owner read/write only)
- Location: outside web root and version control
- Format: single line containing the hex-encoded node token

```bash
# Generate/save node token
mkdir -p /etc/aegis
echo "abcdef0123456789..." > /etc/aegis/node.token
chmod 0600 /etc/aegis/node.token
```

**Startup diagnostics check:**
- If `token_file` is not configured → warning (node may not connect)
- If `token_file` does not exist → failed (node cannot authenticate)
- If permissions are too open → warning (security risk)

**Security properties:**
- Raw token is never returned by health/status endpoints
- Diagnostics output includes only the file path, not the file contents
- Token is loaded into memory only (never cached to disk as plaintext)

---

## 5. Cache Directory

The cache directory stores:
- `routing_table.json` — cached routing table entries
- `desired_state.json` — last desired state from control plane
- `actual_state.json` — last reported actual state

```yaml
cache_dir: "/var/lib/aegis"
```

**Startup diagnostics check:**
- If not configured → warning (caching disabled)
- If parent doesn't exist → failed
- If not writable → failed

**Security:**
- Cache directory does NOT store raw tokens or GatewayLink secrets
- Secrets are held in `InMemorySecretProvider` (RAM only)
- On restart, secrets must be re-fetched from control plane API

---

## 6. Local Gateway Health / Status

### `GET /__aegis/local/health`

Returns a simple health check:

```json
{
  "status": "ok",
  "service": "aegis-local-gateway"
}
```

**Security:**
- Never returns tokens, secrets, or routing data
- Always returns 200 when gateway is accepting connections
- Does not reflect routing table status (use `/__aegis/local/status`)

### `GET /__aegis/local/status`

Returns gateway operational status:

```json
{
  "node_id": "node-a",
  "local_gateway": {
    "enabled": true,
    "bind_addr": "127.0.0.1",
    "port": 18080,
    "status": "online",
    "last_error": "",
    "updated_at": "2026-06-27T12:00:00Z"
  },
  "routing_table": {
    "loaded": true,
    "entries": 3,
    "revision": 4
  },
  "cache": {
    "desired_state": true,
    "routing_table": true
  }
}
```

**Security guarantees:**
- Never returns raw tokens
- Never returns GatewayLink secret values
- Never returns full internal routing table (only metadata)
- All `X-Aegis-*` headers are stripped before processing

**Implementation:**

The `local_gateway` section comes from `GatewayStatusInfo`:

| Field | Source | Example |
|-------|--------|---------|
| `enabled` | Config | `true` |
| `bind_addr` | Config | `"127.0.0.1"` |
| `port` | Config | `18080` |
| `status` | Runtime | `"online"`, `"starting"`, `"degraded"`, `"failed"` |
| `last_error` | Runtime | `""` or error description |
| `updated_at` | Runtime | RFC3339 timestamp |

The `routing_table` section comes from `RoutingTableStatusProvider`
(a pluggable interface). If no provider is set, `loaded` is `false`.

The `cache` section is derived from routing table status:
- `desired_state` = routing_table.loaded
- `routing_table` = routing_table.loaded

---

## 7. Startup Diagnostics

Performed at gateway startup. Checks:

| # | Check | Condition | Level |
|---|-------|-----------|-------|
| 1 | `node_id` | Non-empty | failed if empty |
| 2 | `token_file` | Exists, not directory, permissions 0600 | failed/warning |
| 3 | `cache_dir` | Exists (or parent exists), writable | failed/warning |
| 4 | `routing_table` | Loaded and resolvable | warning if not loaded |
| 5 | `bind_port` | TCP port bind test | failed if in use |
| 6 | `secret_provider` | Configured | warning if not configured |
| 7 | `control_plane` | URL configured | warning if not configured |

### Using diagnostics programmatically:

```go
import "aegis/internal/localgateway"

result := localgateway.RunStartupDiagnostics(params)
if result.HasFailed {
    fmt.Printf("Startup checks failed: %d checks failed\n", len(result.Checks))
}
// result.SafeString() returns safe log output (no token leak)
fmt.Println(result.SafeString())
```

### Diagnostic levels:
- **ok** — check passed
- **warning** — non-fatal issue (gateway may still function)
- **failed** — fatal issue (gateway will not start correctly)

### Security:
- Error messages never contain raw token contents
- Path output is limited to the configured path (safe)
- Use `SafeString()` for log output

---

## 8. Systemd Runbook

**Status:** Systemd service files provided as blueprints. CLI entrypoint
`aegis local-gateway run` and `aegis node run` must exist for full
systemd integration. If these subcommands are not yet implemented,
the service files serve as documentation for when the entrypoint is added.

### Service Files

Two service files are provided in `packaging/systemd/`:

| File | Purpose |
|------|---------|
| `aegis-node.service` | Full node runtime with local gateway |
| `aegis-local-gateway.service` | Standalone local gateway |

### Installation

```bash
# Install service file
sudo cp packaging/systemd/aegis-node.service /etc/systemd/system/
sudo systemctl daemon-reload

# Enable on boot
sudo systemctl enable aegis-node

# Start
sudo systemctl start aegis-node

# Check status
sudo systemctl status aegis-node

# View logs
journalctl -u aegis-node -f
```

### Environment File

Optional environment file at `/etc/aegis/aegis-node.conf`:

```bash
# Aegis node configuration overrides
AEGIS_CACHE_DIR=/var/lib/aegis
AEGIS_RUNTIME_DIR=/run/aegis
AEGIS_NODE_TOKEN_FILE=/etc/aegis/node.token
```

### Pre-requisites for systemd

Before running via systemd, ensure:

1. **Aegis binary** at `/usr/local/bin/aegis`
2. **Node token** at `/etc/aegis/node.token` (permissions 0600)
3. **Cache directory** at `/var/lib/aegis` (writable by `aegis` user)
4. **Runtime directory** at `/run/aegis` (writable by `aegis` user)
5. **Config file** at `/etc/aegis/node.yaml`
6. **User** `aegis` exists (`sudo useradd -r -s /bin/false aegis`)

### Systemd Configuration Note

The provided service files assume a CLI entrypoint exists for running
the local gateway as a daemon. If the binary does not yet support
`aegis local-gateway run` or `aegis node run`, use one of these alternatives:

- `go run ./cmd/aegis/ ...` (development)
- Direct binary invocation with custom flags
- Wrapper script in `/usr/local/bin/aegis-node-wrapper`

---

## 9. Hosts File Workflow

### Adding Domains

Edit your hosts file:

```bash
# Linux/macOS:
sudo nano /etc/hosts

# Windows (as Administrator):
notepad C:\Windows\System32\drivers\etc\hosts
```

Add entries:

```
127.0.0.1 api-b.example.com
127.0.0.1 api-c.example.com
127.0.0.1 local-a.example.com
```

### Important Limitations

| Limitation | Impact | Workaround |
|-----------|--------|------------|
| Hosts cannot encode ports | Must use `:18080` in URL | Mode A (Host header) |
| Port 80 requires root | Cannot omit port without sudo | `sudo` or capability-based setup |
| No wildcard support | Each domain needs an entry | Script hosts generation |
| HTTPS not supported | Local gateway is HTTP-only | Deferred (v1.8D or later) |

### Security

- Hosts file only affects the local machine
- No system-wide DNS hijack is performed
- Unmanaged domains (not in routing table) are rejected with 421
- The gateway does NOT proxy unmanaged domains (no open proxy vulnerability)

### Automated Script

The acceptance script does NOT modify `/etc/hosts`. If you need hosts
entries, add them manually as shown above.

---

## 10. Dev Port vs Port 80

| Aspect | Dev Port (18080) | Port 80 |
|--------|-----------------|---------|
| Permissions | None required | Root/admin or capabilities |
| Conflicts | Low (uncommon port) | High (HTTP servers) |
| URL format | `http://host:18080/path` | `http://host/path` |
| Host header mode | `curl -H "Host: x" http://127.0.0.1:18080/` | `curl -H "Host: x" http://127.0.0.1/` |
| Security | Default | Needs port 80 lockdown |
| Production fit | Development only | Closer to production |

**Recommendation for development:** Use Mode A (Host header) on port 18080.
No system files are modified, and all functionality works identically.

---

## 11. HTTPS (Deferred)

- Local gateway binds to HTTP only (no TLS)
- HTTPS full transparency is deferred
- No root CA installation is performed
- For encrypted remote relay, the relay handler on the remote node
  uses `X-Aegis-*` headers for authentication (not TLS)
- Relay transport between gateways is HTTP-based (TLS can be added
  by running behind a reverse proxy like Caddy)

---

## 12. Not Supported

| Feature | Status | Reason |
|---------|--------|--------|
| System-wide DNS hijack | ❌ Deferred | Violates transparency principle |
| Root CA installation | ❌ Deferred | HTTPS transparency not ready |
| HTTPS full transparency | ❌ Deferred | Depends on DNS + CA infra |
| Raw TCP / CONNECT | ❌ Deferred | v1.8C is HTTP-only |
| WebSocket tunnel | ❌ Deferred | Not in v1.8 scope |
| UDP / iptables / eBPF | ❌ Deferred | Beyond v1.8 scope |
| Service mesh | ❌ Deferred | Out of scope |
| UI | ❌ Deferred | CLI + API only |
| Automatic secret rotation | ❌ Deferred | Manual rotation API exists |
| Direct remote fallback | ❌ Blocked | Always through relay |
| Port 80 without root | ❌ Not possible | OS restriction |

---

## 13. Entering Real Two-node VPS Acceptance

To upgrade from `dev_entry_verified` to `real_two_node_verified`:

**Prerequisites:**

1. Two VPS with port 80/443 open in security group
2. Aegis binary built and deployed on both
3. SSH access configured
4. `docs/v1.8/real-two-node-vps-acceptance-runbook.md` followed

**Transition checklist:**

| # | Item | Status |
|---|------|--------|
| 1 | Control plane running on Node A | pending |
| 2 | Master key configured | pending |
| 3 | Node A registered | pending |
| 4 | Node B registered | pending |
| 5 | GatewayLink created (encrypted) | pending |
| 6 | Service/route/endpoint configured | pending |
| 7 | Routing table generated | pending |
| 8 | Node A local gateway started | pending |
| 9 | Node B relay handler started | pending |
| 10 | curl verification (12 tests) | pending |

**After VPS acceptance:**
- Update verification label to `real_two_node_verified`
- Update all documentation with real curl output
- Capture log evidence showing no token leaks

---

## Appendix: Test Coverage

| Test | What it Verifies | File |
|------|-----------------|------|
| TestLocalHealthEndpoint | Health returns 200 with status=ok | gateway_v18c7_test.go |
| TestLocalStatusEndpoint | Status returns valid JSON with all fields | gateway_v18c7_test.go |
| TestLocalStatusWithoutRTProvider | Status works without routing table | gateway_v18c7_test.go |
| TestLocalStatusNoTokenLeak | Status response has no raw tokens | gateway_v18c7_test.go |
| TestLocalHealthOnManagedDomainStillWorks | Managed domain routing not broken | gateway_v18c7_test.go |
| TestLocalHealthUnmanagedDomainRejected | Unmanaged still rejected (421) | gateway_v18c7_test.go |
| TestLocalHealthLowercasePath | Lowercase path works | gateway_v18c7_test.go |
| TestLocalHealthMixedCasePath | Mixed case path works | gateway_v18c7_test.go |
| TestStartupDiagnosticsAllOK | All checks pass with valid config | gateway_v18c7_test.go |
| TestStartupDiagnosticsMissingNodeID | node_id empty → failed | gateway_v18c7_test.go |
| TestStartupDiagnosticsMissingTokenFile | Token file not found → failed | gateway_v18c7_test.go |
| TestStartupDiagnosticsCacheDirNotWritable | Cache not writable → failed | gateway_v18c7_test.go |
| TestStartupDiagnosticsPortBindFailure | Port conflict → failed | gateway_v18c7_test.go |
| TestStartupDiagnosticsSafeString | SafeString no token leak | gateway_v18c7_test.go |
| TestStartupDiagnosticsTokenFileNotLeaked | Error msg no token content | gateway_v18c7_test.go |
| TestStartupDiagnosticsAllWarnings | Minimal config → warnings | gateway_v18c7_test.go |

## Changelog

| Date | Change | Author |
|------|--------|--------|
| 2026-06-27 | Initial developer entry documentation | Aegis Dev |
