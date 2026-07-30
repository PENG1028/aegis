package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The default search path checks ./.aegis/ and ~/.aegis/ before /etc/aegis/config.yaml,
// so a command run from the wrong directory silently reads a different file than the
// service. That surfaced as `aegis settings` printing acme_server as blank on a host
// where ACME was demonstrably using the staging directory. Reporting the source path
// is what makes the discrepancy visible, so it needs to stay wired up.
func TestLoadRecordsSourcePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "proxy:\n" +
		"    provider: caddy\n" +
		"    acme_server: \"https://acme-staging-v02.api.letsencrypt.org/directory\"\n" +
		"store:\n" +
		"    sqlite_path: " + filepath.Join(dir, "aegis.db") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.SourcePath(); got != path {
		t.Errorf("SourcePath() = %q, want %q", got, path)
	}
	if cfg.Proxy.ACMEServer == "" {
		t.Error("acme_server did not survive the load, so a set value would read as unset")
	}
}

// Defaults come from no file at all. An empty source path is how a caller tells the
// two apart, so it must not report a path it never read.
func TestDefaultConfigHasNoSourcePath(t *testing.T) {
	if got := DefaultConfig().SourcePath(); got != "" {
		t.Errorf("SourcePath() = %q, want empty for defaults", got)
	}
}

// A nil receiver shows up when config loading failed upstream; SourcePath is called
// while reporting that failure, so it must not panic.
func TestSourcePathNilSafe(t *testing.T) {
	var cfg *Config
	if got := cfg.SourcePath(); got != "" {
		t.Errorf("SourcePath() on nil = %q, want empty", got)
	}
}
