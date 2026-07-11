package deploy

import (
	"context"
	"encoding/json"
	"fmt"
)

// ─── Preflight Report Types ──────────────────────────────────────────────────
// Structured detection report — one SSH call, one JSON response.
// Frontend renders directly from this data, no hardcoded checks.

type PreflightReport struct {
	Aegis  *BinaryInfo  `json:"aegis"`
	Caddy  *BinaryInfo  `json:"caddy"`
	Config *ConfigInfo   `json:"config"`
	Ports  []PortInfo    `json:"ports"`
}

type BinaryInfo struct {
	Found   bool   `json:"found"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Running bool   `json:"running"`
	Service string `json:"service,omitempty"` // systemd service name if found
}

type ConfigInfo struct {
	Found bool   `json:"found"`
	Path  string `json:"path,omitempty"`
}

type PortInfo struct {
	Port    int    `json:"port"`
	Process string `json:"process"`
	Listen  string `json:"listen"` // "0.0.0.0:80" or "127.0.0.1:7380"
}

// ─── Embedded detection script ───────────────────────────────────────────────
// One shell script, uploaded and executed once. Outputs JSON to stdout.
// Uses only POSIX sh + standard tools (which, test, ss, systemctl).

const preflightScript = `#!/bin/sh
# Aegis Preflight — single-pass dependency and port detection
# Output: JSON object to stdout

aegis_found="false"; aegis_path=""; aegis_ver=""; aegis_running="false"; aegis_svc=""
caddy_found="false"; caddy_path=""; caddy_ver=""; caddy_running="false"; caddy_svc=""
cfg_found="false"; cfg_path=""

# ── Aegis ──
for p in /usr/local/bin/aegis /usr/bin/aegis; do
	if [ -f "$p" ]; then
		aegis_found="true"
		aegis_path="$p"
		aegis_ver=$("$p" version 2>/dev/null | head -1)
		[ -z "$aegis_ver" ] && aegis_ver="unknown"
		break
	fi
done
if systemctl is-active aegis 2>/dev/null | grep -q '^active$'; then
	aegis_running="true"; aegis_svc="aegis"
elif systemctl is-active aegis-node 2>/dev/null | grep -q '^active$'; then
	aegis_running="true"; aegis_svc="aegis-node"
fi

# ── Caddy ──
caddy_path=$(which caddy 2>/dev/null)
if [ -n "$caddy_path" ]; then
	caddy_found="true"
	caddy_ver=$(caddy version 2>/dev/null | head -1)
	[ -z "$caddy_ver" ] && caddy_ver="unknown"
	if systemctl is-active caddy 2>/dev/null | grep -q '^active$'; then
		caddy_running="true"; caddy_svc="caddy"
	fi
fi

# ── Config ──
for p in /etc/aegis/config.yaml /home/ubuntu/.aegis/config.yaml ~/.aegis/config.yaml; do
	eval pp="$p"
	if [ -f "$pp" ]; then cfg_found="true"; cfg_path="$pp"; break; fi
done

# ── Ports ──
ports_json="["
seen=""
for p in 80 443 7380; do
	line=$(ss -tlnp 2>/dev/null | grep -E ":$p\s" | head -1)
	if [ -n "$line" ]; then
		proc=$(echo "$line" | grep -oP 'users:\(\("([^"]+)' | head -1 | sed 's/users:(("//')
		[ -z "$proc" ] && proc="unknown"
		listen=$(echo "$line" | awk '{print $4}')
		case "$listen" in *:*) ;; *) listen="0.0.0.0:$p" ;; esac
		[ -n "$seen" ] && ports_json="$ports_json,"
		ports_json="$ports_json{\"port\":$p,\"process\":\"$proc\",\"listen\":\"$listen\"}"
		seen="1"
	fi
done
ports_json="$ports_json]"

cat <<JSONEOF
{
  "aegis":  {"found":$aegis_found, "path":"$aegis_path", "version":"$aegis_ver", "running":$aegis_running, "service":"$aegis_svc"},
  "caddy":  {"found":$caddy_found, "path":"$caddy_path", "version":"$caddy_ver", "running":$caddy_running, "service":"$caddy_svc"},
  "config": {"found":$cfg_found, "path":"$cfg_path"},
  "ports":  $ports_json
}
JSONEOF
`

// ─── Preflight ────────────────────────────────────────────────────────────────

// Preflight connects to the target and runs the detection script.
// Returns a structured report that the frontend renders directly.
func Preflight(ctx context.Context, cfg SSHConfig) (*PreflightReport, error) {
	conn, err := Connect(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Executor.Close()

	// Write script to temp file on remote
	writeResult := conn.Executor.Run(ctx, "cat > /tmp/aegis-preflight.sh << 'SCRIPTEOF'\n"+preflightScript+"\nSCRIPTEOF\nchmod +x /tmp/aegis-preflight.sh")
	if writeResult.Error != nil {
		return nil, writeResult.Error
	}

	// Execute
	result := conn.Executor.Run(ctx, "sh /tmp/aegis-preflight.sh")
	if result.Error != nil {
		return nil, result.Error
	}

	var report PreflightReport
	if err := json.Unmarshal([]byte(result.Stdout), &report); err != nil {
		// If JSON parse fails, return raw output as error detail
		return nil, fmt.Errorf("parse preflight output: %w\nraw: %s", err, result.Stdout)
	}

	return &report, nil
}

