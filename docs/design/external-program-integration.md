# External Program Integration Boundary

## Status

**Future design. Not implemented. Not part of the current middleware lifecycle
or mode-switch contract.**

This document reserves the external boundary that Aegis may expose to separate
programs. It does not turn Aegis into a request-transformation or plugin runtime.
Middleware execution capabilities and external program APIs are separate models
and must not share one capability namespace.

## Purpose

Aegis is responsible for the small set of traffic-control concerns it owns:

- authenticate a calling program and scope it to a Space;
- create and manage stable ingress and forwarding resources;
- publish desired traffic state through the canonical Apply workflow;
- authorize which program may modify which resource;
- report operation status, effects, failures, and reconciliation state;
- route traffic to a registered external processor when explicitly configured.

Authentication, authorization decisions, translation, HTML injection, body
transformation, and other detailed request processing remain external program
responsibilities.

## Two Separate Interfaces

### Control Plane

The control plane accepts bounded, versioned JSON commands. A caller declares a
desired traffic result, not provider configuration and not internal database rows.

Initial resource operations should be limited to:

- create an HTTP or HTTPS exposure;
- create TLS passthrough, TCP forwarding, or UDP forwarding;
- update the logical target;
- select a TLS strategy or certificate asset;
- disable, enable, preview retirement, and retire an exposure;
- inspect owned resources and operation status;
- register and report health for an external processing service.

Every mutation requires caller identity, Space authorization, an idempotency key,
and an operation ID. A successful API response means the desired state was
accepted; publication status is reported separately when Apply is asynchronous.

External callers must never provide a provider ID, native configuration fragment,
certificate filesystem path, listener implementation, or internal row type.

### Traffic Plane

Traffic is not carried through a generic JSON capability-call endpoint. When an
external processor is attached, it is a normal logical upstream hop:

```text
public ingress -> configured middleware -> external processor -> application
```

The first implementation should require the external processor to forward to the
application itself. Returning processed traffic to an Aegis re-entry endpoint is
out of scope until loop prevention, next-hop identity, and a dedicated re-entry
trust boundary are designed.

The traffic contract must preserve streaming protocol behavior. It must not use
`json.RawMessage` or read complete request or response bodies into memory.

## Data Given To An External Processor

By default Aegis and its execution middleware pass the original protocol data
needed by the processor:

- HTTP method, normalized request target, path, query, and headers;
- streaming request body without mandatory buffering;
- cancellation and connection-close propagation;
- response status, headers, trailers, and streaming body on the return path;
- protocol upgrade semantics when the declared exposure supports them.

Client-supplied trusted identity headers must be removed at the trust boundary.
Aegis may inject a minimal signed caller and resource context. The exact header
names, signature format, expiry, replay protection, and key rotation require a
separate security specification before implementation.

The processor does not receive database records, provider state, native config,
Space secrets, or unrelated route metadata.

## Existing Code That May Be Reused

- Service registration, heartbeat, tickets, and blocklist under
  `internal/serviceauth`.
- Space identity and quotas under `internal/space`.
- Caller propagation and ownership checks under `internal/action`.
- Stable resource and operation identifiers.
- The Action service as the single control-plane mutation facade.
- The canonical preview, Apply, pending-apply, and rollback workflows.

The current `internal/aegisgateway.CapabilityRegistry` is only an authenticated
control RPC registry. The current service-call path buffers JSON responses and is
not a traffic-plane processor contract.

## Reserved Contract Requirements

Any future public control API must provide:

- explicit API and resource schema versions;
- typed operations instead of unbounded generic input blobs;
- Space-scoped authorization on every read and mutation;
- idempotent mutation and stable resource IDs;
- operation preview with allowed, reason, effects, references, and alternatives;
- asynchronous status and reconciliation without requiring callers to retry raw
  provider commands;
- structured error and reason codes;
- audit attribution to service, token, Space, resource, and operation;
- pagination and bounded response sizes for list and event APIs.

## UI Boundary

Core Aegis resources remain visible in the Aegis UI: exposures, targets, TLS
strategy, ownership, operation state, and an attached processor's health.

External program-specific configuration is not automatically an Aegis UI module.
Translation dictionaries, authorization policies, transformation scripts, and
analytics schemas remain in the owning external system unless a later product
decision explicitly adds a dedicated integration UI.

## Non-Goals For The First Version

- an in-process JS or native plugin runtime;
- arbitrary request and response Hook registration;
- generic provider capability access by external callers;
- arbitrary database or native configuration mutation;
- automatic request-body capture or API schema inference;
- multi-hop processor graphs or traffic re-entry into Aegis;
- a drag-and-drop processing-chain editor.

## Implementation Order

This work starts only after the middleware resource lifecycle and execution
binding model are stable:

1. Freeze versioned control-plane resource schemas.
2. Add idempotent, operation-based exposure APIs.
3. Harden service identity, Space authorization, and audit records.
4. Register a processor as a healthy logical target.
5. Validate one HTTP processor with streaming and cancellation preservation.
6. Add WebSocket, SSE, large upload, and failure-policy coverage.
7. Consider richer processor chaining only from demonstrated requirements.

## Acceptance Boundary

The future integration is not complete until automated and staging tests prove:

- one Space cannot read or mutate another Space's resources;
- repeated idempotent commands do not create duplicate resources;
- accepted operations expose pending, applied, failed, and reconciled states;
- forged identity headers are removed and trusted context is verifiable;
- processor timeout and unavailability follow the configured fail policy;
- request cancellation reaches the processor and upstream;
- large uploads, chunked bodies, gzip, SSE, WebSocket, and streaming responses are
  not corrupted or unintentionally buffered;
- external control APIs cannot select providers or bypass lifecycle validation;
- removing a processor attachment has a previewed and deterministic traffic
  effect.
