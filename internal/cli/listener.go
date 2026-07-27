package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"aegis/internal/listener"

	"github.com/spf13/cobra"
)

func newListenerCommand(svc *listener.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listener",
		Short: "Manage listeners",
		Long:  "List and inspect listener bindings.",
	}

	cmd.AddCommand(newListenerListCommand(svc))

	return cmd
}

func newListenerListCommand(svc *listener.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all listeners",
		RunE: func(cmd *cobra.Command, args []string) error {
			listeners, err := svc.ListAll()
			if err != nil {
				return err
			}

			if len(listeners) == 0 {
				fmt.Println("No listeners registered.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "ID\tPROVIDER\tPROTOCOL\tBIND\tPORT\tSTATUS")
			for _, l := range listeners {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
					l.ID, l.Provider, l.Protocol, l.BindIP, l.Port, l.Status)
			}
			w.Flush()
			return nil
		},
	}
}
