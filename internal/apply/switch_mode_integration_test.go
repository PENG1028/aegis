package apply

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aegis/internal/certstore"
	"aegis/internal/hostdep/provider"
	"aegis/internal/route"
	"aegis/internal/service"
	"aegis/internal/store"
	"aegis/internal/topology"
	"aegis/internal/topology/templates"

	_ "modernc.org/sqlite"
)

type stagedSwitchProvider struct {
	id           string
	configPath   string
	capabilities []provider.Capability
	running      bool
	startErr     error
	stopErr      error
	diagnoseErr  bool
}

func (p *stagedSwitchProvider) State() provider.ProviderState {
	return provider.ProviderState{
		ID: p.id, Installed: true, Running: p.running, Status: "ready",
		ConfigPath: p.configPath, Capabilities: p.capabilities,
	}
}
func (p *stagedSwitchProvider) Diagnose() provider.ProviderDiagnostic {
	if p.diagnoseErr {
		return provider.ProviderDiagnostic{LastErrorCode: "TEST_DIAG", LastErrorMessage: "diagnostic failed"}
	}
	return provider.ProviderDiagnostic{}
}
func (p *stagedSwitchProvider) Render(provider.Plan) ([]provider.ConfigFile, error) {
	return []provider.ConfigFile{{Path: p.configPath, Content: []byte("target-" + p.id)}}, nil
}
func (p *stagedSwitchProvider) Apply(configs []provider.ConfigFile) error {
	return p.StageConfig(configs)
}
func (p *stagedSwitchProvider) StageConfig(configs []provider.ConfigFile) error {
	for _, config := range configs {
		if err := os.WriteFile(config.Path, config.Content, 0o600); err != nil {
			return err
		}
	}
	return nil
}
func (p *stagedSwitchProvider) Start() error {
	if p.startErr != nil {
		return p.startErr
	}
	p.running = true
	return nil
}
func (p *stagedSwitchProvider) Stop() error {
	if p.stopErr != nil {
		return p.stopErr
	}
	p.running = false
	return nil
}
func (p *stagedSwitchProvider) Restart() error { return nil }

func TestSwitchModeStagesTargetAndStartsCompleteProviderSet(t *testing.T) {
	w, caddy, haproxy := switchModeWorkflow(t)
	if err := w.SwitchMode(context.Background(), provider.RuntimeModeEdgeMux.ID); err != nil {
		t.Fatal(err)
	}
	if !caddy.running || !haproxy.running {
		t.Fatalf("target providers not running: caddy=%v haproxy=%v", caddy.running, haproxy.running)
	}
	if got, _ := os.ReadFile(caddy.configPath); string(got) != "target-caddy" {
		t.Fatalf("caddy config = %q", got)
	}
}

func TestSwitchModeRollsBackConfigAndServicesOnTargetStartFailure(t *testing.T) {
	w, caddy, haproxy := switchModeWorkflow(t)
	haproxy.startErr = errors.New("start failed")
	if err := w.SwitchMode(context.Background(), provider.RuntimeModeEdgeMux.ID); err == nil {
		t.Fatal("switch unexpectedly succeeded")
	}
	if !caddy.running || haproxy.running {
		t.Fatalf("service state not restored: caddy=%v haproxy=%v", caddy.running, haproxy.running)
	}
	if got, _ := os.ReadFile(caddy.configPath); string(got) != "old-caddy" {
		t.Fatalf("caddy config not restored: %q", got)
	}
}

func TestSwitchModeDiagnosticFailureRollsBackAndReturnsError(t *testing.T) {
	w, caddy, haproxy := switchModeWorkflow(t)
	haproxy.diagnoseErr = true
	err := w.SwitchMode(context.Background(), provider.RuntimeModeEdgeMux.ID)
	if err == nil {
		t.Fatal("diagnostic failure was reported as a successful switch")
	}
	var switchErr *ModeSwitchError
	if !errors.As(err, &switchErr) || switchErr.RollbackStatus != RollbackComplete {
		t.Fatalf("rollback status = %v, error = %v", switchErr, err)
	}
	if !caddy.running || haproxy.running {
		t.Fatalf("service state not restored: caddy=%v haproxy=%v", caddy.running, haproxy.running)
	}
	if got, _ := os.ReadFile(caddy.configPath); string(got) != "old-caddy" {
		t.Fatalf("caddy config not restored: %q", got)
	}
}

func TestSwitchModeReportsIncompleteRollback(t *testing.T) {
	w, _, haproxy := switchModeWorkflow(t)
	haproxy.stopErr = errors.New("stop failed")

	err := w.SwitchMode(context.Background(), provider.RuntimeModeEdgeMux.ID)
	var switchErr *ModeSwitchError
	if !errors.As(err, &switchErr) {
		t.Fatalf("expected ModeSwitchError, got %v", err)
	}
	if switchErr.RollbackStatus != RollbackIncomplete || len(switchErr.RollbackFailures) == 0 {
		t.Fatalf("unexpected rollback result: %+v", switchErr)
	}
}

func switchModeWorkflow(t *testing.T) (*Workflow, *stagedSwitchProvider, *stagedSwitchProvider) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	caddyPath := filepath.Join(dir, "Caddyfile")
	haproxyPath := filepath.Join(dir, "haproxy.cfg")
	if err := os.WriteFile(caddyPath, []byte("old-caddy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(haproxyPath, []byte("old-haproxy"), 0o600); err != nil {
		t.Fatal(err)
	}
	caddy := &stagedSwitchProvider{
		id: "caddy", configPath: caddyPath, running: true,
		capabilities: []provider.Capability{
			provider.CapListenTCP, provider.CapUpstreamTCP, provider.CapTLSTerminate,
			provider.CapHTTP1, provider.CapRouteHost, provider.CapAutoCert,
			provider.CapHotReload, provider.CapValidateConfig,
		},
	}
	haproxy := &stagedSwitchProvider{
		id: "haproxy", configPath: haproxyPath,
		capabilities: []provider.Capability{
			provider.CapListenTCP, provider.CapUpstreamTCP, provider.CapTLSPassthrough,
			provider.CapSNIPreread, provider.CapRawTCP,
		},
	}
	registry := provider.NewRegistry()
	registry.Register(caddy)
	registry.Register(haproxy)
	planner := topology.NewPlanner(templates.Default(), topology.Dependencies{
		RouteRepo:   route.NewRepository(db),
		ServiceRepo: service.NewRepository(db),
		CertStore:   certstore.NewService(certstore.NewRepository(db), t.TempDir()),
	})
	return &Workflow{planner: planner, registry: registry}, caddy, haproxy
}
