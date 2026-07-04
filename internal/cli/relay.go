package cli

import (
	"fmt"

	"aegis/internal/gateway"

	"github.com/spf13/cobra"
)

// relayDeps holds dependencies for relay CLI commands.
type relayDeps struct {
	relaySvc *gateway.Resolver
}

// newRelayCommand creates the relay CLI subcommand tree.
func newRelayCommand(relaySvc *gateway.Resolver) *cobra.Command {
	deps := &relayDeps{relaySvc: relaySvc}

	cmd := &cobra.Command{
		Use:   "relay",
		Short: "Managed egress relay path resolution",
		Long: `Resolve the managed egress relay path for a domain.

	Examples:
	  aegis relay resolve example.com
	  aegis relay resolve example.com --from-node nd_a
	  aegis relay resolve example.com --json`,
	}

	cmd.AddCommand(newRelayResolveCmd(deps))

	return cmd
}

func newRelayResolveCmd(deps *relayDeps) *cobra.Command {
	var fromNode string
	cmd := &cobra.Command{
		Use:   "resolve <domain>",
		Short: "Resolve managed egress relay path for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOutput, _ := cmd.Flags().GetBool("json")
			result := deps.relaySvc.ResolveManagedRelay(args[0], fromNode)
			if jsonOutput {
				return printJSON(result)
			}
			return printRelayResult(result)
		},
	}
	cmd.Flags().StringVar(&fromNode, "from-node", "self", "Source node ID to resolve from")
	cmd.Flags().Bool("json", false, "Output as JSON")
	return cmd
}

func printRelayResult(r *gateway.RelayResult) error {
	fmt.Printf("Relay Resolve: %s\n", r.Domain)
	fmt.Printf("  Managed:            %v\n", r.Managed)
	fmt.Printf("  Mode:               %s\n", r.Mode)
	fmt.Printf("  From Node:          %s (%s)\n", r.FromNodeID, r.FromNodeHostname)

	if r.TargetNodeID != "" {
		fmt.Printf("  Target Node:        %s (%s)\n", r.TargetNodeID, r.TargetNodeHostname)
	}
	if r.GatewayURL != "" {
		fmt.Printf("  Gateway URL:        %s\n", r.GatewayURL)
	}
	if r.RouteID != "" {
		fmt.Printf("  Route ID:           %s\n", r.RouteID)
	}
	if r.ServiceID != "" {
		fmt.Printf("  Service ID:         %s\n", r.ServiceID)
	}
	if r.EndpointID != "" {
		fmt.Printf("  Endpoint ID:        %s\n", r.EndpointID)
	}
	if r.GatewayLinkID != "" {
		fmt.Printf("  Gateway Link ID:    %s\n", r.GatewayLinkID)
	}
	fmt.Printf("  Direct Target Suppressed: %v\n", r.DirectTargetSuppressed)
	if r.FinalLocalTarget != "" {
		fmt.Printf("  Final Local Target: %s\n", r.FinalLocalTarget)
	}

	if len(r.Risks) > 0 {
		fmt.Println()
		fmt.Println("  Risks:")
		for _, risk := range r.Risks {
			icon := "ℹ"
			switch risk.Severity {
			case "warning":
				icon = "⚠"
			case "error":
				icon = "✗"
			}
			fmt.Printf("    %s [%s] %s\n", icon, risk.Code, risk.Message)
		}
	}

	if r.Error != "" {
		fmt.Println()
		fmt.Printf("  Error: %s\n", r.Error)
		if r.ErrorDetail != "" {
			fmt.Printf("  Detail: %s\n", r.ErrorDetail)
		}
	}

	if r.Recommendation != "" {
		fmt.Println()
		fmt.Printf("  Recommendation: %s\n", r.Recommendation)
	}

	return nil
}

