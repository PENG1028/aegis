package main

import (
	"fmt"
	"os"

	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/edgemux"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/health"
	"aegis/internal/httpapi"
	"aegis/internal/listener"
	"aegis/internal/node"
	"aegis/internal/provider"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/project"
	"aegis/internal/proxy"
	"aegis/internal/proxy/caddy"
	"aegis/internal/route"
	"aegis/internal/service"
	"aegis/internal/sync"
	"aegis/internal/store"
	"aegis/internal/tcp"
	"aegis/internal/token"

	cli "aegis/internal/cli"
)

func main() {
	// Determine config path
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

	// Load config
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
		defaultPaths := []string{
			cwd + "/.aegis/config.yaml",
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

	// Open database
	db, err := store.OpenSQLite(cfg.Store.SQLitePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to open database: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: run 'aegis init' to initialize Aegis\n")
		os.Exit(1)
	}
	defer db.Close()

	// Run versioned migrations (idempotent)
	if err := store.Initialize(db); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	// --- Repositories ---
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

	// --- Core Services ---
	logSvc := logs.NewAppService(logRepo)

	// --- App Services ---
	projectSvc := project.NewAppService(projectRepo, logSvc)
	serviceSvc := service.NewAppService(serviceRepo, logSvc)
	edgeSvc := edgemux.NewAppService(edgeRepo, logSvc)
	routeSvc := route.NewAppService(routeRepo, logSvc, edgeSvc)
	mdSvc := manageddomain.NewAppService(mdRepo, logSvc)
	listenerSvc := listener.NewService(listenerRepo)
	listenerSvc.SetEdgeMuxMode(true) // Default EdgeMux mode

	// Register EdgeMux default listeners
	if err := listenerSvc.RegisterDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to register EdgeMux listeners: %v\n", err)
	}

	tcpManager := tcp.NewManager()
	defer tcpManager.Shutdown()

	nodeSvc := node.NewService(nodeRepo)
	if _, err := nodeSvc.RegisterCurrent(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: node registration failed: %v\n", err)
	}

	// Cluster leader election
	leaderSvc := cluster.NewLeaderService(nodeRepo)
	if leader, err := leaderSvc.GetLeader(); err == nil && leader == nil {
		if elected, err := leaderSvc.ElectLeader(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: leader election failed: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "info: elected leader: %s\n", elected.NodeID)
		}
	}

	// State version tracking
	stateVer := cluster.NewStateVersion(db)

	// Reconcile loop (background node sync)
	reconcileLoop := sync.NewReconcileLoop(nodeRepo, leaderSvc, stateVer)
	reconcileLoop.Start()
	defer reconcileLoop.Stop()

	healthSvc := health.NewAppService(healthRepo, serviceRepo, endpointRepo, logSvc)

	// --- Endpoint Resolver ---
	endpointResolver := endpoint.NewResolver(endpointRepo)

	// --- Provider Registry ---
	provRegistry := provider.NewRegistry()
	caddyHTTP := provider.NewCaddyHTTPProvider(cfg)
	haproxyTCP := provider.NewHAProxyTCPProvider(cfg)
	provRegistry.Register(caddyHTTP)
	provRegistry.Register(haproxyTCP)

	exposureSvc := exposure.NewAppService(exposureRepo, logSvc, provRegistry, listenerSvc)

	// Keep legacy proxy adapter for backward compat
	var proxyAdapter proxy.ProxyAdapter = caddy.NewAdapter(cfg)

	// --- Apply Service ---
	applySvc := apply.NewAppService(
		cfg, proxyAdapter, routeRepo, mdRepo, exposureRepo, serviceRepo,
		endpointResolver, applyRepo, logSvc,
	)

	// --- Auth Middleware (with scope checking) ---
	authMiddleware := token.NewAuthMiddleware(cfg.Server.AdminToken, tokenRepo)

	// --- HTTP API Services ---
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
	}

	// --- CLI Services ---
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
		HTTPServices:  httpSvcs,
	}

	// --- Execute ---
	rootCmd := cli.NewRootCommand(cliSvcs)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
