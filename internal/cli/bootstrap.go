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
  - Install system packages (providers installed separately via UI or API)
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

			// 5. Seed the main Caddyfile with the panel reverse proxy.
			// This file will later be managed by Aegis (user routes get appended).
			// MUST be the main caddyfile path — both panel and user routes
			// share the same :80 block inside Caddy.
			caddyfilePath := cfg.Proxy.CaddyfilePath
			if caddyfilePath == "" {
				caddyfilePath = filepath.Join(cfg.Runtime.ConfigDir, "Caddyfile")
			}
			caddyContent := cfg.PanelCaddyfile()
			if err := os.WriteFile(caddyfilePath, []byte(caddyContent), 0644); err != nil {
				fmt.Printf("  warning: could not write Caddyfile: %v\n", err)
			} else {
				fmt.Printf("[caddy] initial Caddyfile → %s\n", caddyfilePath)
			}

			// 6. Production mode: auto-integrate with system Caddy
			if production {
				fmt.Println()
				fmt.Println("--- Production Setup ---")

				caddyPath, caddyFound := config.FindCaddyBinary()
				if caddyFound {
					fmt.Printf("  Caddy binary: %s\n", caddyPath)

					// Symlink Aegis-managed Caddyfile into Caddy's config dir.
					// Caddy in production uses /etc/caddy/Caddyfile by default.
					// We symlink the Aegis-managed file so 'caddy reload' picks it up.
					caddySysPath := "/etc/caddy/Caddyfile"
					if _, err := os.Stat(filepath.Dir(caddySysPath)); err == nil {
						// Back up existing Caddyfile if present
						if _, err := os.Stat(caddySysPath); err == nil {
							backup := caddySysPath + ".bak"
							os.Rename(caddySysPath, backup)
							fmt.Printf("  Backed up existing: %s → %s\n", caddySysPath, backup)
						}
						// Symlink Aegis-managed file
						os.Remove(caddySysPath)
						if err := os.Symlink(caddyfilePath, caddySysPath); err != nil {
							// Symlink failed — copy instead
							data, readErr := os.ReadFile(caddyfilePath)
							if readErr != nil {
								fmt.Printf("  warning: cannot read Caddyfile for copy: %v\n", readErr)
							} else {
								if writeErr := os.WriteFile(caddySysPath, data, 0644); writeErr != nil {
									fmt.Printf("  warning: cannot write Caddyfile copy: %v\n", writeErr)
								}
							}
						}
						fmt.Printf("  Linked: %s → %s\n", caddySysPath, caddyfilePath)

						// Reload Caddy
						if out, err := exec.Command("systemctl", "reload", "caddy").CombinedOutput(); err != nil {
							fmt.Printf("  warning: caddy reload: %v\n  %s\n", err, string(out))
						} else {
							fmt.Println("  Caddy reloaded ✓")
						}
					} else {
						fmt.Printf("  Note: /etc/caddy not found — run: caddy run --config %s\n", caddyfilePath)
					}
				} else {
					fmt.Println("  Caddy not found. Install it to serve the panel on port 80:")
					fmt.Println("    sudo apt-get install -y <provider-package>")
				}
			}

			fmt.Println()
			fmt.Println("Bootstrap complete. Next steps:")
			if !production {
				fmt.Println("  1. Install the gateway provider: sudo apt-get install -y <provider-package>")
				fmt.Println("  2. Start Aegis: aegis serve")
				fmt.Println("  3. Access panel: http://127.0.0.1:7380")
			} else {
				fmt.Println("  1. Install the gateway provider: sudo apt-get install -y <provider-package>")
				fmt.Println("  2. Start Aegis: systemctl start aegis")
				fmt.Println("  3. Access panel: http://<server-public-ip>")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&production, "production", false, "Production mode: use system paths and auto-configure Caddy")
	return cmd
}
