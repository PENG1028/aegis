package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"aegis/internal/config"
	"aegis/internal/listener"
	"aegis/internal/provider"

	"github.com/spf13/cobra"
)

func newDoctorCommand(cfg *config.Config, listenerSvc *listener.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check deployment readiness",
		Long:  "Checks OS, binaries, permissions, ports, and provider status for EdgeMux deployment.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("=== Aegis Doctor ===")
			fmt.Println()

			// OS
			fmt.Println("[os]")
			fmt.Printf("  OS:       %s\n", runtime.GOOS)
			fmt.Printf("  Arch:     %s\n", runtime.GOARCH)
			hostname, _ := os.Hostname()
			fmt.Printf("  Hostname: %s\n", hostname)
			fmt.Println()

			// User
			fmt.Println("[user]")
			uid := os.Geteuid()
			fmt.Printf("  UID:      %d\n", uid)
			if uid == 0 {
				fmt.Println("  root:     yes")
				fmt.Println("  sudo:     not needed (running as root)")
			} else {
				fmt.Println("  root:     no")
				_, sudoErr := exec.LookPath("sudo")
				if sudoErr == nil {
					fmt.Println("  sudo:     available")
				} else {
					fmt.Println("  sudo:     not found — some operations may need root")
				}
			}
			fmt.Println()

			// Binaries
			fmt.Println("[binaries]")
			checkBinary("haproxy")
			checkBinary("caddy")
			checkBinary("openssl")
			checkBinary("ss")
			checkBinary("systemctl")
			fmt.Println()

			// Provider status
			fmt.Println("[providers]")
			hpStatus := provider.CheckHAProxyStatus(cfg.Proxy.CaddyfilePath)
			fmt.Printf("  haproxy_edge_mux: %s\n", hpStatus.Status)
			if hpStatus.Version != "" && hpStatus.Version != "unknown" {
				fmt.Printf("    version:        %s\n", hpStatus.Version)
			}
			if hpStatus.Message != "" {
				fmt.Printf("    message:        %s\n", hpStatus.Message)
			}

			caddyStatus := provider.CheckCaddyStatus(cfg.Proxy.CaddyfilePath)
			fmt.Printf("  caddy_http:       %s\n", caddyStatus.Status)
			if caddyStatus.Version != "" && caddyStatus.Version != "unknown" {
				fmt.Printf("    version:        %s\n", caddyStatus.Version)
			}
			if caddyStatus.Message != "" {
				fmt.Printf("    message:        %s\n", caddyStatus.Message)
			}
			fmt.Println()

			// Config paths
			fmt.Println("[config paths]")
			checkWritable(cfg.Proxy.CaddyfilePath, "Caddyfile")
			checkWritable("/etc/haproxy/haproxy.cfg", "haproxy.cfg")
			checkWritable(cfg.Proxy.BackupDir, "backup_dir")
			checkWritable(cfg.Store.SQLitePath, "sqlite_path")
			fmt.Println()

			// Ports
			fmt.Println("[ports]")
			checkPort("0.0.0.0", 443, "HAProxy EdgeMux TLS")
			checkPort("0.0.0.0", 80, "Caddy HTTP")
			checkPort("127.0.0.1", 8443, "Caddy internal HTTPS")
			fmt.Println()

			// Listeners
			fmt.Println("[listeners]")
			listeners, _ := listenerSvc.ListAll()
			for _, l := range listeners {
				fmt.Printf("  %s: %s:%d (%s) → %s\n", l.Provider, l.BindIP, l.Port, l.Protocol, l.Status)
			}
			if len(listeners) == 0 {
				fmt.Println("  (none registered)")
			}
			fmt.Println()

			// Firewall hints
			fmt.Println("[firewall]")
			fmt.Println("  hint: ensure ports 80 and 443 are open in firewall")
			fmt.Println("  check: sudo ufw status  OR  sudo firewall-cmd --list-all")
			fmt.Println("  Aegis does NOT modify firewall rules automatically.")
			fmt.Println()

			// ACME hints
			fmt.Println("[acme]")
			httpPortInUse := isPortListening("0.0.0.0", 80)
			tlsPortInUse := isPortListening("0.0.0.0", 443)
			internalInUse := isPortListening("127.0.0.1", 8443)

			if httpPortInUse {
				fmt.Println("  acme_http_01_possible:  true (port 80 in use)")
			} else {
				fmt.Println("  acme_http_01_possible:  WARNING — port 80 not listening, HTTP-01 will fail")
			}
			if tlsPortInUse {
				fmt.Println("  edgemux_443_active:      true")
			} else {
				fmt.Println("  edgemux_443_active:      WARNING — port 443 not listening, EdgeMux not active")
			}
			if internalInUse {
				fmt.Println("  caddy_8443_active:       true")
			} else {
				fmt.Println("  caddy_8443_active:       WARNING — 127.0.0.1:8443 not listening, Caddy internal HTTPS not active")
			}

			fmt.Println()
			fmt.Println("=== Done ===")
			return nil
		},
	}
}

func checkBinary(name string) {
	path, err := exec.LookPath(name)
	if err != nil {
		fmt.Printf("  %-12s MISSING\n", name+":")
		return
	}
	// Try to get version
	var version string
	switch name {
	case "haproxy":
		out, _ := exec.Command(path, "-v").CombinedOutput()
		version = firstLine(string(out))
	case "caddy":
		out, _ := exec.Command(path, "version").CombinedOutput()
		version = strings.TrimSpace(string(out))
	case "openssl":
		out, _ := exec.Command(path, "version").CombinedOutput()
		version = strings.TrimSpace(string(out))
	case "systemctl":
		out, _ := exec.Command(path, "--version").CombinedOutput()
		version = firstLine(string(out))
	}
	if version != "" {
		fmt.Printf("  %-12s %s (%s)\n", name+":", path, version)
	} else {
		fmt.Printf("  %-12s %s\n", name+":", path)
	}
}

func checkWritable(path, label string) {
	if path == "" {
		fmt.Printf("  %-20s (not configured)\n", label+":")
		return
	}
	dir := path
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		// it's a directory
	} else {
		dir = path[:strings.LastIndex(path, "/")+1]
		if dir == "" {
			dir = "."
		}
	}
	testFile := path + ".aegis-doctor-test"
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		fmt.Printf("  %-20s NOT WRITABLE (%v)\n", label+":", err)
	} else {
		os.Remove(testFile)
		fmt.Printf("  %-20s writable\n", label+":")
	}
}

func checkPort(bindIP string, port int, label string) {
	labelText := fmt.Sprintf("%s:%d", bindIP, port)
	if isPortListening(bindIP, port) {
		fmt.Printf("  %-20s LISTENING ✓ (%s)\n", labelText+":", label)
	} else {
		fmt.Printf("  %-20s NOT LISTENING — %s\n", labelText+":", label)
	}
}

func isPortListening(bindIP string, port int) bool {
	// Try ss first
	out, err := exec.Command("ss", "-ltnp").CombinedOutput()
	if err == nil {
		if strings.Contains(string(out), fmt.Sprintf(":%d", port)) {
			return true
		}
	}
	// Fallback: try connecting
	cmd := exec.Command("nc", "-z", "-w", "1", bindIP, fmt.Sprintf("%d", port))
	err = cmd.Run()
	return err == nil
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}
