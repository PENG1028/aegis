package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"aegis/internal/project"
	"aegis/internal/service"

	"github.com/spf13/cobra"
)

func newServiceCommand(svc *service.AppService, projSvc *project.AppService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage services",
		Long:  "Add, list, show, enable, disable, and update backend services.",
	}

	cmd.AddCommand(newServiceAddCommand(svc, projSvc))
	cmd.AddCommand(newServiceListCommand(svc))
	cmd.AddCommand(newServiceShowCommand(svc))
	cmd.AddCommand(newServiceEnableCommand(svc))
	cmd.AddCommand(newServiceDisableCommand(svc))
	cmd.AddCommand(newServiceUpdateCommand(svc))

	return cmd
}

func resolveProjectID(projSvc *project.AppService, name string) (string, error) {
	ctx := context.Background()
	p, err := projSvc.GetProject(ctx, name)
	if err != nil {
		return "", fmt.Errorf("resolve project %q: %w", name, err)
	}
	return p.ID, nil
}

func resolveServiceID(svcSvc *service.AppService, name string) (string, error) {
	ctx := context.Background()
	s, err := svcSvc.GetService(ctx, name)
	if err != nil {
		return "", fmt.Errorf("resolve service %q: %w", name, err)
	}
	return s.ID, nil
}

func newServiceAddCommand(svc *service.AppService, projSvc *project.AppService) *cobra.Command {
	var projectName, env, kind string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectName == "" {
				return fmt.Errorf("--project is required")
			}

			projID, err := resolveProjectID(projSvc, projectName)
			if err != nil {
				return err
			}

			ctx := context.Background()
			s, err := svc.CreateService(ctx, service.CreateServiceInput{
				ProjectID: projID,
				Name:      args[0],
				Kind:      kind,
				Env:       env,
			})
			if err != nil {
				return err
			}
			fmt.Printf("Service %q created (ID: %s, kind: %s)\n", s.Name, s.ID, s.Kind)
			fmt.Println("Use 'aegis endpoint add' to add upstream endpoints to this service.")
			return nil
		},
	}

	cmd.Flags().StringVar(&projectName, "project", "", "Project name or ID (required)")
	cmd.Flags().StringVar(&env, "env", "prod", "Environment: dev, preview, or prod")
	cmd.Flags().StringVar(&kind, "kind", "http", "Service kind: http, tcp, or file")
	return cmd
}

func newServiceListCommand(svc *service.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all services",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			services, err := svc.ListServices(ctx)
			if err != nil {
				return err
			}

			if len(services) == 0 {
				fmt.Println("No services found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NAME\tKIND\tENV\tSTATUS")
			for _, s := range services {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					s.Name, s.Kind, s.Env, s.Status)
			}
			w.Flush()
			return nil
		},
	}
}

func newServiceShowCommand(svc *service.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name-or-id>",
		Short: "Show service details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			s, err := svc.GetService(ctx, args[0])
			if err != nil {
				return err
			}

			fmt.Printf("ID:              %s\n", s.ID)
			fmt.Printf("Name:            %s\n", s.Name)
			fmt.Printf("Project ID:      %s\n", s.ProjectID)
			fmt.Printf("Kind:            %s\n", s.Kind)
			fmt.Printf("Environment:     %s\n", s.Env)
			fmt.Printf("Status:          %s\n", s.Status)
			fmt.Printf("Note:            %s\n", s.Note)
			fmt.Printf("Created:         %s\n", s.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Updated:         %s\n", s.UpdatedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}
}

func newServiceEnableCommand(svc *service.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name-or-id>",
		Short: "Enable a disabled service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := svc.EnableService(ctx, args[0]); err != nil {
				return err
			}
			fmt.Printf("Service %q enabled.\n", args[0])
			return nil
		},
	}
}

func newServiceDisableCommand(svc *service.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name-or-id>",
		Short: "Disable a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := svc.DisableService(ctx, args[0]); err != nil {
				return err
			}
			fmt.Printf("Service %q disabled.\n", args[0])
			return nil
		},
	}
}

func newServiceUpdateCommand(svc *service.AppService) *cobra.Command {
	var kind, env, note string

	cmd := &cobra.Command{
		Use:   "update <name-or-id>",
		Short: "Update a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			input := service.UpdateServiceInput{}
			if cmd.Flags().Changed("kind") {
				input.Kind = &kind
			}
			if cmd.Flags().Changed("env") {
				input.Env = &env
			}
			if cmd.Flags().Changed("note") {
				input.Note = &note
			}

			s, err := svc.UpdateService(ctx, args[0], input)
			if err != nil {
				return err
			}
			fmt.Printf("Service %q updated.\n", s.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&kind, "kind", "", "New service kind")
	cmd.Flags().StringVar(&env, "env", "", "New environment")
	cmd.Flags().StringVar(&note, "note", "", "New note")

	return cmd
}
