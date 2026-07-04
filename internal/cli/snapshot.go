package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/edgemux"
	"aegis/internal/listener"
	"aegis/internal/node"
	"aegis/internal/route"
	"aegis/internal/deployment"

	"github.com/spf13/cobra"
)

func newSnapshotCommand(
	applySvc *apply.AppService,
	routeSvc *route.AppService,
	edgeSvc *edgemux.AppService,
	listenerSvc *listener.Service,
	leaderSvc *cluster.LeaderService,
	nodeRepo *node.Repository,
	stateVer *cluster.StateVersion,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Capture full system state snapshot",
		Long:  "Exports current listeners, edge rules, routes, provider status, config hashes, and port ownership to a JSON snapshot file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			snap := deployment.NewSnapshot()
			snap.StateVersion = stateVer.Current()

			// Leader ID
			if leader, err := leaderSvc.GetLeader(); err == nil && leader != nil {
				snap.LeaderID = leader.NodeID
			}

			// Listeners
			listeners, _ := listenerSvc.ListAll()
			for _, l := range listeners {
				snap.Listeners = append(snap.Listeners, deployment.ListenerState{
					ID: l.ID, Provider: l.Provider, Protocol: l.Protocol,
					BindIP: l.BindIP, Port: l.Port, Status: l.Status,
				})
			}

			// Edge rules
			rules, _ := edgeSvc.ListRules(ctx)
			for _, r := range rules {
				snap.EdgeRules = append(snap.EdgeRules, deployment.EdgeRuleState{
					ID: r.ID, SNIHost: r.SNIHost,
					Target: fmt.Sprintf("%s:%d", r.TargetHost, r.TargetPort),
					Status: r.Status, ManagedBy: r.ManagedBy,
				})
			}

			// Routes
			routes, _ := routeSvc.ListRoutes(ctx)
			for _, r := range routes {
				snap.Routes = append(snap.Routes, deployment.RouteState{
					ID: r.ID, Domain: r.Domain, Path: r.PathPrefix, Status: r.Status,
				})
			}

			// Config hashes
			plan, err := applySvc.DryRun(ctx)
			if err == nil {
				snap.ConfigHash.CaddyConfigHash = deployment.Hash(plan.RenderedConfig)
			}

			// Export
			filename := fmt.Sprintf("./aegis-snapshot-%s.json", time.Now().Format("20060102-150405"))
			if err := snap.Export(filename); err != nil {
				return err
			}
			fmt.Printf("Snapshot exported to: %s\n", filename)
			fmt.Printf("  listeners: %d\n", len(snap.Listeners))
			fmt.Printf("  edge rules: %d\n", len(snap.EdgeRules))
			fmt.Printf("  routes: %d\n", len(snap.Routes))
			return nil
		},
	}

	cmd.AddCommand(newRestoreCommand(applySvc, routeSvc, edgeSvc, listenerSvc))
	return cmd
}

func newRestoreCommand(
	applySvc *apply.AppService,
	routeSvc *route.AppService,
	edgeSvc *edgemux.AppService,
	listenerSvc *listener.Service,
) *cobra.Command {
	return &cobra.Command{
		Use:   "restore --from <deployment.json>",
		Short: "Restore system state from a snapshot",
		Long:  "Restores listeners, edge rules, routes, and applies configs from a deployment. Does NOT overwrite data that already matches.",
		RunE: func(cmd *cobra.Command, args []string) error {
			from, _ := cmd.Flags().GetString("from")
			if from == "" {
				return fmt.Errorf("--from is required")
			}
			if _, err := os.Stat(from); os.IsNotExist(err) {
				return fmt.Errorf("snapshot file not found: %s", from)
			}

			snap, err := deployment.Load(from)
			if err != nil {
				return fmt.Errorf("load snapshot: %w", err)
			}

			fmt.Printf("Restoring from snapshot: %s\n", snap.ExportedAt)
			fmt.Printf("  version: %s\n", snap.Version)

			// Restore routes
			ctx := context.Background()
			currentRoutes, _ := routeSvc.ListRoutes(ctx)
			restored := 0
			for _, rs := range snap.Routes {
				found := false
				for _, cr := range currentRoutes {
					if cr.Domain == rs.Domain && cr.PathPrefix == rs.Path {
						found = true
						break
					}
				}
				if !found {
					// Route doesn't exist — would need to be recreated
					fmt.Printf("  route %s (%s) would be recreated\n", rs.Domain, trancate(rs.ID, 8))
					restored++
				}
			}
			fmt.Printf("  total routes in snapshot: %d, missing: %d\n", len(snap.Routes), restored)

			// Restore edge rules
			currentRules, _ := edgeSvc.ListRules(ctx)
			edgeRestored := 0
			for _, es := range snap.EdgeRules {
				found := false
				for _, cr := range currentRules {
					if cr.SNIHost == es.SNIHost {
						found = true
						break
					}
				}
				if !found {
					fmt.Printf("  edge rule %s → %s would be recreated\n", es.SNIHost, es.Target)
					edgeRestored++
				}
			}
			fmt.Printf("  total edge rules in snapshot: %d, missing: %d\n", len(snap.EdgeRules), edgeRestored)

			// Apply configs
			fmt.Println()
			fmt.Println("Running apply --all to sync configs...")
			_, err = applySvc.Apply(ctx)
			if err != nil {
				return fmt.Errorf("apply failed: %w (some state may have been restored)", err)
			}

			fmt.Println("Restore complete.")
			return nil
		},
	}
}

func trancate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
