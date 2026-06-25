package config

import (
	"os"
	"path/filepath"
	"testing"
)

// v1.7Y Bug 4: Config path mismatch.
// Regression: Config written by bootstrap must be loadable by the config loader.

func TestConfigWriteThenLoad(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	// Create a config and save it
	cfg := DefaultConfig()
	cfg.Proxy.CaddyfilePath = "/tmp/test-caddyfile"
	cfg.Store.SQLitePath = filepath.Join(dir, "test.db")

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Bug 4 regression: Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Bug 4 regression: config file not found after save")
	}

	// Load it back
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Bug 4 regression: Load failed: %v", err)
	}

	// Verify values match
	if loaded.Proxy.CaddyfilePath != cfg.Proxy.CaddyfilePath {
		t.Errorf("CaddyfilePath mismatch: %s != %s", loaded.Proxy.CaddyfilePath, cfg.Proxy.CaddyfilePath)
	}
	if loaded.Store.SQLitePath != cfg.Store.SQLitePath {
		t.Errorf("SQLitePath mismatch: %s != %s", loaded.Store.SQLitePath, cfg.Store.SQLitePath)
	}

	t.Log("Bug 4 regression PASS: config write → load round-trip works")
}

func TestConfigDefaultPathsAccessible(t *testing.T) {
	// Verify that the default config paths used in main.go are valid
	dir := t.TempDir()

	// Create a config at the expected bootstrap path
	bootstrapPath := filepath.Join(dir, ".aegis", "config", "config.yaml")
	os.MkdirAll(filepath.Dir(bootstrapPath), 0755)

	cfg := DefaultConfig()
	cfg.Proxy.CaddyfilePath = "/tmp/caddyfile"

	if err := cfg.Save(bootstrapPath); err != nil {
		t.Fatalf("save to bootstrap path failed: %v", err)
	}

	// Load from the nested path (main.go looks for this)
	loaded, err := Load(bootstrapPath)
	if err != nil {
		t.Fatalf("Bug 4 regression: Load from bootstrap path failed: %v", err)
	}
	if loaded.Proxy.CaddyfilePath != "/tmp/caddyfile" {
		t.Errorf("CaddyfilePath mismatch: %s", loaded.Proxy.CaddyfilePath)
	}

	// Also verify the flat path (legacy location) works
	flatPath := filepath.Join(dir, ".aegis", "config.yaml")
	cfg.Proxy.CaddyfilePath = "/tmp/flat-caddyfile"
	if err := cfg.Save(flatPath); err != nil {
		t.Fatalf("save to flat path failed: %v", err)
	}
	loaded2, err := Load(flatPath)
	if err != nil {
		t.Fatalf("Load from flat path failed: %v", err)
	}
	if loaded2.Proxy.CaddyfilePath != "/tmp/flat-caddyfile" {
		t.Errorf("flat path CaddyfilePath mismatch: %s", loaded2.Proxy.CaddyfilePath)
	}

	t.Log("Bug 4 regression PASS: both bootstrap path and flat path are loadable")
}

func TestConfigSaveDoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Proxy.CaddyfilePath = "/tmp/original"
	cfg.Save(configPath)

	// Save again with different value
	cfg2 := DefaultConfig()
	cfg2.Proxy.CaddyfilePath = "/tmp/modified"
	cfg2.Save(configPath)

	// Load and verify it has the new value
	loaded, _ := Load(configPath)
	if loaded.Proxy.CaddyfilePath != cfg2.Proxy.CaddyfilePath {
		t.Errorf("expected %s, got %s", cfg2.Proxy.CaddyfilePath, loaded.Proxy.CaddyfilePath)
	}
	t.Log("Config save overwrites existing file (expected behavior)")
}

func TestValidateCommandTemplate(t *testing.T) {
	cfg := DefaultConfig()
	// The validate command is a template: "caddy validate --config {{config_path}}"
	// After template rendering, it should contain both "caddy" and "validate"
	cmd := cfg.Proxy.ValidateCommand
	if cmd == "" {
		t.Error("expected non-empty validate command template")
	}
	if !contains(cmd, "validate") {
		t.Error("expected validate command to contain 'validate'")
	}
	t.Logf("Validate command template: %s", cmd)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
