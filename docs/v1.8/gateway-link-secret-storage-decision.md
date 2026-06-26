# v1.8B-0 — GatewayLink Secret Storage Decision

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Sub-phase:** v1.8B-0 — Design only, no code
> **Status:** DESIGN COMPLETE
> **Theme:** Encrypt GatewayLink static token at rest in SQLite

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Scheme Comparison](#2-scheme-comparison)
3. [Recommendation](#3-recommendation)
4. [Master Key Design](#4-master-key-design)
5. [DB Schema Migration](#5-db-schema-migration)
6. [Runtime Behavior](#6-runtime-behavior)
7. [API Contract Changes](#7-api-contract-changes)
8. [Security Boundaries](#8-security-boundaries)
9. [Deferred Items](#9-deferred-items)
10. [Acceptance Checklist](#10-acceptance-checklist)

---

## 1. Problem Statement

### Current State

GatewayLink `auth_value` is a static token stored as **plaintext** in SQLite:

```go
type GatewayLinkRecord struct {
    // ...
    AuthValue   string  // plaintext — visible in DB file
    // ...
}
```

This token is injected into Caddy's reverse proxy config as an HTTP header:

```
header_up X-Aegis-Gateway-Token <raw_token>
```

### Risks

| Risk | Severity | Detail |
|------|----------|--------|
| SQLite file compromise | High | Anyone with `.aegis/aegis.db` read access obtains all static tokens |
| Backup leakage | High | DB backups contain plaintext secrets |
| Log exposure | Medium | Error logs could leak auth_value if rendering fails |
| Screen/terminal exposure | Low | CLI `gateway-link show` returns raw token (intentional, but visible) |

### Constraint

Aegis needs the **raw token** at apply/render time to generate Caddy config:

```
header_up X-Aegis-Gateway-Token <raw_token>
```

This rules out one-way hashing (like `bcrypt` for passwords) because the raw value must be recoverable.

---

## 2. Scheme Comparison

### Scheme A: HMAC Hash Only

| Property | Evaluation |
|----------|-----------|
| **Re-render Caddy config?** | ❌ **No.** HMAC is one-way. Cannot recover raw token for `header_up`. |
| **Restart → apply?** | ❌ No. Apply needs raw token, which is gone after creation. |
| **Support rotate?** | ❌ Cannot generate new raw token from hash. Would need to store rotation metadata, which defeats the purpose. |
| **External secret required?** | No. Uses application secret key. |
| **Backup/restore impact** | Low — tokens are already hashed. |
| **Multi-node future** | Poor — each restore would lose the ability to re-render. |
| **Security** | Strong — one-way, no plaintext anywhere. |
| **Complexity** | Low — hash on write, discard raw. |

**Verdict: Not suitable.** The raw token is a functional requirement for Caddy config rendering. HMAC-only destroys that capability.

**Why this was in v1.8B scope draft:** The draft was written before the Caddy `header_up` constraint was re-evaluated. HMAC-only is correct for secrets that are compared (passwords, API keys for verification) but incorrect for secrets that must be re-injected into config.

---

### Scheme B: Encrypted Token at Rest (Recommended)

| Property | Evaluation |
|----------|-----------|
| **Re-render Caddy config?** | ✅ **Yes.** Decrypt on apply/render to recover raw token. |
| **Restart → apply?** | ✅ Yes. Decrypt with master key at runtime. |
| **Support rotate?** | ✅ **Yes.** Generate new token → encrypt → store → increment version. |
| **External secret required?** | Yes — master key file (`/etc/aegis/secret.key`). Not embedded in DB or code. |
| **Backup/restore impact** | ⚠️ **Must backup both DB and secret.key.** Loss of key = permanent loss of all encrypted tokens. |
| **Multi-node future** | Good — same key on all nodes, or per-node key with node ID derivation. |
| **Security** | Medium-high — SQLite plaintext risk eliminated. Key file is separate attack surface. |
| **Complexity** | Medium — AEAD encryption (AES-256-GCM or ChaCha20-Poly1305), nonce management, version tracking. |

---

### Scheme C: External Secret File / Secret Reference

| Property | Evaluation |
|----------|-----------|
| **Re-render Caddy config?** | ✅ **Yes** — if the secret file path is stored and the file is readable. |
| **Restart → apply?** | ✅ Yes — read secret file on demand. |
| **Support rotate?** | ⚠️ Partial — need to track which secret file version is active. Native rotation requires file naming convention (e.g., `secret_v1`, `secret_v2`). |
| **External secret required?** | Yes — the secret file itself IS the external secret. |
| **Backup/restore impact** | ⚠️ Same as Scheme B — must backup both DB and secret directory. |
| **Multi-node future** | Moderate — file must be distributed to all nodes (scp, rsync, or config management). |
| **Security** | Medium — plaintext on filesystem. Relies on filesystem permissions (0600). |
| **Complexity** | Low — read from file, store file path in DB. No encryption/decryption in app. |

**Detailed analysis:**

Scheme C moves the plaintext from SQLite to a file. This is a marginal improvement:
- SQLite backup no longer contains secrets
- But the secret is still plaintext on disk
- File permission 0600 is similar protection to SQLite file permission
- The attacker who can read `.aegis/aegis.db` can likely also read `/etc/aegis/secret/*.key`

**Verdict: Better than HMAC-only, weaker than encryption-at-rest.** Useful if the user wants to manage secrets via an external tool (Vault, sops, age). Adds operational complexity (file distribution, path management) with limited security gain over Scheme B.

---

### Comparison Matrix

| Criterion | A: HMAC Only | B: Encrypted at Rest | C: External File |
|-----------|:---:|:---:|:---:|
| Re-render Caddy config | ❌ | ✅ | ✅ |
| Restart → apply | ❌ | ✅ | ✅ |
| Token rotation | ❌ | ✅ | ⚠️ Partial |
| Eliminates SQLite plaintext | ✅ | ✅ | ✅ |
| No new external dependency | ✅ | ⚠️ Key file | ❌ Secret file |
| Single-file backup possible | ✅ | ❌ | ❌ |
| Implementation complexity | Low | Medium | Low |
| Security at rest | Strong | Strong | Medium |

---

## 3. Recommendation

### Chosen: Encrypted Token at Rest (Scheme B)

**Rationale:**

1. **Functional requirement:** Aegis must re-render Caddy config with the raw token after restart/apply. Only encryption preserves this capability.

2. **Threat model fit:** The primary threat is SQLite file compromise (backup leakage, filesystem read). Encryption eliminates this while keeping the token usable at runtime.

3. **Operational simplicity for single-node:** One key file (`/etc/aegis/secret.key`) is manageable for a personal gateway. No external secret management service needed.

4. **Future-proof:** Multi-node can share the same key or derive per-node keys. Rotation is supported via version tracking.

5. **HMAC-only ruled out:** See Scheme A analysis — it breaks the core rendering requirement.

6. **External file rejected:** Moves plaintext from one file to another without meaningful security gain. Encryption provides actual cryptographic protection.

**Cost:** One additional file to backup (secret.key). One environment variable or well-known path to configure.

---

## 4. Master Key Design

### Key Source

**Recommended: `/etc/aegis/secret.key`**

| Property | Value |
|----------|-------|
| Path | `/etc/aegis/secret.key` |
| Format | Raw 32 bytes (AES-256), hex-encoded (64 hex chars) |
| Permission | `0600`, owner `aegis` (or the user running aegis) |
| Group read | `0640` only if backup agent needs read access |
| Size | 64 hex characters (32 bytes) |

### Generation

```bash
# One-time generation:
head -c 32 /dev/urandom | xxd -p -c 64 > /etc/aegis/secret.key
chmod 0600 /etc/aegis/secret.key
```

Or, if Aegis runs without a dedicated user:

```bash
mkdir -p /etc/aegis
head -c 32 /dev/urandom | xxd -p -c 64 > /etc/aegis/secret.key
chmod 0600 /etc/aegis/secret.key
```

### Fallback Sources

| Source | Priority | Use Case |
|--------|----------|----------|
| `/etc/aegis/secret.key` | 1 (highest) | Default production path |
| `AEGIS_SECRET_KEY` env var | 2 | Container/Docker deployment |
| Config file `secret_key_path` | 3 | Custom path override |

If none found at startup: **error, refuse to start**. No auto-generation (would break across restarts).

### Rules

| Rule | Enforcement |
|------|-------------|
| Key never written to DB | Validate in code — never persist key to any DB table |
| Key never in logs | Sanitize all log output — replace key with `[REDACTED]` |
| Key never in API response | All `GET` gateway-link endpoints exclude key |
| Key never in error message | Error: "failed to decrypt secret" — never "invalid key <key>" |

### Key Lost Scenario

If `/etc/aegis/secret.key` is lost:

1. **All encrypted GatewayLink tokens become unrecoverable** — AES-GCM decryption will fail
2. **Recovery only from backup** — restore both `aegis.db` AND `secret.key` together
3. **No brute-force** — AES-256 is computationally infeasible to brute-force
4. **Mitigation:** The `create` API returns the raw token once. If the user saved it, they can recreate. If not, they must delete and recreate each GatewayLink.

---

## 5. DB Schema Migration

### Migration Number: 015

(Following existing migration 014 from spaces implementation.)

### Current Schema

```sql
CREATE TABLE IF NOT EXISTS gateway_links (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    target_url TEXT NOT NULL,
    auth_value TEXT NOT NULL,          -- plaintext — to be encrypted
    status TEXT DEFAULT 'active',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
```

### Target Schema

```sql
CREATE TABLE IF NOT EXISTS gateway_links (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    target_url TEXT NOT NULL,
    -- auth_value remains as the plaintext field but becomes DEPRECATED
    -- After migration: auth_value will be empty for migrated records
    auth_value TEXT DEFAULT '',
    -- New encrypted fields
    encrypted_secret BLOB,             -- AES-256-GCM ciphertext
    secret_nonce BLOB,                 -- AEAD nonce (12 bytes for GCM)
    secret_version INTEGER DEFAULT 0,  -- starts at 0, incremented on rotate
    secret_created_at TEXT,            -- RFC3339
    secret_rotated_at TEXT,            -- RFC3339, NULL until first rotate
    -- Existing fields unchanged
    status TEXT DEFAULT 'active',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
```

### Migration Behavior

**Up (encrypt existing):**

1. Add all new columns (`encrypted_secret`, `secret_nonce`, `secret_version`, `secret_created_at`, `secret_rotated_at`)
2. For each row where `auth_value != ''`:
   - Generate random 12-byte nonce
   - Encrypt `auth_value` with master key + nonce → `encrypted_secret`
   - Set `secret_nonce`, `secret_version = 0`, `secret_created_at = now()`
3. After all rows migrated: **clear `auth_value` to `''`** for all rows
4. Mark migration as irreversible (see Rollback below)

**Rollback:**

- **Irreversible if `auth_value` is cleared.** The plaintext is gone after step 3.
- **Before running migration:** user MUST take a SQLite backup
- **Rollback procedure:** restore from backup
  ```bash
  cp .aegis/aegis.db.backup .aegis/aegis.db
  ```
- **Alternative safe rollback:** skip step 3 (keep `auth_value` populated during a grace period). Mark `auth_value` as `DEPRECATED` in v1.8B-1 and remove in v1.8B-2. This allows reverting by simply dropping the new columns.

**Recommended approach for v1.8B-0:**

| Step | Action | Reversible? |
|------|--------|-------------|
| 1 | Add new columns | ✅ Yes (ALTER TABLE DROP COLUMN) |
| 2 | Encrypt and populate `encrypted_secret` | ✅ Yes (keep auth_value) |
| 3 | Clear `auth_value` only after N days | ❌ No (need backup to restore) |

**Implementation plan (to be done in v1.8B-1):**

- Migration 015 does steps 1 + 2
- Clears `auth_value` only if a flag `--migrate-secrets` is passed or after a grace period (v1.8B-2)
- All reads use `encrypted_secret` if available, fall back to `auth_value` deprecated field

### Model Changes

```go
type GatewayLinkRecord struct {
    // ... existing fields ...
    
    // DEPRECATED: kept for backward compatibility during migration
    // After migration 015, this field is no longer populated
    AuthValue string `json:"-"` // never serialized
    
    // New encrypted fields
    EncryptedSecret   []byte  `json:"-"` // never serialized
    SecretNonce       []byte  `json:"-"` // never serialized
    SecretVersion     int     `json:"secret_version"`
    SecretCreatedAt   string  `json:"secret_created_at,omitempty"`
    SecretRotatedAt   string  `json:"secret_rotated_at,omitempty"`
}
```

Note: `AuthValue`, `EncryptedSecret`, and `SecretNonce` all use `json:"-"` — they never appear in API responses. Only `SecretVersion` and timestamps are visible.

---

## 6. Runtime Behavior

### 6.1 Startup Sequence

```
Aegis startup
  ├── Load config
  ├── Open SQLite
  ├── Run migrations (015 ensures new columns exist)
  ├── Load master key
  │     ├── Try /etc/aegis/secret.key
  │     ├── Try AEGIS_SECRET_KEY env var
  │     ├── Try config.secret_key_path
  │     └── All fail → FATAL: "no secret key found"
  ├── Initialize secret service (encrypt/decrypt wrapper)
  └── Continue startup...
```

**If key is missing at startup:** Fatal error, process exits. This is intentional — running with encrypted data but no key would cause silent decryption failures during apply.

### 6.2 Create GatewayLink

```
POST /api/admin/v1/gateway-links
  ├── Generate random token (64 hex chars, same as current)
  ├── Encrypt token:
  │     ├── nonce = crypto/rand(12)
  │     ├── ciphertext = AES-256-GCM(key, nonce, token)
  │     └── encrypted_secret = ciphertext
  ├── Store:
  │     ├── encrypted_secret → BLOB
  │     ├── secret_nonce → BLOB
  │     ├── secret_version = 0
  │     ├── secret_created_at = now()
  │     └── auth_value = '' (cleared, deprecated)
  ├── Return raw token ONCE in response:
  │     {
  │       "id": "gwlink_abc123",
  │       "name": "server-b",
  │       "secret_version": 0,
  │       "secret_created_at": "2026-06-26T...",
  │       "raw_token": "a1b2c3d4...64hex..."   ← only time raw token is returned
  │     }
  └── Log: "gateway-link created: gwlink_abc123 (version 0)" — no raw token
```

**Important:** The raw token is returned **exactly once** in the create response. If the user loses it, they must rotate to get a new one.

### 6.3 Get / List GatewayLink

```
GET /api/admin/v1/gateway-links/{id}
GET /api/admin/v1/gateway-links
  ├── Return: id, name, description, target_url, status
  ├── Return: secret_version, secret_created_at, secret_rotated_at
  ├── NEVER: raw token, encrypted_secret, secret_nonce
  └── Response JSON:
      {
        "id": "gwlink_abc123",
        "name": "server-b",
        "target_url": "https://43.159.34.11:443",
        "status": "active",
        "secret_version": 0,
        "secret_created_at": "2026-06-26T10:00:00Z",
        "secret_rotated_at": null
      }
```

### 6.4 Update GatewayLink

```
PATCH /api/admin/v1/gateway-links/{id}
  ├── Only updates non-secret fields (name, description, target_url, status)
  ├── Does NOT touch encrypted_secret
  └── If secret changes needed → rotate endpoint
```

### 6.5 Render Caddy Config (Apply)

```
Planner.Plan()
  └── resolveRouteConfigWithService()
        └── For each route with GatewayLink:
              ├── Load GatewayLinkRecord
              ├── Decrypt:
              │     ├── key = master key from startup
              │     ├── nonce = record.secret_nonce
              │     ├── raw_token = AES-256-GCM-Decrypt(key, nonce, encrypted_secret)
              │     └── FAIL: return apply error → "failed to decrypt gateway-link secret"
              ├── Inject header:
              │     └── header_up X-Aegis-Gateway-Token <raw_token>
              └── Continue with config generation
```

**Decryption failure handling:**

| Scenario | Behavior |
|----------|----------|
| Key file missing at startup | Process exits — no startup |
| Key file changed/rotated | Decryption fails at apply → apply error with clear message |
| `encrypted_secret` is NULL | Fall back to `auth_value` (deprecated) with warning log |
| Both NULL/empty | Apply error: "gateway-link {id} has no secret" |
| Nonce/ciphertext corrupted | Apply error: "failed to decrypt gateway-link secret: cipher: message authentication failed" |

### 6.6 Rotate Secret

```
POST /api/admin/v1/gateway-links/{id}/rotate
  ├── Generate new random token
  ├── Encrypt with master key (new nonce)
  ├── Store:
  │     ├── encrypted_secret = new ciphertext
  │     ├── secret_nonce = new nonce
  │     ├── secret_version = old_version + 1
  │     ├── secret_rotated_at = now()
  ├── Mark pending apply (MarkPending)
  ├── Return raw token ONCE:
  │     {
  │       "id": "gwlink_abc123",
  │       "secret_version": 1,
  │       "raw_token": "e5f6g7h8...64hex..."
  │     }
  └── Log: "gateway-link rotated: gwlink_abc123 (version 0 → 1)"
```

**What rotation does NOT do:**
- Does NOT automatically apply — user must run `aegis apply` separately
- Does NOT update Caddy config in flight — only the next apply includes the new token
- Does NOT invalidate the old token immediately — both old and new may be valid until apply

**Downstream impact:** After rotation without apply, the gateway and downstream are out of sync. The downstream will reject requests with the old token. This is by design — rotation + apply should be done together.

### 6.7 Delete GatewayLink

```
DELETE /api/admin/v1/gateway-links/{id}
  ├── Delete the record normally
  ├── No special secret handling needed
  └── Secret is unrecoverable after deletion
```

### 6.8 Trace / Logs / Diagnostics

| Context | Secret Handling |
|---------|----------------|
| `aegis safety trace-egress` | Shows `secret_version` only. No raw token. |
| `aegis gateway-link show <id>` | Shows `secret_version`, not raw token. (Breaking change from current behavior.) |
| `aegis gateway-link show <id> --reveal-secret` | New flag — decrypts and shows raw token. Logs: WARN "gateway-link secret revealed by operator". |
| `GET /api/admin/v1/gateway-links/{id}/trace` | Shows `secret_version` only. |
| Log files | Never write raw token. Sanitize any error context that might contain secret material. |
| Error messages | "failed to decrypt gateway-link secret" — never include key or ciphertext in error |

---

## 7. API Contract Changes

### Breaking Changes

| Change | Version | Impact |
|--------|---------|--------|
| `GET /api/admin/v1/gateway-links` no longer returns `auth_value` | v1.8B-1 | Any consumer parsing `auth_value` breaks |
| `GET /api/admin/v1/gateway-links/{id}` no longer returns `auth_value` | v1.8B-1 | Same |
| `aegis gateway-link show <id>` no longer shows raw token | v1.8B-1 | CLI user must use `--reveal-secret` flag |
| GatewayLink create response includes `raw_token` field (new) | v1.8B-1 | Additive, non-breaking |

### Additive Changes

| Endpoint | Method | Description |
|----------|--------|-------------|
| `POST /api/admin/v1/gateway-links/{id}/rotate` | POST | Rotate secret, return new raw token once |

### Response Diff

**Before (v1.8A):**
```json
{
  "id": "gwlink_abc123",
  "auth_value": "a1b2c3d4e5f6...",
  "status": "active"
}
```

**After (v1.8B-1):**
```json
{
  "id": "gwlink_abc123",
  "secret_version": 0,
  "secret_created_at": "2026-06-26T10:00:00Z",
  "secret_rotated_at": null,
  "status": "active"
}
```

**Create response (v1.8B-1):**
```json
{
  "id": "gwlink_abc123",
  "secret_version": 0,
  "secret_created_at": "2026-06-26T10:00:00Z",
  "raw_token": "a1b2c3d4e5f6...",
  "status": "active"
}
```

---

## 8. Security Boundaries

### What This Solves

| Threat | Mitigated? | Detail |
|--------|:----------:|--------|
| SQLite file read → token leakage | ✅ **Yes** | Encrypted at rest with AES-256-GCM |
| Backup file → token leakage | ✅ **Yes** | Must also have secret.key |
| Clipboard/screen capture of `auth_value` in list | ✅ **Yes** | `auth_value` no longer returned in GET/list |
| Error log leaking plaintext | ✅ **Yes** | Sanitized error messages |

### What This Does NOT Solve

| Risk | Status | Why |
|------|--------|-----|
| Caddy rendered config contains raw token | ⚠️ **Accepted** | Caddyfile must have the plaintext token to send in HTTP headers. This is inherent to the static token model. |
| Process memory contains raw token | ⚠️ **Accepted** | Decrypted token exists in memory during apply. Memory dump would reveal it. |
| HMAC dynamic signing | ❌ **Deferred** | Static token is fundamentally weaker than HMAC-SHA256 request signing. See v1.8C or later. |
| Source IP allowlist | ❌ **v1.8B-2** | Separate concern — IP-level access control for downstream. |
| Full DB encryption | ❌ **Deferred** | SQLite Encryption Extension (SEE) or similar is out of scope. |
| HSM / TPM integration | ❌ **Deferred** | Overkill for personal gateway. |

### Caddyfile Plaintext Risk

The rendered Caddy config will contain:

```
reverse_proxy 43.159.34.11:80 {
    header_up X-Aegis-Gateway-Token a1b2c3d4e5f6...64hex...
}
```

This is **unavoidable** with the static token model. Anyone with read access to `/etc/caddy/Caddyfile` (or equivalent) can see the token.

**Mitigations:**
1. Caddyfile permissions should be `0600` or `0640`, owner caddy/root
2. Aegis should clean up rendered config files after reload (done already)
3. Long-term: HMAC dynamic signing eliminates this entirely

---

## 9. Deferred Items

| Item | Reason | Target |
|------|--------|--------|
| HMAC dynamic signing | Architectural change, not just DB hardening | v1.8C+ |
| Master key rotation | Low benefit for single-node personal gateway. Key rotation would require re-encrypting all secrets. | Deferred |
| Per-node derived keys | Not needed until multi-node is real (currently FAKE_ONLY) | Future |
| Full DB encryption | Overkill — only gateway-link secrets are sensitive | Deferred |
| Auto-apply after rotate | User should explicitly approve config changes | Deferred |
| Vault/HSM integration | Personal gateway does not need this | Never planned |
| Grace period for `auth_value` deprecation | Migration clears `auth_value` immediately. User should backup first. | Kept simple |

---

## 10. Acceptance Checklist

### Design Questions Answered

| # | Question | Answer |
|---|----------|--------|
| 1 | Why can't we do HMAC-only? | Raw token is needed to render `header_up X-Aegis-Gateway-Token` in Caddy config. HMAC is one-way — can't recover raw value. |
| 2 | Why encrypted token at rest? | Only scheme that both eliminates SQLite plaintext AND preserves ability to re-render Caddy config after restart. |
| 3 | Where does the master key come from? | `/etc/aegis/secret.key` (preferred), `AEGIS_SECRET_KEY` env var (fallback), or config file path (last resort). |
| 4 | How is DB migrated? | Migration 015 adds new columns, encrypts existing secrets in-place, clears `auth_value` irreversibly. Backup required before migration. |
| 5 | How do create/get/list/render/rotate handle secrets? | Create: encrypt + store, return raw once. Get/List: never return raw. Render: decrypt + inject header. Rotate: new token + version bump, return raw once. |
| 6 | What about Caddyfile raw token? | Accepted risk — Caddyfile inherently contains plaintext. Mitigated by file permissions and deferred HMAC signing. |
| 7 | What if key is lost? | Encrypted secrets are unrecoverable. Must restore both DB and key from backup. |
| 8 | What's deferred to v1.8B-1? | Actual migration code, encryption service, API changes, CLI `--reveal-secret`, rotate endpoint, Caddy render changes, tests. |

### Verification (post-implementation, v1.8B-1)

- [ ] SQLite file read no longer reveals `auth_value` plaintext
- [ ] Create GatewayLink returns raw token once
- [ ] Get/List GatewayLink return `secret_version` only, never raw token
- [ ] CLI `aegis gateway-link show <id>` shows `secret_version`, hides raw token
- [ ] CLI `aegis gateway-link show <id> --reveal-secret` shows raw token with warning log
- [ ] Apply decryption succeeds and Caddy config has correct `header_up`
- [ ] Apply decryption failure produces clear error (not a panic or silent skip)
- [ ] Migration 015 encrypts existing secrets and clears `auth_value`
- [ ] Migration rollback by backup restore works
- [ ] Missing key file at startup: fatal error with clear message
- [ ] Key file changed after startup: decryption fails at apply with clear error
- [ ] Rotate endpoint creates new encrypted token, increments version
- [ ] Logs never contain raw token (verified by grep)
- [ ] API responses never contain raw token except create and reveal
- [ ] All existing tests still pass
- [ ] `go vet` clean

---

**v1.8B-0 GatewayLink Secret Storage Decision: COMPLETE**

Next: v1.8B-1 — Implementation: encryption service, migration 015, API changes, CLI changes, rotate endpoint.
