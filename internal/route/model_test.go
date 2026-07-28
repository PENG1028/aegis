package route

import "testing"

func TestCapabilityKeysUseOneCertificateStrategy(t *testing.T) {
	auto := &Route{Composition: "https_route", TLSBindingMode: TLSBindingProviderAuto}
	manual := &Route{Composition: "https_route", TLSBindingMode: TLSBindingCertificate}

	autoCaps := stringSet(auto.CapabilityKeys())
	if !autoCaps["auto_cert"] || autoCaps["load_cert"] {
		t.Fatalf("provider_auto capabilities are wrong: %v", auto.CapabilityKeys())
	}
	manualCaps := stringSet(manual.CapabilityKeys())
	if !manualCaps["load_cert"] || manualCaps["auto_cert"] {
		t.Fatalf("certificate capabilities are wrong: %v", manual.CapabilityKeys())
	}
}

func TestHTTPRouteDoesNotRequireCertificateCapability(t *testing.T) {
	rt := &Route{Composition: "http_route"}
	caps := stringSet(rt.CapabilityKeys())
	if caps["auto_cert"] || caps["load_cert"] || caps["tls_terminate"] {
		t.Fatalf("HTTP route has TLS requirements: %v", rt.CapabilityKeys())
	}
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
