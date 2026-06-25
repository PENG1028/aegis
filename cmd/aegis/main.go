package main

import (
	"fmt"
	"os"

	"aegis/internal/action"
	"aegis/internal/adminauth"
	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/edgemux"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/health"
	"aegis/internal/httpapi"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/node"
	"aegis/internal/provider"
	"aegis/internal/project"
	"aegis/internal/proxy"
	"aegis/internal/proxy/caddy"
	"aegis/internal/route"
	"aegis/internal/service"
	"aegis/internal/space"
	"aegis/internal/store"
	"aegis/internal/sync"
	"aegis/internal/tcp"
	"aegis/internal/token"
	"aegis/internal/trace"

	cli "aegis/internal/cli"
)

func main() {
	configPath := ""
	for i, arg := range os.Args {
		if arg == "--config" && i+1 < len(os.Args) {
			configPath = os.Args[i+1]
			break
		}
		if len(arg) > 9 && arg[:9] == "--config=" {
			configPath = arg[9:]
			break
		}
	}

	var cfg *config.Config
	if configPath != "" {
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else {
		cwd, _ := os.Getwd()
		home, _ := os.UserHomeDir()
		defaultPaths := []string{
			cwd + "/.aegis/config/config.yaml",
			cwd + "/.aegis/config.yaml",
			home + "/.aegis/config/config.yaml",
			home + "/.aegis/config.yaml",
			"/etc/aegis/config.yaml",
		}
		loaded := false
		for _, p := range defaultPaths {
			if c, err := config.Load(p); err == nil {
				cfg = c
				loaded = true
				break
			}
		}
		if !loaded {
			cfg = config.DefaultConfig()
		}
	}

	db, err := store.OpenSQLite(cfg.Store.SQLitePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to open database: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: run 'aegis init' to initialize Aegis\n")
		os.Exit(1)
	}
	defer db.Close()

	if err := store.Initialize(db); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	projectRepo := project.NewRepository(db)
	serviceRepo := service.NewRepository(db)
	routeRepo := route.NewRepository(db)
	endpointRepo := endpoint.NewRepository(db)
	healthRepo := health.NewRepository(db)
	applyRepo := apply.NewRepository(db)
	logRepo := logs.NewRepository(db)
	mdRepo := manageddomain.NewRepository(db)
	exposureRepo := exposure.NewRepository(db)
	listenerRepo := listener.NewRepository(db)
	edgeRepo := edgemux.NewRepository(db)
	nodeRepo := node.NewRepository(db)
	tokenRepo := token.NewRepository(db)

	logSvc := logs.NewAppService(logRepo)
	applyLogRepo := logs.NewApplyLogRepository(db)
	auditLogRepo := logs.NewAuditLogRepository(db)
	nodeEventRepo := logs.NewNodeEventRepository(db)
	logSvc.SetApplyRepo(applyLogRepo)
	logSvc.SetAuditRepo(auditLogRepo)
	logSvc.SetNodeEventRepo(nodeEventRepo)

	projectSvc := project.NewAppService(projectRepo, logSvc)
	serviceSvc := service.NewAppService(serviceRepo, logSvc)
	edgeSvc := edgemux.NewAppService(edgeRepo, logSvc)
	routeSvc := route.NewAppService(routeRepo, logSvc, edgeSvc)
	mdSvc := manageddomain.NewAppService(mdRepo, logSvc)
	listenerSvc := listener.NewService(listenerRepo)
	listenerSvc.SetEdgeMuxMode(true)
	if err := listenerSvc.RegisterDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to register listeners: %v\n", err)
	}

	tcpManager := tcp.NewManager()
	defer tcpManager.Shutdown()

	nodeSvc := node.NewService(nodeRepo)
	if _, err := nodeSvc.RegisterCurrent(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: node registration failed: %v\n", err)
	}

	leaderSvc := cluster.NewLeaderService(nodeRepo)
	if leader, err := leaderSvc.GetLeader(); err == nil && leader == nil {
		if elected, err := leaderSvc.ElectLeader(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: leader election failed: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "info: elected leader: %s\n", elected.NodeID)
		}
	}

	stateVer := cluster.NewStateVersion(db)
	reconcileLoop := sync.NewReconcileLoop(nodeRepo, leaderSvc, stateVer)
	reconcileLoop.Start()
	defer reconcileLoop.Stop()

	healthSvc := health.NewAppService(healthRepo, serviceRepo, endpointRepo, logSvc)
	endpointResolver := endpoint.NewResolver(endpointRepo)

	provRegistry := provider.NewRegistry()
	provRegistry.Register(provider.NewCaddyHTTPProvider(cfg))
	provRegistry.Register(provider.NewHAProxyTCPProvider(cfg))

	exposureSvc := exposure.NewAppService(exposureRepo, logSvc, provRegistry, listenerSvc)
	var proxyAdapter proxy.ProxyAdapter = caddy.NewAdapter(cfg)

	applySvc := apply.NewAppService(
		cfg, proxyAdapter, routeRepo, mdRepo, exposureRepo, serviceRepo,
		endpointResolver, applyRepo, logSvc,
	)

	adminUserRepo := adminauth.NewAdminUserRepository(db)
	adminSessionRepo := adminauth.NewAdminSessionRepository(db)
	adminAuthSvc := adminauth.NewService(adminUserRepo, adminSessionRepo)
	if _, err := adminAuthSvc.EnsureAdmin("admin", "admin"); err != nil {
		fmt.Printf("  admin user: %v\n", err)
	}

	pendingState := cluster.NewPendingState(db)
	applySvc.SetPendingState(pendingState)

	spaceRepo := space.NewRepository(db)
	spaceSvc := space.NewAppService(spaceRepo, logSvc)
	actionSvc := action.NewActionService(serviceSvc, routeSvc, edgeSvc, endpointRepo, applySvc, spaceRepo, logSvc, listenerSvc)

	token.SetAuditLogger(logSvc)
	adminauth.SetAuditLogger(logSvc)

	traceSvc := trace.NewService(trace.Dependencies{
		RouteRepo:    routeRepo,
		EdgeSvc:      edgeSvc,
		ListenerSvc:  listenerSvc,
		NodeRepo:     nodeRepo,
		EndpointRepo: endpointRepo,
	})

	authMiddleware := token.NewAuthMiddleware(cfg.Server.AdminToken, tokenRepo)

	httpSvcs := &httpapi.Services{
		Config:        cfg,
		Project:       projectSvc,
		Service:       serviceSvc,
		EndpointRepo:  endpointRepo,
		Route:         routeSvc,
		ManagedDomain: mdSvc,
		Exposure:      exposureSvc,
		Apply:         applySvc,
		Health:        healthSvc,
		Logs:          logSvc,
		Auth:          authMiddleware,
		Action:        actionSvc,
		Space:         spaceSvc,
		TokenRepo:     tokenRepo,
		AdminAuth:     adminAuthSvc,
		EdgeSvc:       edgeSvc,
		ListenerSvc:   listenerSvc,
		NodeRepo:      nodeRepo,
		Gateway:       nil,
		DepSvc:        nil,
		PendingState:  pendingState,
		TraceSvc:      traceSvc,
	}

	cliSvcs := &cli.Services{
		Config:        cfg,
		Project:       projectSvc,
		Service:       serviceSvc,
		Route:         routeSvc,
		EndpointRepo:  endpointRepo,
		ManagedDomain: mdSvc,
		Exposure:      exposureSvc,
		ListenerSvc:   listenerSvc,
		EdgeSvc:       edgeSvc,
		LeaderSvc:     leaderSvc,
		NodeRepo:      nodeRepo,
		StateVer:      stateVer,
		DB:            db,
		Apply:         applySvc,
		Health:        healthSvc,
		Logs:          logSvc,
		Action:        actionSvc,
		Space:         spaceSvc,
		HTTPServices:  httpSvcs,
		PendingState:  pendingState,
		TraceSvc:      traceSvc,
	}

	rootCmd := cli.NewRootCommand(cliSvcs)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
