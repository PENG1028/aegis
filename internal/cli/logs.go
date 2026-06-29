package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"aegis/internal/logs"

	"github.com/spf13/cobra"
)

func newLogsCommand(svc logs.Logger) *cobra.Command {
	var action, target string

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View operation logs",
		Long:  "Show recent operation logs, optionally filtered by action or target.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			entries, err := svc.ListLogs(ctx, action, target)
			if err != nil {
				return err
			}

			if len(entries) == 0 {
				fmt.Println("No operation logs found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TIME\tACTION\tTARGET\tRESULT\tMESSAGE")
			for _, e := range entries {
				msg := e.Message
				if len(msg) > 50 {
					msg = msg[:47] + "..."
				}
				targetInfo := e.TargetType
				if e.TargetID != "" {
					targetInfo = targetInfo + ":" + e.TargetID
				}
				if targetInfo == "" {
					targetInfo = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					e.CreatedAt.Format("01-02 15:04:05"),
					e.Action, targetInfo, e.Result, msg)
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&action, "action", "", "Filter by action (e.g., apply, project.create)")
	cmd.Flags().StringVar(&target, "target", "", "Filter by target ID")

	return cmd
}
