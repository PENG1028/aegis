#!/usr/bin/env bash
# Aegis Update Script — safely update a running Aegis instance.
#
# Usage:
#   bash scripts/update.sh <target_ip> [ssh_user]
#
# What it does:
#   1. Build new binary locally
#   2. Health-check the target before updating
#   3. Stop service → backup binary + database (consistent, no WAL drift) → upload → start
#   4. Health-check after update
#   5. Auto-rollback (binary only) on failure
#
# Examples:
#   bash scripts/update.sh ${SERVER_B:?set SERVER_B env var}          # Update Server B
#   bash scripts/update.sh ${SERVER_A:?set SERVER_A env var}        # Update Server A
#   bash scripts/update.sh ${SERVER_A:?set SERVER_A env var} ubuntu

set -euo pipefail

TARGET_IP="${1:?Usage: $0 <target_ip> [ssh_user]}"
SSH_USER="${2:-ubuntu}"
BINARY="aegis"
BINARY_PATH="/usr/local/bin/${BINARY}"
DATA_DIR="/var/lib/aegis"
BACKUP_DIR="/var/lib/aegis/backups"
PANEL_PORT="7380"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
info()  { echo -e "${CYAN}[info]${NC}  $*"; }
ok()    { echo -e "${GREEN}[ ok ]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[warn]${NC}  $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $*"; exit 1; }

SSH_TARGET="${SSH_USER}@${TARGET_IP}"
SSH="ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new ${SSH_TARGET}"

echo ""
echo -e "${BOLD}================================================${NC}"
echo -e "${BOLD}  Aegis Update — ${TARGET_IP} (${TIMESTAMP})${NC}"
echo -e "${BOLD}================================================${NC}"
echo ""

# ─── Step 0: Build ───
info "Building Aegis binary (linux/amd64)..."
cd "$(dirname "$0")/.."

if [ -f "${BINARY}" ]; then
  # Check if binary is stale (> 1 hour old)
  if [ "$(find "${BINARY}" -mmin +60 2>/dev/null)" ]; then
    warn "Existing binary is >1h old. Rebuilding..."
    make build-linux 2>&1 | tail -3
  else
    info "Using existing binary (built $(stat -c %y "${BINARY}" 2>/dev/null || stat -f %Sm "${BINARY}" 2>/dev/null))"
  fi
else
  make build-linux 2>&1 | tail -3
fi

if [ ! -f "${BINARY}" ]; then
  fail "Build failed — binary not found"
fi
VERSION=$(./aegis version 2>/dev/null || echo "unknown")
ok "Binary ready: ${BINARY} (version: ${VERSION})"

# ─── Step 1: Pre-update health check ───
info "Pre-update health check..."
HTTP_CODE=$(${SSH} "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://127.0.0.1:${PANEL_PORT}/api/healthz" 2>/dev/null || echo "000")
if [ "${HTTP_CODE}" = "200" ]; then
  ok "Target healthy (HTTP ${HTTP_CODE})"
else
  warn "Target returned HTTP ${HTTP_CODE} — proceeding anyway"
fi

# Get current version from API
CURRENT_VERSION=$(${SSH} "curl -s --connect-timeout 3 http://127.0.0.1:${PANEL_PORT}/api/system/status 2>/dev/null" | python3 -c "import sys,json; print(json.load(sys.stdin).get('version','unknown'))" 2>/dev/null || echo "unknown")
info "Current running version: ${CURRENT_VERSION}"

# ─── Step 2: Stop service first ───
# Backup ordering matters: with WAL enabled (internal/store/sqlite.go), `cp` on
# aegis.db while Aegis is running leaves behind a backup that is missing
# transactions committed into the -wal file but not yet checkpointed. Stopping
# before backup makes the rollback target consistent.
info "Stopping Aegis gracefully (WAL consistency)..."
${SSH} "sudo systemctl stop aegis" || warn "systemctl stop returned non-zero"
sleep 2

# Verify stopped
if ${SSH} "systemctl is-active aegis 2>/dev/null" 2>/dev/null; then
  warn "Aegis still running. Force stopping..."
  ${SSH} "sudo systemctl kill aegis" || true
  sleep 2
fi
ok "Aegis stopped"

# ─── Step 3: Backup ───
info "Creating backup before update..."
${SSH} "sudo mkdir -p ${BACKUP_DIR}"

# Backup binary
${SSH} "sudo cp ${BINARY_PATH} ${BACKUP_DIR}/aegis.${TIMESTAMP}" 2>/dev/null || warn "Binary backup skipped (may not exist)"
ok "Binary backed up: ${BACKUP_DIR}/aegis.${TIMESTAMP}"

# Backup database (consistent state — service is stopped, WAL is checkpointed)
if ${SSH} "test -f ${DATA_DIR}/aegis.db" 2>/dev/null; then
  ${SSH} "sudo cp ${DATA_DIR}/aegis.db ${BACKUP_DIR}/aegis.${TIMESTAMP}.db"
  DB_SIZE=$(${SSH} "du -h ${DATA_DIR}/aegis.db | cut -f1" 2>/dev/null || echo "?")
  ok "Database backed up: ${BACKUP_DIR}/aegis.${TIMESTAMP}.db (${DB_SIZE})"
fi

# Backup config
if ${SSH} "test -f /etc/aegis/config.yaml" 2>/dev/null; then
  ${SSH} "sudo cp /etc/aegis/config.yaml ${BACKUP_DIR}/config.${TIMESTAMP}.yaml"
  ok "Config backed up"
fi

# ─── Step 4: Upload new binary ───
info "Uploading new binary (gzip compressed)..."

# Clean stale uploads from previous failed attempts
${SSH} "sudo pkill -9 -f 'tee ${BINARY_PATH}' 2>/dev/null; sudo pkill -9 -f 'tee /tmp/aegis' 2>/dev/null; sudo rm -f /tmp/aegis.upload.tmp" || true
sleep 1

LOCAL_SIZE=$(stat -c%s "${BINARY}" 2>/dev/null || echo "0")

# Upload to /tmp first (gzip to reduce transfer size ~22MB→~7MB).
# Atomic mv after success — no "Text file busy" window.
# If SSH drops, only /tmp/aegis.upload.tmp is affected, not the running binary.
gzip -c "${BINARY}" | ${SSH} "gunzip | sudo tee /tmp/aegis.upload.tmp > /dev/null" || {
  warn "Upload interrupted — cleaning up"
  ${SSH} "sudo rm -f /tmp/aegis.upload.tmp" 2>/dev/null || true
  fail "Upload failed. Check network and retry."
}

# Verify upload
REMOTE_SIZE=$(${SSH} "stat -c%s /tmp/aegis.upload.tmp" 2>/dev/null || echo "0")
if [ "${REMOTE_SIZE}" = "${LOCAL_SIZE}" ] && [ "${LOCAL_SIZE}" != "0" ]; then
  ok "Binary uploaded and verified (${REMOTE_SIZE} bytes)"
else
  ${SSH} "sudo rm -f /tmp/aegis.upload.tmp" 2>/dev/null || true
  fail "Binary size mismatch! Local=${LOCAL_SIZE} Remote=${REMOTE_SIZE}"
fi

# Atomic replace — mv on same filesystem is instant, no partial-write window
${SSH} "sudo mv /tmp/aegis.upload.tmp ${BINARY_PATH} && sudo chmod +x ${BINARY_PATH}"
ok "Binary installed atomically"

# ─── Rollback helper ───
# Restores the binary from the pre-upgrade backup and restarts the service.
# Does NOT touch the database: if the new version ran schema migrations, the
# old binary may not understand the newer schema, so a DB restore would be
# worse than leaving the data forward.
#
# Returns 0 if the service is healthy after rollback, 1 otherwise.
ROLLBACK_HEALTHY=0
do_rollback() {
  echo ""
  echo -e "${RED}================================================${NC}"
  echo -e "${RED}  UPDATE FAILED — auto-rolling back${NC}"
  echo -e "${RED}================================================${NC}"
  echo ""

  ${SSH} "sudo systemctl stop aegis 2>/dev/null || true"
  ${SSH} "sudo cp ${BACKUP_DIR}/aegis.${TIMESTAMP} ${BINARY_PATH} && sudo chmod +x ${BINARY_PATH} && sudo systemctl start aegis" || {
    warn "Rollback copy failed — manual intervention required:"
    echo "  ${SSH} 'sudo cp ${BACKUP_DIR}/aegis.${TIMESTAMP} ${BINARY_PATH} && sudo systemctl start aegis'"
    ROLLBACK_HEALTHY=1
    return
  }

  # Wait for API
  RETRIES=0
  ROLLBACK_HTTP="000"
  while [ ${RETRIES} -lt 5 ]; do
    ROLLBACK_HTTP=$(${SSH} "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://127.0.0.1:${PANEL_PORT}/api/healthz" 2>/dev/null || echo "000")
    if [ "${ROLLBACK_HTTP}" = "200" ]; then
      break
    fi
    RETRIES=$((RETRIES + 1))
    sleep 2
  done

  if [ "${ROLLBACK_HTTP}" = "200" ]; then
    ok "Rollback succeeded — service healthy on previous binary (HTTP 200)"
    warn ""
    warn "DB was NOT rolled back. If the new version ran schema migrations,"
    warn "the current DB may be on a newer schema than the old binary expects."
    warn "Pre-upgrade DB backup: ${BACKUP_DIR}/aegis.${TIMESTAMP}.db"
    ROLLBACK_HEALTHY=0
  else
    warn "Rollback service did not become healthy (HTTP ${ROLLBACK_HTTP})."
    warn "Manual intervention required:"
    echo "  ${SSH} 'sudo systemctl status aegis && sudo journalctl -u aegis --no-pager -n 50'"
    ROLLBACK_HEALTHY=1
  fi
}

# ─── Step 5: Start ───
info "Starting Aegis..."
${SSH} "sudo systemctl start aegis"
sleep 3

# ─── Step 6: Post-update health check ───
info "Post-update health check..."

# Check systemd status
if ! ${SSH} "systemctl is-active aegis" 2>/dev/null; then
  warn "Systemd: NOT active — checking journal..."
  ${SSH} "sudo journalctl -u aegis --no-pager -n 20" 2>/dev/null || true
  do_rollback
  if [ "${ROLLBACK_HEALTHY}" = "0" ]; then
    fail "Aegis failed to start — rolled back automatically to ${CURRENT_VERSION}."
  else
    fail "Aegis failed to start AND rollback failed — manual intervention required."
  fi
fi
ok "Systemd: active ✓"

# Check API
RETRIES=0
HTTP_CODE="000"
while [ ${RETRIES} -lt 5 ]; do
  HTTP_CODE=$(${SSH} "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://127.0.0.1:${PANEL_PORT}/api/healthz" 2>/dev/null || echo "000")
  if [ "${HTTP_CODE}" = "200" ]; then
    break
  fi
  RETRIES=$((RETRIES + 1))
  info "Waiting for API... (attempt ${RETRIES}/5, HTTP ${HTTP_CODE})"
  sleep 2
done

if [ "${HTTP_CODE}" != "200" ]; then
  do_rollback
  if [ "${ROLLBACK_HEALTHY}" = "0" ]; then
    fail "API not responding — rolled back automatically to ${CURRENT_VERSION}."
  else
    fail "API not responding AND rollback failed — manual intervention required."
  fi
fi
ok "API responding (HTTP ${HTTP_CODE}) ✓"

# Get new version
NEW_VERSION=$(${SSH} "curl -s --connect-timeout 3 http://127.0.0.1:${PANEL_PORT}/api/system/status 2>/dev/null" | python3 -c "import sys,json; print(json.load(sys.stdin).get('version','unknown'))" 2>/dev/null || echo "unknown")

# ─── Step 7: Clean old backups (keep last 5) ───
info "Cleaning old backups (keeping last 5)..."
${SSH} "cd ${BACKUP_DIR} && ls -t aegis.????????_?????? 2>/dev/null | tail -n +6 | xargs -r sudo rm" || true
${SSH} "cd ${BACKUP_DIR} && ls -t aegis.????????_??????.db 2>/dev/null | tail -n +6 | xargs -r sudo rm" || true

# ─── Done ───
echo ""
echo -e "${BOLD}================================================${NC}"
echo -e "${BOLD}  Update Complete!${NC}"
echo -e "${BOLD}================================================${NC}"
echo ""
echo -e "  ${BOLD}Target:${NC}       ${TARGET_IP}"
echo -e "  ${BOLD}Old version:${NC}  ${CURRENT_VERSION}"
echo -e "  ${BOLD}New version:${NC}  ${NEW_VERSION}"
echo -e "  ${BOLD}Backup:${NC}       ${BACKUP_DIR}/aegis.${TIMESTAMP}"
echo ""
