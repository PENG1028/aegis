// Scenario F: Port 80 Unified Forwarding
// Verifies that Aegis can intercept on port 80 and forward to backend services
// on arbitrary ports, both locally and cross-machine via Gateway Links.
package e2e

import (
	"context"
	"database/sql"
	"net"
	"strings"
	"testing"

	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/edgemux"
	"aegis/internal/endpoint"
	"aegis/internal/logs"
	"aegis/internal/project"
	"aegis/internal/provider"
	"aegis/internal/fake"
	"aegis/internal/topology"
	"aegis/internal/topology/templates"
	"aegis/internal/route"
	"aegis/internal/safety"
	"aegis/internal/secrets"
	"aegis/internal/service"

	gatewaylink "aegis/internal/gateway_link"
)

func startTCPListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start tcp listener: %v", err)
	}
	return l, l.Addr().String()
}

func setupApplySvc(t *testing.T, db *sql.DB, cfg *config.Config, gwLinkRepo *gatewaylink.Repository, masterKey *secrets.MasterKey) *apply.AppService {
	t.Helper()
	logRepo := logs.NewRepository(db)
	logSvc := logs.NewAppService(logRepo)

	routeRepo := route.NewRepository(db)
	serviceRepo := service.NewRepository(db)
	endpointResolver := endpoint.NewResolver(endpoint.NewRepository(db))
	applyRepo := apply.NewRepository(db)
	safetySvc := safety.NewService(safety.Dependencies{})

	provReg := provider.NewRegistry()
	provReg.Register(fake.NewFakeProvider("caddy", "http"))
	topoPlanner := topology.NewPlanner(templates.Default(), topology.Dependencies{
		RouteRepo:        routeRepo,
		ServiceRepo:      serviceRepo,
		EndpointResolver: endpointResolver,
		GwLinkRepo:       gwLinkRepo,
		SafetySvc:        safetySvc,
		MasterKey:        masterKey,
	})
	workflow := apply.NewWorkflow(topoPlanner, provReg, applyRepo, cfg, logSvc)
	applySvc := apply.NewAppService(cfg, workflow, applyRepo, logSvc)
	applySvc.SetPendingState(cluster.NewPendingState(db))
	return applySvc
}

func TestPort80Unified_LocalBackendOnHighPort(t *testing.T) {
	st, cleanup := setupTempDB(t)
	defer cleanup()
	ctx := context.Background()

	l, addr := startTCPListener(t)
	defer l.Close()

	cfg := config.DefaultConfig()
	tmpDir := t.TempDir()
	cfg.Proxy.CaddyfilePath = tmpDir + "/Caddyfile"
	cfg.Proxy.BackupDir = tmpDir + "/backups"
	cfg.Proxy.Email = "test@aegis.local"

	logRepo := logs.NewRepository(st.DB)
	logSvc := logs.NewAppService(logRepo)

	projectSvc := project.NewAppService(project.NewRepository(st.DB), logSvc)
	serviceSvc := service.NewAppService(service.NewRepository(st.DB), logSvc)
	edgeSvc := edgemux.NewAppService(edgemux.NewRepository(st.DB), logSvc)
	routeSvc := route.NewAppService(route.NewRepository(st.DB), logSvc, edgeSvc)
	endpointRepo := endpoint.NewRepository(st.DB)
	endpointSvc := endpoint.NewAppService(endpointRepo, logSvc)

	applySvc := setupApplySvc(t, st.DB, cfg, nil, nil)

	proj, _ := projectSvc.CreateProject(ctx, project.CreateProjectInput{Name: "port80-test"})
	svc, _ := serviceSvc.CreateService(ctx, service.CreateServiceInput{
		ProjectID: proj.ID, ProjectName: proj.Name, Name: "backend-api", Kind: "http",
	})
	ep, _ := endpointSvc.CreateEndpoint(ctx, endpoint.CreateEndpointInput{
		ServiceID: svc.ID, Type: "local", Address: addr,
	})
	ep.Enabled = true
	endpointRepo.Update(ep)

	_, err := routeSvc.CreateRoute(ctx, route.CreateRouteInput{
		Domain: "api.example.com", ServiceID: svc.ID, PathPrefix: "/",
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	plan, err := applySvc.DryRun(ctx)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}

	rendered := plan.RenderedConfig
	t.Logf("Caddyfile (routes=%d warnings=%d):\n%s", plan.RouteCount, len(plan.Warnings), rendered)

	if !strings.Contains(rendered, "api.example.com") {
		t.Error("Caddyfile must contain route domain")
	}
	if !strings.Contains(rendered, addr) {
		t.Errorf("Caddyfile must reverse_proxy to %s", addr)
	}
	t.Logf("PASS: backend on %s proxied through Caddy :80", addr)
}

func TestPort80Unified_CrossMachineViaGatewayLink(t *testing.T) {
	st, cleanup := setupTempDB(t)
	defer cleanup()
	ctx := context.Background()

	l, addr := startTCPListener(t)
	defer l.Close()

	cfg := config.DefaultConfig()
	tmpDir := t.TempDir()
	cfg.Proxy.CaddyfilePath = tmpDir + "/Caddyfile"
	cfg.Proxy.BackupDir = tmpDir + "/backups"

	logRepo := logs.NewRepository(st.DB)
	logSvc := logs.NewAppService(logRepo)

	projectSvc := project.NewAppService(project.NewRepository(st.DB), logSvc)
	serviceSvc := service.NewAppService(service.NewRepository(st.DB), logSvc)
	edgeSvc := edgemux.NewAppService(edgemux.NewRepository(st.DB), logSvc)
	routeRepo := route.NewRepository(st.DB)
	routeSvc := route.NewAppService(routeRepo, logSvc, edgeSvc)
	endpointRepo := endpoint.NewRepository(st.DB)
	endpointSvc := endpoint.NewAppService(endpointRepo, logSvc)

	gwLinkRepo := gatewaylink.NewRepository(st.DB)
	masterKey, _ := secrets.LoadMasterKey(true)

	remoteGW, rawSecret, err := gatewaylink.NewService(gwLinkRepo, "gw_src", "source", masterKey).Register(
		"machine-b", "<SERVER_B_IP>", "10.0.0.2", 80,
		gatewaylink.TypeUpstream, true,
	)
	if err != nil {
		t.Fatalf("register gateway: %v", err)
	}
	_ = rawSecret

	applySvc := setupApplySvc(t, st.DB, cfg, gwLinkRepo, masterKey)

	proj, _ := projectSvc.CreateProject(ctx, project.CreateProjectInput{Name: "cross-node"})
	svc, _ := serviceSvc.CreateService(ctx, service.CreateServiceInput{
		ProjectID: proj.ID, ProjectName: proj.Name, Name: "b-service", Kind: "http",
	})
	ep, _ := endpointSvc.CreateEndpoint(ctx, endpoint.CreateEndpointInput{
		ServiceID: svc.ID, Type: "private", Address: addr, NodeID: "node_machine_b",
	})
	rt, _ := routeSvc.CreateRoute(ctx, route.CreateRouteInput{
		Domain: "b-service.example.com", ServiceID: svc.ID, PathPrefix: "/",
	})
	rt.GatewayLinkID = remoteGW.ID
	routeRepo.Update(rt)
	ep.Enabled = true
	endpointRepo.Update(ep)

	plan, err := applySvc.DryRun(ctx)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}

	rendered := plan.RenderedConfig
	t.Logf("Cross-machine Caddyfile (routes=%d):\n%s", plan.RouteCount, rendered)

	// Must NOT proxy to local endpoint (only on Machine B)
	if strings.Contains(rendered, addr) {
		t.Errorf("FAIL: must NOT proxy to %s — only on Machine B", addr)
	}
	// Must proxy to Machine B's IP:80 via gateway link
	if !strings.Contains(rendered, "<SERVER_B_IP>:80") && !strings.Contains(rendered, "10.0.0.2:80") {
		t.Errorf("FAIL: must proxy to remote :80, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "X-Aegis-Gateway-Link") {
		t.Error("FAIL: must include gateway link auth header")
	}
	t.Logf("PASS: cross-machine forwarding via remote Caddy :80")
}

func TestPort80Unified_MultipleServicesOnDifferentPorts(t *testing.T) {
	st, cleanup := setupTempDB(t)
	defer cleanup()
	ctx := context.Background()

	var listeners []net.Listener
	var addrs []string
	for i := 0; i < 3; i++ {
		l, addr := startTCPListener(t)
		listeners = append(listeners, l)
		addrs = append(addrs, addr)
	}
	defer func() {
		for _, l := range listeners {
			l.Close()
		}
	}()

	cfg := config.DefaultConfig()
	tmpDir := t.TempDir()
	cfg.Proxy.CaddyfilePath = tmpDir + "/Caddyfile"
	cfg.Proxy.BackupDir = tmpDir + "/backups"

	logRepo := logs.NewRepository(st.DB)
	logSvc := logs.NewAppService(logRepo)

	projectSvc := project.NewAppService(project.NewRepository(st.DB), logSvc)
	serviceSvc := service.NewAppService(service.NewRepository(st.DB), logSvc)
	edgeSvc := edgemux.NewAppService(edgemux.NewRepository(st.DB), logSvc)
	routeSvc := route.NewAppService(route.NewRepository(st.DB), logSvc, edgeSvc)
	endpointSvc := endpoint.NewAppService(endpoint.NewRepository(st.DB), logSvc)

	applySvc := setupApplySvc(t, st.DB, cfg, nil, nil)

	proj, _ := projectSvc.CreateProject(ctx, project.CreateProjectInput{Name: "multi-port"})

	services := []struct {
		name, domain, addr string
	}{
		{"api", "api.example.com", addrs[0]},
		{"web", "www.example.com", addrs[1]},
		{"admin", "admin.internal.com", addrs[2]},
	}

	for _, s := range services {
		svc, _ := serviceSvc.CreateService(ctx, service.CreateServiceInput{
			ProjectID: proj.ID, ProjectName: proj.Name, Name: s.name, Kind: "http",
		})
		endpointSvc.CreateEndpoint(ctx, endpoint.CreateEndpointInput{
			ServiceID: svc.ID, Type: "local", Address: s.addr,
		})
		routeSvc.CreateRoute(ctx, route.CreateRouteInput{
			Domain: s.domain, ServiceID: svc.ID, PathPrefix: "/",
		})
	}

	plan, err := applySvc.DryRun(ctx)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}

	rendered := plan.RenderedConfig
	t.Logf("Multi-service Caddyfile (routes=%d):\n%s", plan.RouteCount, rendered)

	for _, s := range services {
		if !strings.Contains(rendered, s.domain) {
			t.Errorf("service %s: missing domain %s", s.name, s.domain)
		}
		if !strings.Contains(rendered, s.addr) {
			t.Errorf("service %s: missing upstream %s", s.name, s.addr)
		}
	}
	if plan.RouteCount < 3 {
		t.Errorf("expected >=3 routes, got %d", plan.RouteCount)
	}
	t.Logf("PASS: %d services all proxied through port 80", len(services))
}

func TestPort80Unified_NoExtraPortManagement(t *testing.T) {
	st, cleanup := setupTempDB(t)
	defer cleanup()
	ctx := context.Background()

	l, addr := startTCPListener(t)
	defer l.Close()

	cfg := config.DefaultConfig()
	tmpDir := t.TempDir()
	cfg.Proxy.CaddyfilePath = tmpDir + "/Caddyfile"
	cfg.Proxy.BackupDir = tmpDir + "/backups"

	logRepo := logs.NewRepository(st.DB)
	logSvc := logs.NewAppService(logRepo)

	projectSvc := project.NewAppService(project.NewRepository(st.DB), logSvc)
	serviceSvc := service.NewAppService(service.NewRepository(st.DB), logSvc)
	edgeSvc := edgemux.NewAppService(edgemux.NewRepository(st.DB), logSvc)
	routeSvc := route.NewAppService(route.NewRepository(st.DB), logSvc, edgeSvc)
	endpointRepo := endpoint.NewRepository(st.DB)
	endpointSvc := endpoint.NewAppService(endpointRepo, logSvc)

	applySvc := setupApplySvc(t, st.DB, cfg, nil, nil)

	proj, _ := projectSvc.CreateProject(ctx, project.CreateProjectInput{Name: "no-extra-ports"})
	svc, _ := serviceSvc.CreateService(ctx, service.CreateServiceInput{
		ProjectID: proj.ID, ProjectName: proj.Name, Name: "internal", Kind: "http",
	})
	ep, _ := endpointSvc.CreateEndpoint(ctx, endpoint.CreateEndpointInput{
		ServiceID: svc.ID, Type: "local", Address: addr,
	})
	ep.Enabled = true
	endpointRepo.Update(ep)

	_, err := routeSvc.CreateRoute(ctx, route.CreateRouteInput{
		Domain: "internal.example.com", ServiceID: svc.ID, PathPrefix: "/",
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	plan, err := applySvc.DryRun(ctx)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}

	rendered := plan.RenderedConfig

	// Port 80 is Caddy's ingress — no backend should be on port 80
	if strings.Contains(rendered, "reverse_proxy http://127.0.0.1:80") {
		t.Error("no backend on port 80 — Caddy owns port 80 for ingress")
	}
	if !strings.Contains(rendered, addr) {
		t.Errorf("backend should use its actual port %s, not port 80", addr)
	}

	t.Logf("PASS: port 80 reserved for Caddy ingress, backend on %s", addr)
}
