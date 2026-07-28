package certstore

import (
	"testing"
	"time"
)

func TestCoversDomainWildcardMatchesOneLabel(t *testing.T) {
	cert := &Certificate{Domains: `["*.example.com"]`}
	if !CoversDomain(cert, "api.example.com") {
		t.Fatal("wildcard should cover one subdomain label")
	}
	if CoversDomain(cert, "deep.api.example.com") {
		t.Fatal("wildcard must not cover multiple subdomain labels")
	}
	if CoversDomain(cert, "example.com") {
		t.Fatal("wildcard must not cover the apex domain")
	}
}

func TestCoversDomainNormalizesCaseAndTrailingDot(t *testing.T) {
	cert := &Certificate{Domains: `["API.Example.COM"]`}
	if !CoversDomain(cert, "api.example.com.") {
		t.Fatal("domain matching should normalize case and trailing dots")
	}
}

func TestValidAtUsesBothCertificateBounds(t *testing.T) {
	now := time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)
	cert := &Certificate{
		NotBefore: now.Add(-time.Hour).Format(time.RFC3339),
		NotAfter:  now.Add(time.Hour).Format(time.RFC3339),
	}
	if !ValidAt(cert, now) {
		t.Fatal("currently valid certificate was rejected")
	}
	cert.NotBefore = now.Add(time.Minute).Format(time.RFC3339)
	if ValidAt(cert, now) {
		t.Fatal("not-yet-valid certificate was accepted")
	}
	cert.NotBefore = now.Add(-2 * time.Hour).Format(time.RFC3339)
	cert.NotAfter = now.Format(time.RFC3339)
	if ValidAt(cert, now) {
		t.Fatal("expired certificate was accepted")
	}
}
