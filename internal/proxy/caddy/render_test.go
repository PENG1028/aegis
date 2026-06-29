package caddy

import (
	"aegis/internal/proxy"
	"strings"
	"testing"
)

func TestRenderRoute(t *testing.T) {
	cfg := proxy.GatewayConfig{
		Routes: []proxy.RouteConfig{
			{
				Domain:      "example.com",
				Kind:        "reverse_proxy",
				UpstreamURL: "http://127.0.0.1:3001",
				TLSEnabled:   true,
				Options: proxy.ProxyOptions{
					EnableGzip: true,
				},
			},
		},
	}

	result := renderCaddyfile(cfg, "")
	if !strings.Contains(result, "example.com") {
		t.Error("expected domain in output")
	}
	if !strings.Contains(result, "reverse_proxy http://127.0.0.1:3001") {
		t.Error("expected reverse_proxy directive")
	}
	if !strings.Contains(result, "encode gzip") {
		t.Error("expected gzip encoding")
	}
}

func TestRenderMaintenance(t *testing.T) {
	cfg := proxy.GatewayConfig{
		Routes: []proxy.RouteConfig{
			{
				Domain:             "example.com",
				MaintenanceEnabled:  true,
				MaintenanceMessage:  "Down for maintenance",
			},
		},
	}

	result := renderCaddyfile(cfg, "")
	if !strings.Contains(result, `respond "Down for maintenance" 503`) {
		t.Error("expected maintenance respond block")
	}
	if strings.Contains(result, "reverse_proxy") {
		t.Error("maintenance route should NOT have reverse_proxy")
	}
}

// ── sanitizeCaddyValue tests (C5/C6 fix) ──

func TestSanitizeCaddyValue_StripsNewlines(t *testing.T) {
	input := "hello\nworld\r\nextra"
	result := sanitizeCaddyValue(input)
	if strings.Contains(result, "\n") || strings.Contains(result, "\r") {
		t.Errorf("expected newlines stripped, got %q", result)
	}
}

func TestSanitizeCaddyValue_StripsCurlyBraces(t *testing.T) {
	input := "evil.com {\n    reverse_proxy http://attacker.com\n}"
	result := sanitizeCaddyValue(input)
	if strings.Contains(result, "{") || strings.Contains(result, "}") {
		t.Errorf("expected curly braces stripped, got %q", result)
	}
}

func TestSanitizeCaddyValue_PlainValueUnchanged(t *testing.T) {
	input := "example.com"
	result := sanitizeCaddyValue(input)
	if result != input {
		t.Errorf("plain value should be unchanged, got %q", result)
	}
}

func TestSanitizeCaddyValue_EmptyString(t *testing.T) {
	result := sanitizeCaddyValue("")
	if result != "" {
		t.Errorf("empty string should remain empty, got %q", result)
	}
}

// ── Caddyfile injection tests (C5/C6 fix) ──

func TestRenderCaddyfile_EmailInjectionBlocked(t *testing.T) {
	// An email containing a newline+curly brace could inject a new site block.
	// sanitizeCaddyValue strips these before rendering.
	maliciousEmail := "admin@example.com\n}\n\nevil.com {\n    reverse_proxy http://attacker.com\n}\n"
	cfg := proxy.GatewayConfig{
		Routes: []proxy.RouteConfig{
			{Domain: "legit.com", UpstreamURL: "http://127.0.0.1:3001"},
		},
	}
	result := renderCaddyfile(cfg, maliciousEmail)
	// After sanitization, newlines and curly braces are stripped.
	// The key security property: no new Caddy site block is created for
	// the attacker domain. Check that "evil.com {" (with braces) does NOT
	// appear as a separate site definition.
	if strings.Contains(result, "\nevil.com {") || strings.Contains(result, "\nevil.com{") {
		t.Error("C5 FAIL: evil.com must NOT appear as a Caddy site block")
	}
	// Legitimate domain should still be rendered
	if !strings.Contains(result, "legit.com") {
		t.Error("legitimate domain should still appear")
	}
}

func TestRenderCaddyfile_DomainInjectionBlocked(t *testing.T) {
	// A domain containing injected Caddy directives
	maliciousDomain := "legit.com {\n    reverse_proxy http://evil.com\n}\n\nhijacked.com"
	cfg := proxy.GatewayConfig{
		Routes: []proxy.RouteConfig{
			{Domain: maliciousDomain, UpstreamURL: "http://127.0.0.1:3001"},
		},
	}
	result := renderCaddyfile(cfg, "")
	// The injected text "hijacked.com" may appear in the output (merged with
	// the mangled domain), but it must NOT create a standalone site block.
	// A true injected site block would look like "\nhijacked.com {".
	if strings.Contains(result, "\nhijacked.com {") || strings.Contains(result, "\nhijacked.com{") {
		t.Error("C6 FAIL: hijacked.com must NOT appear as a Caddy site block")
	}
	if strings.Contains(result, "\nevil.com {") || strings.Contains(result, "\nevil.com{") {
		t.Error("C6 FAIL: evil.com must NOT appear as a Caddy site block")
	}
	if len(result) == 0 {
		t.Error("output should not be empty")
	}
}

func TestRenderCaddyfile_UpstreamURLInjectionBlocked(t *testing.T) {
	cfg := proxy.GatewayConfig{
		Routes: []proxy.RouteConfig{
			{
				Domain:      "legit.com",
				UpstreamURL: "http://127.0.0.1:3001\n}\n\nevil.com {\n    reverse_proxy http://attacker.com",
			},
		},
	}
	result := renderCaddyfile(cfg, "")
	// Verify no new site block for evil.com
	if strings.Contains(result, "\nevil.com {") || strings.Contains(result, "\nevil.com{") {
		t.Error("C6 FAIL: evil.com must NOT appear as a site block")
	}
	// Verify no reverse_proxy to attacker.com as a standalone directive.
	// The upstream text may appear merged with the legitimate upstream in the
	// mangled URL, but "reverse_proxy http://attacker.com" on its own line
	// would indicate a successful injection.
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "reverse_proxy http://attacker.com" {
			t.Error("C6 FAIL: standalone reverse_proxy to attacker.com must NOT appear")
		}
	}
}

func TestRenderCaddyfile_HeaderValueInjectionBlocked(t *testing.T) {
	cfg := proxy.GatewayConfig{
		Routes: []proxy.RouteConfig{
			{
				Domain:      "legit.com",
				UpstreamURL: "http://127.0.0.1:3001",
				Options: proxy.ProxyOptions{
					ExtraHeaders: map[string]string{
						"X-Custom": "value\n}\n\nevil.com {\n    reverse_proxy http://attacker.com",
					},
				},
			},
		},
	}
	result := renderCaddyfile(cfg, "")
	// Header value injection should be blocked — no injected site blocks
	if strings.Contains(result, "evil.com {") {
		t.Error("header value injection should be blocked — no evil.com site block")
	}
}

func TestRenderDisabledRouteNotIncluded(t *testing.T) {
	// This test validates that the planner excludes disabled routes
	// — tested at the planner level, not renderer level
	cfg := proxy.GatewayConfig{
		Routes: []proxy.RouteConfig{},
	}

	result := renderCaddyfile(cfg, "")
	if strings.Contains(result, "example.com") {
		t.Error("empty routes should produce no site blocks")
	}
	_ = result
}
