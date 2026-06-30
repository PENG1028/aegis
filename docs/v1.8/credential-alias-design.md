# v1.8K — Credential Alias (Encrypted Connection String) Design

> **Status:** implemented ✅
> **Use case:** Store database connection strings encrypted, reference by alias in TCP exposures.

## Problem

Connection strings contain credentials that must not be stored in plaintext:

```
postgres://systemname:systempass@10.0.0.5:5432/mydb
mysql://root:secret@10.0.0.6:3306/appdb
redis://:token@10.0.0.7:6379/0
```

These need to be:
1. Encrypted at rest (AES-256-GCM)
2. Referenced by alias from TCP exposures
3. Resolvable at proxy-start time
4. Consumable by external tools (dbmanage, etc.) via API

## Architecture

```
┌─ UI / API ──────────────────────────────────────────┐
│                                                       │
│  创建凭据:                                             │
│    POST /api/admin/v1/credentials                     │
│    { alias: "db.mydomain.com/c/mydbothername",        │
│      conn_string: "postgres://user:pass@10.0.0.5:5432/mydb",
│      description: "生产数据库" }                       │
│                                                       │
│  加密存储:                                             │
│    AES-256-GCM(master_key, raw) → "nonce:ciphertext"  │
│    存入 credentials 表                                 │
│                                                       │
│  TCP Exposure 引用:                                    │
│    target: credential://db.mydomain.com/c/mydbothername│
│                                                       │
│  激活时解析:                                           │
│    credential://alias                                 │
│      → DecryptFromStorage(master_key, encrypted)      │
│      → uri.Parse(raw) → host, port                    │
│      → tcp.Manager.StartProxy(listen, target)         │
│                                                       │
│  外部工具解析 (dbmanage):                              │
│    GET /api/admin/v1/credentials/resolve?alias=xxx    │
│      → { host, port, user, password, database, ... }  │
│                                                       │
│    POST /api/admin/v1/credentials/{id}/reveal         │
│      → { raw_conn_string } (审计记录, 仅显示一次)       │
└───────────────────────────────────────────────────────┘
```

## Usage Case: dbmanage

```bash
# 1. Create credential (one time)
curl -X POST -H "Cookie: aegis_admin_session=xxx" \
  -d '{"alias":"db.mydomain.com/c/mydbothername",
       "conn_string":"postgres://systemname:systempass@10.0.0.5:5432/mydb",
       "description":"生产数据库"}' \
  http://panel/api/admin/v1/credentials

# 2. Resolve for dbmanage connection
curl -H "Cookie: aegis_admin_session=xxx" \
  "http://panel/api/admin/v1/credentials/resolve?alias=db.mydomain.com%2Fc%2Fmydbothername"
# → { host: "10.0.0.5", port: 5432, user: "systemname", password: "systempass", database: "mydb" }

# 3. dbmanage uses returned params to connect directly:
# psql -h 10.0.0.5 -p 5432 -U systemname -d mydb
```

## Security

| Property | Implementation |
|----------|---------------|
| Encryption at rest | AES-256-GCM via `secrets.EncryptToStorage` |
| Master key | `/etc/aegis/secret.key` (0600), `AEGIS_SECRET_KEY` env var |
| Raw secret in API list | Never — `json:"-"` on `EncryptedConnString` |
| Masked display | `postgres://user:***@host:5432/db` |
| Reveal audit | `credential.reveal` log event with alias + version |
| Rotate support | `POST .../rotate` — new connection string, version++ |

## API Reference

```
GET    /api/admin/v1/credentials                      List all (masked)
POST   /api/admin/v1/credentials                      Create (encrypt)
GET    /api/admin/v1/credentials/resolve?alias=xxx    Resolve alias → structured params
GET    /api/admin/v1/credentials/{id}                 Get one (no raw secret)
POST   /api/admin/v1/credentials/{id}/reveal          Reveal raw string once (audited)
POST   /api/admin/v1/credentials/{id}/rotate          Rotate to new connection string
DELETE /api/admin/v1/credentials/{id}                 Delete
```

## Alias Format

Aliases are free-form unique strings. Support any format:

```
pg-prod
db.mydomain.com/mydb
db.mydomain.com/c/mydbothername
redis/cache-session
mysql/staging/replica
```

Aliases with `/` use query parameter for resolve: `?alias=db.mydomain.com%2Fc%2Fmydbothername`
