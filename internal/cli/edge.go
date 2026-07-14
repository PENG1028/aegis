package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"aegis/internal/edgemux"
	"aegis/internal/hostdep/provider"

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
	var runtime bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Runtime smoke checks for EdgeMux",
		Long:  "Checks provider status, listener ownership, and optionally runtime port/SNI tests (--runtime).",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			rules, _ := svc.ListRules(ctx)

			fmt.Println("=== EdgeMux Diagnostics ===")
			fmt.Println()

			// 1. Provider status
			fmt.Println("[providers]")
			hpStatus := provider.CheckHAProxyStatus("/etc/haproxy/haproxy.cfg")
			printProviderStatus(hpStatus)
			fmt.Println()
			caddyStatus := provider.CheckCaddyStatus("/etc/caddy/Caddyfile")
			printProviderStatus(caddyStatus)
			fmt.Println()

			// 2. Listeners
			fmt.Println("[listeners]")
			fmt.Println("  expected: 0.0.0.0:443 → haproxy_edge_mux (tls_mux)")
			fmt.Println("  expected: 0.0.0.0:80  → caddy_http (http)")
			fmt.Println("  expected: 127.0.0.1:8443 → caddy_http (https)")
			fmt.Println()

			// 3. Edge rules with backend health
			fmt.Println("[edge rules]")
			if len(rules) == 0 {
				fmt.Println("  (none)")
			}
			for _, r := range rules {
				healthy, healthMsg := checkTCPConnect(r.TargetHost, r.TargetPort)
				healthStr := "healthy"
				if !healthy {
					healthStr = "unhealthy"
				}
				fmt.Printf("  %s → %s:%d [%s] kind=%s managed_by=%s %s\n",
					r.SNIHost, r.TargetHost, r.TargetPort, healthStr, r.DeclaredKind, r.ManagedBy, healthMsg)
			}
			fmt.Println()

			// 4. Certificate diagnostics
			fmt.Println("[certificate]")
			http80 := isPortListening("0.0.0.0", 80)
			tls443 := isPortListening("0.0.0.0", 443)
			internal8443 := isPortListening("127.0.0.1", 8443)

			if http80 {
				fmt.Println("  acme_http_01_possible:          true (port 80 in use)")
			} else {
				fmt.Println("  acme_http_01_possible:          WARNING — port 80 not listening, HTTP-01 challenge will FAIL")
			}
			if tls443 && http80 {
				fmt.Println("  acme_tls_alpn_passthrough:       possible (HAProxy 443→Caddy 8443 passthrough)")
			} else if tls443 {
				fmt.Println("  acme_tls_alpn_passthrough:       WARNING — 443 active but 80 not, check HTTP-01")
			} else {
				fmt.Println("  acme_tls_alpn_passthrough:       WARNING — 443 not active, EdgeMux not running")
			}
			if internal8443 {
				fmt.Println("  caddy_internal_https:            active (127.0.0.1:8443)")
			} else {
				fmt.Println("  caddy_internal_https:            WARNING — 127.0.0.1:8443 NOT LISTENING")
			}
			fmt.Println()
			fmt.Println("  cert troubleshooting:")
			fmt.Println("    caddy logs:  sudo journalctl -u caddy --no-pager -n 50")
			fmt.Println("    haproxy logs: sudo journalctl -u haproxy --no-pager -n 50")
			fmt.Println("    HTTP-01:      ensure port 80 is publicly reachable")
			fmt.Println("    DNS-01:       configure DNS challenge if HTTP-01 fails")
			fmt.Println("    TLS-ALPN-01:  requires HAProxy passthrough to Caddy 8443")
			fmt.Println()

			// --runtime: actual port + SNI checks
			if runtime {
				fmt.Println("[runtime]")
				runRuntimeChecks(rules)
			} else {
				fmt.Println("(use --runtime for real port/SNI smoke tests)")
			}

			fmt.Println()
			fmt.Println("=== Done ===")
			return nil
		},
	}
	cmd.Flags().BoolVar(&runtime, "runtime", false, "Run real port (ss) and SNI (openssl) smoke tests")
	return cmd
}

func printProviderStatus(s provider.ProviderStatus) {
	fmt.Printf("  %s:\n", s.Provider)
	fmt.Printf("    status:          %s\n", s.Status)
	fmt.Printf("    running:         %v\n", s.Running)
	if s.Version != "" && s.Version != "unknown" {
		fmt.Printf("    version:         %s\n", s.Version)
	}
	fmt.Printf("    config_ok:       %v\n", s.ConfigOK)
	if s.Message != "" {
		fmt.Printf("    message:         %s\n", s.Message)
	}
}

func checkTCPConnect(host string, port int) (bool, string) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return false, fmt.Sprintf("(%v)", err)
	}
	conn.Close()
	return true, ""
}

func runRuntimeChecks(rules []edgemux.Rule) {
	// ss check
	fmt.Println("  [ss -ltnp]")
	ssOut := runCmd("ss", "-ltnp")
	if ssOut != "" {
		checkPortInOutput(ssOut, ":443", "0.0.0.0:443")
		checkPortInOutput(ssOut, ":80", "0.0.0.0:80")
		checkPortInOutput(ssOut, ":8443", "127.0.0.1:8443")
	} else {
		fmt.Println("    ss not available (missing_binary)")
	}

	// openssl checks
	fmt.Println("  [openssl s_client]")
	if _, err := exec.LookPath("openssl"); err != nil {
		fmt.Println("    openssl: missing_binary — skipping SNI tests")
		return
	}

	for _, r := range rules {
		if r.Status != "active" {
			continue
		}
		result := runCmdTimeout("openssl", "s_client", "-connect", "127.0.0.1:443",
			"-servername", r.SNIHost)
		if result != "" {
			fmt.Printf("    SNI %s: connected ✓\n", r.SNIHost)
		} else {
			fmt.Printf("    SNI %s: FAILED\n", r.SNIHost)
		}
	}
	unk := runCmdTimeout("openssl", "s_client", "-connect", "127.0.0.1:443",
		"-servername", "unknown.aegis-test.invalid")
	if unk == "" {
		fmt.Println("    unknown SNI: rejected ✓")
	} else {
		fmt.Println("    unknown SNI: WARNING — should be rejected")
	}
	noSNI := runCmdTimeout("openssl", "s_client", "-connect", "127.0.0.1:443")
	if noSNI == "" {
		fmt.Println("    no SNI: rejected ✓")
	} else {
		fmt.Println("    no SNI: WARNING — should be rejected")
	}
}

func checkPortInOutput(output, port, label string) {
	if strings.Contains(output, port) {
		fmt.Printf("    %s: in use ✓\n", label)
	} else {
		fmt.Printf("    %s: NOT LISTENING\n", label)
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
