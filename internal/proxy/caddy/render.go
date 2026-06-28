package caddy

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"aegis/internal/proxy"
)

func renderCaddyfile(gwCfg proxy.GatewayConfig, email string) string {
	var buf bytes.Buffer
	if email != "" {
		buf.WriteString("{\n    email " + email + "\n}\n\n")
	}

	// Split routes: HTTP vs TCP/UDP port forwarding
	var httpRoutes []proxy.RouteConfig
	var tcpRoutes []proxy.RouteConfig
	for _, r := range gwCfg.Routes {
		if r.Kind == "tcp_proxy" {
			tcpRoutes = append(tcpRoutes, r)
		} else {
			httpRoutes = append(httpRoutes, r)
		}
	}

	// Render HTTP routes (existing logic)
	domainRoutes := make(map[string][]proxy.RouteConfig)
	var domainOrder []string
	for _, r := range httpRoutes {
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
		buf.WriteString(fmt.Sprintf("%s {\n", domain))
		hasDomainFallback := false
		for _, r := range routes {
			if r.MaintenanceEnabled {
				buf.WriteString(fmt.Sprintf("    handle %s {\n", pathPattern(r.PathPrefix)))
				buf.WriteString(fmt.Sprintf("        respond \"%s\" 503\n", r.MaintenanceMessage))
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

	// Render TCP/UDP port forwarding routes (layer4 blocks)
	if len(tcpRoutes) > 0 {
		buf.WriteString("\n# TCP/UDP port forwarding (caddy_layer4)\n")
		for _, r := range tcpRoutes {
			buf.WriteString(fmt.Sprintf("%s {\n", r.Domain))
			buf.WriteString(fmt.Sprintf("    layer4 {\n"))
			buf.WriteString(fmt.Sprintf("        proxy %s\n", r.UpstreamURL))
			buf.WriteString("    }\n")
			buf.WriteString("}\n\n")
		}
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
	if len(headers) > 0 {
		buf.WriteString(fmt.Sprintf("%sreverse_proxy %s {\n", indent, upstream))
		for k, v := range headers {
			buf.WriteString(fmt.Sprintf("%s    header_up %s \"%s\"\n", indent, k, v))
		}
		buf.WriteString(fmt.Sprintf("%s}\n", indent))
	} else {
		buf.WriteString(fmt.Sprintf("%sreverse_proxy %s\n", indent, upstream))
	}
}

func renderRoute(buf *bytes.Buffer, route proxy.RouteConfig) {
	buf.WriteString(fmt.Sprintf("%s {\n", route.Domain))
	buf.WriteString("    encode gzip\n")
	writeReverseProxy(buf, route.UpstreamURL, route.Options.ExtraHeaders, "    ")
	buf.WriteString("}\n")
}
