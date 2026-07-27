package deploy

import (
	"context"
	"encoding/json"
	"fmt"
)

// ─── Preflight Report Types ──────────────────────────────────────────────────

type PreflightReport struct {
	Host      *HostInfo              `json:"host,omitempty"`
	Aegis     *BinaryInfo            `json:"aegis"`
	Providers map[string]*BinaryInfo `json:"providers"` // per-provider binary detection (not just Caddy)
	Config    *ConfigInfo            `json:"config"`
	Ports     []PortInfo             `json:"ports"`
}

type HostInfo struct {
	OS   string `json:"os,omitempty"`
	Arch string `json:"arch,omitempty"`
}

type BinaryInfo struct {
	Found      bool   `json:"found"`
	Path       string `json:"path,omitempty"`
	Version    string `json:"version,omitempty"`
	Running    bool   `json:"running"`
	Service    string `json:"service,omitempty"`
	ConfigPath string `json:"config_path,omitempty"`
}

type ConfigInfo struct {
	Found bool   `json:"found"`
	Path  string `json:"path,omitempty"`
}

type PortInfo struct {
	Port    int    `json:"port"`
	Process string `json:"process"`
	Listen  string `json:"listen"`
}

// ─── Embedded detection script ───────────────────────────────────────────────

// preflightScript detects installed middleware by probing common binary names
// rather than hardcoding "caddy". The Go side extracts per-provider results
// from the JSON output indexed by provider ID.
const preflightScript = `#!/bin/sh
osn=$(uname -s 2>/dev/null | tr '[:upper:]' '[:lower:]')
arch=$(uname -m 2>/dev/null)
af="false"; ap=""; av=""; ar="false"; as=""
cof="false"; cop=""

# Aegis
for p in /usr/local/bin/aegis /usr/bin/aegis; do
	if [ -f "$p" ]; then af="true"; ap="$p"; av=$("$p" version 2>/dev/null | head -1); [ -z "$av" ] && av="unknown"; break; fi
done
if systemctl is-active aegis 2>/dev/null | grep -q '^active$'; then ar="true"; as="aegis"
elif systemctl is-active aegis-node 2>/dev/null | grep -q '^active$'; then ar="true"; as="aegis-node"; fi

# Middleware providers — detect each by binary presence.
# The Go side maps these to provider IDs from the registry.
provs=""
for name in caddy haproxy; do
	pf="false"; pp=""; pv=""; pr="false"; ps=""; pc=""
	if pp=$(which $name 2>/dev/null); then
		pf="true"; pv=$($name version 2>/dev/null | head -1); [ -z "$pv" ] && pv="unknown"
		if systemctl is-active $name 2>/dev/null | grep -q '^active$'; then pr="true"; ps="$name"; fi
	fi
	if [ "$name" = "caddy" ] && [ -f /etc/caddy/Caddyfile ]; then pc="/etc/caddy/Caddyfile"; fi
	if [ "$name" = "haproxy" ] && [ -f /etc/haproxy/haproxy.cfg ]; then pc="/etc/haproxy/haproxy.cfg"; fi
	[ -n "$provs" ] && provs="$provs,"
	provs="$provs\"$name\":{\"found\":$pf,\"path\":\"$pp\",\"version\":\"$pv\",\"running\":$pr,\"service\":\"$ps\",\"config_path\":\"$pc\"}"
done

# Config
for p in /etc/aegis/config.yaml /home/ubuntu/.aegis/config.yaml ~/.aegis/config.yaml; do
	if [ -f "$p" ]; then cof="true"; cop="$p"; break; fi
done

# Ports
pj="["
sn=""
for port in 80 443 7380; do
	line=$(sudo ss -tlnp 2>/dev/null | grep -E ":$port\s" | head -1)
	if [ -n "$line" ]; then
		proc=$(echo "$line" | awk -F'"' '{print $2}')
		[ -z "$proc" ] && proc="-"
		listen=$(echo "$line" | awk '{print $4}')
		[ -n "$sn" ] && pj="$pj,"
		pj="$pj{\"port\":$port,\"process\":\"$proc\",\"listen\":\"$listen\"}"
		sn="1"
	fi
done
pj="$pj]"

cat <<JSONEOF
{
  "host": {"os":"$osn","arch":"$arch"},
  "aegis":  {"found":$af,"path":"$ap","version":"$av","running":$ar,"service":"$as"},
  "providers": {$provs},
  "config": {"found":$cof,"path":"$cop"},
  "ports":  $pj
}
JSONEOF
`

// ─── Preflight ────────────────────────────────────────────────────────────────

func Preflight(ctx context.Context, cfg SSHConfig) (*PreflightReport, error) {
	conn, err := Connect(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Executor.Close()

	return PreflightConnection(ctx, conn)
}

// PreflightConnection runs target detection over an existing connection.
func PreflightConnection(ctx context.Context, conn *Connection) (*PreflightReport, error) {
	// Write + execute script in one session
	scriptFile := "/tmp/aegis-preflight.sh"
	writeCmd := fmt.Sprintf("cat > %s << 'SCRIPTEOF'\n%s\nSCRIPTEOF\nchmod +x %s", scriptFile, preflightScript, scriptFile)
	if result := conn.Executor.Run(ctx, writeCmd); result.Error != nil {
		return nil, result.Error
	}

	result := conn.Executor.Run(ctx, "sh "+scriptFile)
	if result.Error != nil {
		return nil, result.Error
	}

	var report PreflightReport
	if err := json.Unmarshal([]byte(result.Stdout), &report); err != nil {
		return nil, fmt.Errorf("parse preflight output: %w\nraw: %s", err, result.Stdout)
	}

	return &report, nil
}
