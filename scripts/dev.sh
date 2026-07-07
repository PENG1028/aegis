#!/usr/bin/env bash
# Aegis Dev — single command, kills everything stale, starts fresh
# Usage: bash scripts/dev.sh

set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "==> Killing stale processes..."
taskkill //F //IM "aegis-dev.exe" 2>/dev/null || true
for pid in $(netstat -ano | grep ':3000 ' | grep LISTEN | awk '{print $5}' | sort -u); do
  taskkill //F //PID "$pid" 2>/dev/null || true
done
sleep 1

echo "==> Rebuilding backend..."
go build -o aegis-dev.exe ./cmd/aegis/

echo "==> Starting backend (127.0.0.1:7380)..."
nohup ./aegis-dev.exe serve --addr 127.0.0.1:7380 > /tmp/aegis-dev.log 2>&1 &
BACKEND_PID=$!
sleep 3

# Check backend health
if curl -s --max-time 2 http://127.0.0.1:7380/api/healthz 2>/dev/null | grep -q alive; then
  echo "  Backend OK (PID $BACKEND_PID)"
else
  echo "  ⚠ Backend not responding. Check /tmp/aegis-dev.log"
  tail -5 /tmp/aegis-dev.log
fi

echo "==> Starting frontend (localhost:3000)..."
cd ui
nohup npm run dev > /tmp/vite-dev.log 2>&1 &
VITE_PID=$!
sleep 3

if curl -s --max-time 2 http://localhost:3000/ 2>/dev/null | grep -q html; then
  echo "  Frontend OK (PID $VITE_PID)"
else
  echo "  ⚠ Frontend not responding. Check /tmp/vite-dev.log"
  tail -5 /tmp/vite-dev.log
fi

cd "$ROOT"
echo ""
echo "  ───────────────────────────────────────────"
echo "   Backend:  http://127.0.0.1:7380"
echo "   Frontend: http://localhost:3000"
echo "   Login:    admin / admin"
echo "   Logs:"
echo "     Backend  → tail -f /tmp/aegis-dev.log"
echo "     Frontend → tail -f /tmp/vite-dev.log"
echo "  ───────────────────────────────────────────"
