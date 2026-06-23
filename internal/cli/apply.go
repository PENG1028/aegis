package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"aegis/internal/apply"

	"github.com/spf13/cobra"
)

func newApplyCommand(svc *apply.AppService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply gateway configuration",
		Long:  "Generate and apply the gateway configuration (Caddyfile).",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			dryRun, _ := cmd.Flags().GetBool("dry-run")
			showDiff, _ := cmd.Flags().GetBool("diff")

			if dryRun {
				plan, err := svc.DryRun(ctx)
				if err != nil {
					return err
				}
				// Print warnings
				for _, w := range plan.Warnings {
					fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", w.Severity, w.Code, w.Message)
				}

				if showDiff {
					current, _ := svc.GetCurrentConfig()
					fmt.Println("--- current")
					fmt.Println("+++ preview")
					if current != plan.RenderedConfig {
						fmt.Println(plan.RenderedConfig)
					} else {
						fmt.Println("(no changes)")
					}
				} else {
					fmt.Println(plan.RenderedConfig)
				}
				return nil
			}

			plan, err := svc.Apply(ctx)
			if err != nil {
				return err
			}

			for _, w := range plan.Warnings {
				fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", w.Severity, w.Code, w.Message)
			}
			fmt.Printf("Applied successfully: %d routes, %d managed domains\n",
				plan.RouteCount, plan.ManagedDomainCount)
			if plan.SkippedCount > 0 {
				fmt.Printf("  (%d skipped)\n", plan.SkippedCount)
			}
			return nil
		},
	}

	cmd.Flags().Bool("dry-run", false, "Generate and display config without applying")
	cmd.Flags().Bool("diff", false, "Show diff instead of full config")
	cmd.AddCommand(newApplyHistoryCommand(svc))

	return cmd
}

func newApplyHistoryCommand(svc *apply.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Show apply history",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			versions, err := svc.History(ctx)
			if err != nil {
				return err
			}

			if len(versions) == 0 {
				fmt.Println("No apply history.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "VERSION\tSTATUS\tMESSAGE\tCREATED")
			for _, v := range versions {
				msg := v.Message
				if len(msg) > 50 {
					msg = msg[:47] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					v.Version, v.Status, msg,
					v.CreatedAt.Format("2006-01-02 15:04:05"))
			}
			w.Flush()
			return nil
		},
	}
}

func newValidateCommand(svc *apply.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the generated configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := svc.Validate(ctx); err != nil {
				return err
			}
			fmt.Println("Configuration is valid.")
			return nil
		},
	}
}

func newRollbackCommand(svc *apply.AppService) *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback to the last successful configuration",
		Long:  "Rollback to the last successful configuration. Use --version to specify a specific version.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := svc.Rollback(ctx, version); err != nil {
				return err
			}
			if version != "" {
				fmt.Printf("Rolled back to version %s.\n", version)
			} else {
				fmt.Println("Rolled back to last successful version.")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "Rollback to a specific apply version")
	return cmd
}
