package cli

import (
	"fmt"

	"aegis/internal/config"
	"aegis/internal/listener"
	"aegis/internal/store"

	"github.com/spf13/cobra"
)

func newBootstrapCommand(cfg *config.Config, listenerSvc *listener.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "bootstrap",
		Short: "Initialize Aegis state on a fresh VPS",
		Long: `Bootstraps Aegis control plane state without installing software.

Creates:
  - Config directory and default config (if missing)
  - SQLite database and migrations
  - Default EdgeMux listeners (HAProxy 443, Caddy 80, Caddy 8443)
  - Provider registry

Does NOT:
  - Install haproxy, caddy, or any system packages
  - Modify existing config files
  - Start or stop any services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("=== Aegis Bootstrap ===")
			fmt.Println()

			// 1. Ensure config
			configPath := cfg.Runtime.ConfigDir + "/config.yaml"
			if err := config.EnsureDirs(cfg); err != nil {
				return fmt.Errorf("create dirs: %w", err)
			}
			fmt.Printf("[config] %s\n", configPath)

			// 2. Init database
			db, err := store.OpenSQLite(cfg.Store.SQLitePath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer db.Close()

			if err := store.Initialize(db); err != nil {
				return fmt.Errorf("run migrations: %w", err)
			}
			fmt.Printf("[database] %s (migrations applied)\n", cfg.Store.SQLitePath)

			// 3. Register default listeners (EdgeMux mode)
			if err := listenerSvc.RegisterDefaults(); err != nil {
				return fmt.Errorf("register listeners: %w", err)
			}
			listeners, _ := listenerSvc.ListAll()
			fmt.Printf("[listeners] %d registered\n", len(listeners))
			for _, l := range listeners {
				fmt.Printf("  %s %s:%d (%s) → %s\n", l.Provider, l.BindIP, l.Port, l.Protocol, l.Status)
			}

			// 4. Save config template
			if err := cfg.Save(configPath); err != nil {
				fmt.Printf("  warning: could not save config: %v\n", err)
			}

			fmt.Println()
			fmt.Println("Bootstrap complete. Next steps:")
			fmt.Println("  1. Install haproxy and caddy")
			fmt.Println("  2. Edit config: " + configPath)
			fmt.Println("  3. Run: aegis apply --all")
			return nil
		},
	}
}
