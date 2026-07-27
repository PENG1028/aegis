package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"aegis/internal/health"
	"aegis/internal/service"

	"github.com/spf13/cobra"
)

func newHealthCommand(svc *health.AppService, serviceSvc *service.AppService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health [service-name-or-id]",
		Short: "Check service health",
		Long: `Check health of all services or a specific service.

Without arguments, shows the latest health check for each service.
With a service name or ID, performs a live health check.
Use --all to perform live checks on all services.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			allFlag, _ := cmd.Flags().GetBool("all")

			if len(args) > 0 {
				// Check specific service
				s, err := serviceSvc.GetService(ctx, args[0])
				if err != nil {
					return err
				}
				results, _ := svc.CheckService(ctx, s)
				printHealthResults(results, serviceSvc)
				return nil
			}

			if allFlag {
				// Live check all
				results, err := svc.CheckAll(ctx)
				if err != nil {
					return err
				}
				printHealthResults(results, serviceSvc)
				return nil
			}

			// Show latest cached results
			results, err := svc.GetLatestForAll(ctx)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Println("No health checks recorded. Run 'aegis health --all' to check all services.")
				return nil
			}
			printHealthResults(results, serviceSvc)
			return nil
		},
	}

	cmd.Flags().Bool("all", false, "Perform live health checks on all services")
	return cmd
}

func printHealthResults(results []health.HealthCheck, svc *service.AppService) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tSTATUS\tLATENCY\tMESSAGE\tCHECKED")
	for _, h := range results {
		svcName := h.ServiceID
		if s, err := svc.GetService(context.Background(), h.ServiceID); err == nil {
			svcName = s.Name
		}
		latency := fmt.Sprintf("%dms", h.LatencyMS)
		if h.LatencyMS == 0 {
			latency = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			svcName, h.Status, latency, h.Message,
			h.CheckedAt.Format("15:04:05"))
	}
	w.Flush()
}

