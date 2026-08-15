package route

import "testing"

// TestNormalizeDomainLowercases is a regression test: "EXAMPLE.com" and
// "example.com" previously both passed validation, creating two Caddy site
// blocks for the same hostname — Caddy refuses to load ("duplicate site
// address") and the whole gateway config breaks.
func TestNormalizeDomainLowercases(t *testing.T) {
	got, err := NormalizeDomain("EXAMPLE.COM")
	if err != nil {
		t.Fatal(err)
	}
	if got != "example.com" {
		t.Fatalf("normalized = %q, want example.com", got)
	}
}

// TestNormalizeDomainRejectsWildcard is a regression test: "*.example.com"
// renders as a literal SNI match in HAProxy (its -i flag is not a glob), so
// traffic never matches. Wildcards must be rejected up front.
func TestNormalizeDomainRejectsWildcard(t *testing.T) {
	if _, err := NormalizeDomain("*.example.com"); err == nil {
		t.Fatal("wildcard domain must be rejected")
	}
}

func TestNormalizeDomainRejectsMalformed(t *testing.T) {
	for _, d := range []string{"", ".", "..", "a..b.com", ".example.com", "exa mple.com", "exa?mple.com", "exa#mple.com"} {
		if _, err := NormalizeDomain(d); err == nil {
			t.Errorf("NormalizeDomain(%q) accepted, want error", d)
		}
	}
}

// TestNormalizeDomainStripsTrailingDot: an FQDN trailing dot is legal DNS
// and must normalize away rather than be rejected.
func TestNormalizeDomainStripsTrailingDot(t *testing.T) {
	got, err := NormalizeDomain("example.com.")
	if err != nil {
		t.Fatal(err)
	}
	if got != "example.com" {
		t.Fatalf("normalized = %q, want example.com", got)
	}
}

func TestNormalizeDomainAcceptsValid(t *testing.T) {
	for _, d := range []string{"example.com", "api.example.com", "a-b.example.co.uk", "xn--bcher-kva.example", "sub_domain.example.com"} {
		if _, err := NormalizeDomain(d); err != nil {
			t.Errorf("NormalizeDomain(%q) = %v, want nil", d, err)
		}
	}
}
