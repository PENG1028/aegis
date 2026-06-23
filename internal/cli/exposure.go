package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"aegis/internal/exposure"
	"aegis/internal/service"

	"github.com/spf13/cobra"
)

func newExposureCommand(expSvc *exposure.AppService, svcSvc *service.AppService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exposure",
		Short: "Manage service exposures",
		Long:  "Create, list, activate, and disable service exposures. HTTP exposures generate Caddy routes; TCP/UDP are record-only.",
	}

	cmd.AddCommand(newExposureAddCommand(expSvc, svcSvc))
	cmd.AddCommand(newExposureListCommand(expSvc))
	cmd.AddCommand(newExposureActivateCommand(expSvc))
	cmd.AddCommand(newExposureDisableCommand(expSvc))
	cmd.AddCommand(newExposureShowCommand(expSvc))
	cmd.AddCommand(newExposureStatsCommand(expSvc))

	return cmd
}

func newExposureAddCommand(expSvc *exposure.AppService, svcSvc *service.AppService) *cobra.Command {
	var expType, mode, host, path, nodeID, ownerRef, targetRef string
	var port int

	cmd := &cobra.Command{
		Use:   "add <service-name-or-id>",
		Short: "Add an exposure for a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			svc, err := svcSvc.GetService(ctx, args[0])
			if err != nil {
				return err
			}

			if expType == "" {
				expType = "http"
			}
			if ownerRef == "" {
				return fmt.Errorf("--owner is required")
			}
			if host == "" {
				return fmt.Errorf("--host is required")
			}

			e, err := expSvc.CreateExposure(ctx, exposure.CreateExposureInput{
				Type:      expType,
				Mode:      mode,
				Host:      host,
				Port:      port,
				Path:      path,
				ServiceID: svc.ID,
				NodeID:    nodeID,
				OwnerRef:  ownerRef,
				TargetRef: targetRef,
			})
			if err != nil {
				return err
			}

			generates := "no"
			if exposure.GeneratesConfig(e.Type) {
				generates = "yes (Caddy route will be generated)"
			}
			fmt.Printf("Exposure created (ID: %s)\n", e.ID)
			fmt.Printf("  Type: %s | Host: %s:%d | Generates config: %s\n", e.Type, e.Host, e.Port, generates)
			fmt.Printf("  Status: %s — run 'aegis exposure activate %s' to activate\n", e.Status, e.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&expType, "type", "http", "Exposure type: http, tcp, udp, tunnel, internal")
	cmd.Flags().StringVar(&mode, "mode", "private", "Mode: public, private, internal")
	cmd.Flags().StringVar(&host, "host", "", "Host/domain (required)")
	cmd.Flags().IntVar(&port, "port", 0, "Port")
	cmd.Flags().StringVar(&path, "path", "", "URL path prefix")
	cmd.Flags().StringVar(&nodeID, "node", "", "Gateway node ID")
	cmd.Flags().StringVar(&ownerRef, "owner", "", "Owner reference (required)")
	cmd.Flags().StringVar(&targetRef, "target", "", "Target reference")
	return cmd
}

func newExposureListCommand(expSvc *exposure.AppService) *cobra.Command {
	var ownerRef string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List exposures",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			var exposures []exposure.Exposure
			var err error
			if ownerRef != "" {
				exposures, err = expSvc.ListExposuresByOwner(ctx, ownerRef)
			} else {
				exposures, err = expSvc.ListExposures(ctx)
			}
			if err != nil {
				return err
			}

			if len(exposures) == 0 {
				fmt.Println("No exposures found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "ID\tTYPE\tHOST:PORT\tOWNER\tSTATUS\tGENERATES")
			for _, e := range exposures {
				gen := "no"
				if exposure.GeneratesConfig(e.Type) && e.Status == "active" {
					gen = "YES"
				}
				fmt.Fprintf(w, "%s\t%s\t%s:%d\t%s\t%s\t%s\n",
					e.ID, e.Type, e.Host, e.Port, e.OwnerRef, e.Status, gen)
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&ownerRef, "owner", "", "Filter by owner reference")
	return cmd
}

func newExposureActivateCommand(expSvc *exposure.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "activate <exposure-id>",
		Short: "Activate an exposure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			e, err := expSvc.ActivateExposure(ctx, args[0], "")
			if err != nil {
				return err
			}
			fmt.Printf("Exposure %s activated (status: %s)\n", e.ID, e.Status)
			if e.Status == "active" {
				fmt.Println("This HTTP exposure will generate a Caddy route on next apply.")
			} else {
				fmt.Println("This exposure is recorded only — no proxy config generated.")
			}
			return nil
		},
	}
}

func newExposureDisableCommand(expSvc *exposure.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <exposure-id>",
		Short: "Disable an exposure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			e, err := expSvc.DisableExposure(ctx, args[0], "")
			if err != nil {
				return err
			}
			fmt.Printf("Exposure %s disabled.\n", e.ID)
			return nil
		},
	}
}

func newExposureShowCommand(expSvc *exposure.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "show <exposure-id>",
		Short: "Show exposure details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			e, err := expSvc.GetExposure(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("ID:             %s\n", e.ID)
			fmt.Printf("Type:           %s\n", e.Type)
			fmt.Printf("Mode:           %s\n", e.Mode)
			fmt.Printf("Host:           %s\n", e.Host)
			fmt.Printf("Port:           %d\n", e.Port)
			fmt.Printf("Path:           %s\n", e.Path)
			fmt.Printf("Service ID:     %s\n", e.ServiceID)
			fmt.Printf("Node ID:        %s\n", e.NodeID)
			fmt.Printf("Owner Ref:      %s\n", e.OwnerRef)
			fmt.Printf("Target Ref:     %s\n", e.TargetRef)
			fmt.Printf("Status:         %s\n", e.Status)
			fmt.Printf("Message:        %s\n", e.Message)
			fmt.Printf("Generates Config: %v\n", exposure.GeneratesConfig(e.Type))
			fmt.Printf("Created:        %s\n", e.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Updated:        %s\n", e.UpdatedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}
}

func newExposureStatsCommand(expSvc *exposure.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show exposure statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			stats, err := expSvc.GetStats(ctx)
			if err != nil {
				return err
			}
			fmt.Printf("Total exposures:       %d\n", stats.Total)
			fmt.Printf("HTTP active:           %d\n", stats.HTTPActive)
			fmt.Printf("Non-HTTP recorded:     %d\n", stats.NonHTTPRecorded)
			fmt.Println()
			fmt.Println("By type:")
			for typ, count := range stats.ByType {
				fmt.Printf("  %s: %d\n", typ, count)
			}
			fmt.Println()
			fmt.Println("By status:")
			for status, count := range stats.ByStatus {
				fmt.Printf("  %s: %d\n", status, count)
			}
			return nil
		},
	}
}
