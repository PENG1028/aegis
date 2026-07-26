package tool

// DetectACME checks whether ACME certificate management is available.
// Uses the embedded lego client — no external certbot dependency.
func DetectACME(email string) Status {
	s := Status{
		Name:     "acme",
		Label:    "ACME 证书 (lego)",
		Category: "acme",
	}
	s.Installed = true
	s.Available = true
	s.Version = "lego (embedded)"
	if email != "" {
		s.Message = "已配置"
	} else {
		s.Message = "已就绪（未配置邮箱，到期不会收到通知）"
	}
	return s
}

// DetectCertbot is kept for backward compat. It now delegates to DetectACME
// since we no longer depend on the external certbot binary.
func DetectCertbot(email string) Status {
	return DetectACME(email)
}
