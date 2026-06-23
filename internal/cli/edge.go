package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"aegis/internal/edgemux"

	"github.com/spf13/cobra"
)

func newEdgeCommand(svc *edgemux.AppService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edge",
		Short: "Manage EdgeMux SNI rules",
		Long:  "Status, list, add, remove, enable, and disable EdgeMux TLS SNI passthrough rules.",
	}

	cmd.AddCommand(newEdgeStatusCmd(svc))

	ruleCmd := &cobra.Command{Use: "rule", Short: "Manage edge rules"}
	ruleCmd.AddCommand(newEdgeRuleListCmd(svc))
	ruleCmd.AddCommand(newEdgeRuleAddCmd(svc))
	ruleCmd.AddCommand(newEdgeRuleRemoveCmd(svc))
	ruleCmd.AddCommand(newEdgeRuleEnableCmd(svc))
	ruleCmd.AddCommand(newEdgeRuleDisableCmd(svc))
	cmd.AddCommand(ruleCmd)

	return cmd
}

func newEdgeStatusCmd(svc *edgemux.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show EdgeMux status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			rules, err := svc.ListRules(ctx)
			if err != nil {
				return err
			}
			active := 0
			for _, r := range rules {
				if r.Status == "active" { active++ }
			}
			fmt.Printf("EdgeMux: enabled\n")
			fmt.Printf("Public 443 owner: haproxy_edge_mux\n")
			fmt.Printf("Unknown SNI policy: reject\n")
			fmt.Printf("Active rules: %d / %d total\n", active, len(rules))
			return nil
		},
	}
}

func newEdgeRuleListCmd(svc *edgemux.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List edge rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			rules, err := svc.ListRules(ctx)
			if err != nil { return err }
			if len(rules) == 0 {
				fmt.Println("No edge rules.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "SNI_HOST\tKIND\tTARGET\tSTATUS")
			for _, r := range rules {
				fmt.Fprintf(w, "%s\t%s\t%s:%d\t%s\n", r.SNIHost, r.DeclaredKind, r.TargetHost, r.TargetPort, r.Status)
			}
			w.Flush()
			return nil
		},
	}
}

func newEdgeRuleAddCmd(svc *edgemux.AppService) *cobra.Command {
	var sni, kind, targetHost string
	var targetPort int

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an edge SNI rule",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sni == "" { return fmt.Errorf("--sni is required") }
			if targetHost == "" { return fmt.Errorf("--target is required") }
			ctx := context.Background()
			rule, err := svc.CreateRule(ctx, edgemux.CreateRuleInput{
				SNIHost: sni, DeclaredKind: kind, TargetHost: targetHost, TargetPort: targetPort,
			})
			if err != nil { return err }
			fmt.Printf("Edge rule: SNI %s -> %s:%d (%s)\n", rule.SNIHost, rule.TargetHost, rule.TargetPort, rule.DeclaredKind)
			return nil
		},
	}

	cmd.Flags().StringVar(&sni, "sni", "", "SNI hostname (required)")
	cmd.Flags().StringVar(&targetHost, "target", "", "Target host (required)")
	cmd.Flags().IntVar(&targetPort, "target-port", 8443, "Target port")
	cmd.Flags().StringVar(&kind, "kind", "https_app", "Kind: https_app, proxy_node, db_proxy, tunnel, unknown_tls_backend")
	return cmd
}

func newEdgeRuleRemoveCmd(svc *edgemux.AppService) *cobra.Command {
	return &cobra.Command{
		Use: "remove <id>", Short: "Remove an edge rule", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := svc.DeleteRule(context.Background(), args[0]); err != nil { return err }
			fmt.Printf("Edge rule %s removed.\n", args[0])
			return nil
		},
	}
}

func newEdgeRuleEnableCmd(svc *edgemux.AppService) *cobra.Command {
	return &cobra.Command{
		Use: "enable <id>", Short: "Enable an edge rule", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := svc.EnableRule(context.Background(), args[0]); err != nil { return err }
			fmt.Printf("Edge rule %s enabled.\n", args[0])
			return nil
		},
	}
}

func newEdgeRuleDisableCmd(svc *edgemux.AppService) *cobra.Command {
	return &cobra.Command{
		Use: "disable <id>", Short: "Disable an edge rule", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := svc.DisableRule(context.Background(), args[0]); err != nil { return err }
			fmt.Printf("Edge rule %s disabled.\n", args[0])
			return nil
		},
	}
}
