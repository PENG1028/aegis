package dns

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderCreatesParentDir is a regression test: Render used to fail when
// the config's parent directory did not exist, spamming
// "open <path>: no such file or directory" on every refresh loop.
func TestRenderCreatesParentDir(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "aegis", "dnsmasq") // does not exist yet
	cfg := &DnsmasqConfig{
		ConfigPath: filepath.Join(nested, "aegis.conf"),
		Upstream:   "1.1.1.1",
	}

	if err := cfg.Render(map[string]ResolvedEntry{
		"svc.example.com": {Domain: "svc.example.com", TargetIP: "10.0.0.5"},
	}); err != nil {
		t.Fatalf("Render with missing parent dir: %v", err)
	}

	data, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		t.Fatalf("read rendered config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "server=1.1.1.1\n") || !strings.Contains(content, "address=/svc.example.com/10.0.0.5\n") {
		t.Errorf("unexpected config content:\n%s", content)
	}
}
