//go:build !linux

package transparent

import "fmt"

// On non-Linux platforms, iptables is not available. The Manager will return
// informative errors when StartRedirect is called.

type iptablesManager struct{}

func newIPTablesManager() *iptablesManager {
	return &iptablesManager{}
}

func (m *iptablesManager) addRule(rule RedirectRule) error {
	return fmt.Errorf("transparent proxy requires Linux iptables (current OS does not support it)")
}

func (m *iptablesManager) removeRule(ruleID string) error {
	return nil
}

func (m *iptablesManager) listRules() ([]string, error) {
	return nil, nil
}

func (m *iptablesManager) cleanupAll() {}
