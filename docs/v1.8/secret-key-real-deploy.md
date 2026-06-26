# v1.8B-6 — Secret Key Real Deployment

> **Phase:** v1.8B — Route Path Safety & Secret Hardening
> **Date:** 2026-06-26

---

## Deployment Summary

Master key deployed to both production servers for GatewayLink secret-at-rest encryption.

### Key Configuration

| Server | Key Source | Path | Permissions | Owner |
|--------|-----------|------|-------------|-------|
| Server A (<SERVER_A_IP>) | file | `/etc/aegis/secret.key` | 0600 | root:root |
| Server B (<SERVER_B_IP>) | file | `/etc/aegis/secret.key` | 0600 | root:root |

Both servers use the **same master key** (32 bytes = 64 hex chars).

### Binary Version

| Server | Binary | Size | Build |
|--------|--------|------|-------|
| Server A | `/home/ubuntu/aegis` | 17,386,280 bytes | v1.8B-6 (migration 027) |
| Server B | `/home/ubuntu/aegis` | 17,386,280 bytes | v1.8B-6 (migration 027) |

### Migration Status

Both servers: migration 027 `add_gateway_link_encryption` **applied**.

### Key File Setup

```bash
# Generate key (done once, distributed to both servers)
openssl rand -hex 32 > /etc/aegis/secret.key
chmod 0600 /etc/aegis/secret.key
chown root:root /etc/aegis/secret.key
```

### Verification

```
Server A:
  Key: 65 bytes, 600
  migration 027 (add_gateway_link_encryption) applied
  Aegis API server starting on 127.0.0.1:9000

Server B:
  Key: 65 bytes, 600
  migration 027 (add_gateway_link_encryption) applied
  Aegis API server starting on 127.0.0.1:9000
```

## Security Notes

- Key value is **redacted** from this document
- Key is backed up alongside SQLite database backups
- If key is lost, encrypted secrets become undecryptable (legacy HMAC fallback still works for existing gateways)
- Key is read once at process startup, stays in memory for the lifetime of the process
- The `aegis` process runs as root (able to read `/etc/aegis/secret.key`)
- CLI commands run as `ubuntu` user display a warning about key unavailability but continue in legacy mode
