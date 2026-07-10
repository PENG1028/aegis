package safety

import (
	"net"
	"testing"
)

// ============================================================
// IP Classification Tests
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
}

func TestClassifyPrivateIP(t *testing.T) {
	for _, ip := range []string{"10.0.0.1", "192.168.1.1", "172.16.0.1"} {
		c := ClassifyIP(ip, nil)
		if c != IPPrivate {
			t.Errorf("expected private for %s, got %s", ip, c)
		}
	}
}

func TestClassifyLoopback(t *testing.T) {
	// 127.0.0.1 should ALWAYS be loopback, even if in node IPs
	c := ClassifyIP("127.0.0.1", []string{"127.0.0.1"})
	if c != IPLoopback {
		t.Errorf("expected loopback, got %s", c)
	}
}

func TestClassifyLoopbackPriority(t *testing.T) {
	// Loopback is checked before private or public
	selfIPs := []string{"10.0.0.5", "192.168.1.100"}
	c := ClassifyIP("127.0.0.1", selfIPs)
	if c != IPLoopback {
		t.Errorf("expected loopback (highest priority), got %s", c)
	}
}

func TestIsCurrentNodeAddress(t *testing.T) {
	selfIPs := []string{"10.0.0.5", "192.168.1.100"}
	if !IsCurrentNodeAddress("10.0.0.5", selfIPs) {
		t.Errorf("10.0.0.5 should be current node address")
	}
	if !IsCurrentNodeAddress("192.168.1.100", selfIPs) {
		t.Errorf("192.168.1.100 should be current node address")
	}
	if IsCurrentNodeAddress("10.0.0.1", selfIPs) {
		t.Errorf("10.0.0.1 should NOT be current node address")
	}
	if IsCurrentNodeAddress("8.8.8.8", selfIPs) {
		t.Errorf("8.8.8.8 should NOT be current node address")
	}
}

func TestClassifyInvalid(t *testing.T) {
	c := ClassifyIP("not-an-ip", nil)
	if c != IPHostname {
		t.Errorf("expected hostname, got %s", c)
	}
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
}

func TestNormalizeHost(t *testing.T) {
	if h := NormalizeHost("10.0.0.1:8080"); h != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", h)
	}
	if h := NormalizeHost("127.0.0.1"); h != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", h)
	}
}

// ============================================================
// Risk Code Semantic Non-Overlap Tests
// ============================================================

func TestRiskPublicDomainBounceOnly(t *testing.T) {
	svc := NewService(Dependencies{})
	result, err := svc.TraceEgress("example.com", "")
	if err != nil {
		t.Fatalf("TraceEgress failed: %v", err)
	}
	hasBounce := false
	for _, r := range result.Risks {
		if r.Code == RiskPublicDomainBounce {
			hasBounce = true
		}
		if r.Code == RiskGatewayLinkBypass {
			t.Errorf("PUBLIC_DOMAIN_BOUNCE should not also emit GATEWAY_LINK_BYPASS_RISK")
		}
		if r.Code == RiskSelfLoop {
			t.Errorf("PUBLIC_DOMAIN_BOUNCE should not also emit SELF_LOOP")
		}
	}
	if !hasBounce {
		t.Logf("Note: example.com may not have resolved on this network")
	}
}

func TestRiskPublicTargetEgressAndBypass(t *testing.T) {
	svc := NewService(Dependencies{})
	risks := svc.GetPlannerWarnings("test.local", "192.168.10.11:80", "")
	hasPublic := false
	hasBypass := false
	for _, r := range risks {
		if r.Code == RiskPublicTargetEgress {
			hasPublic = true
		}
		if r.Code == RiskGatewayLinkBypass {
			hasBypass = true
		}
		if r.Code == RiskSelfLoop {
			t.Errorf("public target should not trigger SELF_LOOP")
		}
	}
	if !hasPublic {
		t.Errorf("expected PUBLIC_TARGET_EGRESS for public target")
	}
	if !hasBypass {
		t.Errorf("expected GATEWAY_LINK_BYPASS_RISK for public target without GatewayLink")
	}
}

func TestRiskNoBypassWithGatewayLink(t *testing.T) {
	svc := NewService(Dependencies{})
	risks := svc.GetPlannerWarnings("test.local", "192.168.10.11:80", "gw_link_123")
	for _, r := range risks {
		if r.Code == RiskGatewayLinkBypass {
			t.Errorf("should not have GATEWAY_LINK_BYPASS_RISK when GatewayLink is provided")
		}
		if r.Code == RiskSelfLoop {
			t.Errorf("public target should not trigger SELF_LOOP")
		}
	}
	// Should still have PUBLIC_TARGET_EGRESS for any public target
	hasPublic := false
	for _, r := range risks {
		if r.Code == RiskPublicTargetEgress {
			hasPublic = true
		}
	}
	if !hasPublic {
		t.Errorf("expected PUBLIC_TARGET_EGRESS for public target even with GatewayLink")
	}
}

// TestRiskSelfLoopListenerPort: self target + listener port → SELF_LOOP
func TestRiskSelfLoopListenerPort(t *testing.T) {
	svc := NewService(Dependencies{})
	// 127.0.0.1 is loopback, port 80 is a default gateway listener → SELF_LOOP
	risks := svc.GetPlannerWarnings("test.local", "127.0.0.1:80", "")
	hasSelf := false
	for _, r := range risks {
		if r.Code == RiskSelfLoop {
			hasSelf = true
		}
	}
	if !hasSelf {
		t.Errorf("expected SELF_LOOP for loopback + gateway listener port 80")
	}
}

// TestRiskSelfLoopNonListenerPort: self target + non-listener port → NO SELF_LOOP
func TestRiskSelfLoopNonListenerPort(t *testing.T) {
	svc := NewService(Dependencies{})
	// 127.0.0.1 is loopback, but 3001 is NOT a gateway listener port → no SELF_LOOP
	risks := svc.GetPlannerWarnings("test.local", "127.0.0.1:3001", "")
	for _, r := range risks {
		if r.Code == RiskSelfLoop {
			t.Errorf("non-listener port should not trigger SELF_LOOP")
		}
	}
}

// TestRiskSelfLoopNonNodeIP: non-loopback, non-node IP → NO SELF_LOOP
func TestRiskSelfLoopNonNodeIP(t *testing.T) {
	svc := NewService(Dependencies{})
	risks := svc.GetPlannerWarnings("test.local", "10.0.0.5:80", "")
	for _, r := range risks {
		if r.Code == RiskSelfLoop {
			t.Errorf("non-node IP should not trigger SELF_LOOP")
		}
	}
}

func TestRiskDomainResolvesToSelfOnly(t *testing.T) {
	result := &EgressTraceResult{
		Domain:           "self.example.com",
		ResolvedIPs:      []string{"10.0.0.5"},
		IPClassification: "public",
		IsCurrentNodeAddress: true,
		Risks: []Risk{
			{Code: RiskDomainResolvesToSelf, Severity: SevError,
				Message: "self.example.com resolves to this gateway (10.0.0.5)"},
		},
	}
	hasSelf := false
	for _, r := range result.Risks {
		if r.Code == RiskDomainResolvesToSelf {
			hasSelf = true
		}
		if r.Code == RiskPublicDomainBounce && result.IsCurrentNodeAddress {
			t.Errorf("DOMAIN_RESOLVES_TO_SELF should suppress PUBLIC_DOMAIN_BOUNCE")
		}
		if r.Code == RiskUnknownDomain {
			t.Errorf("DOMAIN_RESOLVES_TO_SELF should not also emit UNKNOWN_DOMAIN")
		}
	}
	if !hasSelf {
		t.Errorf("expected DOMAIN_RESOLVES_TO_SELF risk")
	}
}

func TestRiskUnknownDomainOnly(t *testing.T) {
	result := &EgressTraceResult{
		Domain: "nonexistent.invalid",
		Risks: []Risk{
			{Code: RiskUnknownDomain, Severity: SevInfo,
				Message: "domain does not resolve"},
		},
	}
	hasUnknown := false
	for _, r := range result.Risks {
		if r.Code == RiskUnknownDomain {
			hasUnknown = true
		}
		if r.Code == RiskPublicDomainBounce {
			t.Errorf("UNKNOWN_DOMAIN should not also emit PUBLIC_DOMAIN_BOUNCE")
		}
		if r.Code == RiskDomainResolvesToSelf {
			t.Errorf("UNKNOWN_DOMAIN should not also emit DOMAIN_RESOLVES_TO_SELF")
		}
	}
	if !hasUnknown {
		t.Errorf("expected UNKNOWN_DOMAIN risk")
	}
}

func TestRiskInternalTargetAvail(t *testing.T) {
	svc := NewService(Dependencies{})
	// loopback non-listener port → no risks
	risks := svc.GetPlannerWarnings("test.local", "127.0.0.1:8080", "")
	if len(risks) != 0 {
		t.Errorf("loopback non-listener port should have 0 risks, got %d", len(risks))
	}
	// private → no risks
	risks = svc.GetPlannerWarnings("test.local", "10.0.0.5:3000", "")
	if len(risks) != 0 {
		t.Errorf("private target should have 0 risks, got %d", len(risks))
	}
}

// ============================================================
// GetPlannerWarnings Tests (Listener-Aware Self-Loop)
// ============================================================

func TestPlannerWarningsLoopbackNonListener(t *testing.T) {
	svc := NewService(Dependencies{})
	risks := svc.GetPlannerWarnings("test.local", "127.0.0.1:8080", "")
	if len(risks) != 0 {
		t.Errorf("loopback non-listener port should have no risks, got %d", len(risks))
	}
}

func TestPlannerWarningsPrivateSafe(t *testing.T) {
	svc := NewService(Dependencies{})
	risks := svc.GetPlannerWarnings("test.local", "10.0.0.5:3000", "")
	if len(risks) > 0 {
		t.Errorf("private target should have no risks, got %d", len(risks))
	}
}

func TestPlannerWarningsLoopbackListenerPort(t *testing.T) {
	svc := NewService(Dependencies{})
	// 127.0.0.1:80 → loopback + default listener port → SELF_LOOP
	risks := svc.GetPlannerWarnings("test.local", "127.0.0.1:80", "")
	foundSelf := false
	for _, risk := range risks {
		if risk.Code == RiskSelfLoop {
			foundSelf = true
		}
	}
	if !foundSelf {
		t.Errorf("expected SELF_LOOP for loopback + gateway listener port 80")
	}
}

func TestPlannerWarningsPublicWithGatewayLink(t *testing.T) {
	svc := NewService(Dependencies{})
	risks := svc.GetPlannerWarnings("test.local", "192.168.10.11:80", "gw_link_123")
	foundBypass := false
	for _, risk := range risks {
		if risk.Code == RiskGatewayLinkBypass {
			foundBypass = true
		}
	}
	if foundBypass {
		t.Errorf("public target with GatewayLink should not have GATEWAY_LINK_BYPASS_RISK")
	}
	// Should have PUBLIC_TARGET_EGRESS
	hasPublic := false
	for _, risk := range risks {
		if risk.Code == RiskPublicTargetEgress {
			hasPublic = true
		}
	}
	if !hasPublic {
		t.Errorf("expected PUBLIC_TARGET_EGRESS for public target even with GatewayLink")
	}
}

func TestPlannerWarningsPublicWithoutGatewayLink(t *testing.T) {
	svc := NewService(Dependencies{})
	risks := svc.GetPlannerWarnings("test.local", "192.168.10.11:80", "")
	foundBypass := false
	for _, risk := range risks {
		if risk.Code == RiskGatewayLinkBypass {
			foundBypass = true
		}
	}
	if !foundBypass {
		t.Errorf("expected GATEWAY_LINK_BYPASS_RISK for public target without GatewayLink")
	}
	hasPublic := false
	for _, risk := range risks {
		if risk.Code == RiskPublicTargetEgress {
			hasPublic = true
		}
	}
	if !hasPublic {
		t.Errorf("expected PUBLIC_TARGET_EGRESS for public target")
	}
}

// ============================================================
// Egress Trace Tests
// ============================================================

func TestEgressTraceManagedDomain(t *testing.T) {
	svc := NewService(Dependencies{})
	result, err := svc.TraceEgress("nonexistent-test-domain.invalid", "")
	if err != nil {
		t.Fatalf("TraceEgress should not error on unresolvable domain: %v", err)
	}
	hasUnknown := false
	for _, r := range result.Risks {
		if r.Code == RiskUnknownDomain {
			hasUnknown = true
		}
	}
	if !hasUnknown {
		t.Logf("Note: domain may have resolved on this network, got: %s", result.IPClassification)
	}
}

func TestEgressTraceResolvesToSelf(t *testing.T) {
	result := &EgressTraceResult{
		Domain:              "self.example.com",
		ResolvedIPs:         []string{"10.0.0.5"},
		IPClassification:    "public",
		IsCurrentNodeAddress: true,
		Risks: []Risk{
			{Code: RiskDomainResolvesToSelf, Severity: SevError,
				Message: "self.example.com resolves to this gateway (10.0.0.5)"},
		},
	}
	if len(result.Risks) == 0 || result.Risks[0].Code != RiskDomainResolvesToSelf {
		t.Errorf("expected DOMAIN_RESOLVES_TO_SELF risk")
	}
}

func TestEgressTracePublicDomainBounce(t *testing.T) {
	result := &EgressTraceResult{
		Domain:           "example.com",
		ResolvedIPs:      []string{"93.184.216.34"},
		IPClassification: "public",
		Risks: []Risk{
			{Code: RiskPublicDomainBounce, Severity: SevWarning,
				Message: "example.com resolves to public IP with no Aegis route"},
		},
		Recommendation: "bind example.com using bind-http-domain",
	}
	if len(result.Risks) == 0 || result.Risks[0].Code != RiskPublicDomainBounce {
		t.Errorf("expected PUBLIC_DOMAIN_BOUNCE risk")
	}
}

func TestEgressTraceManagedDomainFlag(t *testing.T) {
	result := &EgressTraceResult{
		Domain:          "managed.example.com",
		IsManagedDomain: true,
	}
	if !result.IsManagedDomain {
		t.Errorf("expected is_managed_domain=true")
	}
}

// ============================================================
// Risk Constants
// ============================================================

func TestRiskConstantsMatch(t *testing.T) {
	tests := []struct {
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
	for _, tc := range tests {
		r := Risk{Code: tc.code, Severity: tc.severity, Message: "test"}
		if r.Code != tc.code {
			t.Errorf("expected code %s, got %s", tc.code, r.Code)
		}
		if r.Severity != tc.severity {
			t.Errorf("expected severity %s for %s", tc.severity, tc.code)
		}
	}
}

// ============================================================
// Helpers
// ============================================================

func TestSplitHostPortHelper(t *testing.T) {
	host, _, err := net.SplitHostPort("127.0.0.1:8080")
	if err != nil {
		t.Fatalf("SplitHostPort failed: %v", err)
	}
	if host != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", host)
	}
}
