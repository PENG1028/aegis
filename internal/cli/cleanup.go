package cli

import (
	"database/sql"
	"fmt"

	"aegis/internal/maintenance"

	"github.com/spf13/cobra"
)

func newCleanupCommand(db *sql.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Cleanup stale and orphan data",
		Long: `Removes orphan edge rules, stale nodes (>7 days), and old upgrade sessions (>30 days).

Run periodically to prevent unbounded state growth. Safe to re-run.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("=== Aegis Cleanup ===")
			fmt.Println()

			stats, err := maintenance.RunCleanup(db)
			if err != nil {
				return err
			}

			fmt.Printf("orphan edge rules:     %d removed\n", stats.OrphanEdgeRules)
			fmt.Printf("stale nodes:           %d removed\n", stats.StaleNodes)
			fmt.Printf("old upgrade sessions:  %d removed\n", stats.OldSessions)
			fmt.Printf("total removed:         %d\n", stats.TotalRemoved)
			fmt.Println()
			fmt.Println("Cleanup complete.")
			return nil
		},
	}
}
