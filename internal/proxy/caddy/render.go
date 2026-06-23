package caddy

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"aegis/internal/proxy"
)

// renderCaddyfile generates a complete Caddyfile string with path route support.
func renderCaddyfile(gwCfg proxy.GatewayConfig, email string) string {
	var buf bytes.Buffer

	// Global options block
	if email != "" {
		buf.WriteString("{\n")
		buf.WriteString(fmt.Sprintf("    email %s\n", email))
		buf.WriteString("}\n\n")
	}

	// Group routes by domain
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
		// Sort: path routes by depth desc, domain-only last
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

		// Check if there's only one domain-only route → simple block (backward compat)
		if len(routes) == 1 && routes[0].PathPrefix == "" && !routes[0].MaintenanceEnabled {
			renderSimpleBlock(&buf, routes[0])
			continue
		}

		// Multi-route domain → render with braces
		buf.WriteString(fmt.Sprintf("%s {\n", domain))

		hasDomainFallback := false
		for _, r := range routes {
			if r.MaintenanceEnabled {
				// Maintenance path
				if r.PathPrefix != "" {
					pathPattern := r.PathPrefix
					if !strings.HasSuffix(pathPattern, "*") && !strings.HasSuffix(pathPattern, "/*") {
						pathPattern = strings.TrimSuffix(pathPattern, "/") + "/*"
					}
					buf.WriteString(fmt.Sprintf("    handle %s {\n", pathPattern))
				} else {
					buf.WriteString("    handle {\n")
				}
				msg := r.MaintenanceMessage
				if msg == "" {
					msg = "Service temporarily unavailable"
				}
				msg = strings.ReplaceAll(msg, `"`, `\"`)
				buf.WriteString(fmt.Sprintf("        respond \"%s\" 503\n", msg))
				buf.WriteString("    }\n")
				continue
			}

			gzipLine := "    encode gzip\n"
			if r.PathPrefix != "" {
				// Path route
				pathPattern := r.PathPrefix
				if !strings.HasSuffix(pathPattern, "*") && !strings.HasSuffix(pathPattern, "/*") {
					pathPattern = strings.TrimSuffix(pathPattern, "/") + "/*"
				}

				if r.Options.StripPrefix {
					buf.WriteString(fmt.Sprintf("    handle_path %s {\n", pathPattern))
				} else {
					buf.WriteString(fmt.Sprintf("    handle %s {\n", pathPattern))
				}
				buf.WriteString(gzipLine)
				buf.WriteString(fmt.Sprintf("        reverse_proxy %s\n", r.UpstreamURL))
				buf.WriteString("    }\n")
			} else {
				hasDomainFallback = true
			}
		}

		// Domain fallback (no path) — render as catch-all handle
		if hasDomainFallback {
			for _, r := range routes {
				if r.PathPrefix == "" && !r.MaintenanceEnabled {
					buf.WriteString("    handle {\n")
					buf.WriteString("        encode gzip\n")
					buf.WriteString(fmt.Sprintf("        reverse_proxy %s\n", r.UpstreamURL))
					buf.WriteString("    }\n")
					break
				}
			}
		}

		buf.WriteString("}\n")
	}

	return buf.String()
}

// renderSimpleBlock renders a single domain-only route (backward compat).
func renderSimpleBlock(buf *bytes.Buffer, route proxy.RouteConfig) {
	buf.WriteString(fmt.Sprintf("%s {\n", route.Domain))
	buf.WriteString("    encode gzip\n")
	buf.WriteString(fmt.Sprintf("    reverse_proxy %s\n", route.UpstreamURL))
	buf.WriteString("}\n")
}
