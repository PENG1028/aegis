package cli

import (
	"fmt"

	"aegis/internal/config"

	"github.com/spf13/cobra"
)

func newSettingsCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "settings",
		Short: "Show current settings",
		Long:  "Display the current Aegis configuration settings (read-only).",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Current Aegis Settings")
			fmt.Println("======================")
			fmt.Println()
			fmt.Println("[proxy]")
			fmt.Printf("  provider:        %s\n", cfg.Proxy.Provider)
			fmt.Printf("  caddyfile_path:  %s\n", cfg.Proxy.CaddyfilePath)
			fmt.Printf("  caddy_binary:    %s\n", cfg.Proxy.CaddyBinary)
			fmt.Printf("  reload_command:  %s\n", cfg.Proxy.ReloadCommand)
			fmt.Printf("  validate_command:%s\n", cfg.Proxy.ValidateCommand)
			fmt.Printf("  backup_dir:      %s\n", cfg.Proxy.BackupDir)
			fmt.Printf("  email:           %s\n", cfg.Proxy.Email)
			fmt.Println()
			fmt.Println("[store]")
			fmt.Printf("  sqlite_path:     %s\n", cfg.Store.SQLitePath)
			fmt.Println()
			fmt.Println("[runtime]")
			fmt.Printf("  config_dir:      %s\n", cfg.Runtime.ConfigDir)
			fmt.Printf("  data_dir:        %s\n", cfg.Runtime.DataDir)
			return nil
		},
	}
}
