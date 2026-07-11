package deploy

import (
	"context"
	"encoding/json"
	"fmt"
)

// ─── Preflight Report Types ──────────────────────────────────────────────────

type PreflightReport struct {
	Aegis  *BinaryInfo `json:"aegis"`
	Caddy  *BinaryInfo `json:"caddy"`
	Config *ConfigInfo  `json:"config"`
	Ports  []PortInfo   `json:"ports"`
}

type BinaryInfo struct {
	Found   bool   `json:"found"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Running bool   `json:"running"`
	Service string `json:"service,omitempty"`
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

const preflightScript = `#!/bin/sh
af="false"; ap=""; av=""; ar="false"; as=""
cf="false"; cp=""; cv=""; cr="false"; cs=""
cof="false"; cop=""

# Aegis
for p in /usr/local/bin/aegis /usr/bin/aegis; do
	if [ -f "$p" ]; then af="true"; ap="$p"; av=$("$p" version 2>/dev/null | head -1); [ -z "$av" ] && av="unknown"; break; fi
done
if systemctl is-active aegis 2>/dev/null | grep -q '^active$'; then ar="true"; as="aegis"
elif systemctl is-active aegis-node 2>/dev/null | grep -q '^active$'; then ar="true"; as="aegis-node"; fi

# Caddy
if cp=$(which caddy 2>/dev/null); then
	cf="true"; cv=$(caddy version 2>/dev/null | head -1); [ -z "$cv" ] && cv="unknown"
	if systemctl is-active caddy 2>/dev/null | grep -q '^active$'; then cr="true"; cs="caddy"; fi
fi

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
  "aegis":  {"found":$af,"path":"$ap","version":"$av","running":$ar,"service":"$as"},
  "caddy":  {"found":$cf,"path":"$cp","version":"$cv","running":$cr,"service":"$cs"},
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
