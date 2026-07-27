package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"aegis/internal/endpoint"
	"aegis/internal/service"

	"github.com/spf13/cobra"
)

func newEndpointCommand(epRepo *endpoint.Repository, epSvc *endpoint.AppService, svcSvc *service.AppService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endpoint",
		Short: "Manage service endpoints",
		Long:  "Add, list, enable, and disable endpoints for services.",
	}

	cmd.AddCommand(newEndpointAddCommand(epSvc, svcSvc))
	cmd.AddCommand(newEndpointListCommand(epRepo, svcSvc))
	cmd.AddCommand(newEndpointEnableCommand(epSvc))
	cmd.AddCommand(newEndpointDisableCommand(epSvc))

	return cmd
}

func newEndpointAddCommand(epSvc *endpoint.AppService, svcSvc *service.AppService) *cobra.Command {
	var epType, address string

	cmd := &cobra.Command{
		Use:   "add <service-name-or-id>",
		Short: "Add an endpoint to a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			svc, err := svcSvc.GetService(ctx, args[0])
			if err != nil {
				return err
			}

			if epType == "" {
				epType = "local"
			}
			if address == "" {
				return fmt.Errorf("--address is required")
			}

			ep, err := epSvc.CreateEndpoint(ctx, endpoint.CreateEndpointInput{
				ServiceID: svc.ID,
				Type:      epType,
				Address:   address,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Endpoint added to service %q (ID: %s, type: %s, address: %s)\n",
				svc.Name, ep.ID, epType, address)
			return nil
		},
	}

	cmd.Flags().StringVar(&epType, "type", "local", "Endpoint type: local, private, or public")
	cmd.Flags().StringVar(&address, "address", "", "Endpoint address (e.g., http://127.0.0.1:3001) (required)")
	return cmd
}

func newEndpointListCommand(epRepo *endpoint.Repository, svcSvc *service.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "list <service-name-or-id>",
		Short: "List endpoints for a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			svc, err := svcSvc.GetService(ctx, args[0])
			if err != nil {
				return err
			}

			endpoints, err := epRepo.FindByServiceID(svc.ID)
			if err != nil {
				return err
			}

			if len(endpoints) == 0 {
				fmt.Printf("No endpoints for service %q.\n", svc.Name)
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "ID\tTYPE\tADDRESS\tENABLED")
			for _, ep := range endpoints {
				enabledStr := "yes"
				if !ep.Enabled {
					enabledStr = "no"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					ep.ID, ep.Type, ep.Address, enabledStr)
			}
			w.Flush()
			return nil
		},
	}
}

func newEndpointEnableCommand(epSvc *endpoint.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <endpoint-id>",
		Short: "Enable an endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := epSvc.EnableEndpoint(ctx, args[0]); err != nil {
				return err
			}
			fmt.Printf("Endpoint %s enabled.\n", args[0])
			return nil
		},
	}
}

func newEndpointDisableCommand(epSvc *endpoint.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <endpoint-id>",
		Short: "Disable an endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := epSvc.DisableEndpoint(ctx, args[0]); err != nil {
				return err
			}
			fmt.Printf("Endpoint %s disabled.\n", args[0])
			return nil
		},
	}
}
