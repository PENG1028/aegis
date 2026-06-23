package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"aegis/internal/route"

	"github.com/spf13/cobra"
)

func newMaintenanceCommand(svc *route.AppService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "maintenance",
		Short: "Manage maintenance mode for routes",
		Long:  "Enable or disable maintenance mode on routes.",
	}

	cmd.AddCommand(newMaintenanceOnCommand(svc))
	cmd.AddCommand(newMaintenanceOffCommand(svc))
	cmd.AddCommand(newMaintenanceStatusCommand(svc))

	return cmd
}

func newMaintenanceOnCommand(svc *route.AppService) *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:   "on <domain-or-id>",
		Short: "Enable maintenance mode for a route",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := svc.SetMaintenance(ctx, args[0], true, message); err != nil {
				return err
			}
			fmt.Printf("Maintenance mode enabled for %q.\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&message, "message", "", "Maintenance message to display")
	return cmd
}

func newMaintenanceOffCommand(svc *route.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "off <domain-or-id>",
		Short: "Disable maintenance mode for a route",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := svc.SetMaintenance(ctx, args[0], false, ""); err != nil {
				return err
			}
			fmt.Printf("Maintenance mode disabled for %q.\n", args[0])
			return nil
		},
	}
}

func newMaintenanceStatusCommand(svc *route.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show maintenance status for all routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			routes, err := svc.ListMaintenanceStatus(ctx)
			if err != nil {
				return err
			}

			if len(routes) == 0 {
				fmt.Println("No routes found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "DOMAIN\tMAINTENANCE\tMESSAGE")
			for _, r := range routes {
				maint := "off"
				if r.MaintenanceEnabled {
					maint = "ON"
				}
				msg := r.MaintenanceMessage
				if msg == "" {
					msg = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", r.Domain, maint, msg)
			}
			w.Flush()
			return nil
		},
	}
}
