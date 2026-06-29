package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"aegis/internal/config"
	"aegis/internal/listener"
	"aegis/internal/store"

	"github.com/spf13/cobra"
)

func newBootstrapCommand(cfg *config.Config, listenerSvc *listener.Service) *cobra.Command {
	var production bool

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Initialize Aegis state on a fresh VPS",
		Long: `Bootstraps Aegis control plane state without installing software.

Creates:
  - Config directory and default config (if missing)
  - SQLite database and migrations
  - Default EdgeMux listeners (HAProxy 443, Caddy 80, Caddy 8443)
  - Panel Caddyfile for public access (--production)

Does NOT:
  - Install haproxy, caddy, or any system packages
  - Modify existing config files
  - Start or stop any services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// When --production is set, use production paths regardless of
			// whether a config file exists. This allows bootstrapping on a
			// clean VPS without a pre-existing config.
			if production {
				prod := config.ProductionConfig()
				cfg.Proxy = prod.Proxy
				cfg.Store = prod.Store
				cfg.Server = prod.Server
				cfg.DNS = prod.DNS
				cfg.ManagedDomain = prod.ManagedDomain
				cfg.Runtime = prod.Runtime
			}

			fmt.Println("=== Aegis Bootstrap ===")
			if production {
				fmt.Println("Mode: production")
			}
			fmt.Println()

			// 1. Ensure config directories
			if err := config.EnsureDirs(cfg); err != nil {
				return fmt.Errorf("create dirs: %w", err)
			}

			configPath := filepath.Join(cfg.Runtime.ConfigDir, "config.yaml")
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

			// 4. Save config
			if err := cfg.Save(configPath); err != nil {
				fmt.Printf("  warning: could not save config: %v\n", err)
			}

			// 5. Generate panel Caddyfile for public access
			panelCaddyfile := filepath.Join(cfg.Runtime.ConfigDir, "Caddyfile.panel")
			caddyContent := cfg.PanelCaddyfile()
			if err := os.WriteFile(panelCaddyfile, []byte(caddyContent), 0644); err != nil {
				fmt.Printf("  warning: could not write panel Caddyfile: %v\n", err)
			} else {
				fmt.Printf("[caddy] panel Caddyfile → %s\n", panelCaddyfile)
			}

			// 6. Production mode: auto-integrate with Caddy
			if production {
				fmt.Println()
				fmt.Println("--- Production Setup ---")

				caddyPath, caddyFound := config.FindCaddyBinary()
				if caddyFound {
					fmt.Printf("  Caddy binary: %s\n", caddyPath)

					// Install Caddy systemd service if needed
					caddyConfigDir := "/etc/caddy"
					if _, err := os.Stat(caddyConfigDir); err == nil {
						symlink := filepath.Join(caddyConfigDir, "aegis-panel.conf")
						os.Remove(symlink)
						if err := os.Symlink(panelCaddyfile, symlink); err != nil {
							// Symlink failed — copy instead
							data, _ := os.ReadFile(panelCaddyfile)
							if err := os.WriteFile(symlink, data, 0644); err != nil {
								fmt.Printf("  warning: could not install panel Caddyfile: %v\n", err)
							} else {
								fmt.Printf("  Installed: %s\n", symlink)
							}
						} else {
							fmt.Printf("  Symlinked: %s → %s\n", symlink, panelCaddyfile)
						}

						// Reload Caddy
						if out, err := exec.Command("systemctl", "reload", "caddy").CombinedOutput(); err != nil {
							fmt.Printf("  warning: caddy reload: %v\n  %s\n", err, string(out))
						} else {
							fmt.Println("  Caddy reloaded ✓")
						}
					} else {
						fmt.Printf("  Note: Caddy config dir not found (%s)\n", caddyConfigDir)
						fmt.Printf("  To serve the panel, run:\n")
						fmt.Printf("    caddy run --config %s\n", panelCaddyfile)
					}
				} else {
					fmt.Println("  Caddy not found. Install it to serve the panel on port 80:")
					fmt.Println("    sudo apt-get install -y caddy")
					fmt.Printf("    caddy run --config %s\n", panelCaddyfile)
				}
			}

			fmt.Println()
			fmt.Println("Bootstrap complete. Next steps:")
			if !production {
				fmt.Println("  1. Install caddy: sudo apt-get install -y caddy")
				fmt.Println("  2. Start Aegis: aegis serve")
				fmt.Println("  3. Access panel: http://127.0.0.1:7380")
			} else {
				fmt.Println("  1. Install caddy: sudo apt-get install -y caddy")
				fmt.Println("  2. Start Aegis: systemctl start aegis")
				fmt.Println("  3. Access panel: http://<server-public-ip>")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&production, "production", false, "Production mode: use system paths and auto-configure Caddy")
	return cmd
}
