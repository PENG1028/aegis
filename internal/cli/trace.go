package cli

import (
	"context"
	"fmt"

	"aegis/internal/trace"

	"github.com/spf13/cobra"
)

// traceDeps holds the dependencies needed for trace CLI commands.
type traceDeps struct {
	traceSvc *trace.Service
}

func newTraceCommand(traceSvc *trace.Service) *cobra.Command {
	deps := &traceDeps{traceSvc: traceSvc}

	cmd := &cobra.Command{
		Use:   "trace",
		Short: "Trace access path for domain, SNI, or route",
		Long: `Trace the complete access path for a domain, SNI host, or route ID.

Shows the full chain: entry listener → EdgeMux SNI match → Caddy TLS termination → route → target.

Examples:
  aegis trace domain example.com
  aegis trace sni example.com
  aegis trace route rt_abc123`,
	}

	cmd.AddCommand(newTraceDomainCmd(deps))
	cmd.AddCommand(newTraceSNICmd(deps))
	cmd.AddCommand(newTraceRouteCmd(deps))

	return cmd
}

func newTraceDomainCmd(deps *traceDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "domain <domain>",
		Short: "Trace access path for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result := deps.traceSvc.TraceDomain(context.Background(), args[0])
			printTraceResult(result)
			return nil
		},
	}
}

func newTraceSNICmd(deps *traceDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "sni <sni_host>",
		Short: "Trace access path for an SNI host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result := deps.traceSvc.TraceSNI(context.Background(), args[0])
			printTraceResult(result)
			return nil
		},
	}
}

func newTraceRouteCmd(deps *traceDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "route <route_id>",
		Short: "Trace access path for a route",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result := deps.traceSvc.TraceRoute(context.Background(), args[0])
			printTraceResult(result)
			return nil
		},
	}
}

func printTraceResult(t *trace.AccessPathTrace) {
	fmt.Printf("Trace: %s (type=%s)\n", t.Input, t.InputType)
	fmt.Printf("Status: %s\n\n", t.TraceStatus)

	fmt.Println("Access Path:")
	for _, step := range t.Steps {
		icon := "✓"
		switch step.Status {
		case "missing":
			icon = "✗"
		case "error":
			icon = "✗"
		case "skipped":
			icon = "-"
		}
		fmt.Printf("  [%d] %s %-12s %s\n", step.Order, icon, step.Component, step.Detail)
		if step.Address != "" {
			fmt.Printf("       → %s\n", step.Address)
		}
	}

	if t.FinalTarget != nil {
		fmt.Println()
		fmt.Printf("Final Target: %s:%d (%s)\n", t.FinalTarget.Host, t.FinalTarget.Port, t.FinalTarget.Protocol)
		if t.FinalTarget.Reachable != nil {
			if *t.FinalTarget.Reachable {
				fmt.Println("  Target connectivity: reachable ✓")
			} else {
				fmt.Printf("  Target connectivity: UNREACHABLE ✗\n")
				if t.FinalTarget.ErrorCode != "" {
					fmt.Printf("  Error code: %s\n", t.FinalTarget.ErrorCode)
				}
				if t.FinalTarget.ConnectError != "" {
					fmt.Printf("  Error: %s\n", t.FinalTarget.ConnectError)
				}
			}
		}
	}

	if len(t.Warnings) > 0 {
		fmt.Println()
		fmt.Println("Warnings:")
		for _, w := range t.Warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}
	}

	if len(t.Errors) > 0 {
		fmt.Println()
		fmt.Println("Errors:")
		for _, e := range t.Errors {
			fmt.Printf("  ✗ %s\n", e)
		}
	}
}
