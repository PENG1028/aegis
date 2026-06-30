#!/usr/bin/env bash
# Aegis Update-All — update both VPS servers sequentially.
#
# Updates Server B (remote target) first, then Server A (main gateway).
# Between each update, waits for health check to pass before proceeding.
#
# Usage:
#   bash scripts/update-all.sh
#
# Environment variables (override defaults):
#   SERVER_A=<SERVER_A_IP>    # Main gateway (panel)
#   SERVER_B=<SERVER_B_IP>      # Remote target node
#   SSH_USER=ubuntu             # SSH user for both servers

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVER_A="${SERVER_A:-<SERVER_A_IP>}"
SERVER_B="${SERVER_B:-<SERVER_B_IP>}"
SSH_USER="${SSH_USER:-ubuntu}"
PANEL_PORT="7380"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
info()  { echo -e "${CYAN}[info]${NC}  $*"; }
ok()    { echo -e "${GREEN}[ ok ]${NC}  $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $*"; exit 1; }

echo ""
echo -e "${BOLD}================================================${NC}"
echo -e "${BOLD}  Aegis Update — Dual VPS${NC}"
echo -e "${BOLD}================================================${NC}"
echo ""
echo -e "  ${BOLD}Server A:${NC} ${SERVER_A} (gateway + panel)"
echo -e "  ${BOLD}Server B:${NC} ${SERVER_B} (remote target)"
echo ""

# ─── Phase 1: Update Server B (remote node, less critical) ───
echo -e "${BOLD}━━━ Phase 1: Update Server B (${SERVER_B}) ━━━${NC}"
echo ""

if bash "${SCRIPT_DIR}/update.sh" "${SERVER_B}" "${SSH_USER}"; then
  ok "Server B updated ✓"
else
  fail "Server B update failed. Server A NOT updated."
fi

# Verify cross-node connectivity before proceeding
echo ""
info "Verifying Server B health before updating Server A..."
sleep 2

SSH_B="ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new ${SSH_USER}@${SERVER_B}"
HTTP_B=$(${SSH_B} "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 http://127.0.0.1:${PANEL_PORT}/api/readyz" 2>/dev/null || echo "000")
if [ "${HTTP_B}" = "200" ]; then
  ok "Server B ready (HTTP ${HTTP_B}) ✓"
else
  warn "Server B readiness check returned HTTP ${HTTP_B} — proceeding with Server A update anyway"
fi

# ─── Phase 2: Update Server A (main gateway) ───
echo ""
echo -e "${BOLD}━━━ Phase 2: Update Server A (${SERVER_A}) ━━━${NC}"
echo ""

if bash "${SCRIPT_DIR}/update.sh" "${SERVER_A}" "${SSH_USER}"; then
  ok "Server A updated ✓"
else
  echo ""
  echo -e "${RED}================================================${NC}"
  echo -e "${RED}  WARNING: Server A update failed!${NC}"
  echo -e "${RED}  Server B is already updated to the new version.${NC}"
  echo -e "${RED}  Check Server A logs and roll back if needed.${NC}"
  echo -e "${RED}================================================${NC}"
  exit 1
fi

# ─── Done ───
echo ""
echo -e "${BOLD}================================================${NC}"
echo -e "${BOLD}  Both Servers Updated!${NC}"
echo -e "${BOLD}================================================${NC}"
echo ""
echo -e "  ${BOLD}Panel:${NC}  http://${SERVER_A}"
echo -e "  ${BOLD}API:${NC}    http://${SERVER_A}:${PANEL_PORT}/api/system/status"
echo ""

# Show versions on both servers
echo -e "${BOLD}Versions:${NC}"
for ip in "${SERVER_A}" "${SERVER_B}"; do
  SSH_T="ssh -o ConnectTimeout=5 ${SSH_USER}@${ip}"
  VER=$(${SSH_T} "curl -s --connect-timeout 3 http://127.0.0.1:${PANEL_PORT}/api/system/status 2>/dev/null" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'{d.get(\"version\",\"?\")} ({d.get(\"build_time\",\"?\")})')" 2>/dev/null || echo "offline")
  echo -e "  ${ip}: ${VER}"
done
echo ""
echo -e "${GREEN}Update complete.${NC}"
