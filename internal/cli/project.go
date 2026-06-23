package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"aegis/internal/project"

	"github.com/spf13/cobra"
)

func newProjectCommand(svc *project.AppService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
		Long:  "Create, list, show, and archive projects.",
	}

	cmd.AddCommand(newProjectCreateCommand(svc))
	cmd.AddCommand(newProjectListCommand(svc))
	cmd.AddCommand(newProjectShowCommand(svc))
	cmd.AddCommand(newProjectArchiveCommand(svc))

	return cmd
}

func newProjectCreateCommand(svc *project.AppService) *cobra.Command {
	var description string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			p, err := svc.CreateProject(ctx, project.CreateProjectInput{
				Name:        args[0],
				Description: description,
			})
			if err != nil {
				return err
			}
			fmt.Printf("Project %q created (ID: %s)\n", p.Name, p.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Project description")
	return cmd
}

func newProjectListCommand(svc *project.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projects, err := svc.ListProjects(ctx)
			if err != nil {
				return err
			}

			if len(projects) == 0 {
				fmt.Println("No projects found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NAME\tDESCRIPTION\tSTATUS\tCREATED")
			for _, p := range projects {
				desc := p.Description
				if len(desc) > 40 {
					desc = desc[:37] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					p.Name, desc, p.Status,
					p.CreatedAt.Format("2006-01-02 15:04"))
			}
			w.Flush()
			return nil
		},
	}
}

func newProjectShowCommand(svc *project.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name-or-id>",
		Short: "Show project details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			p, err := svc.GetProject(ctx, args[0])
			if err != nil {
				return err
			}

			fmt.Printf("ID:          %s\n", p.ID)
			fmt.Printf("Name:        %s\n", p.Name)
			fmt.Printf("Description: %s\n", p.Description)
			fmt.Printf("Status:      %s\n", p.Status)
			fmt.Printf("Created:     %s\n", p.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Updated:     %s\n", p.UpdatedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}
}

func newProjectArchiveCommand(svc *project.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "archive <name-or-id>",
		Short: "Archive a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := svc.ArchiveProject(ctx, args[0]); err != nil {
				return err
			}
			fmt.Printf("Project %q archived.\n", args[0])
			return nil
		},
	}
}
