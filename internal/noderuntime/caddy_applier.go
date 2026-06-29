package noderuntime

import (
	"fmt"
	"os"

	"aegis/internal/apply"
	"aegis/internal/config"
	"aegis/internal/proxy"
	"aegis/internal/proxy/caddy"
)

// CaddyfileApplier renders and applies a Caddyfile from a routing table.
// Implementations handle the full render→validate→backup→replace→reload cycle.
type CaddyfileApplier interface {
	Apply(entries []RoutingTableEntry) error
}

// caddyApplier is the production implementation of CaddyfileApplier.
type caddyApplier struct {
	adapter  proxy.ProxyAdapter
	executor *apply.Executor
	cfg      *config.Config
}

// NewCaddyApplier creates a new production Caddyfile applier.
func NewCaddyApplier(cfg *config.Config) CaddyfileApplier {
	return &caddyApplier{
		adapter:  caddy.NewAdapter(cfg),
		executor: apply.NewExecutor(cfg),
		cfg:      cfg,
	}
}

// Apply converts routing table entries to route configs, renders a Caddyfile,
// validates it, backs up the current config, replaces it, and reloads Caddy.
func (a *caddyApplier) Apply(entries []RoutingTableEntry) error {
	// 1. Convert routing table entries to proxy route configs
	routes := routingTableToRouteConfigs(entries)
	if len(routes) == 0 {
		return fmt.Errorf("no available routes to apply")
	}

	// 2. Render Caddyfile
	rendered, err := a.adapter.Render(proxy.GatewayConfig{
		Routes: routes,
		Email:  a.cfg.Proxy.Email,
	})
	if err != nil {
		return fmt.Errorf("render caddyfile: %w", err)
	}
	if len(rendered) == 0 {
		return fmt.Errorf("rendered caddyfile is empty")
	}

	// 3. Write temp file
	tempPath, err := a.executor.WriteTemp(rendered)
	if err != nil {
		return fmt.Errorf("write temp caddyfile: %w", err)
	}

	// 4. Validate
	if err := a.executor.ValidateAdapter(a.adapter, tempPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("validate caddyfile: %w", err)
	}

	// 5. Backup current config
	backupPath, backupErr := a.executor.Backup()
	if backupErr != nil {
		// Log but continue — replace will overwrite, but we tried
		_ = backupPath
	}

	// 6. Replace with rollback on reload failure
	if err := a.executor.Replace(tempPath); err != nil {
		return fmt.Errorf("replace caddyfile: %w", err)
	}

	// 7. Reload — if this fails, restore from backup
	if err := a.executor.ReloadAdapter(a.adapter); err != nil {
		// Attempt rollback
		if backupPath != "" {
			_ = a.executor.RestoreBackup(backupPath)
			_ = a.executor.ReloadAdapter(a.adapter)
		}
		return fmt.Errorf("reload caddy: %w (rolled back to backup)", err)
	}

	return nil
}

// routingTableToRouteConfigs converts node routing table entries to proxy route configs.
// Only entries with status "available" are included.
//
// Routing logic:
//   - Same-node endpoint: use TargetLocalHost:TargetLocalPort (direct to backend)
//   - Cross-node with candidate: use the best candidate's GatewayURL (proxy to remote Caddy :80)
//   - Cross-node without candidate: skipped (no route to target)
func routingTableToRouteConfigs(entries []RoutingTableEntry) []proxy.RouteConfig {
	var routes []proxy.RouteConfig
	for _, entry := range entries {
		if entry.Status != "available" {
			continue
		}

		var upstreamURL string
		var extraHeaders map[string]string

		if entry.TargetNodeID == entry.FromNodeID || entry.TargetNodeID == "" {
			// Same node: proxy directly to the local endpoint
			if entry.TargetLocalHost == "" || entry.TargetLocalPort == 0 {
				continue
			}
			upstreamURL = fmt.Sprintf("http://%s:%d", entry.TargetLocalHost, entry.TargetLocalPort)
		} else {
			// Cross-node: proxy to the remote Caddy on port 80 via the best candidate
			if len(entry.Candidates) == 0 {
				continue
			}
			best := entry.Candidates[0] // sorted by priority (lowest first)
			upstreamURL = best.GatewayURL

			// Inject gateway link auth headers so the remote Caddy can
			// validate and relay the request
			if best.GatewayLinkID != "" {
				extraHeaders = map[string]string{
					"X-Aegis-Gateway-Link": best.GatewayLinkID,
				}
			}
		}

		routes = append(routes, proxy.RouteConfig{
			Domain:      entry.Domain,
			Kind:        "reverse_proxy",
			UpstreamURL: upstreamURL,
			Options: proxy.ProxyOptions{
				EnableGzip:   true,
				ExtraHeaders: extraHeaders,
			},
		})
	}
	return routes
}
