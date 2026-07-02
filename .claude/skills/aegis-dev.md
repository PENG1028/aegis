---
name: aegis-dev
description: Develop Aegis — add features, fix bugs, run tests, understand the codebase
model: sonnet
---

# Aegis Development Skill

Use this skill when developing Aegis — writing code, fixing bugs, adding features, running tests.

## Quick Start (Local Dev)

```bash
cd "F:/Work Document/project/aegis"

# Build and run locally
make build
./aegis bootstrap
./aegis serve    # API at 127.0.0.1:7380

# Frontend dev server (separate terminal)
make dev-ui      # Vite HMR at localhost:5173

# All tests
make test

# Specific package
go test ./internal/apply/... -v -count=1
go test ./internal/tcp/... -v -count=1

# Quality
make vet && make lint
```

## Architecture Rules (MUST FOLLOW)

1. **Aegis is NOT in the data path** — Caddy/HAProxy serve traffic independently
2. **All admin mutation endpoints MUST call `MarkPending()`**
3. **Service API keys CANNOT access `/api/admin/v1/*`** — blocked by `isSystemRoute()`
4. **Gateway Link is metadata on a route** — NOT a route source of truth
5. **Only ports 80 and 443 open on cloud security group** — never test with other ports cross-server
6. **Content-Type enforcement** — all POST/PATCH/PUT must send `application/json`

## Adding a New Feature

### Backend (Go)

1. **Model** → `internal/<domain>/` package
2. **Repository** → DB operations following `internal/store/` patterns
3. **Service** → Business logic
4. **Handler** → `internal/httpapi/handlers/`
5. **Route** → Register in `internal/httpapi/routes.go`
6. **Service field** → Add to `internal/httpapi/server.go` Services struct (if new dependency)
7. **Assembly** → Wire up in `cmd/aegis/main.go`

### Frontend (React + TypeScript)

1. **Type** → `ui/src/types/` or inline
2. **API Client** → `ui/src/lib/real-api-client.ts`
3. **API Bridge (mock)** → `ui/src/lib/api-bridge.ts`
4. **Page** → `ui/src/pages/`
5. **Route** → `ui/src/App.tsx`
6. **Sidebar** → `ui/src/components/layout/Sidebar.tsx`

## Key Files

| File | What |
|------|------|
| `cmd/aegis/main.go` | Entry point, all dependency wiring |
| `internal/httpapi/routes.go` | All 130+ route registrations |
| `internal/httpapi/server.go` | Services struct |
| `internal/httpapi/handlers/*.go` | Request handlers |
| `internal/config/config.go` | YAML config loading |
| `internal/store/` | SQLite + migrations |
| `internal/apply/` | Config apply pipeline |
| `internal/proxy/caddy/` | Caddyfile renderer |
| `internal/endpoint/resolver.go` | Endpoint resolution (local→private→public) |
| `internal/gateway_link/` | Cross-machine HMAC auth |
| `internal/nodeauth/` | Node registration + heartbeat auth |
| `internal/credential/` | AES-256-GCM encrypted credentials |
| `internal/addr/addr.go` | Unified address type (TCP/UDP/unix) |
| `internal/tcp/proxy.go` | TCP port forwarding |
| `internal/udp/proxy.go` | UDP port forwarding (with Unix socket support) |
| `internal/recovery/recovery.go` | Panic recovery + stack trace |
| `internal/validate/validate.go` | Input validation helpers |
| `internal/testutil/testutil.go` | Test DB setup |
| `ui/src/lib/real-api-client.ts` | Frontend API client |
| `ui/src/lib/api-bridge.ts` | Mock/real API switch |

## Test Patterns

```go
func TestMyFeature(t *testing.T) {
    db := testutil.SetupTestDB(t)
    repo := mypkg.NewRepository(db)
    // ...
}

func TestMyFeature_Integration(t *testing.T) {
    db, path := testutil.SetupTestDBFile(t)
    defer os.Remove(path)
    // ...
}
```

## Before Committing

```bash
make test                    # All tests
go build ./...               # Build
go vet ./...                 # Vet
cd ui && npx tsc --noEmit   # Frontend type check
git commit -m "v1.8L: description"
```

## Commit Convention

```
v1.8L: 中文或英文描述
v1.8L-2: 小修复
```
Version in `cmd/aegis/main.go`: `var Version = "dev"` (replaced by `make release`).

## Env Vars

| Variable | Purpose | Default |
|----------|---------|---------|
| `LOG_LEVEL` | debug/info/warn/error | info |
| `LOG_FORMAT` | json/text | text |
| `VITE_USE_MOCK` | Frontend mock API | false |
