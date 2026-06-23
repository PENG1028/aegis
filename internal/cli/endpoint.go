package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"aegis/internal/endpoint"
	"aegis/internal/id"
	"aegis/internal/logs"
	"aegis/internal/service"

	"github.com/spf13/cobra"
)

func newEndpointCommand(epRepo *endpoint.Repository, svcSvc *service.AppService, logSvc *logs.AppService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endpoint",
		Short: "Manage service endpoints",
		Long:  "Add, list, enable, and disable endpoints for services.",
	}

	cmd.AddCommand(newEndpointAddCommand(epRepo, svcSvc, logSvc))
	cmd.AddCommand(newEndpointListCommand(epRepo, svcSvc))
	cmd.AddCommand(newEndpointEnableCommand(epRepo, logSvc))
	cmd.AddCommand(newEndpointDisableCommand(epRepo, logSvc))

	return cmd
}

func newEndpointAddCommand(epRepo *endpoint.Repository, svcSvc *service.AppService, logSvc *logs.AppService) *cobra.Command {
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

			now := time.Now()
			ep := &endpoint.Endpoint{
				ID:        id.New("ep"),
				ServiceID: svc.ID,
				Type:      epType,
				Address:   address,
				Enabled:   true,
				CreatedAt: now,
				UpdatedAt: now,
			}

			if err := epRepo.Create(ep); err != nil {
				return fmt.Errorf("create endpoint: %w", err)
			}

			logSvc.Log(ctx, "endpoint.create", "endpoint", ep.ID, "success",
				fmt.Sprintf("added %s endpoint %s for service %s", epType, address, svc.Name), "cli")

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

func newEndpointEnableCommand(epRepo *endpoint.Repository, logSvc *logs.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <endpoint-id>",
		Short: "Enable an endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ep, err := epRepo.FindByID(args[0])
			if err != nil {
				return err
			}
			if ep == nil {
				return fmt.Errorf("endpoint %q not found", args[0])
			}
			if ep.Enabled {
				return fmt.Errorf("endpoint %s is already enabled", ep.ID)
			}
			ep.Enabled = true
			ep.UpdatedAt = time.Now()
			if err := epRepo.Update(ep); err != nil {
				return err
			}
			logSvc.Log(ctx, "endpoint.enable", "endpoint", ep.ID, "success", "enabled endpoint", "cli")
			fmt.Printf("Endpoint %s enabled.\n", ep.ID)
			return nil
		},
	}
}

func newEndpointDisableCommand(epRepo *endpoint.Repository, logSvc *logs.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <endpoint-id>",
		Short: "Disable an endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ep, err := epRepo.FindByID(args[0])
			if err != nil {
				return err
			}
			if ep == nil {
				return fmt.Errorf("endpoint %q not found", args[0])
			}
			if !ep.Enabled {
				return fmt.Errorf("endpoint %s is already disabled", ep.ID)
			}
			ep.Enabled = false
			ep.UpdatedAt = time.Now()
			if err := epRepo.Update(ep); err != nil {
				return err
			}
			logSvc.Log(ctx, "endpoint.disable", "endpoint", ep.ID, "success", "disabled endpoint", "cli")
			fmt.Printf("Endpoint %s disabled.\n", ep.ID)
			return nil
		},
	}
}
