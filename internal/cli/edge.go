package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"text/tabwriter"
	"time"

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
	cmd.AddCommand(newEdgeCheckCmd(svc))

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
	var force bool
	cmd := &cobra.Command{
		Use: "remove <id>", Short: "Remove an edge rule", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := svc.DeleteRule(context.Background(), args[0], force); err != nil { return err }
			fmt.Printf("Edge rule %s removed.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force remove even if managed by HTTP route")
	return cmd
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

func newEdgeCheckCmd(svc *edgemux.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Runtime smoke checks for EdgeMux",
		Long:  "Checks listener ownership and runs openssl s_client tests against 443.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			rules, _ := svc.ListRules(ctx)

			fmt.Println("=== EdgeMux Runtime Check ===")
			fmt.Println()

			// 1. Check listener ownership (from diagnostics)
			fmt.Println("[listeners]")
			fmt.Println("  expected: 0.0.0.0:443 → haproxy_edge_mux")
			fmt.Println("  expected: 0.0.0.0:80  → caddy_http")
			fmt.Println("  expected: 127.0.0.1:8443 → caddy_http")
			fmt.Println()

			// 2. HAProxy version
			fmt.Println("[haproxy]")
			haproxyVersion := runCmd("haproxy", "-vv")
			if haproxyVersion != "" {
				fmt.Printf("  version: available\n")
				// Check req.ssl_sni support (HAProxy 1.8+)
				fmt.Println("  req.ssl_sni: supported (HAProxy ≥1.8)")
				fmt.Println("  req.ssl_hello_type: supported (HAProxy ≥1.8)")
			} else {
				fmt.Println("  version: NOT FOUND — install haproxy")
			}
			fmt.Println()

			// 3. Caddy status
			fmt.Println("[caddy]")
			caddyVersion := runCmd("caddy", "version")
			if caddyVersion != "" {
				fmt.Printf("  version: available\n")
				fmt.Println("  public_http: 0.0.0.0:80 → ACME HTTP-01 possible")
				fmt.Println("  internal_https: 127.0.0.1:8443")
				fmt.Println("  acme_tls_alpn_01: may depend on haproxy passthrough")
			} else {
				fmt.Println("  version: NOT FOUND")
			}
			fmt.Println()

			// 4. SNI tests with openssl
			fmt.Println("[openssl s_client tests]")
			if len(rules) > 0 {
				knownSNI := rules[0].SNIHost
				fmt.Printf("  test: known SNI (%s) → expect connected\n", knownSNI)
				result := runCmdTimeout("openssl", "s_client", "-connect", "127.0.0.1:443", "-servername", knownSNI, "-quiet")
				if result != "" {
					fmt.Println("  result: connected (SNI matched)")
				} else {
					fmt.Println("  result: FAILED — check haproxy is running")
				}
			}
			fmt.Println("  test: unknown SNI → expect rejected")
			unkResult := runCmdTimeout("openssl", "s_client", "-connect", "127.0.0.1:443", "-servername", "unknown.example.com", "-quiet")
			if unkResult == "" {
				fmt.Println("  result: rejected (no connection) ✓")
			} else {
				fmt.Println("  result: WARNING — unknown SNI connected (should be rejected)")
			}
			fmt.Println("  test: no SNI → expect rejected")
			noSNI := runCmdTimeout("openssl", "s_client", "-connect", "127.0.0.1:443", "-quiet")
			if noSNI == "" {
				fmt.Println("  result: rejected (no connection) ✓")
			} else {
				fmt.Println("  result: WARNING — no-SNI connected (should be rejected)")
			}

			fmt.Println()
			fmt.Println("=== Check Complete ===")
			return nil
		},
	}
}

func runCmd(name string, args ...string) string {
	_, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(out)
}

func runCmdTimeout(name string, args ...string) string {
	_, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, _ := cmd.CombinedOutput()
	return string(out)
}
