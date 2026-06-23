package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"aegis/internal/project"
	"aegis/internal/route"
	"aegis/internal/service"

	"github.com/spf13/cobra"
)

func newRouteCommand(svc *route.AppService, serviceSvc *service.AppService, projSvc *project.AppService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Manage routes",
		Long:  "Add, list, show, enable, disable, and switch routes.",
	}

	cmd.AddCommand(newRouteAddCommand(svc, serviceSvc))
	cmd.AddCommand(newRouteListCommand(svc))
	cmd.AddCommand(newRouteShowCommand(svc))
	cmd.AddCommand(newRouteEnableCommand(svc))
	cmd.AddCommand(newRouteDisableCommand(svc))
	cmd.AddCommand(newRouteSwitchCommand(svc, serviceSvc))

	return cmd
}

func newRouteAddCommand(svc *route.AppService, serviceSvc *service.AppService) *cobra.Command {
	var serviceName string

	cmd := &cobra.Command{
		Use:   "add <domain>",
		Short: "Add a new route",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if serviceName == "" {
				return fmt.Errorf("--service is required")
			}

			svcID, err := resolveServiceID(serviceSvc, serviceName)
			if err != nil {
				return err
			}

			ctx := context.Background()
			rt, err := svc.CreateRoute(ctx, route.CreateRouteInput{
				Domain:    args[0],
				ServiceID: svcID,
			})
			if err != nil {
				return err
			}
			fmt.Printf("Route for %q created (ID: %s)\n", rt.Domain, rt.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&serviceName, "service", "", "Service name or ID (required)")
	return cmd
}

func newRouteListCommand(svc *route.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			routes, err := svc.ListRoutes(ctx)
			if err != nil {
				return err
			}

			if len(routes) == 0 {
				fmt.Println("No routes found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "DOMAIN\tTLS\tMAINTENANCE\tSTATUS")
			for _, r := range routes {
				tls := "off"
				if r.TLSEnabled {
					tls = "on"
				}
				maint := "off"
				if r.MaintenanceEnabled {
					maint = "on"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					r.Domain, tls, maint, r.Status)
			}
			w.Flush()
			return nil
		},
	}
}

func newRouteShowCommand(svc *route.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "show <domain-or-id>",
		Short: "Show route details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			r, err := svc.GetRoute(ctx, args[0])
			if err != nil {
				return err
			}

			tls := "off"
			if r.TLSEnabled {
				tls = "on"
			}
			maint := "off"
			if r.MaintenanceEnabled {
				maint = "on"
			}

			fmt.Printf("ID:                    %s\n", r.ID)
			fmt.Printf("Domain:                %s\n", r.Domain)
			fmt.Printf("Service ID:            %s\n", r.ServiceID)
			fmt.Printf("TLS:                   %s\n", tls)
			fmt.Printf("Status:                %s\n", r.Status)
			fmt.Printf("Maintenance:           %s\n", maint)
			fmt.Printf("Maintenance Message:   %s\n", r.MaintenanceMessage)
			fmt.Printf("Created:               %s\n", r.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Updated:               %s\n", r.UpdatedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}
}

func newRouteEnableCommand(svc *route.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <domain-or-id>",
		Short: "Enable a route",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := svc.EnableRoute(ctx, args[0]); err != nil {
				return err
			}
			fmt.Printf("Route %q enabled.\n", args[0])
			return nil
		},
	}
}

func newRouteDisableCommand(svc *route.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <domain-or-id>",
		Short: "Disable a route",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := svc.DisableRoute(ctx, args[0]); err != nil {
				return err
			}
			fmt.Printf("Route %q disabled.\n", args[0])
			return nil
		},
	}
}

func newRouteSwitchCommand(svc *route.AppService, serviceSvc *service.AppService) *cobra.Command {
	var serviceName string

	cmd := &cobra.Command{
		Use:   "switch <domain-or-id>",
		Short: "Switch a route to a different service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if serviceName == "" {
				return fmt.Errorf("--service is required")
			}

			svcID, err := resolveServiceID(serviceSvc, serviceName)
			if err != nil {
				return err
			}

			ctx := context.Background()
			if err := svc.SwitchRoute(ctx, args[0], svcID); err != nil {
				return err
			}
			fmt.Printf("Route %q switched to service %q.\n", args[0], serviceName)
			return nil
		},
	}

	cmd.Flags().StringVar(&serviceName, "service", "", "Target service name or ID (required)")
	return cmd
}
