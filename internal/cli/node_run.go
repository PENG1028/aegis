package cli

import (
	"fmt"
	"os"

	"aegis/internal/config"
	"aegis/internal/nodeagent"
	"aegis/internal/noderuntime"

	"github.com/spf13/cobra"
)

func newNodeRunCommand() *cobra.Command {
	var (
		configPath string
		proxyCfgPath string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the Aegis node agent daemon",
		Long: `Run the Aegis node agent on a managed node.

The agent periodically syncs with the control plane:
  1. Sends heartbeats with node status and gateway inventory
  2. Pulls desired state (routing table) from the control plane
  3. Applies the Caddy configuration locally
  4. Reports actual state back to the control plane

On first run, the agent must join the cluster. Provide a join token
via AEGIS_JOIN_TOKEN env var or place it in a join.token file
alongside the node token file.

Examples:
  aegis node run --config /etc/aegis/node.yaml
  AEGIS_JOIN_TOKEN=xxx aegis node run --config /etc/aegis/node.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load node config
			nodeCfg, err := noderuntime.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load node config: %w", err)
			}

			// Validate required fields
			if nodeCfg.ControlPlaneURL == "" {
				return fmt.Errorf("control_plane_url is required in node config")
			}

			// Load proxy config (for Caddyfile apply)
			var proxyCfg *config.Config
			if proxyCfgPath != "" {
				proxyCfg, err = config.Load(proxyCfgPath)
				if err != nil {
					return fmt.Errorf("load proxy config: %w", err)
				}
			} else {
				// Use defaults from node config locations, or fall back to production config
				proxyCfg = config.ProductionConfig()
				// Override with paths from node config if available
				if nodeCfg.CacheDir != "" {
					proxyCfg.Store.SQLitePath = "" // node doesn't need SQLite
				}
			}

			agent, err := nodeagent.New(nodeCfg, proxyCfg)
			if err != nil {
				return fmt.Errorf("create agent: %w", err)
			}

			return agent.Run()
		},
	}

	cmd.Flags().StringVar(&configPath, "config", noderuntime.DefaultConfigPath, "path to node config YAML")
	cmd.Flags().StringVar(&proxyCfgPath, "proxy-config", "", "path to proxy config YAML (defaults to production config)")

	return cmd
}


// NewNodeCommand creates the 'aegis node' parent command.
func NewNodeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Node agent management commands",
		Long:  `Manage the Aegis node agent daemon on this machine.`,
	}

	cmd.AddCommand(newNodeRunCommand())

	return cmd
}


var _ = os.Stderr
