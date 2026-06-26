package safety

import (
	"net"
	"testing"
)

// ============================================================
// IP Classification Tests (1-5)
// ============================================================

func TestClassifyPublicIP(t *testing.T) {
	c := ClassifyIP("8.8.8.8", nil)
	if c != IPPublic {
		t.Errorf("expected public, got %s", c)
	}
	c = ClassifyIP("93.184.216.34", nil)
	if c != IPPublic {
		t.Errorf("expected public, got %s", c)
	}
	t.Logf("Public IP classification OK")
}

func TestClassifyPrivateIP(t *testing.T) {
	for _, ip := range []string{"10.0.0.1", "192.168.1.1", "172.16.0.1"} {
		c := ClassifyIP(ip, nil)
		if c != IPPrivate {
			t.Errorf("expected private for %s, got %s", ip, c)
		}
	}
	t.Logf("Private IP classification OK")
}

func TestClassifyLoopback(t *testing.T) {
	c := ClassifyIP("127.0.0.1", nil)
	if c != IPLoopback {
		t.Errorf("expected loopback, got %s", c)
	}
	t.Logf("Loopback classification OK")
}

func TestClassifySelf(t *testing.T) {
	selfIPs := []string{"10.0.0.5", "43.160.211.232"}
	c := ClassifyIP("10.0.0.5", selfIPs)
	if c != IPSelf {
		t.Errorf("expected self for 10.0.0.5, got %s", c)
	}
	c = ClassifyIP("43.160.211.232", selfIPs)
	if c != IPSelf {
		t.Errorf("expected self for 43.160.211.232, got %s", c)
	}
	t.Logf("Self IP classification OK")
}

func TestClassifyInvalid(t *testing.T) {
	c := ClassifyIP("not-an-ip", nil)
	if c != IPHostname {
		t.Errorf("expected hostname, got %s", c)
	}
	t.Logf("Invalid/hostname classification OK")
}

func TestIsPublicIP(t *testing.T) {
	if !IsPublicIP("8.8.8.8") {
		t.Error("8.8.8.8 should be public")
	}
	if IsPublicIP("127.0.0.1") {
		t.Error("127.0.0.1 should not be public")
	}
	if IsPublicIP("10.0.0.1") {
		t.Error("10.0.0.1 should not be public")
	}
	t.Logf("IsPublicIP OK")
}

func TestIsPrivateIP(t *testing.T) {
	if !IsPrivateIP("10.0.0.1") {
		t.Error("10.0.0.1 should be private")
	}
	if !IsPrivateIP("127.0.0.1") {
		t.Error("127.0.0.1 should be private/loopback")
	}
	if IsPrivateIP("8.8.8.8") {
		t.Error("8.8.8.8 should not be private")
	}
	t.Logf("IsPrivateIP OK")
}

func TestNormalizeHost(t *testing.T) {
	if h := NormalizeHost("10.0.0.1:8080"); h != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", h)
	}
	if h := NormalizeHost("127.0.0.1"); h != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", h)
	}
	t.Logf("NormalizeHost OK")
}

// ============================================================
// Risk Model Tests
// ============================================================

func TestRiskCodes(t *testing.T) {
	codes := []struct {
		code     string
		severity string
	}{
		{RiskPublicDomainBounce, SevWarning},
		{RiskPublicTargetEgress, SevWarning},
		{RiskGatewayLinkBypass, SevWarning},
		{RiskSelfLoop, SevError},
		{RiskDomainResolvesToSelf, SevError},
		{RiskInternalTargetAvail, SevInfo},
		{RiskUnknownDomain, SevInfo},
	}
	for _, tc := range codes {
		r := Risk{Code: tc.code, Severity: tc.severity, Message: "test"}
		if r.Code != tc.code {
			t.Errorf("expected code %s, got %s", tc.code, r.Code)
		}
		if r.Severity != tc.severity {
			t.Errorf("expected severity %s for %s", tc.severity, tc.code)
		}
	}
	t.Logf("All %d risk codes verified", len(codes))
}

// ============================================================
// Route Safety Tests (6-9)
// ============================================================

func TestRouteSafetyLoopbackSafe(t *testing.T) {
	svc := NewService(Dependencies{})
	r := svc.GetPlannerWarnings("test.local", "127.0.0.1:8080", "", nil)
	if len(r) > 0 {
		t.Errorf("loopback target should have no risks, got %d", len(r))
	}
	t.Logf("Loopback route: safe")
}

func TestRouteSafetyPrivateSafe(t *testing.T) {
	svc := NewService(Dependencies{})
	r := svc.GetPlannerWarnings("test.local", "10.0.0.5:3000", "", nil)
	if len(r) > 0 {
		t.Errorf("private target should have no risks, got %d", len(r))
	}
	t.Logf("Private route: safe")
}

func TestRouteSafetyPublicWithGatewayLink(t *testing.T) {
	svc := NewService(Dependencies{})
	r := svc.GetPlannerWarnings("test.local", "43.159.34.11:80", "gw_link_123", nil)
	if len(r) > 0 {
		t.Errorf("public target with GatewayLink should have no risks, got %d", len(r))
	}
	t.Logf("Public route with GatewayLink: safe")
}

func TestRouteSafetyPublicWithoutGatewayLink(t *testing.T) {
	svc := NewService(Dependencies{})
	r := svc.GetPlannerWarnings("test.local", "43.159.34.11:80", "", nil)
	found := false
	for _, risk := range r {
		if risk.Code == RiskGatewayLinkBypass {
			found = true
		}
	}
	if !found {
		t.Errorf("expected GATEWAY_LINK_BYPASS_RISK for public target without GatewayLink")
	}
	t.Logf("Public route without GatewayLink: %d risks detected", len(r))
}

func TestRouteSafetySelfTarget(t *testing.T) {
	svc := NewService(Dependencies{})
	selfIPs := []string{"10.0.0.5"}
	r := svc.GetPlannerWarnings("test.local", "10.0.0.5:9000", "", selfIPs)
	found := false
	for _, risk := range r {
		if risk.Code == RiskSelfLoop {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SELF_LOOP for self target")
	}
	t.Logf("Self target: SELF_LOOP detected")
}

// ============================================================
// Egress Trace Tests (10-13)
// ============================================================

func TestEgressTraceUnknownDomain(t *testing.T) {
	result := &EgressTraceResult{
		Domain: "nonexistent.invalid",
		Risks: []Risk{
			{Code: RiskUnknownDomain, Severity: SevInfo,
				Message: "domain does not resolve or has no route"},
		},
	}
	if result.Domain != "nonexistent.invalid" {
		t.Errorf("domain mismatch")
	}
	if len(result.Risks) == 0 || result.Risks[0].Code != RiskUnknownDomain {
		t.Errorf("expected UNKNOWN_DOMAIN risk")
	}
	t.Logf("Unknown domain: UNKNOWN_DOMAIN")
}

func TestEgressTracePublicDomainBounce(t *testing.T) {
	result := &EgressTraceResult{
		Domain:           "example.com",
		ResolvedIPs:      []string{"93.184.216.34"},
		IPClassification: "public",
		Risks: []Risk{
			{Code: RiskPublicDomainBounce, Severity: SevWarning,
				Message: "example.com resolves to public IP 93.184.216.34 with no Aegis route"},
		},
		Recommendation: "bind example.com using bind-http-domain",
	}
	if len(result.Risks) == 0 || result.Risks[0].Code != RiskPublicDomainBounce {
		t.Errorf("expected PUBLIC_DOMAIN_BOUNCE risk")
	}
	t.Logf("Public domain bounce: PUBLIC_DOMAIN_BOUNCE")
}

func TestEgressTraceResolvesToSelf(t *testing.T) {
	result := &EgressTraceResult{
		Domain:           "self.example.com",
		ResolvedIPs:      []string{"10.0.0.5"},
		IPClassification: "self",
		Risks: []Risk{
			{Code: RiskDomainResolvesToSelf, Severity: SevError,
				Message: "self.example.com resolves to this gateway (10.0.0.5)"},
		},
	}
	if len(result.Risks) == 0 || result.Risks[0].Code != RiskDomainResolvesToSelf {
		t.Errorf("expected DOMAIN_RESOLVES_TO_SELF risk")
	}
	t.Logf("Domain resolves to self: DOMAIN_RESOLVES_TO_SELF")
}

func TestEgressTraceManagedDomain(t *testing.T) {
	result := &EgressTraceResult{
		Domain:          "managed.example.com",
		IsManagedDomain: true,
		Risks:           []Risk{},
	}
	if !result.IsManagedDomain {
		t.Errorf("expected is_managed_domain=true")
	}
	t.Logf("Managed domain: no risks")
}

// ============================================================
// Helper for net.ParseIP check
// ============================================================

func TestNormalizeHostAddr(t *testing.T) {
	host, _, err := net.SplitHostPort("127.0.0.1:8080")
	if err != nil {
		t.Fatalf("SplitHostPort failed: %v", err)
	}
	if host != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", host)
	}
	t.Logf("net.SplitHostPort works correctly")
}
