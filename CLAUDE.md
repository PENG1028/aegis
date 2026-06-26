# Aegis — Claude Project Guide

## Infrastructure

- This is a personal infrastructure gateway control plane (Go + SQLite)
- NOT SaaS, NOT multi-admin, NOT multi-tenant
- Single-node verified; multi-node support is FAKE_ONLY
- Two VPS: Server A (43.160.211.232, gateway) and Server B (43.159.34.11, remote target)
- SSH from dev machine to both servers works (keys configured in ~/.ssh/config)

## Network / Ports

**Only ports 80 (TCP) and 443 (TCP+UDP) are open on the cloud security group.**
Never test with any other port. If a service needs to be reached cross-server, it must run on port 80 or 443.

Pre-flight check before any cross-server test:
```bash
ssh ubuntu@<source> "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://<target_ip>:<port>/"
```
Expected: 2xx/4xx. Anything else → port not open or service not running.

## Architecture Principles

- Aegis is NOT in the data path. Caddy/HAProxy serve traffic independently.
- Gateway Link is NOT a route source of truth. It's metadata on a route.
- Gateway Link auth: HMAC-SHA256 with timestamp replay protection (5 min window).
- All mutation endpoints for admin MUST call MarkPending.
- Service API keys CANNOT access /api/admin/v1/*. Blocked by isSystemRoute().

## Security

- Admin password: bcrypt (cost 12). Random password generated on first run, printed once.
- Admin token: random 64-hex token generated at init. Redacted in GET /api/settings.
- Session cookie: Secure flag configurable (default false dev, true prod).
- Request body limit: 1 MB max.
- Login rate limiting: 5 attempts/minute/IP.
- Gateway link secrets: HMAC-SHA256 hashed in DB, raw secret returned once at creation.

## Commit Convention

Prefix commits with the version tag: `v1.7AD`, `v1.7Z`, etc.
Use full sentences in Chinese or English describing what was done.

## Current State (v1.7AD)

- Gateway Link: TWO_NODE_VERIFIED (HMAC-SHA256 with timestamp replay protection)
- All capability matrices: docs/boundary/dimension-map.md
- Gateway link secret NOT in list/get/log/trace
- Two-node acceptance PASSED: A→B:80 with verifier 401/200/403
- Security hardened: bcrypt passwords, random credentials, rate limiting, body limits
