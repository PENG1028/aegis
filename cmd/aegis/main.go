package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aegis/internal/acme"
	"aegis/internal/action"
	"aegis/internal/adminauth"
	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/credential"
	"aegis/internal/dns"
	"aegis/internal/edgemux"
	"aegis/internal/distnode"
	"aegis/internal/httpapi/handlers"
	"aegis/internal/certstore"
	"aegis/internal/egress"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/gateway"
	"aegis/internal/health"
	"aegis/internal/httpapi"
	"aegis/internal/core"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/node"
	"aegis/internal/nodeauth"
	"aegis/internal/project"
	"aegis/internal/provider"
	"aegis/internal/route"
	"aegis/internal/routingpolicy"
	"aegis/internal/routingtable"
	"aegis/internal/safety"
	"aegis/internal/secrets"
	"aegis/internal/service"
	"aegis/internal/serviceauth"
	serviceauthaegis "aegis/internal/serviceauth/aegis"
	"aegis/internal/space"
	"aegis/internal/store"
	"aegis/internal/sync"
	"aegis/internal/tcp"
	"aegis/internal/token"
	"aegis/internal/topology"
	"aegis/internal/topology/templates"
	"aegis/internal/trace"
	"aegis/internal/transparent"
	"aegis/internal/udp"
	"time"

	cli "aegis/internal/cli"
)

// Build-time variables injected by ldflags:
//
//	go build -ldflags="-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"
var (
	Version   = "dev"
	BuildTime = "unknown"
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
			c, err := config.Load(p)
			if err == nil {
				cfg = c
				loaded = true
				break
			}
			if _, statErr := os.Stat(p); statErr == nil {
				fmt.Fprintf(os.Stderr, "warning: config file %s exists but could not be loaded: %v\n", p, err)
			}
		}
		if !loaded {
			fmt.Fprintf(os.Stderr, "warning: no valid config file found, using development defaults\n")
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
	provRegistry.Register(provider.NewCaddyProvider(cfg))
	provRegistry.Register(provider.NewHAProxyProvider("", "", cfg.Proxy.BackupDir))
	exposureSvc := exposure.NewAppService(exposureRepo, logSvc, provRegistry, listenerSvc)

	tcpMgr := tcp.NewManager()
	exposureSvc.SetTCPManager(tcpMgr)
	defer tcpMgr.Shutdown()
	udpMgr := udp.NewManager()
	exposureSvc.SetUDPManager(udpMgr)
	defer udpMgr.Shutdown()

	// --- Gateway Link (v1.7AB) ---
	gwLinkRepo := gateway.NewLinkRepository(db)
	masterKey, err := secrets.LoadMasterKey(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: master key not available — gateway link secrets will use legacy HMAC storage: %v\n", err)
		masterKey = nil
	}
	credRepo := credential.NewRepository(db)
	credSvc := credential.NewService(credRepo, masterKey, logSvc)
	exposureSvc.SetCredentialService(credSvc)
	safetySvc := safety.NewService(safety.Dependencies{
		RouteRepo:    routeRepo,
		MDRRepo:      mdRepo,
		EndpointRepo: endpointRepo,
		NodeRepo:     nodeRepo,
		GWLinkRepo:   gwLinkRepo,
		ListenerRepo: listenerRepo,
	})
	relaySvc := gateway.NewResolver(gateway.Dependencies{
		RouteRepo:    routeRepo,
		ServiceRepo:  serviceRepo,
		EndpointRepo: endpointRepo,
		NodeRepo:     nodeRepo,
		GWLinkRepo:   gwLinkRepo,
		ListenerRepo: listenerRepo,
	})

	// ── Certificate Store (v1.9C) ──
	certRepo := certstore.NewRepository(db)
	certDir := cfg.Runtime.DataDir + "/certs"
	certStoreSvc := certstore.NewService(certRepo, certDir)

	// ── ACME Client (v1.9C) — embedded lego, replaces certbot ──
	acmeClient, err := acme.NewClient(certStoreSvc, cfg.Proxy.Email, "", cfg.Runtime.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  acme client: %v (ACME disabled)\n", err)
		acmeClient = nil
	}

	// --- v1.8L: Topology Planner (dimension 2) + Workflow orchestrator ---
	topoPlanner := topology.NewPlanner(templates.Default(), topology.Dependencies{
		RouteRepo:        routeRepo,
		ServiceRepo:      serviceRepo,
		EndpointResolver: endpointResolver,
		GwLinkRepo:       gwLinkRepo,
		SafetySvc:        safetySvc,
		MasterKey:        masterKey,
		CertStore:        certStoreSvc,
	})
	workflow := apply.NewWorkflow(topoPlanner, provRegistry, applyRepo, cfg, logSvc, certStoreSvc)

		applySvc := apply.NewAppService(cfg, workflow, applyRepo, logSvc)

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
	gatewayInvRepo := gateway.NewInventoryRepository(db)
	gatewayInvSvc := gateway.NewInventoryService(gatewayInvRepo)
	topologyRepo := topology.NewRepository(db)
	topologySvc := topology.NewService(topologyRepo)
	gwSelfID := "gw_" + core.GenerateRandomHex(8)
	gwLinkSvc := gateway.NewLinkService(gwLinkRepo, gwSelfID, "main-gateway", masterKey)
	spaceRepo := space.NewRepository(db)
	spaceSvc := space.NewAppService(spaceRepo, logSvc)
	endpointSvc := endpoint.NewAppService(endpointRepo, logSvc)
	actionSvc := action.NewActionService(serviceSvc, routeSvc, edgeSvc, endpointRepo, endpointSvc, applySvc, spaceRepo, logSvc, listenerSvc)

	routingPolicyRepo := routingpolicy.NewRepository(db)
	routingPolicySvc := routingpolicy.NewService(routingPolicyRepo)
	routingTableSvc := routingtable.NewService()

	transparentMgr := transparent.NewManager()
	defer transparentMgr.Shutdown()
	if currentNode, err := nodeRepo.FindCurrent(); err == nil && currentNode != nil {
		transparentMgr.SetCurrentNodeID(currentNode.NodeID)
	}
	if err := transparentMgr.CleanupStaleRules(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: transparent proxy cleanup: %v\n", err)
	}

	dsHook := &desiredStateHook{
		gen:            nil,
		transparentMgr: transparentMgr,
		nodeRepo:       nodeRepo,
	}
	routeSvc.SetMutationHook(dsHook)
	serviceSvc.SetMutationHook(dsHook)
	endpointSvc.SetMutationHook(dsHook)

	token.SetAuditLogger(logSvc)
	adminauth.SetAuditLogger(logSvc)

	// --- Service Auth (v1.9A) ---
	serviceAuthRepo := serviceauth.NewRepository(db)
	serviceAuthSvc, err := serviceauth.NewService(serviceauth.Dependencies{
		Repo:        serviceAuthRepo,
		
		NodeChecker: serviceauthaegis.NewNodeCheckerAdapter(nodeRepo),
		LogWriter:   serviceauthaegis.NewLogWriterAdapter(logSvc),
		IDGen:       func() string { return core.NewID("sa") },
		MasterKey:   masterKey.Bytes(),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: service auth init failed: %v\n", err)
	} else {
		token.SetServiceAuthChecker(serviceAuthSvc)
			actionSvc.SetCallReporter(func(ctx context.Context, caller, target, api string, allowed bool, latencyMs int, errMsg string) error {
					return serviceAuthSvc.Report(ctx, serviceauth.ReportRequest{
						CallerService: caller,
						TargetService: target,
						TargetAPI:     api,
						Allowed:       allowed,
						LatencyMs:     latencyMs,
						ErrorMsg:      errMsg,
					})
				})

			// Aegis self-registration with persistent key
			go func() {
				ctx := context.Background()
				const aegisName = "aegis-gateway"
				instanceID := "aegis-" + core.NewID("id")[3:]

				// Load or generate persistent Ed25519 key at /var/lib/aegis/keys/
				keyDir := "/var/lib/aegis/keys"
				keyPath := filepath.Join(keyDir, aegisName+".key")
				os.MkdirAll(keyDir, 0700)

				privKeyB64 := ""
				if data, err := os.ReadFile(keyPath); err == nil && len(data) > 0 {
					privKeyB64 = string(data)
				}
				var pubKey string
				if privKeyB64 != "" {
					privBytes, err := base64.StdEncoding.DecodeString(privKeyB64)
					if err == nil {
						priv := ed25519.PrivateKey(privBytes)
						pub := priv.Public().(ed25519.PublicKey)
						pubKey = base64.StdEncoding.EncodeToString(pub)
					}
				}
				if pubKey == "" {
					pub, priv, err := ed25519.GenerateKey(nil)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: aegis self-registration key generation failed: %v\n", err)
						return
					}
					pubKey = base64.StdEncoding.EncodeToString(pub)
					privKeyB64 = base64.StdEncoding.EncodeToString(priv)
					os.WriteFile(keyPath, []byte(privKeyB64), 0600)
				}

				_, err := serviceAuthSvc.Register(ctx, serviceauth.RegisterRequest{
					ServiceName: aegisName,
					PublicKey:   pubKey,
					InstanceID:  instanceID,
				}, "127.0.0.1")
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: aegis self-registration failed: %v\n", err)
					return
				}
				fmt.Fprintf(os.Stderr, "info: aegis self-registered as %s (%s) key=%s\n", aegisName, instanceID, keyPath)

				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						if err := serviceAuthSvc.Heartbeat(ctx, aegisName, instanceID); err != nil {
							fmt.Fprintf(os.Stderr, "warning: aegis heartbeat: %v\n", err)
						}
					case <-ctx.Done():
						return
					}
				}
			}() // bridge: Ticket → ActionContext
	}
	traceSvc := trace.NewService(trace.Dependencies{
		RouteRepo:       routeRepo,
		EdgeSvc:         edgeSvc,
		ListenerSvc:     listenerSvc,
		NodeRepo:        nodeRepo,
		EndpointRepo:    endpointRepo,
		GatewayLinkRepo: gwLinkRepo,
	})
	authMiddleware := token.NewAuthMiddleware(cfg.Server.AdminToken)

	// ── Egress Gateway (v1.9A-5) ──
	egressRepo := egress.NewRepository(db)
	egressSvc := egress.NewService(egress.Dependencies{Repo: egressRepo, IDGen: nil})
	egressRuleChecker := egress.NewRuleChecker(egressSvc)
	egressRuleChecker.Refresh()

	dnsMgmt := dns.NewManager(
		routeRepo,
		service.NewRepository(db),
		endpointRepo,
		nodeRepo,
		cfg.DNS.ListenAddr,
		cfg.DNS.Upstream,
		cfg.DNS.RefreshSec,
	)
	dnsMgmt.Resolver.SetAllowlistChecker(egressRuleChecker)
	// Dnsmasq integration: write config to /etc/dnsmasq.d/ for independent DNS serving.
	// Falls back to in-process UDP server if dnsmasq is not installed.
	dnsMgmt.Dnsmasq = &dns.DnsmasqConfig{
		ConfigPath: cfg.Runtime.DataDir + "/dnsmasq/aegis.conf",
		Upstream:   cfg.DNS.Upstream,
		ReloadCmd:  "systemctl reload dnsmasq",
	}
	if cfg.DNS.Enabled {
		if err := dnsMgmt.Enable(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: dns resolver start failed: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "info: dns resolver started on %s\n", cfg.DNS.ListenAddr)
		}
	}


	// v1.9B: Distributed Node Runtime
	var dn *distnode.DistNode
	if cfg.DistNode.Enabled {
		distCfg := distnode.Config{
			ID:     cfg.DistNode.ID,
			Name:   cfg.DistNode.Name,
			Addr:   cfg.DistNode.Addr,
			Secret: cfg.DistNode.Secret,
		}
		for _, p := range cfg.DistNode.Peers {
			distCfg.Peers = append(distCfg.Peers, distnode.PeerConfig{ID: p.ID, Addr: p.Addr})
		}
		dn = distnode.New(distCfg)
		handlers.RegisterDistNodeHandlers(dn)
		fmt.Fprintf(os.Stderr, "info: distnode enabled - id=%s addr=%s peers=%d\n", dn.ID, distCfg.Addr, len(distCfg.Peers))

			// Auto-register discovered peers in the nodes table so they appear in UI.
			dn.Membership.OnEvent(func(evt distnode.PeerEvent) {
				switch evt.Type {
				case distnode.EventPeerAlive:
					nodeRepo.Create(&node.NodeRecord{
						NodeID:    evt.Peer.Info.ID,
						PublicIP:  evt.Peer.Info.Addr,
						Role:      "worker",
						Status:    "online",
						LastSeen:  time.Now(),
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					})
				case distnode.EventPeerDead:
					nodeRepo.UpdateHeartbeat(evt.Peer.Info.ID, "offline", "", "", "", "", "", time.Now())
				}
			})

		go dn.Start(context.Background())
	} else {
		fmt.Fprintf(os.Stderr, "info: distnode disabled\n")
	}
httpSvcs := &httpapi.Services{
		DB:               db,
		Config:           cfg,
		Project:          projectSvc,
		Service:          serviceSvc,
		EndpointRepo:     endpointRepo,
		EndpointSvc:      endpointSvc,
		Route:            routeSvc,
		ManagedDomain:    mdSvc,
		Exposure:         exposureSvc,
		Apply:            applySvc,
		Workflow:         workflow,
		Health:           healthSvc,
		Logs:             logSvc,
		Auth:             authMiddleware,
		Action:           actionSvc,
		Space:            spaceSvc,
		AdminAuth:        adminAuthSvc,
		EdgeSvc:          edgeSvc,
		ListenerSvc:      listenerSvc,
		NodeRepo:         nodeRepo,
		NodeSvc:          nodeSvc,
		NodeAuthSvc:      nodeAuthSvc,
		NodeStateSvc:     nil,
		GatewayInvRepo:   gatewayInvRepo,
		GatewayInvSvc:    gatewayInvSvc,
		TopologySvc:      topologySvc,
		PolicySvc:        routingPolicySvc,
		RoutingTableSvc:  routingTableSvc,
		Gateway:          nil,
		DepSvc:           nil,
		PendingState:     pendingState,
		TraceSvc:         traceSvc,
		GatewayLinkSvc:   gwLinkSvc,
		SafetySvc:        safetySvc,
		RelaySvc:         relaySvc,
		RelayHTTPHandler: gateway.RelayHandlerForMux(gateway.NewRelayHandler(gateway.HandlerDeps{
			RouteRepo:    routeRepo,
			EndpointRepo: endpointRepo,
			NodeRepo:     nodeRepo,
			GWLinkRepo:   gwLinkRepo,
			LogSvc:       logSvc,
			MasterKey:    masterKey,
		})),
		DNSMgmt:        dnsMgmt,
		TransparentMgr: transparentMgr,
		CredentialSvc:  credSvc,
		ServiceAuthSvc: serviceAuthSvc,
		EgressSvc:       egressSvc,
		CertStore:       certStoreSvc,
		ACMEClient:      acmeClient,
		ProvReg:        provRegistry,
		Version:        Version,
		BuildTime:      BuildTime,
		DistNode:        dn,
		OnShutdown: func() {
			fmt.Fprintf(os.Stderr, "stopping subsystems...\n")
			tcpMgr.Shutdown()
			udpMgr.Shutdown()
			transparentMgr.Shutdown()
			reconcileLoop.Stop()
			if backupMgr != nil {
				backupMgr.Stop()
			}
		},
	}
	cliSvcs := &cli.Services{
		Config:         cfg,
		Project:        projectSvc,
		Service:        serviceSvc,
		Route:          routeSvc,
		EndpointRepo:   endpointRepo,
		ManagedDomain:  mdSvc,
		EndpointSvc:    endpointSvc,
		Exposure:       exposureSvc,
		ListenerSvc:    listenerSvc,
		EdgeSvc:        edgeSvc,
		LeaderSvc:      leaderSvc,
		NodeRepo:       nodeRepo,
		StateVer:       stateVer,
		DB:             db,
		Apply:          applySvc,
		Health:         healthSvc,
		Logs:           logSvc,
		Action:         actionSvc,
		Space:          spaceSvc,
		HTTPServices:   httpSvcs,
		PendingState:   pendingState,
		TraceSvc:       traceSvc,
		RelaySvc:       relaySvc,
		SafetySvc:      safetySvc,
		TransparentMgr: transparentMgr,
		Version:        Version,
		BuildTime:      BuildTime,
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
	gen            *interface{}
	transparentMgr *transparent.Manager
	endpointRepo   *endpoint.Repository
	nodeRepo       *node.Repository
}

func (h *desiredStateHook) OnRouteChanged(ctx context.Context, routeID string) error {
	if false {
		return nil
	}
	h.syncTransparentRules()
	return nil
}

func (h *desiredStateHook) OnServiceChanged(ctx context.Context, serviceID string) error {
	if false {
		return nil
	}
	h.syncTransparentRules()
	return nil
}

func (h *desiredStateHook) OnEndpointChanged(ctx context.Context, endpointID string) error {
	if false {
		return nil
	}
	h.syncTransparentRules()
	return nil
}

func (h *desiredStateHook) syncTransparentRules() {
	if h.transparentMgr == nil || h.endpointRepo == nil || h.nodeRepo == nil {
		return
	}

	currentNode, err := h.nodeRepo.FindCurrent()
	if err != nil || currentNode == nil {
		return
	}

	eps, err := h.endpointRepo.FindAllEnabled()
	if err != nil {
		return
	}

	allNodes, err := h.nodeRepo.FindAll()
	if err != nil {
		return
	}
	nodeByID := make(map[string]*node.NodeRecord, len(allNodes))
	for i := range allNodes {
		nodeByID[allNodes[i].NodeID] = &allNodes[i]
	}

	desiredMap := make(map[string]transparent.RedirectRule)
	for _, ep := range eps {
		if ep.NodeID == "" {
			continue
		}

		targetNode := nodeByID[ep.NodeID]
		if targetNode == nil {
			continue
		}

		_, port := ep.HostPort()
		if port == 0 {
			continue
		}

		myNetwork := currentNode.NetworkID
		sameNetwork := myNetwork != "" && targetNode.NetworkID != "" &&
			myNetwork == targetNode.NetworkID

		ips := make(map[string]bool)
		if targetNode.PublicIP != "" {
			ips[targetNode.PublicIP] = true
		}
		if sameNetwork && targetNode.PrivateIP != "" {
			ips[targetNode.PrivateIP] = true
		}
		if targetNode.LocalIP != "" && targetNode.LocalIP != "127.0.0.1" {
			ips[targetNode.LocalIP] = true
		}

		for ip := range ips {
			ruleID := fmt.Sprintf("ep-%s-%s", ep.ID, strings.ReplaceAll(ip, ".", "-"))
			desiredMap[ruleID] = transparent.RedirectRule{
				ID:              ruleID,
				OriginalIP:      ip,
				OriginalPort:    port,
				TargetServiceID: ep.ServiceID,
				TargetNodeID:    ep.NodeID,
				Description:     fmt.Sprintf("%s → %s:%d", ep.ID, ip, port),
			}
		}
	}

	current := h.transparentMgr.ListStatus()
	currentMap := make(map[string]bool)
	for _, s := range current {
		currentMap[s.Rule.ID] = s.Active
	}

	for id := range currentMap {
		if _, ok := desiredMap[id]; !ok {
			h.transparentMgr.StopRedirect(id)
		}
	}

	for _, r := range desiredMap {
		if _, ok := currentMap[r.ID]; !ok {
			if err := h.transparentMgr.StartRedirect(r); err != nil {
				_ = err
			}
		}
	}
}

func generateRandomHex(n int) string {
	return core.GenerateRandomHex(n)
}
