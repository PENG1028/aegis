package caddy

import (
	"bytes"
	"fmt"
	"strings"

	"aegis/internal/proxy"
)

// renderCaddyfile generates a complete Caddyfile string.
func renderCaddyfile(gwCfg proxy.GatewayConfig, email string) string {
	var buf bytes.Buffer

	// Global options block
	hasGlobal := email != ""
	if hasGlobal {
		buf.WriteString("{\n")
		buf.WriteString(fmt.Sprintf("    email %s\n", email))
		buf.WriteString("}\n\n")
	}

	for i, route := range gwCfg.Routes {
		if i > 0 {
			buf.WriteString("\n")
		}
		renderServerBlock(&buf, route)
	}

	return buf.String()
}

// renderServerBlock renders a single site block.
func renderServerBlock(buf *bytes.Buffer, route proxy.RouteConfig) {
	domain := route.Domain

	if route.MaintenanceEnabled {
		msg := route.MaintenanceMessage
		if msg == "" {
			msg = "Service temporarily unavailable"
		}
		msg = strings.ReplaceAll(msg, `"`, `\"`)
		buf.WriteString(fmt.Sprintf("%s {\n", domain))
		buf.WriteString(fmt.Sprintf("    respond \"%s\" 503\n", msg))
		buf.WriteString("}\n")
		return
	}

	buf.WriteString(fmt.Sprintf("%s {\n", domain))

	// Gzip encoding
	if route.Options.EnableGzip {
		buf.WriteString("    encode gzip\n")
	} else {
		// Default: enable gzip for reverse_proxy
		buf.WriteString("    encode gzip\n")
	}

	// Reverse proxy directive
	buf.WriteString(fmt.Sprintf("    reverse_proxy %s\n", route.UpstreamURL))

	// WebSocket support
	if route.Options.WebSocket {
		buf.WriteString("    header_up Connection {http.request.header.Connection}\n")
		buf.WriteString("    header_up Upgrade {http.request.header.Upgrade}\n")
	}

	buf.WriteString("}\n")
}
