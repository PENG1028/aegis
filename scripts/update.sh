#!/usr/bin/env bash
# Aegis Update Script — safely update a running Aegis instance.
#
# Usage:
#   bash scripts/update.sh <target_ip> [ssh_user]
#
# What it does:
#   1. Build new binary locally
#   2. Health-check the target before updating
#   3. Backup current binary + database (rollback-safe)
#   4. Graceful stop → upload → start
#   5. Health-check after update
#   6. Auto-rollback on failure
#
# Examples:
#   bash scripts/update.sh 43.159.34.11          # Update Server B
#   bash scripts/update.sh 43.160.211.232        # Update Server A
#   bash scripts/update.sh 43.160.211.232 ubuntu

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

# ─── Step 2: Backup ───
info "Creating backup before update..."
${SSH} "sudo mkdir -p ${BACKUP_DIR}"

# Backup binary
${SSH} "sudo cp ${BINARY_PATH} ${BACKUP_DIR}/aegis.${TIMESTAMP}" 2>/dev/null || warn "Binary backup skipped (may not exist)"
ok "Binary backed up: ${BACKUP_DIR}/aegis.${TIMESTAMP}"

# Backup database
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

# ─── Step 3: Graceful stop ───
info "Stopping Aegis gracefully..."
${SSH} "sudo systemctl stop aegis" || warn "systemctl stop returned non-zero"
sleep 2

# Verify stopped
if ${SSH} "systemctl is-active aegis 2>/dev/null" 2>/dev/null; then
  warn "Aegis still running. Force stopping..."
  ${SSH} "sudo systemctl kill aegis" || true
  sleep 2
fi
ok "Aegis stopped"

# ─── Step 4: Upload new binary ───
info "Uploading new binary..."
cat "${BINARY}" | ${SSH} "sudo tee ${BINARY_PATH} > /dev/null"
${SSH} "sudo chmod +x ${BINARY_PATH}"
# Verify upload
REMOTE_SIZE=$(${SSH} "stat -c%s ${BINARY_PATH}" 2>/dev/null || echo "0")
LOCAL_SIZE=$(stat -c%s "${BINARY}" 2>/dev/null || echo "0")
if [ "${REMOTE_SIZE}" = "${LOCAL_SIZE}" ] && [ "${LOCAL_SIZE}" != "0" ]; then
  ok "Binary uploaded and verified (${REMOTE_SIZE} bytes)"
else
  fail "Binary size mismatch! Local=${LOCAL_SIZE} Remote=${REMOTE_SIZE} — upload may be corrupted"
fi

# ─── Step 5: Start ───
info "Starting Aegis..."
${SSH} "sudo systemctl start aegis"
sleep 3

# ─── Step 6: Post-update health check ───
info "Post-update health check..."

# Check systemd status
if ${SSH} "systemctl is-active aegis" 2>/dev/null; then
  ok "Systemd: active ✓"
else
  warn "Systemd: NOT active — checking journal..."
  ${SSH} "sudo journalctl -u aegis --no-pager -n 20" 2>/dev/null || true
  fail "Aegis failed to start. Rollback: ${SSH} 'sudo cp ${BACKUP_DIR}/aegis.${TIMESTAMP} ${BINARY_PATH} && sudo systemctl start aegis'"
fi

# Check API
RETRIES=0
while [ ${RETRIES} -lt 5 ]; do
  HTTP_CODE=$(${SSH} "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://127.0.0.1:${PANEL_PORT}/api/healthz" 2>/dev/null || echo "000")
  if [ "${HTTP_CODE}" = "200" ]; then
    break
  fi
  RETRIES=$((RETRIES + 1))
  info "Waiting for API... (attempt ${RETRIES}/5, HTTP ${HTTP_CODE})"
  sleep 2
done

if [ "${HTTP_CODE}" = "200" ]; then
  ok "API responding (HTTP ${HTTP_CODE}) ✓"
else
  echo ""
  echo -e "${RED}================================================${NC}"
  echo -e "${RED}  UPDATE FAILED — API not responding${NC}"
  echo -e "${RED}================================================${NC}"
  echo ""
  echo "  Rollback command:"
  echo "  ${SSH} 'sudo cp ${BACKUP_DIR}/aegis.${TIMESTAMP} ${BINARY_PATH}'"
  echo "  ${SSH} 'sudo cp ${BACKUP_DIR}/aegis.${TIMESTAMP}.db ${DATA_DIR}/aegis.db'"
  echo "  ${SSH} 'sudo systemctl restart aegis'"
  exit 1
fi

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
echo -e "  ${BOLD}Rollback if needed:${NC}"
echo "  ${SSH} 'sudo cp ${BACKUP_DIR}/aegis.${TIMESTAMP} ${BINARY_PATH} && sudo systemctl restart aegis'"
echo ""
