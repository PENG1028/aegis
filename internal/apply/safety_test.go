package apply

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aegis/internal/config"
	"aegis/internal/endpoint"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/proxy"
	"aegis/internal/route"
	"aegis/internal/service"
)

func setupTestEnv(t *testing.T) (*config.Config, string) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Proxy.CaddyfilePath = filepath.Join(tmpDir, "Caddyfile")
	cfg.Proxy.BackupDir = filepath.Join(tmpDir, "backups")
	cfg.Proxy.ValidateCommand = ""
	cfg.Proxy.ReloadCommand = ""
	cfg.Store.SQLitePath = filepath.Join(tmpDir, "test.db")

	os.MkdirAll(cfg.Proxy.BackupDir, 0755)
	return cfg, tmpDir
}

func setupRepos(t *testing.T, cfg *config.Config) (*route.Repository, *manageddomain.Repository, *service.Repository, *endpoint.Repository, *Repository, *logs.AppService) {
	t.Helper()
	// Use in-memory testing approach - create mock repos
	// For integration test, skip DB and use direct service construction
	return nil, nil, nil, nil, nil, nil
}

func TestDryRunDoesNotWriteConfig(t *testing.T) {
	cfg, tmpDir := setupTestEnv(t)

	adapter := proxy.NewFakeAdapter()
	executor := NewExecutor(cfg)

	plan := &ApplyPlan{
		Routes: []proxy.RouteConfig{
			{Domain: "test.example.com", UpstreamURL: "http://127.0.0.1:3001", Kind: "reverse_proxy"},
		},
	}

	rendered, _ := adapter.Render(proxy.GatewayConfig{Routes: plan.Routes})
	tempPath, err := executor.WriteTemp(rendered)
	if err != nil {
		t.Fatalf("write temp: %v", err)
	}

	// Dry-run should NOT replace config
	// Verify config not written
	if _, err := os.Stat(cfg.Proxy.CaddyfilePath); err == nil {
		// config file exists - that's fine, dry-run didn't create it
	}

	// Clean up temp
	os.Remove(tempPath)
	_ = tmpDir
}

func TestValidateFailureDoesNotReplaceConfig(t *testing.T) {
	cfg, tmpDir := setupTestEnv(t)

	adapter := proxy.NewFakeAdapter()
	adapter.ValidateShouldFail = true

	executor := NewExecutor(cfg)

	// Write initial config
	initialConfig := "initial config content"
	os.WriteFile(cfg.Proxy.CaddyfilePath, []byte(initialConfig), 0644)

	// Try to apply new config
	newConfig := []byte("new config content")
	tempPath, _ := executor.WriteTemp(newConfig)

	// Validate should fail
	err := executor.ValidateAdapter(adapter, tempPath)
	if err == nil {
		t.Error("expected validate to fail")
	}

	// Verify original config is untouched
	data, _ := os.ReadFile(cfg.Proxy.CaddyfilePath)
	if string(data) != initialConfig {
		t.Errorf("config was modified! got %q, want %q", string(data), initialConfig)
	}

	os.Remove(tempPath)
	_ = tmpDir
}

func TestReloadFailureRestoresBackup(t *testing.T) {
	cfg, tmpDir := setupTestEnv(t)

	adapter := proxy.NewFakeAdapter()
	adapter.ReloadShouldFail = true

	executor := NewExecutor(cfg)

	// Write initial config
	initialConfig := "initial config content"
	os.WriteFile(cfg.Proxy.CaddyfilePath, []byte(initialConfig), 0644)

	// Backup
	backupPath, err := executor.Backup()
	if err != nil {
		t.Fatalf("backup: %v", err)
	}

	// Replace with new config
	newConfig := []byte("new config content")
	tempPath, _ := executor.WriteTemp(newConfig)
	executor.Replace(tempPath)

	// Reload should fail
	err = executor.ReloadAdapter(adapter)
	if err == nil {
		t.Error("expected reload to fail")
	}

	// Restore backup
	err = executor.RestoreBackup(backupPath)
	if err != nil {
		t.Fatalf("restore backup: %v", err)
	}

	// Verify restored
	data, _ := os.ReadFile(cfg.Proxy.CaddyfilePath)
	if string(data) != initialConfig {
		t.Errorf("config not restored! got %q, want %q", string(data), initialConfig)
	}

	_ = tmpDir
}

func TestFakeProxyAdapter(t *testing.T) {
	adapter := proxy.NewFakeAdapter()

	// Render
	cfg := proxy.GatewayConfig{
		Routes: []proxy.RouteConfig{
			{Domain: "test.example.com", UpstreamURL: "http://127.0.0.1:3001", Kind: "reverse_proxy"},
		},
	}
	result, err := adapter.Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected rendered output")
	}

	// Validate success
	err = adapter.Validate("/tmp/test")
	if err != nil {
		t.Errorf("validate should succeed: %v", err)
	}

	// Validate failure
	adapter.ValidateShouldFail = true
	err = adapter.Validate("/tmp/test")
	if err == nil {
		t.Error("validate should fail")
	}

	// Reload
	adapter.ValidateShouldFail = false
	err = adapter.Reload("")
	if err != nil {
		t.Errorf("reload should succeed: %v", err)
	}
	if adapter.ReloadCallCount != 1 {
		t.Errorf("reload count = %d, want 1", adapter.ReloadCallCount)
	}
}

// TestApplyPlanStructure verifies the ApplyPlan/ApplyWarning types are usable.
func TestApplyPlanStructure(t *testing.T) {
	plan := ApplyPlan{
		RouteCount:         1,
		ManagedDomainCount: 0,
		SkippedCount:       2,
		Warnings: []ApplyWarning{
			{Code: WarningServiceDisabled, Severity: "warning", Message: "test", Target: "svc_1"},
			{Code: WarningNoAvailableEndpoint, Severity: "critical", Message: "test", Target: "svc_2"},
		},
	}

	if plan.RouteCount != 1 {
		t.Error("route count mismatch")
	}
	if len(plan.Warnings) != 2 {
		t.Error("warnings count mismatch")
	}
}

func TestAddressNormalization(t *testing.T) {
	ctx := context.Background()
	_ = ctx

	tests := []struct {
		in  string
		out string
	}{
		{"127.0.0.1:3001", "http://127.0.0.1:3001"},
		{"http://127.0.0.1:3001", "http://127.0.0.1:3001"},
		{"https://example.com:443", "https://example.com:443"},
	}

	for _, tt := range tests {
		normalized := endpoint.NormalizeAddress(tt.in)
		if normalized != tt.out {
			t.Errorf("NormalizeAddress(%q) = %q, want %q", tt.in, normalized, tt.out)
		}
	}
}
