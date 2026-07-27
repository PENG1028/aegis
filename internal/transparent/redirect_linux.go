//go:build linux

package transparent

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
)

// iptablesManager manages Aegis-owned OUTPUT-chain DNAT rules.
type iptablesManager struct {
	mu    sync.Mutex
	rules map[string]RedirectRule
}

func newIPTablesManager() *iptablesManager {
	return &iptablesManager{rules: make(map[string]RedirectRule)}
}

func (m *iptablesManager) addRule(rule RedirectRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[rule.ID]; ok {
		return fmt.Errorf("rule %s already tracked", rule.ID)
	}
	if _, err := exec.LookPath("iptables"); err != nil {
		return fmt.Errorf("iptables not available: %w", err)
	}

	if m.ruleExists(rule) {
		log.Printf("[transparent] rule %s already in kernel (stale?), replacing", rule.ID)
		m.deleteRuleUnsafe(rule)
	}

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
	log.Printf("[transparent] iptables +A: %s:%d -> 127.0.0.1:%d",
		rule.OriginalIP, rule.OriginalPort, rule.LocalProxyPort)
	return nil
}

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

func (m *iptablesManager) ruleExists(rule RedirectRule) bool {
	out, err := exec.Command("iptables-save", "-t", "nat").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf("aegis-transparent-%s", rule.ID))
}

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

func (m *iptablesManager) cleanupAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := exec.LookPath("iptables"); err != nil {
		m.rules = make(map[string]RedirectRule)
		return
	}

	lines, _ := m.listRules()
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "-A" {
			continue
		}
		args := append([]string{"-t", "nat", "-D"}, fields[1:]...)
		if out, err := exec.Command("iptables", args...).CombinedOutput(); err != nil {
			log.Printf("[transparent] cleanup delete failed: %v\n%s", err, string(out))
			continue
		}
		log.Printf("[transparent] cleaned stale rule: %s", line)
	}

	m.rules = make(map[string]RedirectRule)
	log.Printf("[transparent] all iptables rules removed")
}
