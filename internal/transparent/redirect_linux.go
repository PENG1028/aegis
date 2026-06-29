//go:build linux

package transparent

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
)

// iptablesManager manages iptables OUTPUT chain DNAT rules.
//
// Stability note: iptables rules persist across process restarts. If Aegis
// crashes, rules remain in the kernel. Call CleanupStaleRules() at startup
// to remove rules from previous runs before adding new ones.
type iptablesManager struct {
	mu    sync.Mutex
	rules map[string]RedirectRule
}

func newIPTablesManager() *iptablesManager {
	return &iptablesManager{
		rules: make(map[string]RedirectRule),
	}
}

// addRule installs an iptables DNAT rule. Returns an error if a rule with
// the same ID already exists in our tracking map, OR if the exact same
// iptables rule already exists in the kernel (idempotent check).
func (m *iptablesManager) addRule(rule RedirectRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[rule.ID]; ok {
		return fmt.Errorf("rule %s already tracked", rule.ID)
	}

	if _, err := exec.LookPath("iptables"); err != nil {
		return fmt.Errorf("iptables not available: %w", err)
	}

	// Idempotent check: see if this exact rule already exists in the kernel
	// (leftover from a previous crash). If so, remove it first, then add fresh.
	if m.ruleExists(rule) {
		log.Printf("[transparent] rule %s already in kernel (stale?), replacing", rule.ID)
		m.deleteRuleUnsafe(rule)
	}

	// Use -A (append) not -I (insert). We want consistent ordering and
	// no interference with other iptables rules that may be at position 1.
	args := []string{
		"-t", "nat",
		"-A", "OUTPUT",
		"-d", rule.OriginalIP,
		"-p", "tcp",
		"--dport", fmt.Sprintf("%d", rule.OriginalPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("127.0.0.1:%d", rule.LocalProxyPort),
		"-m", "comment", "--comment", fmt.Sprintf("aegis-transparent-%s", rule.ID),
	}

	out, err := exec.Command("iptables", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("iptables add: %w\n%s", err, string(out))
	}

	m.rules[rule.ID] = rule
	log.Printf("[transparent] iptables +A: %s:%d → 127.0.0.1:%d",
		rule.OriginalIP, rule.OriginalPort, rule.LocalProxyPort)
	return nil
}

// removeRule deletes the iptables rule by exact match.
func (m *iptablesManager) removeRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.rules[ruleID]
	if !ok {
		return fmt.Errorf("rule %s not tracked", ruleID)
	}

	if _, err := exec.LookPath("iptables"); err != nil {
		delete(m.rules, ruleID)
		return nil
	}

	m.deleteRuleUnsafe(rule)
	delete(m.rules, ruleID)
	log.Printf("[transparent] iptables -D: %s", ruleID)
	return nil
}

// ruleExists checks the kernel for an exact matching rule.
func (m *iptablesManager) ruleExists(rule RedirectRule) bool {
	out, err := exec.Command("iptables-save", "-t", "nat").CombinedOutput()
	if err != nil {
		return false
	}
	marker := fmt.Sprintf("aegis-transparent-%s", rule.ID)
	return strings.Contains(string(out), marker)
}

// deleteRuleUnsafe deletes a rule from the kernel without locking.
// Caller must hold m.mu.
func (m *iptablesManager) deleteRuleUnsafe(rule RedirectRule) {
	args := []string{
		"-t", "nat",
		"-D", "OUTPUT",
		"-d", rule.OriginalIP,
		"-p", "tcp",
		"--dport", fmt.Sprintf("%d", rule.OriginalPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("127.0.0.1:%d", rule.LocalProxyPort),
		"-m", "comment", "--comment", fmt.Sprintf("aegis-transparent-%s", rule.ID),
	}
	out, err := exec.Command("iptables", args...).CombinedOutput()
	if err != nil {
		log.Printf("[transparent] iptables delete %s: %v (ignoring)\n%s", rule.ID, err, string(out))
	}
}

// listRules returns all aegis-managed iptables rules currently in the kernel.
func (m *iptablesManager) listRules() ([]string, error) {
	if _, err := exec.LookPath("iptables"); err != nil {
		return nil, nil
	}

	out, err := exec.Command("iptables-save", "-t", "nat").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("iptables-save: %w", err)
	}

	var rules []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "aegis-transparent-") {
			rules = append(rules, strings.TrimSpace(line))
		}
	}
	return rules, nil
}

// cleanupAll removes ALL aegis-managed rules from the kernel.
func (m *iptablesManager) cleanupAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := exec.LookPath("iptables"); err != nil {
		m.rules = make(map[string]RedirectRule)
		return
	}

	// Scan kernel for any aegis rules (including from crashed previous runs)
	lines, _ := m.listRules()
	for _, line := range lines {
		// Extract -d IP and --dport from the saved rule
		// Format: -A OUTPUT -d 10.0.0.1/32 -p tcp -m tcp --dport 9100 -j DNAT --to-destination 127.0.0.1:18100 -m comment --comment aegis-transparent-xxx
		fields := strings.Fields(line)
		var dstIP, dport, toDst string
		for i, f := range fields {
			if f == "-d" && i+1 < len(fields) {
				dstIP = strings.TrimSuffix(fields[i+1], "/32")
			}
			if f == "--dport" && i+1 < len(fields) {
				dport = fields[i+1]
			}
			if f == "--to-destination" && i+1 < len(fields) {
				toDst = fields[i+1]
			}
		}
		if dstIP != "" && dport != "" {
			args := []string{
				"-t", "nat", "-D", "OUTPUT",
				"-d", dstIP, "-p", "tcp", "--dport", dport,
				"-j", "DNAT", "--to-destination", toDst,
			}
			exec.Command("iptables", args...).Run()
			log.Printf("[transparent] cleaned stale rule: %s:%s → %s", dstIP, dport, toDst)
		}
	}

	m.rules = make(map[string]RedirectRule)
	log.Printf("[transparent] all iptables rules removed")
}
