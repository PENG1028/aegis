package cli

import (
	"context"
	"fmt"

	"aegis/internal/apply"

	"github.com/spf13/cobra"
)

func newConfigCommand(svc *apply.AppService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and compare configuration",
		Long:  "View current, preview, or diff of the proxy configuration.",
	}

	cmd.AddCommand(newConfigCurrentCmd(svc))
	cmd.AddCommand(newConfigPreviewCmd(svc))
	cmd.AddCommand(newConfigDiffCmd(svc))

	return cmd
}

func newConfigCurrentCmd(svc *apply.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show current proxy configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := svc.GetCurrentConfig()
			if err != nil {
				return err
			}
			if config == "" {
				fmt.Println("(no current configuration)")
				return nil
			}
			fmt.Print(config)
			return nil
		},
	}
}

func newConfigPreviewCmd(svc *apply.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "preview",
		Short: "Show preview of the next configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			plan, err := svc.DryRun(ctx)
			if err != nil {
				return err
			}
			fmt.Print(plan.RenderedConfig)
			return nil
		},
	}
}

func newConfigDiffCmd(svc *apply.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Show diff between current and preview config",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			current, _ := svc.GetCurrentConfig()
			plan, err := svc.DryRun(ctx)
			if err != nil {
				return err
			}

			if current == "" {
				fmt.Println("--- (empty)")
			} else {
				fmt.Println("--- current")
			}
			fmt.Println("+++ preview")
			if current != plan.RenderedConfig {
				fmt.Print(plan.RenderedConfig)
			} else {
				fmt.Println("(no changes)")
			}
			return nil
		},
	}
}
