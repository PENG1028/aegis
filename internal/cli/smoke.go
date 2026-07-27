package cli

import (
	"context"
	"fmt"
	"strings"

	"aegis/internal/smoke"

	"github.com/spf13/cobra"
)

func newSmokeCommand(smokeSvc *smoke.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "smoke",
		Short: "Runtime smoke tests for acceptance verification",
		Long: `Run smoke tests to verify system health and acceptance criteria.

Smoke tests are READ-ONLY by default. They validate existing state
without modifying resources. The failure-matrix subcommand uses an
in-memory fake provider and never touches the real system.

Subcommands:
  golden          Run golden path checks (config, DB, listeners, providers, state)
  provider        Check provider health and diagnostics
  trace           Verify trace output for a domain
  failure-matrix  Run failure injection matrix using fake provider
  restart-check   Verify state integrity after restart`,
	}

	cmd.AddCommand(newSmokeGoldenCmd(smokeSvc))
	cmd.AddCommand(newSmokeProviderCmd(smokeSvc))
	cmd.AddCommand(newSmokeTraceCmd(smokeSvc))
	cmd.AddCommand(newSmokeFailureMatrixCmd(smokeSvc))
	cmd.AddCommand(newSmokeRestartCheckCmd(smokeSvc))

	return cmd
}

func newSmokeGoldenCmd(smokeSvc *smoke.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "golden",
		Short: "Run golden path checks (read-only)",
		Long: `Verify core system health without modifying any state.

Checks:
  - Config file exists and is readable
  - Database is accessible
  - Listeners are registered
  - Providers are available
  - State version is initialized
  - No pending apply (clean state)
  - Routes are queryable`,
		RunE: func(cmd *cobra.Command, args []string) error {
			result := smokeSvc.RunGoldenPath(context.Background())
			printSmokeResult(result)
			return nil
		},
	}
}

func newSmokeProviderCmd(smokeSvc *smoke.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "provider",
		Short: "Check provider health and diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			result := smokeSvc.RunProviderSmoke(context.Background())
			printSmokeResult(result)
			return nil
		},
	}
}

func newSmokeTraceCmd(smokeSvc *smoke.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "trace <domain>",
		Short: "Verify trace output for a domain",
		Long:  "Runs a trace for the specified domain and verifies the access path is complete and consistent.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result := smokeSvc.RunTraceSmoke(context.Background(), args[0])
			printSmokeResult(result)
			return nil
		},
	}
}

func newSmokeFailureMatrixCmd(smokeSvc *smoke.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "failure-matrix",
		Short: "Run failure injection matrix using fake provider",
		Long: `Run all failure matrix cases using the in-memory fake provider.

This does NOT modify the real system. It creates a FakeProvider,
injects each failure mode, verifies the correct error code is
produced, then resets.

Covers:
  - 7 provider diagnostic error codes
  - Apply locked scenario
  - Gateway mutation frozen scenario`,
		RunE: func(cmd *cobra.Command, args []string) error {
			result := smokeSvc.RunFailureMatrix(context.Background())
			printSmokeResult(result)
			return nil
		},
	}
}

func newSmokeRestartCheckCmd(smokeSvc *smoke.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "restart-check",
		Short: "Verify state integrity after restart",
		Long: `Run post-restart checks to verify control plane state recovered cleanly.

Checks:
  - Database is accessible
  - State version is preserved (not reset to 0)
  - pending_apply is clean (not erroneously set)
  - Listeners are preserved
  - Config file still exists`,
		RunE: func(cmd *cobra.Command, args []string) error {
			result := smokeSvc.RunRestartCheck(context.Background())
			printSmokeResult(result)
			return nil
		},
	}
}

func printSmokeResult(r *smoke.SmokeResult) {
	// Header
	statusIcon := "✓"
	if !r.Passed {
		statusIcon = "✗"
	}
	fmt.Printf("\n%s Smoke: %s\n", statusIcon, r.Name)
	fmt.Printf("%s\n", strings.Repeat("─", 60))

	// Checks
	for _, c := range r.Checks {
		icon := "✓"
		switch c.Status {
		case "fail":
			icon = "✗"
		case "warn":
			icon = "⚠"
		case "skip":
			icon = "-"
		}
		fmt.Printf("  %s %-20s %s\n", icon, c.Name+":", c.Message)
		if c.Detail != "" {
			for _, line := range strings.Split(c.Detail, "\n") {
				fmt.Printf("       %s\n", line)
			}
		}
	}

	// Summary
	fmt.Printf("%s\n", strings.Repeat("─", 60))
	fmt.Printf("Result: %d/%d passed, %d failed", r.Passed_, r.Total, r.Failed)
	if !r.Passed {
		fmt.Printf(" ✗")
	}
	fmt.Println()
	fmt.Println(r.Summary)
	fmt.Println()
}
