# System Boundary — v1.7AA

## What Aegis IS

- Personal infrastructure gateway control plane
- Single-admin system (one admin user at a time)
- Multi-scope API key system (space-isolated service keys)
- Caddy/HAProxy provider config controller (render → validate → reload)
- Domain/route/edge rule lifecycle manager
- Trace/diagnose/apply operational tools
- SQLite-backed stateful control plane
- Action API for safe, scoped mutations

## What Aegis is NOT

| Not This | Reason |
|----------|--------|
| SaaS / multi-tenant platform | Single admin, single VPS, no billing |
| Multi-team system | No team isolation, no RBAC |
| Kubernetes or Service Mesh | No pod orchestration, no sidecar |
| Cloud vendor controller | No AWS/GCP/Azure API integration |
| Unified logging platform | op/apply/audit logs exist but no OTel/structured protocol |
| Auto-deployment platform | No CI/CD integration, no canary executor |
| Auto-healing system | Drift detected but not auto-repaired |
| UI dashboard | CLI-only; no web UI |

## Status: SINGLE_NODE_REAL_VERIFIED

All capabilities verified on real Ubuntu VPS with Caddy 2.6.2 + HAProxy 2.8.16.
Two-node acceptance in progress (v1.7AA).

## Design Principle

> Aegis manages **desired state** (routes in DB) and drives **applied state**
> (Caddy/HAProxy configs) toward it via safe apply. It is NOT in the data path.
> Caddy and HAProxy serve traffic independently of Aegis process lifecycle.
