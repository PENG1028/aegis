package infra

import (
	"os/exec"
	"strings"
)

// DetectCertbot checks if certbot is installed and configured.
// email: ACME registration email (from proxy.email config). Empty = not configured.
func DetectCertbot(email string) Status {
	s := Status{
		Name:     "certbot",
		Label:    "ACME 客户端 (certbot)",
		Category: "acme",
	}

	if email == "" {
		s.Message = "未配置 email — 在 config.yaml 设置 proxy.email"
		return s
	}

	p, err := exec.LookPath("certbot")
	if err != nil {
		s.Message = "未安装 — apt install certbot"
		return s
	}
	s.Installed = true
	s.Path = p

	if out, err := exec.Command("certbot", "--version").CombinedOutput(); err == nil {
		s.Version = strings.TrimSpace(strings.TrimPrefix(string(out), "certbot "))
	}

	s.Available = true
	s.Message = "就绪"
	return s
}
