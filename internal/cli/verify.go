package cli

import (
	"context"
	"fmt"

	"aegis/internal/apply"
	"aegis/internal/edgemux"
	"aegis/internal/listener"
	"aegis/internal/route"

	"github.com/spf13/cobra"
)

func newVerifyCommand(
	applySvc *apply.AppService,
	routeSvc *route.AppService,
	edgeSvc *edgemux.AppService,
	listenerSvc *listener.Service,
) *cobra.Command {
	var full bool

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify system consistency",
		Long:  "Checks that the running system matches Aegis state. Use --full for deep checks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			issues := 0

			fmt.Println("=== Aegis Verify ===")
			fmt.Println()

			// Basic checks (always run)
			fmt.Println("[basic]")
			listeners, _ := listenerSvc.ListAll()
			fmt.Printf("  listeners registered: %d\n", len(listeners))
			if len(listeners) == 0 {
				fmt.Println("  WARNING: no listeners registered")
				issues++
			}

			rules, _ := edgeSvc.ListRules(ctx)
			activeRules := 0
			for _, r := range rules {
				if r.Status == "active" { activeRules++ }
			}
			fmt.Printf("  edge rules: %d total, %d active\n", len(rules), activeRules)

			routes, _ := routeSvc.ListRoutes(ctx)
			activeRoutes := 0
			for _, r := range routes {
				if r.Status == "active" { activeRoutes++ }
			}
			fmt.Printf("  routes: %d total, %d active\n", len(routes), activeRoutes)

			// Full checks
			if full {
				fmt.Println()
				fmt.Println("[full]")

				// Orphan edge rules (no matching route)
				routeDomains := make(map[string]bool)
				for _, r := range routes {
					routeDomains[r.Domain] = true
				}
				for _, r := range rules {
					if r.ManagedBy == "http_route" {
						if !routeDomains[r.SNIHost] {
							fmt.Printf("  ORPHAN: edge rule %s (SNI %s) references non-existent route\n", r.ID, r.SNIHost)
							issues++
						}
					}
				}

				// Edge rules → routes consistency
				for _, r := range routes {
					if r.Status == "active" {
						found := false
						for _, rule := range rules {
							if rule.SNIHost == r.Domain && rule.ManagedBy == "http_route" && rule.Status == "active" {
								found = true
								break
							}
						}
						if !found {
							fmt.Printf("  MISSING: active route %s (%s) has no matching edge rule\n", r.Domain, r.ID)
							issues++
						}
					}
				}

				// Hash consistency (simplified)
				plan, err := applySvc.DryRun(ctx)
				if err == nil && plan.RenderedConfig != "" {
					fmt.Printf("  config preview: available (%d chars)\n", len(plan.RenderedConfig))
				}

				// No duplicate edge rules
				sniSeen := make(map[string]string)
				for _, r := range rules {
					if prev, ok := sniSeen[r.SNIHost]; ok {
						fmt.Printf("  DUPLICATE: SNI %s has multiple rules: %s and %s\n", r.SNIHost, prev, r.ID)
						issues++
					}
					sniSeen[r.SNIHost] = r.ID
				}
			}

			fmt.Println()
			if issues > 0 {
				fmt.Printf("⚠ %d issue(s) found.\n", issues)
			} else {
				fmt.Println("✓ System consistent.")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "Run full consistency checks (orphans, duplicates, edge↔route sync)")
	return cmd
}
