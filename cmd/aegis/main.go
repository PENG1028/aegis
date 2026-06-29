package main
import (
	"aegis/internal/id"
	"context"
	"fmt"
	"os"
	"aegis/internal/action"
	"aegis/internal/adminauth"
	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/dns"
	"aegis/internal/edgemux"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/gateway"
	"aegis/internal/gateway_link"
	"aegis/internal/health"
	"aegis/internal/httpapi"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/node"
	"aegis/internal/nodeauth"
	"aegis/internal/nodestate"
	"aegis/internal/topology"
	"aegis/internal/provider"
	"aegis/internal/project"
	"aegis/internal/relay"
	"aegis/internal/proxy"
	"aegis/internal/proxy/caddy"
	"aegis/internal/route"
	"aegis/internal/routingpolicy"
	"aegis/internal/routingtable"
	"aegis/internal/secrets"
	"aegis/internal/safety"
	"aegis/internal/service"
	"aegis/internal/space"
	"aegis/internal/store"
	"aegis/internal/sync"
	"aegis/internal/tcp"
	"aegis/internal/token"
	"aegis/internal/trace"
	"aegis/internal/transparent"
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

	// Periodic database backup (default: every 6h, keep 7)
	backupMgr := store.NewBackupManager(db, cfg.Store.SQLitePath,
		cfg.Store.BackupDir, cfg.Store.BackupIntervalHrs, cfg.Store.BackupKeepCount)
	if backupMgr != nil {
		backupMgr.Start()
		defer backupMgr.Stop()
	}

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
	nodeAuthRepo := nodeauth.NewRepository(db)
	nodeAuthSvc := nodeauth.NewService(nodeAuthRepo, nodeRepo, nodeSvc)
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
	// --- Gateway Link (v1.7AB) ---
	gwLinkRepo := gatewaylink.NewRepository(db)
	// v1.8B-5: Load master key for secret-at-rest encryption
	masterKey, err := secrets.LoadMasterKey(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: master key not available — gateway link secrets will use legacy HMAC storage: %v\n", err)
		masterKey = nil
	}
	safetySvc := safety.NewService(safety.Dependencies{
		RouteRepo:    routeRepo,
		MDRRepo:      mdRepo,
		EndpointRepo: endpointRepo,
		NodeRepo:     nodeRepo,
		GWLinkRepo:   gwLinkRepo,
		ListenerRepo: listenerRepo,
	})
	// --- Relay Resolver (v1.8B) ---
	relaySvc := relay.NewResolver(relay.Dependencies{
		RouteRepo:    routeRepo,
		ServiceRepo:  serviceRepo,
		EndpointRepo: endpointRepo,
		NodeRepo:     nodeRepo,
		GWLinkRepo:   gwLinkRepo,
		ListenerRepo: listenerRepo,
	})
	applySvc := apply.NewAppService(
		cfg, proxyAdapter, routeRepo, mdRepo, exposureRepo, serviceRepo,
		endpointResolver, applyRepo, logSvc,
		gwLinkRepo, safetySvc, masterKey,
	)
	adminUserRepo := adminauth.NewAdminUserRepository(db)
	adminSessionRepo := adminauth.NewAdminSessionRepository(db)
	adminAuthSvc := adminauth.NewService(adminUserRepo, adminSessionRepo)
	adminPassword := generateRandomHex(16)
	if _, err := adminAuthSvc.EnsureAdmin("admin", adminPassword); err != nil {
		fmt.Fprintf(os.Stderr, "  admin user: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "\n=== AEGIS FIRST-RUN ADMIN CREDENTIALS ===\n")
		fmt.Fprintf(os.Stderr, "  Username: admin\n")
		fmt.Fprintf(os.Stderr, "  Password: %s\n", adminPassword)
		fmt.Fprintf(os.Stderr, "  Store this securely — it will not be shown again.\n")
		fmt.Fprintf(os.Stderr, "=========================================\n\n")
	}
	pendingState := cluster.NewPendingState(db)
	applySvc.SetPendingState(pendingState)
	nodeStateRepo := nodestate.NewRepository(db)
	nodeStateSvc := nodestate.NewService(nodeStateRepo)
	gatewayInvRepo := gateway.NewInventoryRepository(db)
	gatewayInvSvc := gateway.NewInventoryService(gatewayInvRepo)
	topologyRepo := topology.NewRepository(db)
	topologySvc := topology.NewService(topologyRepo)
	// v1.8G-3: Use a random unique self ID instead of hardcoded "gw_main".
	// This ensures the gateway identity check in HMAC auth is meaningful —
	// each Aegis instance has a distinct identity that cannot be impersonated.
	gwSelfID := "gw_" + id.GenerateRandomHex(8)
	gwLinkSvc := gatewaylink.NewService(gwLinkRepo, gwSelfID, "main-gateway", masterKey)
	spaceRepo := space.NewRepository(db)
	spaceSvc := space.NewAppService(spaceRepo, logSvc)
	endpointSvc := endpoint.NewAppService(endpointRepo, logSvc)
	actionSvc := action.NewActionService(serviceSvc, routeSvc, edgeSvc, endpointRepo, endpointSvc, applySvc, spaceRepo, logSvc, listenerSvc)

	// --- Desired State Generator (v1.8F multi-node) ---
	routingPolicyRepo := routingpolicy.NewRepository(db)
	routingPolicySvc := routingpolicy.NewService(routingPolicyRepo)
	routingTableSvc := routingtable.NewService()
	dsDataSource := nodestate.NewDBDataSource(
		nodeRepo, routeRepo, endpointRepo, gatewayInvRepo, gwLinkRepo, topologyRepo, routingPolicySvc,
	)
	dsGenerator := nodestate.NewGenerator(nodeStateSvc, dsDataSource)

	// --- Transparent Proxy Manager (v1.8H) ---
	// Intercepts direct IP:port outbound connections and routes them
	// through Aegis port 80 for unified traffic management.
	transparentMgr := transparent.NewManager()
	defer transparentMgr.Shutdown()

	// Set current node ID so StartRedirect knows local vs cross-node
	if currentNode, err := nodeRepo.FindCurrent(); err == nil && currentNode != nil {
		transparentMgr.SetCurrentNodeID(currentNode.NodeID)
	}
	// Remove any iptables rules left over from a previous crash
	if err := transparentMgr.CleanupStaleRules(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: transparent proxy cleanup: %v\n", err)
	}

	// Wire mutation hooks: any change triggers desired state regeneration
	// AND transparent proxy rule sync (v1.8H)
	dsHook := &desiredStateHook{
		gen:            dsGenerator,
		transparentMgr: transparentMgr,
		endpointRepo:   endpointRepo,
	}
	routeSvc.SetMutationHook(dsHook)
	serviceSvc.SetMutationHook(dsHook)
	endpointSvc.SetMutationHook(dsHook)

	token.SetAuditLogger(logSvc)
	adminauth.SetAuditLogger(logSvc)
	traceSvc := trace.NewService(trace.Dependencies{
		RouteRepo:    routeRepo,
		EdgeSvc:      edgeSvc,
		ListenerSvc:  listenerSvc,
		NodeRepo:     nodeRepo,
		EndpointRepo: endpointRepo,
		GatewayLinkRepo: gwLinkRepo,
	})
	authMiddleware := token.NewAuthMiddleware(cfg.Server.AdminToken, tokenRepo)

	// --- DNS Resolver (v1.8E) ---
	dnsMgmt := dns.NewManager(
		routeRepo,
		service.NewRepository(db),
		endpointRepo,
		nodeRepo,
		cfg.DNS.ListenAddr,
		cfg.DNS.Upstream,
		cfg.DNS.RefreshSec,
	)
	if cfg.DNS.Enabled {
		if err := dnsMgmt.Enable(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: dns resolver start failed: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "info: dns resolver started on %s\n", cfg.DNS.ListenAddr)
		}
	}

	httpSvcs := &httpapi.Services{
		Config:        cfg,
		Project:       projectSvc,
		Service:       serviceSvc,
		EndpointRepo:  endpointRepo,
		EndpointSvc:   endpointSvc,
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
		NodeSvc:       nodeSvc,
		NodeAuthSvc:   nodeAuthSvc,
		NodeStateSvc:  nodeStateSvc,
		GatewayInvRepo: gatewayInvRepo,
		GatewayInvSvc:  gatewayInvSvc,
		TopologySvc:    topologySvc,
		PolicySvc:       routingPolicySvc,
		RoutingTableSvc: routingTableSvc,
		Gateway:       nil,
		DepSvc:        nil,
		PendingState:  pendingState,
		TraceSvc:      traceSvc,
		GatewayLinkSvc: gwLinkSvc,
		SafetySvc:     safetySvc,
		RelaySvc:      relaySvc,
		RelayHTTPHandler: relay.RelayHandlerForMux(relay.NewRelayHandler(relay.HandlerDeps{
			RouteRepo:     routeRepo,
			EndpointRepo:  endpointRepo,
			NodeRepo:      nodeRepo,
			GWLinkRepo:    gwLinkRepo,
			LogSvc:        logSvc,
			MasterKey:     masterKey,
		})),
		DNSMgmt:         dnsMgmt,
		TransparentMgr:  transparentMgr,
	}
	cliSvcs := &cli.Services{
		Config:        cfg,
		Project:       projectSvc,
		Service:       serviceSvc,
		Route:         routeSvc,
		EndpointRepo:  endpointRepo,
		ManagedDomain: mdSvc,
		EndpointSvc:   endpointSvc,
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
		RelaySvc:       relaySvc,
		SafetySvc:      safetySvc,
		TransparentMgr: transparentMgr,
	}
	rootCmd := cli.NewRootCommand(cliSvcs)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
// desiredStateHook implements route.MutationHook, service.MutationHook,
// and endpoint.MutationHook to trigger desired state regeneration AND
// transparent proxy rule sync on any change.
type desiredStateHook struct {
	gen            *nodestate.Generator
	transparentMgr *transparent.Manager
	endpointRepo   *endpoint.Repository
}

func (h *desiredStateHook) OnRouteChanged(ctx context.Context, routeID string) error {
	if err := h.gen.GenerateForAllNodes(ctx); err != nil {
		return err
	}
	h.syncTransparentRules()
	return nil
}

func (h *desiredStateHook) OnServiceChanged(ctx context.Context, serviceID string) error {
	if err := h.gen.GenerateForAllNodes(ctx); err != nil {
		return err
	}
	h.syncTransparentRules()
	return nil
}

func (h *desiredStateHook) OnEndpointChanged(ctx context.Context, endpointID string) error {
	if err := h.gen.GenerateForAllNodes(ctx); err != nil {
		return err
	}
	h.syncTransparentRules()
	return nil
}

// syncTransparentRules reconciles transparent proxy iptables rules with
// the current endpoint database. New cross-node endpoints get interception
// rules; removed endpoints get cleaned up.
func (h *desiredStateHook) syncTransparentRules() {
	if h.transparentMgr == nil || h.endpointRepo == nil {
		return
	}

	eps, err := h.endpointRepo.FindAllEnabled()
	if err != nil {
		return
	}

	// Convert to EndpointInfo
	var infos []transparent.EndpointInfo
	for _, ep := range eps {
		host, port := ep.HostPort()
		if host == "" || port == 0 || host == "127.0.0.1" || host == "localhost" || host == "::1" {
			continue
		}
		infos = append(infos, transparent.EndpointInfo{
			EndpointID: ep.ID,
			ServiceID:  ep.ServiceID,
			Host:       host,
			Port:       port,
			NodeID:     ep.NodeID,
		})
	}

	// Discover desired rules from current endpoints
	desired := h.transparentMgr.DiscoverTargets(infos)

	// Get current rules
	current := h.transparentMgr.ListStatus()
	currentMap := make(map[string]bool)
	for _, s := range current {
		currentMap[s.Rule.ID] = s.Active
	}

	// Build desired set
	desiredMap := make(map[string]transparent.RedirectRule)
	for _, r := range desired {
		desiredMap[r.ID] = r
	}

	// Remove rules no longer desired
	for id := range currentMap {
		if _, ok := desiredMap[id]; !ok {
			h.transparentMgr.StopRedirect(id)
		}
	}

	// Add new desired rules
	for _, r := range desired {
		if _, ok := currentMap[r.ID]; !ok {
			if err := h.transparentMgr.StartRedirect(r); err != nil {
				// Log but don't fail — iptables may not be available
				// (non-Linux, no root, etc.)
				_ = err
			}
		}
	}
}

// generateRandomHex returns a cryptographically random hex string of n bytes.
// Delegates to id.GenerateRandomHex — the project's canonical random-hex generator.
func generateRandomHex(n int) string {
	return id.GenerateRandomHex(n)
}
