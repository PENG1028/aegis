# Aegis — Claude Project Guide

## Infrastructure

- This is a personal infrastructure gateway control plane (Go + SQLite)
- NOT SaaS, NOT multi-admin, NOT multi-tenant
- Single-node verified; multi-node support is FAKE_ONLY
- Two VPS: Server A (<SERVER_A_IP>, gateway) and Server B (<SERVER_B_IP>, remote target)
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
- Static shared token mode (not HMAC). HMAC deferred.
- All mutation endpoints for admin MUST call MarkPending.
- Service API keys CANNOT access /api/admin/v1/*. Blocked by isSystemRoute().

## Commit Convention

Prefix commits with the version tag: `v1.7AC`, `v1.7Z`, etc.
Use full sentences in Chinese or English describing what was done.

## Current State (v1.7AC-2)

- Gateway Link: TWO_NODE_VERIFIED (static token via port 80)
- All capability matrices: docs/boundary/dimension-map.md
- Gateway link secret NOT in list/get/log/trace
- Two-node acceptance PASSED: A→B:80 with verifier 401/200/403
