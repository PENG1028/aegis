#!/usr/bin/env bash
# Aegis One-Click Deploy Script (v1.8L)
#
# Deploys Aegis to a clean VPS in one command.
# Prerequisites on dev machine: ssh access to target, bash, go, npm
# Prerequisites on target VPS: Ubuntu 20.04+/22.04+, SSH key or password
#
# Usage:
#   bash scripts/deploy.sh <target_ip> [ssh_user]
#
# Examples:
#   bash scripts/deploy.sh <SERVER_A_IP>
#   bash scripts/deploy.sh <SERVER_A_IP> ubuntu
#
# What it does:
#   1. Builds the aegis binary with embedded UI (linux/amd64)
#   2. Installs Caddy (port 80/443 for HTTP + Let's Encrypt TLS)
#   3. Uploads binary to target VPS
#   4. Runs 'aegis bootstrap --production' (creates config, DB, Caddyfile)
#   5. Fixes Caddyfile permissions (root:caddy 0640)
#   6. Installs systemd service
#   7. Starts Aegis + Caddy
#   8. Verifies: DNS, Caddy, Aegis API
#   9. Prints panel URL, admin credentials, and guided next steps

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
SSH="ssh -C -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new ${SSH_TARGET}"

echo ""
echo -e "${BOLD}================================================${NC}"
echo -e "${BOLD}  Aegis Deploy — ${TARGET_IP}${NC}"
echo -e "${BOLD}================================================${NC}"
echo ""

# ─── Step 0: Preflight ───
info "Preflight checks..."

# Check local tools
for tool in go npm; do
  if ! command -v $tool &>/dev/null; then
    fail "$tool is not installed (required locally for build)"
  fi
done
ok "Local build tools: go, npm"

# Check SSH connectivity
if ! ${SSH} "echo ok" 2>/dev/null; then
  fail "Cannot SSH to ${SSH_TARGET}. Check your SSH config or key."
fi
ok "SSH connectivity to ${TARGET_IP}"

# Check target OS
TARGET_OS=$(${SSH} "cat /etc/os-release 2>/dev/null | grep '^ID=' | cut -d= -f2 | tr -d '\"' || echo 'unknown'")
TARGET_ARCH=$(${SSH} "uname -m")
info "Target: ${TARGET_OS} / ${TARGET_ARCH}"

# ─── Step 1: Build ───
info "Building Aegis binary with embedded UI (linux/amd64)..."

cd "$(dirname "$0")/.."

# Build UI
(cd ui && npm run build 2>&1) | tail -3
# Embed UI
rm -rf internal/uiassets/dist
cp -r ui/dist internal/uiassets/dist
# Build Go binary
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o ${BINARY} ./cmd/aegis/

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
cat "${BINARY}" | ${SSH} "sudo tee ${BINARY_PATH} > /dev/null"
${SSH} "sudo chmod +x ${BINARY_PATH}"
ok "Binary uploaded to ${BINARY_PATH}"

# ─── Step 4: Create directories ───
info "Creating directories..."
${SSH} "sudo mkdir -p ${CONFIG_DIR} ${DATA_DIR}"
ok "Directories created"

# ─── Step 4.5: Install Caddy (port 80 HTTP + Let's Encrypt TLS) ───
install_pkg() {
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

info "Checking Caddy..."
if ${SSH} "command -v caddy &>/dev/null" 2>/dev/null; then
  CADDY_VERSION=$(${SSH} "caddy version 2>&1 | head -1" 2>/dev/null || echo "unknown")
  ok "Caddy: ${CADDY_VERSION}"
else
  warn "Caddy not installed. Installing..."
  install_pkg caddy || fail "Caddy install failed"
  ok "Caddy installed"
fi
${SSH} "sudo systemctl enable caddy" 2>/dev/null || true

# NOTE: HAProxy is NOT installed by default. It's only needed for L4 TLS SNI
# passthrough (routing TLS traffic to backends without decryption). If you need
# SNI passthrough later, install via: aegis doctor → 诊断 → 安装 HAProxy

# ─── Step 5: Bootstrap ───
info "Running bootstrap (production mode)..."
BOOTSTRAP_OUT=$(${SSH} "sudo ${BINARY_PATH} bootstrap --production" 2>&1) || {
  echo "${BOOTSTRAP_OUT}"
  fail "Bootstrap failed"
}
echo "${BOOTSTRAP_OUT}" | head -20
ok "Bootstrap complete"

# ─── Step 5.5: Fix Caddyfile permissions ───
# Bootstrap writes Caddyfile as root:root 0600, but Caddy runs as the `caddy` user.
# Fix: chown root:caddy, chmod 0640 (group-readable, not world-readable —
# Caddyfile may contain Gateway Link HMAC tokens).
info "Fixing Caddyfile permissions..."
${SSH} "sudo chown root:caddy /etc/caddy/Caddyfile 2>/dev/null || sudo chown root:caddy /etc/caddy/Caddyfile 2>/dev/null || true"
${SSH} "sudo chmod 640 /etc/caddy/Caddyfile 2>/dev/null || true"
# Verify Caddy can read it
if ${SSH} "sudo -u caddy test -r /etc/caddy/Caddyfile 2>/dev/null" 2>/dev/null; then
  ok "Caddyfile readable by caddy user"
else
  warn "Caddy may not be able to read Caddyfile. If Caddy fails to start, run:"
  warn "  ssh ${SSH_TARGET} 'sudo chown root:caddy /etc/caddy/Caddyfile && sudo chmod 640 /etc/caddy/Caddyfile'"
fi

# ─── Step 6: Install systemd service ───
info "Installing systemd service..."
cat "${SYSTEMD_UNIT}" | ${SSH} "sudo tee /etc/systemd/system/aegis.service > /dev/null"
${SSH} "sudo systemctl daemon-reload"
${SSH} "sudo systemctl enable aegis"
ok "Systemd service installed and enabled"

# ─── Step 7: Start Aegis ───
info "Starting Aegis..."
${SSH} "sudo systemctl start aegis"
sleep 2

if ${SSH} "systemctl is-active aegis" 2>/dev/null; then
  ok "Aegis is running"
else
  warn "Aegis may not have started. Checking logs..."
  ${SSH} "sudo journalctl -u aegis --no-pager -n 20" 2>/dev/null || true
fi

# ─── Step 8: Retrieve admin credentials ───
info "Retrieving admin credentials..."
ADMIN_PASSWORD=$(${SSH} "sudo journalctl -u aegis --no-pager -n 200 2>/dev/null | grep 'Password:' | tail -1 | awk '{print \$NF}'" 2>/dev/null || echo "")
if [ -z "${ADMIN_PASSWORD}" ]; then
  ADMIN_PASSWORD=$(echo "${BOOTSTRAP_OUT}" | grep 'Password:' | tail -1 | awk '{print $NF}' || echo "")
fi

# ─── Step 9: Start Caddy and verify ───
info "Starting Caddy..."
# Validate Caddy config first
if ${SSH} "sudo caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile" 2>/dev/null; then
  ok "Caddy config valid"
else
  warn "Caddy config validation failed — Caddy may not start correctly"
fi

${SSH} "sudo systemctl start caddy" 2>/dev/null || true
sleep 2

CADDY_OK=false
if ${SSH} "systemctl is-active caddy" 2>/dev/null; then
  ok "Caddy is running on port 80"
  CADDY_OK=true
else
  warn "Caddy failed to start. Check: ${SSH} 'sudo journalctl -u caddy --no-pager -n 20'"
fi

# ─── Step 10: Verify DNS ───
info "Verifying DNS resolution..."
if ${SSH} "host acme-v02.api.letsencrypt.org 2>/dev/null | head -1" 2>/dev/null; then
  ok "DNS resolution working — Let's Encrypt TLS will work"
else
  warn "DNS resolution FAILED — Let's Encrypt will not be able to issue certificates!"
  warn "  Fix: ${SSH} 'echo nameserver 1.1.1.1 | sudo tee /etc/resolv.conf'"
fi

# ─── Step 11: Verify API ───
info "Verifying Aegis API..."
sleep 1
HTTP_CODE=$(${SSH} "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://127.0.0.1:${PANEL_PORT}/api/system/status" 2>/dev/null || echo "000")
if [ "${HTTP_CODE}" = "200" ] || [ "${HTTP_CODE}" = "401" ]; then
  ok "Aegis API responding"
else
  warn "API check returned HTTP ${HTTP_CODE} — may need a moment to start"
fi

# ─── Step 12: Verify panel via Caddy ───
info "Verifying panel access via Caddy..."
PANEL_CODE=$(${SSH} "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://127.0.0.1:80/ -H 'Host: ${TARGET_IP}'" 2>/dev/null || echo "000")
if [ "${PANEL_CODE}" = "200" ] || [ "${PANEL_CODE}" = "401" ] || [ "${PANEL_CODE}" = "302" ] || [ "${PANEL_CODE}" = "301" ]; then
  ok "Panel reachable via Caddy :80"
else
  warn "Panel returned HTTP ${PANEL_CODE} via Caddy — check Caddyfile"
fi

# ─── Done ───
echo ""
echo -e "${BOLD}================================================${NC}"
echo -e "${BOLD}  Deploy Complete!${NC}"
echo -e "${BOLD}================================================${NC}"
echo ""

if [ "${CADDY_OK}" = true ]; then
  echo -e "  ${BOLD}Panel:${NC}       http://${TARGET_IP}"
else
  echo -e "  ${BOLD}Panel:${NC}       http://${TARGET_IP}:${PANEL_PORT} (direct — Caddy not running)"
fi
echo -e "  ${BOLD}Username:${NC}     admin"
if [ -n "${ADMIN_PASSWORD}" ]; then
  echo -e "  ${BOLD}Password:${NC}     ${ADMIN_PASSWORD}"
else
  echo -e "  ${BOLD}Password:${NC}     Run: ${SSH} 'sudo journalctl -u aegis --no-pager | grep Password'"
fi
echo ""
echo -e "  ${CYAN}┌─────────────────────────────────────────────┐${NC}"
echo -e "  ${CYAN}│${NC}  ${BOLD}Next: Enable HTTPS for the panel${NC}            ${CYAN}│${NC}"
echo -e "  ${CYAN}│${NC}                                              ${CYAN}│${NC}"
echo -e "  ${CYAN}│${NC}  1. Point a domain (or *.$DOMAIN) DNS to ${TARGET_IP}  ${CYAN}│${NC}"
echo -e "  ${CYAN}│${NC}  2. Open the panel URL above                ${CYAN}│${NC}"
echo -e "  ${CYAN}│${NC}  3. Login → 设置 → 面板域名 · TLS           ${CYAN}│${NC}"
echo -e "  ${CYAN}│${NC}  4. Enter your domain + email → Save        ${CYAN}│${NC}"
echo -e "  ${CYAN}│${NC}  5. Panel auto-upgrades to HTTPS             ${CYAN}│${NC}"
echo -e "  ${CYAN}├─────────────────────────────────────────────┤${NC}"
echo -e "  ${CYAN}│${NC}  ${BOLD}Then create your first route:${NC}               ${CYAN}│${NC}"
echo -e "  ${CYAN}│${NC}  1. 创建映射 → Service + Endpoint + Route    ${CYAN}│${NC}"
echo -e "  ${CYAN}│${NC}  2. 推送配置 → Apply → Caddy 自动加载        ${CYAN}│${NC}"
echo -e "  ${CYAN}└─────────────────────────────────────────────┘${NC}"
echo ""
echo -e "  ${BOLD}Services:${NC}"
echo -e "    Caddy  :80   $([ "${CADDY_OK}" = true ] && echo '✓ running' || echo '✗ check logs')"
echo -e "    Aegis  :7380 ✓ (internal API)"
echo -e "    HAProxy      — (not installed — add via UI if SNI passthrough needed)"
echo ""
echo -e "  ${BOLD}Manage:${NC}"
echo "    ssh ${SSH_TARGET}"
echo "    sudo systemctl status aegis caddy"
echo "    sudo journalctl -u aegis -f"
echo ""
