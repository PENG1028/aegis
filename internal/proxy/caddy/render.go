package caddy

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"aegis/internal/proxy"
)

// sanitizeCaddyValue strips characters that could be used to inject Caddy
// directives into the rendered Caddyfile: newlines and curly braces.
func sanitizeCaddyValue(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	return s
}

func renderCaddyfile(gwCfg proxy.GatewayConfig, email string) string {
	var buf bytes.Buffer
	if email != "" {
		buf.WriteString("{\n    email " + sanitizeCaddyValue(email) + "\n}\n\n")
	}

	// Render all routes as HTTP site blocks (no TCP/UDP port forwarding)
	domainRoutes := make(map[string][]proxy.RouteConfig)
	var domainOrder []string
	for _, r := range gwCfg.Routes {
		if _, ok := domainRoutes[r.Domain]; !ok {
			domainOrder = append(domainOrder, r.Domain)
		}
		domainRoutes[r.Domain] = append(domainRoutes[r.Domain], r)
	}

	for domainIdx, domain := range domainOrder {
		if domainIdx > 0 {
			buf.WriteString("\n")
		}
		routes := domainRoutes[domain]
		sort.Slice(routes, func(i, j int) bool {
			di := len(strings.Split(strings.Trim(routes[i].PathPrefix, "/"), "/"))
			dj := len(strings.Split(strings.Trim(routes[j].PathPrefix, "/"), "/"))
			if routes[i].PathPrefix == "" {
				return false
			}
			if routes[j].PathPrefix == "" {
				return true
			}
			return di > dj
		})

		if len(routes) == 1 && routes[0].PathPrefix == "" && !routes[0].MaintenanceEnabled {
			renderRoute(&buf, routes[0])
			continue
		}
		buf.WriteString(fmt.Sprintf("%s {\n", sanitizeCaddyValue(domain)))
		hasDomainFallback := false
		for _, r := range routes {
			if r.MaintenanceEnabled {
				buf.WriteString(fmt.Sprintf("    handle %s {\n", pathPattern(r.PathPrefix)))
				buf.WriteString(fmt.Sprintf("        respond \"%s\" 503\n", sanitizeCaddyValue(r.MaintenanceMessage)))
				buf.WriteString("    }\n")
				continue
			}
			if r.PathPrefix != "" {
				pp := pathPattern(r.PathPrefix)
				if r.Options.StripPrefix {
					buf.WriteString(fmt.Sprintf("    handle_path %s {\n", pp))
				} else {
					buf.WriteString(fmt.Sprintf("    handle %s {\n", pp))
				}
				buf.WriteString("        encode gzip\n")
				writeReverseProxy(&buf, r.UpstreamURL, r.Options.ExtraHeaders, "        ")
				buf.WriteString("    }\n")
			} else {
				hasDomainFallback = true
			}
		}
		if hasDomainFallback {
			for _, r := range routes {
				if r.PathPrefix == "" && !r.MaintenanceEnabled {
					buf.WriteString("    handle {\n")
					buf.WriteString("        encode gzip\n")
					writeReverseProxy(&buf, r.UpstreamURL, r.Options.ExtraHeaders, "        ")
					buf.WriteString("    }\n")
					break
				}
			}
		}
		buf.WriteString("}\n")
	}


	return buf.String()
}

func pathPattern(p string) string {
	if p != "" && !strings.HasSuffix(p, "*") && !strings.HasSuffix(p, "/*") {
		return strings.TrimSuffix(p, "/") + "/*"
	}
	return p
}

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

func renderRoute(buf *bytes.Buffer, route proxy.RouteConfig) {
	buf.WriteString(fmt.Sprintf("%s {\n", sanitizeCaddyValue(route.Domain)))
	buf.WriteString("    encode gzip\n")
	writeReverseProxy(buf, sanitizeCaddyValue(route.UpstreamURL), route.Options.ExtraHeaders, "    ")
	buf.WriteString("}\n")
}
