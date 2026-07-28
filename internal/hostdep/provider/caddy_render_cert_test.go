package provider

import (
	"strings"
	"testing"
)

func TestCaddyProviderAutoRouteDoesNotRenderFixedPEM(t *testing.T) {
	p := &CaddyProvider{}
	plan := Plan{Routes: []RouteSpec{{
		Transport: "tcp", TLSMode: "terminate", AppProtocol: "http",
		Match:    MatchSpec{Host: "app.example.com"},
		Upstream: UpstreamSpec{Type: "http", Target: "127.0.0.1:8080"},
	}}}
	rendered := string(p.renderCaddyfile(plan))
	if strings.Contains(rendered, "    tls ") {
		t.Fatalf("provider-auto route rendered a fixed PEM directive:\n%s", rendered)
	}
}

func TestCaddyCertificateBindingRendersFixedPEM(t *testing.T) {
	p := &CaddyProvider{}
	plan := Plan{Routes: []RouteSpec{{
		Transport: "tcp", TLSMode: "terminate", AppProtocol: "http",
		Match:    MatchSpec{Host: "app.example.com"},
		Upstream: UpstreamSpec{Type: "http", Target: "127.0.0.1:8080"},
		CertPath: "/etc/aegis/certs/app.crt", KeyPath: "/etc/aegis/certs/app.key",
	}}}
	rendered := string(p.renderCaddyfile(plan))
	if !strings.Contains(rendered, "tls /etc/aegis/certs/app.crt /etc/aegis/certs/app.key") {
		t.Fatalf("certificate binding did not render PEM directive:\n%s", rendered)
	}
}

func TestCaddyACMEChallengeRoutePrecedesCatchAll(t *testing.T) {
	p := &CaddyProvider{}
	plan := Plan{Routes: []RouteSpec{
		{
			Match:    MatchSpec{Host: "http://"},
			Upstream: UpstreamSpec{Type: "http", Target: "http://127.0.0.1:7380"},
		},
		{
			Match:    MatchSpec{Host: "http://", Path: "/.well-known/acme-challenge"},
			Upstream: UpstreamSpec{Type: "http", Target: "http://127.0.0.1:7380"},
			Priority: RoutePriorityControlPlane + 100,
		},
	}}
	rendered := string(p.renderCaddyfile(plan))
	challenge := strings.Index(rendered, "handle /.well-known/acme-challenge/*")
	catchAll := strings.Index(rendered, "handle {")
	if challenge < 0 || catchAll < 0 || challenge > catchAll {
		t.Fatalf("challenge route is not before catch-all:\n%s", rendered)
	}
}
