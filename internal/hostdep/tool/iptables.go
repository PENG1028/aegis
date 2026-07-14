//go:build linux

package tool

import (
	"os/exec"
)

// DetectIPTables checks if iptables is available for transparent proxy.
func DetectIPTables() Status {
	s := Status{
		Name:     "iptables",
		Label:    "透明代理 (iptables)",
		Category: "transparent_proxy",
	}

	p, err := exec.LookPath("iptables")
	if err != nil {
		s.Message = "未安装 — apt install iptables"
		return s
	}
	s.Installed = true
	s.Path = p
	s.Available = true
	s.Message = "就绪 — OUTPUT DNAT + SO_ORIGINAL_DST"
	return s
}
