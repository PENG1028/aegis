# Aegis Console — VPS External Access Deployment

## Architecture

```
Browser ──https──→ Caddy (Server A :443)
                     ├── /api/*     → proxy → localhost:7380 (Go backend)
                     ├── /__aegis/* → proxy → localhost:7380
                     └── /*         → serve SPA (dist/)
```

The Aegis frontend is a **static SPA** built with Vite.  
The backend Go API runs on `127.0.0.1:7380`.  
Caddy handles HTTPS termination and SPA routing.

---

## 1. Production Build

```bash
cd ui/
npm run build          # → dist/
```

Output:
- `dist/index.html` — SPA entry
- `dist/assets/` — JS + CSS bundles

---

## 2. Deploy to VPS (Server A)

```bash
# Copy to server
rsync -avz --delete dist/ ubuntu@<SERVER_A_IP>:/opt/aegis/console/

# On Server A, verify files
ssh ubuntu@<SERVER_A_IP>
ls -la /opt/aegis/console/
# Should see: index.html, assets/
```

---

## 3. Caddy Configuration

Add to `/etc/caddy/Caddyfile` on Server A:

```caddy
aegis.example.com {
    # SPA — serve index.html for all paths (client-side routing)
    root * /opt/aegis/console
    try_files {path} /index.html

    # API proxy — forward to Go backend
    handle /api/* {
        reverse_proxy 127.0.0.1:7380
    }
    handle /__aegis/* {
        reverse_proxy 127.0.0.1:7380
    }

    # Static assets
    handle /assets/* {
        file_server
        header Cache-Control "public, max-age=31536000, immutable"
    }

    header {
        X-Content-Type-Options "nosniff"
        X-Frame-Options "DENY"
        Referrer-Policy "strict-origin-when-cross-origin"
    }
}
```

If deploying at the root domain (no subdomain):

```caddy
<SERVER_A_IP> {
    root * /opt/aegis/console
    try_files {path} /index.html

    handle /api/* {
        reverse_proxy 127.0.0.1:7380
    }
    handle /__aegis/* {
        reverse_proxy 127.0.0.1:7380
    }

    handle /assets/* {
        file_server
        header Cache-Control "public, max-age=31536000, immutable"
    }
}
```

---

## 4. Environment Variables (Production)

The frontend uses `VITE_API_BASE_URL` to locate the backend.

| Variable | Default | Production |
|---|---|---|
| `VITE_API_BASE_URL` | `http://127.0.0.1:7380` | *(omit — same origin via Caddy proxy)* |
| `VITE_USE_MOCK` | `false` | *(omit — real API calls)* |

In production, the SPA is served from the same origin as the API (via Caddy proxy), so the base URL should be empty/relative.

To override at build time:

```bash
VITE_API_BASE_URL=/api npx vite build
```

Or simply omit (defaults to same-origin via empty relative path).

---

## 5. Auth & Cookie Handling

- Login: `POST /api/admin/v1/auth/login` sets `aegis_admin_session` cookie
- Cookie: `HttpOnly`, `SameSite=Strict`, path=`/api/admin/v1`
- SPA credentials: `credentials: 'include'` in fetch calls
- Caddy must NOT strip or rewrite the `Set-Cookie` header from the backend
- All API calls go through the same origin (no CORS needed in production)

---

## 6. CORS in Development

When running `vite dev` on a different port (e.g., `localhost:3000`), the Go backend's
CORS middleware (`Access-Control-Allow-Origin: *`) handles cross-origin requests.

In production (same-origin via Caddy), CORS is not needed.

---

## 7. Verification

```bash
# 1. SPA loads
curl -s -o /dev/null -w '%{http_code}' https://aegis.example.com/
# Expected: 200

# 2. API responds
curl -s -o /dev/null -w '%{http_code}' https://aegis.example.com/api/system/status
# Expected: 200

# 3. Login flow works
curl -s -c /tmp/cookies.txt \
  -X POST https://aegis.example.com/api/admin/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<pass>"}'
# Expected: 200, sets cookie

# 4. Authenticated API call
curl -s -b /tmp/cookies.txt \
  https://aegis.example.com/api/admin/v1/auth/me
# Expected: {"user":{...}}
```

---

## 8. HAProxy Alternative (port 443 TCP mode)

If using HAProxy for TCP/SNI routing:

```
frontend https
  bind :443
  mode tcp
  tcp-request inspect-delay 5s
  tcp-request content accept if { req.ssl_hello_type 1 }
  use_backend aegis_console if { req.ssl_sni -i aegis.example.com }

backend aegis_console
  server caddy 127.0.0.1:8443
```

Caddy listens on `:8443` with TLS, reverse-proxies to Go backend on `:7380`.

---

## Port Requirements

| Port | Protocol | Service | Cloud Firewall |
|---|---|---|---|
| 80 | TCP | Caddy HTTP redirect | ✅ Open |
| 443 | TCP+UDP | Caddy HTTPS | ✅ Open |
| 7380 | TCP | Go Backend | ❌ Closed (localhost only) |
