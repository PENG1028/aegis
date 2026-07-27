# Service API Boundary — v1.7AA

## What Service API Keys Can Do

Service API keys (space-scoped tokens) can perform these actions via the Action API:

| Action | Endpoint | Scope Required | Triggers Apply | Operation Log | Ownership Check |
|--------|----------|:---:|:---:|:---:|:---:|
| bind-http-domain | `POST /api/v1/actions/bind-http-domain` | `domain:bind` | ✅ | ✅ | ✅ domain + space |
| bind-tls-backend | `POST /api/v1/actions/bind-tls-backend` | `domain:bind` | ✅ | ✅ | ✅ space |
| update-target (service) | `PATCH /api/v1/actions/update-target` | `service:update` | ✅ | ✅ | ✅ space+ownership |
| update-target (edge) | `PATCH /api/v1/actions/update-target` | `edge:update` | ✅ | ✅ | ✅ space+ownership |
| disable-domain (route) | `POST /api/v1/actions/disable-domain` | `domain:disable` | ✅ | ✅ | ✅ space+ownership |
| disable-domain (edge) | `POST /api/v1/actions/disable-domain` | `domain:disable` | ✅ | ✅ | ✅ space+ownership |
| delete-domain (route) | `DELETE /api/v1/actions/domain` | `domain:delete` | ✅ | ✅ | ✅ space+ownership |
| delete-domain (edge) | `DELETE /api/v1/actions/domain` | `domain:delete` | ✅ | ✅ | ✅ space+ownership |
| list my routes | `GET /api/v1/my/routes` | `read:own` | ❌ | ❌ | ✅ space |
| list my services | `GET /api/v1/my/services` | `read:own` | ❌ | ❌ | ✅ space |
| list my edge rules | `GET /api/v1/my/edge-rules` | `read:own` | ❌ | ❌ | ✅ space |
| list my operations | `GET /api/v1/my/operations` | `read:own` | ❌ | ❌ | ✅ space |

## What Service API Keys CANNOT Do

| Action | Why |
|--------|-----|
| Access any `/api/admin/v1/*` endpoint | Blocked by `isSystemRoute()` + AdminAuth middleware |
| Access CRUD endpoints (`/api/routes`, `/api/services`, etc.) | Added to `isSystemRoute()` in v1.7W |
| Manage nodes | Admin-only |
| Manage providers | Admin-only |
| Manage listeners | Admin-only |
| Manage cluster (leader, state) | Admin-only |
| Manage upgrades | Admin-only |
| Access resources belonging to other spaces | `requireOwnership()` check in ActionService |
| Access system-owned resources (space_id="") | Blocked for non-admin tokens |

## Endpoint Table

| Method | Path | Action | Scope | Admin | Service Key |
|--------|------|--------|:---:|:---:|:---:|
| POST | `/api/v1/actions/bind-http-domain` | bind-http-domain | `domain:bind` | ❌ | ✅ |
| POST | `/api/v1/actions/bind-tls-backend` | bind-tls-backend | `domain:bind` | ❌ | ✅ |
| PATCH | `/api/v1/actions/update-target` | update-target | `service:update` | ❌ | ✅ |
| POST | `/api/v1/actions/disable-domain` | disable-domain | `domain:disable` | ❌ | ✅ |
| DELETE | `/api/v1/actions/domain` | delete-domain | `domain:delete` | ❌ | ✅ |
| GET | `/api/v1/my/routes` | my routes | `read:own` | ❌ | ✅ |
| GET | `/api/v1/my/services` | my services | `read:own` | ❌ | ✅ |
| GET | `/api/v1/my/edge-rules` | my edge rules | `read:own` | ❌ | ✅ |
| GET | `/api/v1/my/operations` | my operations | `read:own` | ❌ | ✅ |
| GET/POST/PATCH/DELETE | `/api/admin/v1/*` | admin operations | admin:* | ✅ | ❌ |
| GET/POST | `/api/routes`, `/api/services`, etc. | CRUD | — | ✅ | ❌ |

## Current Gaps

| Gap | Impact |
|-----|--------|
| Service keys cannot list all available domains before binding | Must know domain name in advance |
| No pagination on list endpoints | Large result sets not handled |
| No bulk operations | Each domain bind is a separate API call |
| Error messages vary | Some use ActionError, some use generic errors |
