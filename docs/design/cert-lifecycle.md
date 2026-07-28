# Certificate and Domain TLS Lifecycle

## Purpose

Domain routing, TLS binding, certificate assets, and provider-managed certificates
have different owners. They share one UI view but do not share one CRUD lifecycle.

## Data Model

A TLS-terminating route has exactly one binding mode:

| `tls_binding_mode` | `cert_id` | `tls_provider` | Meaning |
|---|---|---|---|
| `provider_auto` | empty | required | The provider obtains and renews certificates internally. |
| `certificate` | required | empty | The provider loads a CertStore PEM asset. |
| `off` | empty | empty | The composition does not terminate TLS. |

`cert_id` never points to a provider-managed observation. Caddy certificates are
read from Caddy storage for display and are not copied into CertStore.

## Ownership

| Certificate source | Lifecycle owner | Direct delete | Renewal |
|---|---|---|---|
| `gateway_auto` observation | Provider | No | Provider internal |
| `local_acme` asset | Aegis | Only when unreferenced | Aegis scheduler |
| `manual_upload` asset | User | Only when unreferenced | Replace manually |
| `external` asset | User | Only when unreferenced | Replace manually |

Deleting a route deletes its TLS binding but retains certificate assets. Provider
observations disappear when the provider no longer reports them; Aegis does not
claim that removing a route immediately deletes provider storage.

## Capability Rules

The abstractions have separate responsibilities:

- `Capability`: static provider primitives such as `auto_cert` and `load_cert`.
- `Composition`: traffic requirements such as TCP + TLS termination + HTTP route.
- `TLSBindingMode`: the selected certificate strategy for one route.
- `TLSLifecycleService`: ownership, references, binding, and deletion policy.

Requirements are derived at runtime:

```text
HTTPS composition + provider_auto = composition requirements + auto_cert
HTTPS composition + certificate   = composition requirements + load_cert
```

`auto_cert` and `load_cert` are alternatives. Persisted `source_capabilities`
is provenance information and must not be compared directly with RuntimeMode atom
keys (`tcp`, `tls`, `http`). Mode compatibility first checks the composition and
then checks the selected TLS strategy against providers present in the target mode.

## Mutation Contract

All certificate deletion uses a preview and a second validation at execution:

- Provider-managed observation: reject with `CERT_MANAGED_BY_PROVIDER`.
- Referenced asset: reject with `CERT_HAS_REFERENCES` and route references.
- Unreferenced asset: delete its metadata and Aegis-managed PEM files.

Binding a certificate validates that its SAN/CN covers the route domain. A wildcard
covers exactly one label: `*.example.com` covers `api.example.com`, not
`deep.api.example.com` or `example.com`.

Certificate assets are bound explicitly at route creation or from route details.
There is no background domain-to-certificate auto-binding, so uploading a wildcard
certificate cannot silently change existing routes.

The certificate UI separates portable **certificate assets** from read-only
**automatic TLS** observations. They share a status page, not a CRUD lifecycle.
Wildcard assets offer a matching-route preview and explicit bulk binding. The
complete selection is validated before one transactional binding update and one
Apply.

Matching routes already using automatic TLS remain eligible because selecting
them is an explicit strategy change, not background auto-binding. Preview marks
those rows as replacing automatic TLS; `already_bound` means the route already
uses the previewed certificate and is not a pending selection.

Certificate references are also enforced by SQLite triggers. Preview checks are
for user guidance; database guards are the final authority under concurrent bind
and delete requests.

Route and TLS mutations mark `pending_apply` before gateway Apply. A successful
Apply clears only the pending revision it started with, so a concurrent mutation
cannot be lost. Failed applies remain pending and the server retries them through
the canonical apply lock.

Deleting a route retains its certificate asset by default. Optional certificate
cleanup is offered only when that route is the asset's final reference and is
rechecked during deletion. Automatic TLS observations are never cleanup targets;
their executor reconciles private storage after the route stops requesting
automatic TLS.

## Provider Contract

A provider may declare `auto_cert`, `load_cert`, both, or neither. `auto_cert` does
not imply that Aegis can force renewal, delete provider storage, or export keys.
Provider-managed certificate inspection is read-only unless a future provider
adapter explicitly adds an optional lifecycle operation.

Local ACME HTTP-01 does not open port 80. lego stores active tokens in memory;
the Planner injects `/.well-known/acme-challenge/*` into Caddy and proxies it to
the local Aegis HTTP server. Renewal holds the Apply lock across stable-path PEM
replacement and forced provider Apply, ensuring the served certificate is reloaded.

Mode switching uses a two-phase provider handoff: target configurations are
rendered, validated, and staged first; current listeners are then stopped and the
complete target provider set is started. Any failure restores configuration files
and prior service-running state.

## Verification

Required regression coverage:

- Migration from legacy `gateway_auto` route bindings.
- Provider-auto routes never render a fixed custom PEM directive.
- Route deletion retains certificate assets.
- Referenced certificate deletion is rejected with references.
- Wildcard coverage follows one-label TLS semantics.
- Mode switching checks both composition and TLS strategy.
- UI delete confirmation reflects backend preview and pending Apply states.

Before production deployment on Linux:

1. Back up the SQLite database, Aegis certificate directory, Caddyfile, and HAProxy config.
2. Set `proxy.acme_server` to the Let's Encrypt staging directory and issue a certificate through the UI.
3. Verify `GET /.well-known/acme-challenge/not-active` returns `404`, not `401`, through public port 80.
4. Bind the staging certificate, Apply, and confirm the served serial changes after a forced renewal.
5. Exercise Legacy to EdgeMux and EdgeMux to Legacy, including one intentional target start failure to verify rollback.
6. Clear `proxy.acme_server` only after staging succeeds, then restart Aegis before production issuance.

Explicit certificate assets stay under Aegis ownership. The active Caddy
consumer receives group-read access (`0750` directory, `0640` PEM files), and
startup repairs assets written by older releases. Atomic renewal replacement
must preserve that group and mode; private keys must never become world-readable.
