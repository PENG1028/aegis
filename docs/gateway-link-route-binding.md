# Gateway Link Route Binding — v1.7AC

## Binding Lifecycle

```
Route ──optional──→ GatewayLinkID (varchar, nullable)
                       │
                  Planner 读取 GatewayLink 记录
                       │
                  Caddy render: header_up X-Aegis-Gateway-*
```

## API

| Action | Method | Path | Field | Notes |
|--------|--------|------|-------|-------|
| Create route | POST /api/routes | body.gateway_link_id | Optional |
| Update route | PATCH /api/routes/{id} | body.gateway_link_id | Set to link ID to attach |
| Detach link | PATCH /api/routes/{id} | body.gateway_link_id="" | Empty string to detach |
| Direct attach | POST /api/admin/v1/routes/{id}/gateway-link | body.link_id | Attach link to route |
| Direct detach | DELETE /api/admin/v1/routes/{id}/gateway-link | — | Remove link from route |

## Current Status

| Binding path | Status | Evidence |
|-------------|--------|----------|
| route model has GatewayLinkID | ✅ | route.Route struct |
| DB column exists (migration 025) | ✅ | routes.gateway_link_id |
| Planner reads GatewayLink | ✅ | apply/planner.go |
| Caddy render injects headers | ✅ | provider/caddy_http.go |
| POST /api/routes accepts gateway_link_id | ⏳ | Handler not yet updated |
| PATCH /api/routes/{id} accepts gateway_link_id | ⏳ | Handler returns 501 |
| Admin attach/detach endpoints | ⏳ | Need route registration |
| Route → link update triggers MarkPending | ⏳ | Via admin CRUD semantics |

## Planner Behavior

When `route.GatewayLinkID` is non-empty:

1. Planner queries `gwLinkRepo.FindByID(route.GatewayLinkID)`
2. If gateway found and active → injects ExtraHeaders:
   - X-Aegis-Gateway-Link → gw.ID
   - X-Aegis-Gateway-Token → gw.AuthValue (hashed token)
3. If gateway not found or inactive → warning in apply plan

## Safe Apply

- Route linking/unlinking is a desired state change
- Must follow admin CRUD semantics: MarkPending → manual apply
- No auto-apply for route→link binding changes
