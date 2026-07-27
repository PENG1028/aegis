package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"aegis/internal/config"
	"aegis/internal/store"

	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Aegis for development (local .aegis directory)",
		Long: `Creates a local .aegis directory with config, database, and migrations.

This is for DEVELOPMENT use. For production (system paths, systemd, Caddy
auto-config), use 'aegis bootstrap --production' instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine config path
			cfgFile := configPath
			if cfgFile == "" {
				cwd, _ := os.Getwd()
				cfgFile = filepath.Join(cwd, ".aegis", "config.yaml")
			}

			// Always use development defaults for 'init'.
			// For production (system paths, systemd, Caddy auto-config),
			// use 'aegis bootstrap --production' instead.
			cfg := config.DefaultConfig()

			// Ensure directories
			if err := config.EnsureDirs(cfg); err != nil {
				return fmt.Errorf("create directories: %w", err)
			}

			// Create database
			db, err := store.OpenSQLite(cfg.Store.SQLitePath)
			if err != nil {
				return fmt.Errorf("create database: %w", err)
			}

			// Run migrations
			if err := store.Initialize(db); err != nil {
				db.Close()
				return fmt.Errorf("run migrations: %w", err)
			}
			db.Close()

			// Save config
			if err := cfg.Save(cfgFile); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			fmt.Println("Aegis initialized successfully.")
			fmt.Println()
			fmt.Printf("  Config:      %s\n", cfgFile)
			fmt.Printf("  Database:    %s\n", cfg.Store.SQLitePath)
			fmt.Printf("  Config dir:  %s\n", cfg.Runtime.ConfigDir)
			fmt.Printf("  Data dir:    %s\n", cfg.Runtime.DataDir)
			fmt.Printf("  Backup dir:  %s\n", cfg.Proxy.BackupDir)
			fmt.Println()

			// Check for caddy
			if path, found := config.FindCaddyBinary(); found {
				fmt.Printf("Caddy binary found: %s\n", path)
			} else {
				fmt.Println("warning: caddy binary not found in PATH")
				fmt.Println("  Install caddy or adjust proxy.caddy_binary in config")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Path to config file (default: ./.aegis/config.yaml)")
	return cmd
}
