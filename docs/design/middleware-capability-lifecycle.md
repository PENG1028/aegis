# Middleware Capability Lifecycle

## Status

Implemented contract for capability metadata, TLS resource operations, and mode
switch previews. This document defines the boundary between stable traffic
intent and the executor currently realizing it.

## Principle

A capability key describes what an executor can do. It does not imply that state
created by two executors is interchangeable.

Every resource operation resolves in this order:

1. Read the resource intent and current execution/lifecycle binding.
2. Prefer the currently bound executor when it remains eligible.
3. If it is not eligible, inspect the capability migration strategy.
4. Re-render declarative state, reload portable assets, or recreate
   provider-managed state on an eligible target executor.
5. Reject unsupported migration before changing files or services.

An executor with the same capability key must never mutate another executor's
private state without an explicit migration plan.

## Capability Semantics

`provider.SemanticsOf` classifies every capability using four properties:

| Property | Values | Purpose |
|---|---|---|
| `state_class` | `declarative`, `portable_asset`, `provider_managed`, `runtime` | What state is produced or consumed |
| `affinity` | `none`, `executor_sticky`, `bundle_sticky` | Whether operations prefer or require the current executor |
| `migration` | `re_render`, `reload_asset`, `recreate`, `restart`, `unsupported` | How a target executor realizes equivalent intent |
| `coupled_with` | capability keys | Capabilities that form one lifecycle responsibility |

The provider inventory API projects static semantics together with runtime
availability as `capability_statuses`. Static support never overrides installed,
running, diagnostic, or active-mode checks.

## TLS Resource Classes

### Certificate Asset

Manual uploads, external certificates, and Aegis local ACME certificates are
portable assets. Routes reference their stable `cert_id`. A target executor may
load the same asset when it supports `load_cert`.

Deleting a route removes its binding and retains the asset by default. The UI may
offer `delete_unused_certificate` only when the route is the asset's final
reference. Database reference triggers remain the concurrency authority.

### Automatic TLS State

An executor-discovered automatic certificate is an observation, not a CertStore
asset. It cannot be bound by `cert_id`, independently deleted, or assumed
exportable. Its lifecycle is coupled to the route's automatic TLS strategy.

During migration, the route identity remains stable. If automatic TLS ownership
moves, the target executor recreates its private state. If the executor remains
the same, Aegis only re-renders its configuration and preserves that state.
Failure to provision or load target state blocks or rolls back the mode switch;
Aegis does not silently downgrade to plaintext or an unrelated certificate.

## Wildcard Binding

Wildcard certificate assets use the same many-routes-to-one-asset reference
model as other SAN certificates. `*.example.com` covers exactly one label.

Binding is explicit:

1. Preview all TLS-terminating routes covered by the certificate.
2. Let the operator select matching routes.
3. Validate the complete selection.
4. Commit all bindings in one database transaction.
5. Run one canonical Apply.

There is no background auto-binding. New-route UI may recommend matching assets
but must not select one without operator intent.

## UI Contract

Certificate management has two peer views:

- **Certificate assets**: bind, bulk bind, replace, renew where supported, delete
  when unreferenced.
- **Automatic TLS**: observe domains, expiry, executor ownership, and route usage;
  change behavior through the owning route's TLS strategy.

Unavailable operations are returned by backend previews with reason codes,
effects, references, and alternatives. UI code must not infer lifecycle actions
from a concrete provider ID.

## Mode Switch Contract

For each route, preview reports current executor, target executor, state class,
and migration strategy:

- route configuration: `declarative / re_render`;
- certificate asset: `portable_asset / reload_asset`;
- automatic TLS: `provider_managed / re_render` when the executor is preserved,
  otherwise `provider_managed / recreate`;
- listeners: `runtime / restart`.

Mode switching preserves route IDs, service targets, TLS strategy, and asset
references. Executor IDs are operational details shown in diagnostics.

## Acceptance

- The current executor is preferred when it remains eligible.
- Same-name capability support never causes an implicit private-state takeover.
- Automatic TLS ownership changes are reported as recreation, not certificate
  transfer; same-executor mode changes are reported as re-rendering.
- Certificate asset migration is reported as asset reload.
- Route deletion retains assets unless explicit final-reference cleanup is set.
- A wildcard batch containing one ineligible route changes no route.
- Automatic TLS observations never enter certificate CRUD or bind selectors.
- An unsupported target mode is rejected before runtime mutation.
- Failed mode switches restore configuration and service-running state.
