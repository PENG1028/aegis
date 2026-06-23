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
			if dryRun {
				result, err := svc.DryRun(ctx)
				if err != nil {
					return err
				}
				for _, w := range result.Warnings {
					fmt.Fprintln(os.Stderr, w)
				}
				fmt.Println(result.Config)
				return nil
			}

			result, err := svc.Apply(ctx)
			if err != nil {
				return err
			}

			for _, w := range result.Warnings {
				fmt.Fprintln(os.Stderr, w)
			}
			fmt.Printf("Applied version %s successfully.\n", result.Version)
			return nil
		},
	}

	cmd.Flags().Bool("dry-run", false, "Generate and display config without applying")

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
	return &cobra.Command{
		Use:   "rollback",
		Short: "Rollback to the last successful configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := svc.Rollback(ctx); err != nil {
				return err
			}
			fmt.Println("Rollback completed.")
			return nil
		},
	}
}
