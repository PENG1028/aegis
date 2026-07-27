package tool

import (
	"os/exec"
	"strings"
)

// DetectDNSMasq checks if dnsmasq is installed.
func DetectDNSMasq() Status {
	s := Status{
		Name:     "dnsmasq",
		Label:    "DNS 服务 (dnsmasq)",
		Category: "dns",
	}

	p, err := exec.LookPath("dnsmasq")
	if err != nil {
		s.Message = "未安装 — apt install dnsmasq"
		return s
	}
	s.Installed = true
	s.Path = p

	if out, err := exec.Command("dnsmasq", "--version").CombinedOutput(); err == nil {
		s.Version = strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	}

	s.Available = true
	s.Message = "就绪"
	return s
}
