package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"aegis/internal/action"

	"github.com/spf13/cobra"
)

func newActionCommand(actionSvc *action.ActionService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "action",
		Short: "Manage domains and resources via the Action API",
		Long:  "High-level action commands that abstract away Caddy/HAProxy details.",
	}

	cmd.AddCommand(newActionBindHTTPDomain(actionSvc))
	cmd.AddCommand(newActionBindTLSBackend(actionSvc))
	cmd.AddCommand(newActionUpdateTarget(actionSvc))
	cmd.AddCommand(newActionDisableDomain(actionSvc))
	cmd.AddCommand(newActionDeleteDomain(actionSvc))
	cmd.AddCommand(newActionListMyRoutes(actionSvc))
	cmd.AddCommand(newActionListMyServices(actionSvc))
	cmd.AddCommand(newActionListMyEdgeRules(actionSvc))
	cmd.AddCommand(newActionListMyOperations(actionSvc))

	return cmd
}

// adminContext creates an admin action context for CLI operations.
func adminContext() context.Context {
	return action.WithActionContext(context.Background(), action.NewAdminContext())
}

func newActionBindHTTPDomain(svc *action.ActionService) *cobra.Command {
	var targetPort int
	cmd := &cobra.Command{
		Use:   "bind-http-domain <domain> <target_host>",
		Short: "Bind an HTTP domain to a backend target",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := action.BindHTTPDomainInput{
				Domain:     args[0],
				TargetHost: args[1],
				TargetPort: targetPort,
			}
			result, err := svc.BindHTTPDomain(adminContext(), input)
			if err != nil {
				return err
			}
			printResult(result)
			return nil
		},
	}
	cmd.Flags().IntVar(&targetPort, "target-port", 80, "Target port (default: 80)")
	return cmd
}

func newActionBindTLSBackend(svc *action.ActionService) *cobra.Command {
	var targetPort int
	var kind string
	cmd := &cobra.Command{
		Use:   "bind-tls-backend <sni_host> <target_host>",
		Short: "Bind a TLS SNI host to a backend target",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := action.BindTLSBackendInput{
				SNIHost:    args[0],
				TargetHost: args[1],
				TargetPort: targetPort,
				Kind:       kind,
			}
			result, err := svc.BindTLSBackend(adminContext(), input)
			if err != nil {
				return err
			}
			printResult(result)
			return nil
		},
	}
	cmd.Flags().IntVar(&targetPort, "target-port", 443, "Target port (default: 443)")
	cmd.Flags().StringVar(&kind, "kind", "", "Declared kind (default: unknown_tls_backend)")
	return cmd
}

func newActionUpdateTarget(svc *action.ActionService) *cobra.Command {
	var resourceType string
	var targetPort int
	cmd := &cobra.Command{
		Use:   "update-target <resource_id> <target_host>",
		Short: "Update the target of a service or edge rule",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := action.UpdateTargetInput{
				ResourceID:   args[0],
				TargetHost:   args[1],
				TargetPort:   targetPort,
				ResourceType: resourceType,
			}
			result, err := svc.UpdateTarget(adminContext(), input)
			if err != nil {
				return err
			}
			printResult(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&resourceType, "type", "service", "Resource type: service or edge_rule")
	cmd.Flags().IntVar(&targetPort, "target-port", 80, "Target port")
	return cmd
}

func newActionDisableDomain(svc *action.ActionService) *cobra.Command {
	var resourceType string
	cmd := &cobra.Command{
		Use:   "disable-domain <domain>",
		Short: "Disable a domain binding",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := action.DisableDomainInput{
				Domain:       args[0],
				ResourceType: resourceType,
			}
			result, err := svc.DisableDomain(adminContext(), input)
			if err != nil {
				return err
			}
			printResult(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&resourceType, "type", "", "Resource type: route or edge_rule (auto-detect if empty)")
	return cmd
}

func newActionDeleteDomain(svc *action.ActionService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-domain <domain>",
		Short: "Delete a domain binding and its managed resources",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := action.DeleteDomainInput{Domain: args[0]}
			result, err := svc.DeleteDomain(adminContext(), input)
			if err != nil {
				return err
			}
			printResult(result)
			return nil
		},
	}
	return cmd
}

func newActionListMyRoutes(svc *action.ActionService) *cobra.Command {
	return &cobra.Command{
		Use:   "list-my-routes",
		Short: "List routes in the current space (or all for admin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			routes, err := svc.ListMyRoutes(adminContext())
			if err != nil {
				return err
			}
			for _, rt := range routes {
				fmt.Printf("  %s  %-30s  %s  %s\n", rt.ID[:12], rt.Domain, rt.Status, rt.SpaceID)
			}
			fmt.Printf("\n%d routes\n", len(routes))
			return nil
		},
	}
}

func newActionListMyServices(svc *action.ActionService) *cobra.Command {
	return &cobra.Command{
		Use:   "list-my-services",
		Short: "List services in the current space (or all for admin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			services, err := svc.ListMyServices(adminContext())
			if err != nil {
				return err
			}
			for _, s := range services {
				fmt.Printf("  %s  %-25s  %s  %s  %s\n", s.ID[:12], s.Name, s.Kind, s.Status, s.SpaceID)
			}
			fmt.Printf("\n%d services\n", len(services))
			return nil
		},
	}
}

func newActionListMyEdgeRules(svc *action.ActionService) *cobra.Command {
	return &cobra.Command{
		Use:   "list-my-edge-rules",
		Short: "List edge rules in the current space (or all for admin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			rules, err := svc.ListMyEdgeRules(adminContext())
			if err != nil {
				return err
			}
			for _, r := range rules {
				fmt.Printf("  %s  %-30s  %s:%d  %s  %s  %s\n", r.ID[:12], r.SNIHost, r.TargetHost, r.TargetPort, r.ManagedBy, r.Status, r.SpaceID)
			}
			fmt.Printf("\n%d edge rules\n", len(rules))
			return nil
		},
	}
}

func newActionListMyOperations(svc *action.ActionService) *cobra.Command {
	return &cobra.Command{
		Use:   "list-my-operations",
		Short: "List recent operations in the current space",
		RunE: func(cmd *cobra.Command, args []string) error {
			ops, err := svc.ListMyOperations(adminContext(), 20)
			if err != nil {
				return err
			}
			for _, op := range ops {
				fmt.Printf("  %s  %-30s  %s  %s\n", op.CreatedAt.Format("2006-01-02 15:04"), op.Action, op.Result, op.Message)
			}
			fmt.Printf("\n%d operations\n", len(ops))
			return nil
		},
	}
}

func printResult(result *action.ActionResult) {
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}
