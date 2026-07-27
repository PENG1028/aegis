package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"aegis/internal/safety"

	"github.com/spf13/cobra"
)

// safetyDeps holds the dependencies needed for safety CLI commands.
type safetyDeps struct {
	safetySvc *safety.Service
}

// newSafetyCommand creates the safety CLI subcommand tree.
// Registered under "aegis trace egress <domain>".
func newSafetyCommand(safetySvc *safety.Service) *cobra.Command {
	deps := &safetyDeps{safetySvc: safetySvc}

	cmd := &cobra.Command{
		Use:   "safety",
		Short: "Route safety checks and egress tracing",
		Long: `Check route safety and trace egress paths for domains.

Examples:
  aegis safety check-route rt_abc123
  aegis safety check-all
  aegis safety trace-egress example.com
  aegis safety trace-egress example.com --from-node nd_123`,
	}

	cmd.AddCommand(newSafetyCheckRouteCmd(deps))
	cmd.AddCommand(newSafetyCheckAllCmd(deps))
	cmd.AddCommand(newSafetyTraceEgressCmd(deps))

	return cmd
}

func newSafetyCheckRouteCmd(deps *safetyDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "check-route <route_id>",
		Short: "Check safety for a single route",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := deps.safetySvc.CheckRouteSafety(args[0])
			if err != nil {
				return fmt.Errorf("route safety check failed: %w", err)
			}
			return printRouteSafetyResult(result)
		},
	}
}

func newSafetyCheckAllCmd(deps *safetyDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "check-all",
		Short: "Check safety for all active routes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := deps.safetySvc.CheckAllRoutesSafety()
			if err != nil {
				return fmt.Errorf("check all routes failed: %w", err)
			}
			if len(results) == 0 {
				fmt.Println("No active routes to check.")
				return nil
			}
			for _, r := range results {
				printRouteSafetyResult(&r)
				fmt.Println("---")
			}
			return nil
		},
	}
}

func newSafetyTraceEgressCmd(deps *safetyDeps) *cobra.Command {
	var fromNode string
	cmd := &cobra.Command{
		Use:   "trace-egress <domain>",
		Short: "Trace egress path for a domain",
		Long: `Trace the complete egress path for a domain, showing
DNS resolution, route matching, gateway link status, and risks.

Examples:
  aegis safety trace-egress example.com
  aegis safety trace-egress example.com --from-node nd_123
  aegis safety trace-egress example.com --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOutput, _ := cmd.Flags().GetBool("json")
			result, err := deps.safetySvc.TraceEgress(args[0], fromNode)
			if err != nil {
				return fmt.Errorf("trace egress failed: %w", err)
			}
			if jsonOutput {
				return printJSON(result)
			}
			return printEgressTraceResult(result)
		},
	}
	cmd.Flags().StringVar(&fromNode, "from-node", "", "Optional node ID to trace from")
	cmd.Flags().Bool("json", false, "Output as JSON")
	return cmd
}

func printRouteSafetyResult(r *safety.RouteSafetyResult) error {
	out, _ := json.MarshalIndent(r, "", "  ")
	fmt.Println(string(out))
	return nil
}

func printEgressTraceResult(t *safety.EgressTraceResult) error {
	fmt.Printf("Egress Trace: %s\n", t.Domain)
	fmt.Printf("  Resolved IPs:       %s\n", formatIPs(t.ResolvedIPs))
	fmt.Printf("  IP Classification:  %s\n", t.IPClassification)
	fmt.Printf("  Managed Domain:     %v\n", t.IsManagedDomain)
	if t.MatchedRouteID != "" {
		fmt.Printf("  Matched Route:      %s\n", t.MatchedRouteID)
	}
	fmt.Printf("  Current Node:       %s\n", t.CurrentNode)
	if t.TargetHost != "" {
		fmt.Printf("  Target:             %s:%d\n", t.TargetHost, t.TargetPort)
	}
	fmt.Printf("  Has Gateway Link:   %v\n", t.HasGatewayLink)
	if t.GatewayLinkID != "" {
		fmt.Printf("  Gateway Link ID:    %s\n", t.GatewayLinkID)
	}

	if len(t.Risks) > 0 {
		fmt.Println()
		fmt.Println("  Risks:")
		for _, risk := range t.Risks {
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

	if t.Recommendation != "" {
		fmt.Println()
		fmt.Printf("  Recommendation: %s\n", t.Recommendation)
	}

	return nil
}

func formatIPs(ips []string) string {
	if len(ips) == 0 {
		return "(none)"
	}
	return strings.Join(ips, ", ")
}

// printJSON prints v as indented JSON to stdout.
// This is the SINGLE CLI JSON output helper in the cli package.
// Do NOT create another printJSON/printRelayJSON/printAnyJSON function.
// Standard library: encoding/json.NewEncoder
func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
