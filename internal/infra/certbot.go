package infra

// DetectACME checks whether ACME certificate management is available.
// Uses the embedded lego client — no external certbot dependency.
func DetectACME(email string) Status {
	s := Status{
		Name:     "acme",
		Label:    "ACME 证书 (lego)",
		Category: "acme",
	}
	if email == "" {
		s.Message = "未配置 email — 在设置中配置 proxy.email"
		return s
	}
	s.Installed = true
	s.Available = true
	s.Version = "lego (embedded)"
	s.Message = "已配置"
	return s
}

// DetectCertbot is kept for backward compat. It now delegates to DetectACME
// since we no longer depend on the external certbot binary.
func DetectCertbot(email string) Status {
	return DetectACME(email)
}
