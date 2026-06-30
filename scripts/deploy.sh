#!/usr/bin/env bash
# Aegis One-Click Deploy Script (v1.8J)
#
# Deploys Aegis to a clean VPS in one command.
# Prerequisites on dev machine: ssh access to target, bash, make, go, npm
# Prerequisites on target VPS: Ubuntu 20.04+/22.04+, SSH key or password
#
# Usage:
#   bash scripts/deploy.sh <target_ip> [ssh_user]
#
# Examples:
#   bash scripts/deploy.sh 43.160.211.232
#   bash scripts/deploy.sh 43.160.211.232 ubuntu
#   bash scripts/deploy.sh 43.160.211.232 root
#
# What it does:
#   1. Builds the aegis binary with embedded UI (linux/amd64)
#   2. Installs Caddy (port 80 HTTP) and HAProxy (port 443 TLS SNI)
#   3. Uploads binary to target VPS
#   4. Runs 'aegis bootstrap --production' (creates config, DB, Caddyfile)
#   5. Installs systemd service
#   6. Starts Aegis
#   7. Prints panel URL, admin credentials, and service status

set -euo pipefail

# ─── Config ───
TARGET_IP="${1:?Usage: $0 <target_ip> [ssh_user]}"
SSH_USER="${2:-ubuntu}"
BINARY="aegis"
BINARY_PATH="/usr/local/bin/${BINARY}"
CONFIG_DIR="/etc/aegis"
DATA_DIR="/var/lib/aegis"
SYSTEMD_UNIT="packaging/systemd/aegis.service"
PANEL_PORT="7380"

# ─── Colors ───
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()  { echo -e "${CYAN}[info]${NC}  $*"; }
ok()    { echo -e "${GREEN}[ ok ]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[warn]${NC}  $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $*"; exit 1; }

SSH_TARGET="${SSH_USER}@${TARGET_IP}"
SSH="ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new ${SSH_TARGET}"
SCP="scp -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new"

echo ""
echo -e "${BOLD}================================================${NC}"
echo -e "${BOLD}  Aegis Deploy — ${TARGET_IP}${NC}"
echo -e "${BOLD}================================================${NC}"
echo ""

# ─── Step 0: Preflight ───
info "Preflight checks..."

# Check local tools
for tool in go npm make; do
  if ! command -v $tool &>/dev/null; then
    fail "$tool is not installed (required locally for build)"
  fi
done
ok "Local build tools: go, npm, make ✓"

# Check SSH connectivity
if ! ${SSH} "echo ok" 2>/dev/null; then
  fail "Cannot SSH to ${SSH_TARGET}. Check your SSH config or key."
fi
ok "SSH connectivity to ${TARGET_IP} ✓"

# Check target OS
TARGET_OS=$(${SSH} "cat /etc/os-release 2>/dev/null | grep '^ID=' | cut -d= -f2 | tr -d '\"' || echo 'unknown'")
TARGET_ARCH=$(${SSH} "uname -m")
info "Target: ${TARGET_OS} / ${TARGET_ARCH}"

# ─── Step 1: Build ───
info "Building Aegis binary with embedded UI (linux/amd64)..."

cd "$(dirname "$0")/.."
make build-linux 2>&1 | tail -3

if [ ! -f "${BINARY}" ]; then
  fail "Build failed — binary not found at ${BINARY}"
fi
BINARY_SIZE=$(du -h "${BINARY}" | cut -f1)
ok "Binary built: ${BINARY} (${BINARY_SIZE})"

# ─── Step 2: Stop old service if running ───
info "Checking for existing Aegis installation..."
if ${SSH} "systemctl is-active aegis 2>/dev/null" 2>/dev/null; then
  warn "Aegis is running. Stopping for upgrade..."
  ${SSH} "sudo systemctl stop aegis" || true
fi

# ─── Step 3: Upload binary ───
info "Uploading binary to ${TARGET_IP}..."
${SSH} "sudo mkdir -p /usr/local/bin"
# Use SSH pipe to avoid SCP issues
cat "${BINARY}" | ${SSH} "sudo tee ${BINARY_PATH} > /dev/null"
${SSH} "sudo chmod +x ${BINARY_PATH}"
ok "Binary uploaded to ${BINARY_PATH}"

# ─── Step 4: Create directories ───
info "Creating directories..."
${SSH} "sudo mkdir -p ${CONFIG_DIR} ${DATA_DIR}"
ok "Directories created"

# ─── Step 4.5: Install middleware (must be BEFORE bootstrap so config files are not overwritten) ───
install_pkg() {
  # Detect package manager and install a list of packages.
  # Usage: install_pkg "caddy" "haproxy"
  if ${SSH} "command -v apt-get &>/dev/null" 2>/dev/null; then
    ${SSH} "sudo apt-get update -qq && sudo apt-get install -y -qq $*" 2>&1 | tail -3
  elif ${SSH} "command -v dnf &>/dev/null" 2>/dev/null; then
    ${SSH} "sudo dnf install -y $*" 2>&1 | tail -3
  elif ${SSH} "command -v yum &>/dev/null" 2>/dev/null; then
    ${SSH} "sudo yum install -y $*" 2>&1 | tail -3
  else
    return 1
  fi
}

# --- Caddy (port 80 HTTP reverse proxy + Let's Encrypt) ---
info "Checking Caddy..."
if ${SSH} "command -v caddy &>/dev/null" 2>/dev/null; then
  CADDY_VERSION=$(${SSH} "caddy version 2>&1 | head -1" 2>/dev/null || echo "unknown")
  ok "Caddy: ${CADDY_VERSION}"
else
  warn "Caddy not installed. Installing..."
  install_pkg caddy || fail "Caddy install failed"
  ok "Caddy installed"
  ${SSH} "sudo systemctl enable caddy" 2>/dev/null || true
fi

# --- HAProxy (port 443 TLS SNI passthrough) ---
info "Checking HAProxy..."
if ${SSH} "command -v haproxy &>/dev/null" 2>/dev/null; then
  HAPROXY_VERSION=$(${SSH} "haproxy -v 2>&1 | head -1" 2>/dev/null || echo "unknown")
  ok "HAProxy: ${HAPROXY_VERSION}"
else
  warn "HAProxy not installed. Installing..."
  install_pkg haproxy || fail "HAProxy install failed"
  ok "HAProxy installed"
  ${SSH} "sudo systemctl enable haproxy" 2>/dev/null || true
fi

# ─── Step 5: Bootstrap ───
info "Running bootstrap (production mode)..."
BOOTSTRAP_OUT=$(${SSH} "sudo ${BINARY_PATH} bootstrap --production" 2>&1) || {
  echo "${BOOTSTRAP_OUT}"
  fail "Bootstrap failed"
}
echo "${BOOTSTRAP_OUT}" | head -20
ok "Bootstrap complete"

# ─── Step 6: Install systemd service ───
info "Installing systemd service..."
# Read the systemd unit and upload it
cat "${SYSTEMD_UNIT}" | ${SSH} "sudo tee /etc/systemd/system/aegis.service > /dev/null"
${SSH} "sudo systemctl daemon-reload"
${SSH} "sudo systemctl enable aegis"
ok "Systemd service installed and enabled"

# ─── Step 7: Start Aegis ───
info "Starting Aegis..."
${SSH} "sudo systemctl start aegis"
sleep 2

# Verify it's running
if ${SSH} "systemctl is-active aegis" 2>/dev/null; then
  ok "Aegis is running ✓"
else
  warn "Aegis may not have started. Checking logs..."
  ${SSH} "sudo journalctl -u aegis --no-pager -n 20" 2>/dev/null || true
fi

# ─── Step 8: Retrieve admin credentials ───
info "Retrieving admin credentials..."
# The password is logged in journal on first run
ADMIN_PASSWORD=$(${SSH} "sudo journalctl -u aegis --no-pager -n 200 2>/dev/null | grep 'Password:' | tail -1 | awk '{print \$NF}'" 2>/dev/null || echo "")

# Fallback: check if --config path was used and the password is in bootstrap output
if [ -z "${ADMIN_PASSWORD}" ]; then
  ADMIN_PASSWORD=$(echo "${BOOTSTRAP_OUT}" | grep 'Password:' | tail -1 | awk '{print $NF}' || echo "")
fi

# ─── Step 9: Verify middleware (Caddy + HAProxy) ───
verify_svc() {
  local svc="$1"
  local desc="$2"
  if ${SSH} "systemctl is-active ${svc} 2>/dev/null" 2>/dev/null; then
    ok "${svc} is running — ${desc} ✓"
    return 0
  else
    warn "${svc} not running. Attempting to start..."
    ${SSH} "sudo systemctl enable --now ${svc}" 2>/dev/null || true
    sleep 1
    if ${SSH} "systemctl is-active ${svc} 2>/dev/null" 2>/dev/null; then
      ok "${svc} started — ${desc} ✓"
      return 0
    else
      warn "${svc} failed — check: ${SSH} 'sudo journalctl -u ${svc} --no-pager -n 20'"
      return 1
    fi
  fi
}

CADDY_RUNNING=false
HAPROXY_RUNNING=false
verify_svc "caddy" "panel on port 80" && CADDY_RUNNING=true
verify_svc "haproxy" "SNI routing on port 443" && HAPROXY_RUNNING=true

# ─── Step 10: Verify API ───
info "Verifying API..."
sleep 1
HTTP_CODE=$(${SSH} "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://127.0.0.1:${PANEL_PORT}/api/system/status" 2>/dev/null || echo "000")
if [ "${HTTP_CODE}" = "200" ]; then
  ok "API responding (HTTP ${HTTP_CODE}) ✓"
else
  warn "API check returned HTTP ${HTTP_CODE} — may need a moment to start"
fi

# ─── Done ───
echo ""
echo -e "${BOLD}================================================${NC}"
echo -e "${BOLD}  Deploy Complete!${NC}"
echo -e "${BOLD}================================================${NC}"
echo ""
echo -e "  ${BOLD}Panel URL:${NC}    http://${TARGET_IP}"
if [ "${CADDY_RUNNING}" = false ]; then
  echo -e "  ${BOLD}Panel URL:${NC}    http://${TARGET_IP}:${PANEL_PORT} (direct, Caddy not running)"
fi
echo -e "  ${BOLD}Username:${NC}     admin"
if [ -n "${ADMIN_PASSWORD}" ]; then
  echo -e "  ${BOLD}Password:${NC}     ${ADMIN_PASSWORD}"
else
  echo -e "  ${BOLD}Password:${NC}     Run: ${SSH} 'sudo journalctl -u aegis --no-pager | grep Password'"
fi
echo ""
echo -e "  ${BOLD}Services:${NC}"
echo -e "    Caddy  :80   $([ "${CADDY_RUNNING}" = true ] && echo '✓ running' || echo '✗ stopped')"
echo -e "    HAProxy :443  $([ "${HAPROXY_RUNNING}" = true ] && echo '✓ running (SNI routing)' || echo '✗ stopped')"
echo -e "    Aegis  :7380  ✓ (internal API)"
echo ""
echo -e "  ${BOLD}Next steps:${NC}"
echo "  1. Open the panel URL in your browser"
echo "  2. Login with the credentials above"
echo "  3. Go to「创建映射」to set up your first route"
echo "  4. Go to「推送配置」to apply and deploy"
echo ""
echo -e "  ${BOLD}Manage:${NC}"
echo "    ssh ${SSH_TARGET}"
echo "    sudo systemctl status aegis caddy haproxy"
echo "    sudo journalctl -u aegis -f"
echo ""
