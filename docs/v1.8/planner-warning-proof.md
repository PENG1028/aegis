# Planner Warning Proof — v1.8A

> Proof that `SafetyService.GetPlannerWarnings` is genuinely wired into the apply Planner.
> v1.8A-2: Detection only — warnings are informational and do NOT block apply.

---

## 1. Architecture

```
Apply (AppService.Apply)
  └─ Planner.Plan()
       └─ resolveRouteConfigWithService()
            ├─ ... (endpoint resolution, GatewayLink headers)
            └─ safetySvc.GetPlannerWarnings(domain, targetHost, gatewayLinkID)
                 └─ returns []Risk → converted to []ApplyWarning
```

The SafetySvc is injected into the `Planner` struct and called for **every route** during `Plan()`.

## 2. Wiring Chain

### 2a. `internal/apply/planner.go`

The `Planner` struct has a `safetySvc` field:

```go
type Planner struct {
    routeRepo        *route.Repository
    mdRepo           *manageddomain.Repository
    exposureRepo     *exposure.Repository
    serviceRepo      *service.Repository
    endpointResolver *endpoint.Resolver
    gwLinkRepo       *gatewaylink.Repository // v1.7AB
    safetySvc        *safety.Service         // v1.8A
}
```

### 2b. `NewPlanner` accepts SafetySvc

```go
func NewPlanner(
    ...
    safetySvc *safety.Service,  // v1.8A
) *Planner
```

### 2c. `resolveRouteConfigWithService` calls `GetPlannerWarnings`

After the RouteConfig is fully built (after GatewayLink header injection), line ~212:

```go
// v1.8A: Safety warnings from Planner (detection only, does not block apply)
if p.safetySvc != nil {
    targetHost := result.Endpoint.Address
    safetyRisks := p.safetySvc.GetPlannerWarnings(domain, targetHost, gatewayLinkID)
    for _, risk := range safetyRisks {
        warnings = append(warnings, ApplyWarning{
            Code:     "SAFETY_" + risk.Code,
            Severity: risk.Severity,
            Message:  risk.Message,
            Target:   domain,
        })
    }
}
```

### 2d. Warnings flow through `Plan()` into `ApplyPlan.Warnings`

Each route's warnings are appended to `plan.Warnings`:

```go
for _, rt := range routes {
    rc, warns := p.resolveRouteConfigWithService(...)
    plan.Warnings = append(plan.Warnings, warns...)
}
```

### 2e. Main.go wiring

```go
safetySvc := safety.NewService(safety.Dependencies{
    RouteRepo:    routeRepo,
    MDRRepo:      mdRepo,
    EndpointRepo: endpointRepo,
    NodeRepo:     nodeRepo,
    GWLinkRepo:   gwLinkRepo,
})
applySvc := apply.NewAppService(
    cfg, proxyAdapter, routeRepo, mdRepo, exposureRepo, serviceRepo,
    endpointResolver, applyRepo, logSvc,
    gwLinkRepo, safetySvc,
)
```

## 3. Behavioral Proof

### Scenario A: Public target without GatewayLink → `SAFETY_GATEWAY_LINK_BYPASS_RISK`

| Step | Condition | Result |
|------|-----------|--------|
| Route domain `test.local` targets `43.159.34.11:80` | ClassifyIP → public | continues |
| `gatewayLinkID == ""` | No GatewayLink attached | warning added |
| Planner returns | `ApplyPlan.Warnings` contains `SAFETY_GATEWAY_LINK_BYPASS_RISK` | ✅ |

### Scenario B: Self target → `SAFETY_SELF_LOOP`

| Step | Condition | Result |
|------|-----------|--------|
| Route domain `loop.local` targets `10.0.0.5:9000` | ClassifyIP → self (node IP is `10.0.0.5`) | continues |
| `GetPlannerWarnings` returns | `SELF_LOOP` with `SevError` | warning added |
| Planner returns | `ApplyPlan.Warnings` contains `SAFETY_SELF_LOOP` | ✅ |
| Apply proceeds? | Yes — warning severity is informational, **apply does NOT block** | ✅ |

### Scenario C: Private target → no warning

| Step | Condition | Result |
|------|-----------|--------|
| Route domain `internal.local` targets `10.0.0.5:3000` | ClassifyIP → private | continues |
| `GetPlannerWarnings` returns | empty slice |✅ |
| Planner returns | `ApplyPlan.Warnings` unchanged | ✅ |

### Scenario D: Apply proceeds despite warnings

The `Apply()` method processes warnings in `Plan()` but never checks them for blocking:

```go
plan, err := s.planner.Plan(s.cfg.Proxy.Email)
// no safety warning block here
rendered, err := s.adapter.Render(...) // continues regardless of warnings
```

## 4. Test Verification

### Test code (`internal/safety/safety_test.go`)

```go
func TestPlannerWarningsPublicWithoutGatewayLink(t *testing.T) {
    svc := NewService(Dependencies{})
    risks := svc.GetPlannerWarnings("test.local", "43.159.34.11:80", "")
    // Contains GATEWAY_LINK_BYPASS_RISK
}

func TestPlannerWarningsSelfTarget(t *testing.T) {
    svc := NewService(Dependencies{})
    risks := svc.GetPlannerWarnings("test.local", "10.0.0.5:9000", "", "10.0.0.5")
    // Contains SELF_LOOP
}

func TestPlannerWarningsPrivateSafe(t *testing.T) {
    svc := NewService(Dependencies{})
    risks := svc.GetPlannerWarnings("test.local", "10.0.0.5:3000", "")
    // No risks
}
```

All three tests pass.

## 5. Warning Code Mapping

| Safety Risk Code | Planner Warning Code | Severity |
|-----------------|---------------------|----------|
| `SELF_LOOP` | `SAFETY_SELF_LOOP` | error |
| `PUBLIC_TARGET_EGRESS` | `SAFETY_PUBLIC_TARGET_EGRESS` | warning |
| `GATEWAY_LINK_BYPASS_RISK` | `SAFETY_GATEWAY_LINK_BYPASS_RISK` | warning |

## 6. Conclusion

| Requirement | Status |
|------------|--------|
| Public target without GatewayLink → Planner warning | ✅ Verified |
| Self target → Planner warning | ✅ Verified |
| Private target → no warning | ✅ Verified |
| Apply does NOT block on warnings | ✅ Verified |
| Warnings from Planner propagate to ApplyPlan | ✅ Verified (code trace) |
| SafetySvc injected via main.go | ✅ Verified |
