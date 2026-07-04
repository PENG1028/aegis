// Package provider — Caddyfile rendering engine.
//
// This file translates a Plan (listeners + routes) into a Caddyfile.
// It is separated from caddy.go (Provider lifecycle) to keep each
// file focused on a single responsibility: caddy.go manages the
// Provider lifecycle, caddy_render.go generates configuration syntax.
//
// ## Caddyfile structure
//
//   Global block (if email or EdgeMux mode):
//     email <addr>
//     https_port <port>      ← only when behind HAProxy
//
//   Per-domain site blocks:
//     <domain> {
//         handle [/path] {
//             encode gzip
//             reverse_proxy <upstream>
//         }
//     }
//
// ## Mode detection
//
// The renderer detects EdgeMux mode by checking if the Plan includes
// a listener with purpose "internal_https" — this comes from RuntimeMode,
// not from shell detection.

package provider

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// ============================================================================
// Core rendering
// ============================================================================

// renderCaddyfile translates a Plan into a complete Caddyfile.
// It detects the active mode from the Plan's listener purposes
// (not from shell detection) so it's consistent with RuntimeMode.
func (p *CaddyProvider) renderCaddyfile(plan Plan) []byte {
	var buf bytes.Buffer

	// Detect if Caddy is behind HAProxy by checking for internal_https listener.
	// This derives the mode from Plan data — the same RuntimeMode that the
	// Planner and API use.
	isEdgeMux := hasListenerPurpose(plan.Listeners, "internal_https")
	needGlobalBlock := p.email != "" || isEdgeMux

	if needGlobalBlock {
		buf.WriteString("{\n")
		if p.email != "" {
			buf.WriteString("    email " + sanitizeCaddyValue(p.email) + "\n")
		}
		if isEdgeMux {
			httpsPort := listenerPort(plan.Listeners, "internal_https")
			if httpsPort == 0 {
				httpsPort = 8443 // fallback
			}
			buf.WriteString(fmt.Sprintf("    https_port %d\n", httpsPort))
		}
		buf.WriteString("}\n\n")
	}

	// Group routes by domain (Match.Host)
	domainRoutes := make(map[string][]RouteSpec)
	var domainOrder []string
	for _, r := range plan.Routes {
		if r.Match.Host == "" {
			continue
		}
		if _, ok := domainRoutes[r.Match.Host]; !ok {
			domainOrder = append(domainOrder, r.Match.Host)
		}
		domainRoutes[r.Match.Host] = append(domainRoutes[r.Match.Host], r)
	}

	for domainIdx, domain := range domainOrder {
		if domainIdx > 0 {
			buf.WriteString("\n")
		}
		siteAddr := caddySiteAddr(domain)
		routes := domainRoutes[domain]

		// Sort by path depth (longer paths first to match before shorter)
		sort.Slice(routes, func(i, j int) bool {
			di := len(strings.Split(strings.Trim(routes[i].Match.Path, "/"), "/"))
			dj := len(strings.Split(strings.Trim(routes[j].Match.Path, "/"), "/"))
			if routes[i].Match.Path == "" {
				return false
			}
			if routes[j].Match.Path == "" {
				return true
			}
			return di > dj
		})

		// Simple case: single route with no path prefix and no maintenance
		if len(routes) == 1 && routes[0].Match.Path == "" && !routes[0].MaintenanceEnabled {
			renderSingleRoute(&buf, routes[0], siteAddr)
			continue
		}

		buf.WriteString(fmt.Sprintf("%s {\n", sanitizeCaddyValue(siteAddr)))
		hasDomainFallback := false

		for _, r := range routes {
			if r.MaintenanceEnabled {
				buf.WriteString(fmt.Sprintf("    handle %s {\n", pathPattern(r.Match.Path)))
				buf.WriteString(fmt.Sprintf("        respond \"%s\" 503\n", sanitizeCaddyValue(r.MaintenanceMessage)))
				buf.WriteString("    }\n")
				continue
			}
			if r.Match.Path != "" {
				pp := pathPattern(r.Match.Path)
				if r.StripPathPrefix {
					buf.WriteString(fmt.Sprintf("    handle_path %s {\n", pp))
				} else {
					buf.WriteString(fmt.Sprintf("    handle %s {\n", pp))
				}
				buf.WriteString("        encode gzip\n")
				writeReverseProxy(&buf, r.Upstream.Target, r.ExtraHeaders, "        ")
				buf.WriteString("    }\n")
			} else {
				hasDomainFallback = true
			}
		}

		if hasDomainFallback {
			for _, r := range routes {
				if r.Match.Path == "" && !r.MaintenanceEnabled {
					buf.WriteString("    handle {\n")
					buf.WriteString("        encode gzip\n")
					writeReverseProxy(&buf, r.Upstream.Target, r.ExtraHeaders, "        ")
					buf.WriteString("    }\n")
					break
				}
			}
		}
		buf.WriteString("}\n")
	}

	return buf.Bytes()
}

// renderSingleRoute renders a simple domain-only site block with a single reverse_proxy.
func renderSingleRoute(buf *bytes.Buffer, route RouteSpec, siteAddr string) {
	buf.WriteString(fmt.Sprintf("%s {\n", sanitizeCaddyValue(siteAddr)))
	buf.WriteString("    encode gzip\n")
	writeReverseProxy(buf, sanitizeCaddyValue(route.Upstream.Target), route.ExtraHeaders, "    ")
	buf.WriteString("}\n")
}

// ============================================================================
// Caddyfile syntax helpers
// ============================================================================

// sanitizeCaddyValue strips characters that would break Caddyfile syntax.
func sanitizeCaddyValue(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	return s
}

// pathPattern appends a wildcard suffix if not already present.
func pathPattern(p string) string {
	if p != "" && !strings.HasSuffix(p, "*") && !strings.HasSuffix(p, "/*") {
		return strings.TrimSuffix(p, "/") + "/*"
	}
	return p
}

// writeReverseProxy writes a Caddy reverse_proxy directive with optional extra headers.
func writeReverseProxy(buf *bytes.Buffer, upstream string, headers map[string]string, indent string) {
	safeUpstream := sanitizeCaddyValue(upstream)
	if len(headers) > 0 {
		buf.WriteString(fmt.Sprintf("%sreverse_proxy %s {\n", indent, safeUpstream))
		for k, v := range headers {
			buf.WriteString(fmt.Sprintf("%s    header_up %s \"%s\"\n", indent, sanitizeCaddyValue(k), sanitizeCaddyValue(v)))
		}
		buf.WriteString(fmt.Sprintf("%s}\n", indent))
	} else {
		buf.WriteString(fmt.Sprintf("%sreverse_proxy %s\n", indent, safeUpstream))
	}
}

// caddySiteAddr returns the Caddy site address for a domain.
func caddySiteAddr(domain string) string {
	if isInternalDomain(domain) {
		return "http://" + domain
	}
	return domain
}

// isInternalDomain returns true if the domain is an internal/local pattern.
func isInternalDomain(domain string) bool {
	return strings.HasSuffix(domain, ".internal") ||
		strings.HasSuffix(domain, ".local") ||
		strings.HasSuffix(domain, ".localhost")
}

// ============================================================================
// Listener helpers — derive mode from Plan data
// ============================================================================

// hasListenerPurpose returns true if any listener has the given purpose.
// Used to detect EdgeMux mode: if a listener has purpose "internal_https",
// then Caddy is behind HAProxy and should configure https_port.
func hasListenerPurpose(listeners []ListenerSpec, purpose string) bool {
	for _, l := range listeners {
		if l.Purpose == purpose {
			return true
		}
	}
	return false
}

// listenerPort returns the port for the first listener with the given purpose.
func listenerPort(listeners []ListenerSpec, purpose string) int {
	for _, l := range listeners {
		if l.Purpose == purpose {
			return l.Port
		}
	}
	return 0
}
