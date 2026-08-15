package templates

import (
	"fmt"

	"aegis/internal/topology"
)

// rejectRawIntents fails loudly when a template cannot carry raw TCP/UDP
// traffic. Silently rendering raw traffic as HTTP (Caddy) or forcing it into
// SNI passthrough (HAProxy) breaks SSH/MySQL-style services without any
// error — the worst kind of failure. Templates that can serve raw traffic
// (dedicated_ports) do not call this.
func rejectRawIntents(tplName string, intents []topology.RouteIntent) error {
	for _, ri := range intents {
		if ri.AppProtocol == "raw" {
			return fmt.Errorf("%s: raw %s route %q cannot be served by this template (no raw forwarding); use dedicated_ports mode or a TCP-capable provider",
				tplName, ri.Transport, ri.Domain)
		}
	}
	return nil
}
