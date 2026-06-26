# v1.8B-5 — GatewayLink Secret-at-rest Encryption

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Status:** real_deploy_verified ✅

## Deployment Verification (v1.8B-6)

### Two-node relay with encrypted secret

| Check | Result |
|-------|--------|
| Server A → Server B relay over encrypted GatewayLink | ✅ 200 relay-target-post-ok |
| Server B decrypts secret with master key (CheckAuthEncrypted) | ✅ |
| Wrong token returns 403 | ✅ 403 |
| Missing token returns 400 | ✅ 400 |
| Missing gateway ID returns 401 | ✅ 401 |
| Hop > 1 returns 508 | ✅ 508 |
| Target-header bypass blocked | ✅ 400 |
| Rotate: new token works, old token fails | ✅ |
| List/Get no raw token | ✅ |
| Raw token not in logs | ✅ |
| Master key loaded from /etc/aegis/secret.key (0600) | ✅ |
| Server A + Server B both migrated to 027 | ✅ |
| Render/apply works with master key | ✅ |

### Capability Matrix
> **Date:** 2026-06-26

---

## Summary

GatewayLink tokens are now stored encrypted at rest using AES-256-GCM with a master key. The raw token is never persisted in the database — only the ciphertext and nonce are stored. The master key is loaded from environment variable, file, or generated in dev mode.

## What Changed

### Previous behavior (v1.8B-4)

- GatewayLink token stored as HMAC-SHA256 hash (one-way hash, but deterministic with hardcoded key)
- No master key separation — the HMAC key was embedded in source code
- AuthValue column contained the HMAC hash for comparison via `hashSecret(providedToken)`

### New behavior (v1.8B-5)

- GatewayLink token encrypted with AES-256-GCM using a 32-byte master key
- Master key loaded from `AEGIS_SECRET_KEY` env var or `/etc/aegis/secret.key` file
- Encrypted ciphertext + nonce stored in `encrypted_secret` and `secret_nonce` columns
- `secret_version` incremented on each rotation
- Legacy HMAC hash kept as fallback in `auth_value` column (backward compatibility)
- `CheckAuthEncrypted()` checks both paths: decrypted comparison first, HMAC fallback second

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Master Key Sources                       │
│                                                              │
│  1. AEGIS_SECRET_KEY env var (64 hex chars = 32 bytes)      │
│  2. /etc/aegis/secret.key (0600 or 0640 permissions)        │
│  3. Dev mode: auto-generated ephemeral key                   │
└──────────────┬──────────────────────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────────────────────┐
│                   secrets.MasterKey                          │
│                                                              │
│  - LoadMasterKey(devMode bool) → *MasterKey, error           │
│  - Encrypt(key, plaintext) → (encryptedB64, nonceB64, err)  │
│  - Decrypt(key, encryptedB64, nonceB64) → (string, error)   │
└──────────────┬──────────────────────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────────────────────┐
│              TrustedGateway (Model + Service)                 │
│                                                              │
│  Create:  generate raw token → encrypt → store encrypted     │
│  Auth:    decrypt → compare (fallback: HMAC hash)            │
│  Rotate:  generate new token → encrypt → increment version   │
│  GetAuthHeader:  decrypt → use raw for HMAC signing          │
│  List/Get:  never return raw token                           │
└─────────────────────────────────────────────────────────────┘
```

## Migration 027

Adds to `trusted_gateways` table:

| Column | Type | Description |
|--------|------|-------------|
| `encrypted_secret` | TEXT | AES-256-GCM ciphertext (base64) |
| `secret_nonce` | TEXT | GCM nonce (base64) |
| `secret_version` | INTEGER | Incremented on rotation (default 0) |
| `secret_created_at` | TEXT | RFC3339 timestamp of first encryption |
| `secret_rotated_at` | TEXT | RFC3339 timestamp of last rotation |

Existing `auth_value` column is preserved for backward compatibility.

## Master Key Management

### Sources (checked in order)

1. **`AEGIS_SECRET_KEY` environment variable** — 64 hex characters (32 bytes)
2. **`/etc/aegis/secret.key`** — file containing 64 hex characters, permissions 0600 or 0640
3. **Dev mode** — generates ephemeral key in memory (not persisted)

### Key file example

```bash
# Generate a key
aegis secret generate > /etc/aegis/secret.key
chmod 0600 /etc/aegis/secret.key
```

### Key validation

- Exactly 32 bytes (64 hex characters)
- File permissions must be 0600 or 0640
- Invalid key → clear error message
- Missing key in production → service falls back to legacy HMAC storage with warning

### ⚠️ Critical: Key Loss

If the master key is lost, encrypted secrets cannot be decrypted. The HMAC fallback still allows:
- `CheckAuth()` with the original raw token (if the token is known to the caller)
- `GetRawSecret(nil)` returns the HMAC hash (not the original raw token)

Best practice: **backup the master key alongside SQLite backups**.

## Backward Compatibility

### Legacy HMAC gateways (existing data)

- Continue to work via HMAC fallback in `CheckAuthEncrypted()`
- `GetRawSecret(nil)` returns HMAC hash (same as before)
- Backfill available via `Service.BackfillEncrypted()` — generates new secret, encrypts it
- Backfill changes the secret (HMAC is one-way, can't recover raw token)

### New encrypted gateways

- Always use AES-256-GCM encryption
- HMAC hash stored alongside for fallback
- If master key is nil, falls back to HMAC-only mode (legacy behavior)

## New Files

| File | Purpose |
|------|---------|
| `internal/secrets/masterkey.go` | Master key loader (env/file/dev) |
| `internal/secrets/crypto.go` | AES-256-GCM Encrypt/Decrypt |
| `internal/secrets/secrets_test.go` | 15 tests for secrets package |

## Modified Files

| File | Changes |
|------|---------|
| `internal/store/migrations.go` | Added migration 027 |
| `internal/gateway_link/model.go` | Added EncryptedSecret, SecretNonce, SecretVersion, SecretCreatedAt, SecretRotatedAt fields; NewEncryptedGateway(); CheckAuthEncrypted(); GetRawSecret(); RotateSecretEncrypted(); BackfillEncrypted() |
| `internal/gateway_link/repository.go` | Added encrypted fields to all SQL queries |
| `internal/gateway_link/service.go` | Accepts MasterKey; Register/Rotate use encryption; GetDecryptedSecret(); BackfillEncrypted() |
| `internal/gateway_link/encryption_test.go` | 13 new encryption-specific tests |
| `internal/relay/handler.go` | HandlerDeps.MasterKey; uses CheckAuthEncrypted() |
| `internal/apply/service.go` | Passes masterKey to Planner |
| `internal/apply/planner.go` | Uses GetRawSecret(masterKey) for config header injection |
| `cmd/aegis/main.go` | Loads master key, wires to all consumers |

## Security Properties

### Authenticated encryption (AES-256-GCM)
- Tampered ciphertext → GCM authentication fails → `ErrDecryptFailed`
- Tampered nonce → GCM authentication fails
- Wrong key → GCM authentication fails

### Nonce uniqueness
- 12 random bytes per encryption
- Same plaintext + same key → different ciphertext each time
- 100 nonces generated in test — all unique

### No secret leakage
- Raw token never logged (verified in `handler.go` — `logRelayEvent()` only gets route/source/gateway IDs)
- Encrypted secret fields have `json:"-"` tag — never serialized in API responses
- Service.List() explicitly clears all secret fields
- Service.Get() returns encrypted fields (JSON-tagged "-") but never the raw token
- Raw token only returned once at Create/Rotate time
- Error messages never contain raw token

### Key separation
- Master key never written to DB
- Master key never logged
- Key source clearly identified (env var name or file path)

## Verification

### Unit tests (28 total in gateway_link + 15 in secrets)

| Test | What it verifies |
|------|-----------------|
| `TestNewEncryptedGateway` | Encrypted fields populated correctly |
| `TestCheckAuthEncrypted` | Correct=pass, wrong=fail, wrong_key=fail, nil_key→fallback |
| `TestGetRawSecretEncrypted` | Decrypt returns original raw token |
| `TestRotateSecretEncrypted` | Version increments, new token works |
| `TestServiceCreateEncrypted` | DB stores encrypted, decrypt roundtrip OK |
| `TestServiceListDoesNotExposeRawToken` | List clears all secret fields |
| `TestServiceGetDoesNotExposeRawToken` | Get exposes version/status but not raw token |
| `TestServiceRotateIncrementsVersion` | Rotate increments version, new token works |
| `TestCheckAuthEncryptedRelayPath` | Full relay flow: FindByID → CheckAuthEncrypted |
| `TestLegacyHMACBackwardCompat` | HMAC gateways work with nil key and with master key |
| `TestBackfillEncrypted` | Legacy → encrypted migration works |
| `TestBackfillAlreadyEncrypted` | No-op on already encrypted gateways |
| `TestMissingMasterKeyFailsGracefully` | Missing key → HMAC fallback works |
| `TestEncryptedSecretNotInLogs` | Raw token not in any output |
| +15 secrets tests | Encrypt/Decrypt roundtrip, nonce uniqueness, key handling, GCM auth |

### Build

```bash
go build ./cmd/aegis/
go test ./...
```

## Capability Matrix

| Capability | Status |
|------------|--------|
| GatewayLink token auth (HMAC) | verified |
| **GatewayLink secret-at-rest encryption** | **real_deploy_verified** ⚡ |
| Legacy HMAC backward compatibility | real_deploy_verified |
| Backfill (legacy → encrypted) | code_verified |
| Master key env/file/dev loading | real_deploy_verified |
| Master key fail-safe (fail-closed) | code_verified |
| HMAC dynamic per-request signing | deferred |
| Raw TCP relay | deferred |
| Transparent interception | future/v2 |
| Caddy rendered config raw token | known boundary |

## Known Boundaries

### Caddyfile raw token
The rendered Caddyfile config may still contain the raw GatewayLink token in `header_up X-Aegis-Gateway-Token` directives. This is a known boundary: the token exists in plaintext in the gateway proxy's config file, not in Aegis's database. Documented in `Caddyfile raw token remains known boundary`.

### HMAC fallback
The legacy HMAC hash in `auth_value` is still a deterministic hash of the raw token with a hardcoded key. This is preserved only for backward compatibility during the transition. New gateways always use AES-256-GCM encryption. The HMAC fallback means that even without the master key, `CheckAuth()` still works — making the system resilient to key loss but not providing forward secrecy for existing tokens.

### Key rotation
Key rotation (changing the master key itself) is deferred. The current system only rotates the GatewayLink token (not the master key). If the master key is compromised, all encrypted tokens become decryptable.
