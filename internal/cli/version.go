package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand(version, buildTime string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Aegis version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Aegis %s (built %s)\n", version, buildTime)
			return nil
		},
	}
}
