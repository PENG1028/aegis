package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"aegis/internal/acme"
	"aegis/internal/action"
	"aegis/internal/adminauth"
	"aegis/internal/apply"
	"aegis/internal/certstore"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/core"
	"aegis/internal/credential"
	"aegis/internal/distnode"
	"aegis/internal/dns"
	"aegis/internal/edgemux"
	"aegis/internal/egress"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/flowbridge"
	"aegis/internal/gateway"
	"aegis/internal/health"
	"aegis/internal/hostdep/provider"
	"aegis/internal/httpapi"
	"aegis/internal/httpapi/handlers"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/node"
	"aegis/internal/project"
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
	"aegis/internal/tcp"
	"aegis/internal/tlslifecycle"
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
	if handled := handleLightweightCommand(os.Args[1:]); handled {
		return
	}

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
	currentNode, err := nodeSvc.RegisterCurrent()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: node registration failed: %v\n", err)
	}
	if currentNode != nil {
		go func(nodeID string) {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				if err := nodeRepo.TouchLiveness(nodeID, node.StatusOnline, "", time.Now()); err != nil {
					fmt.Fprintf(os.Stderr, "warning: current node heartbeat %s: %v\n", nodeID, err)
				}
				<-ticker.C
			}
		}(currentNode.NodeID)
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
	certDir := filepath.Join(cfg.Runtime.DataDir, "certs")
	certStoreSvc := certstore.NewService(certRepo, certDir)
	// WHY: Caddy runs as an unprivileged service user but explicit TLS assets
	// remain owned by Aegis. Group-read access avoids world-readable keys and
	// also repairs root-only files created by older releases.
	if err := certStoreSvc.ConfigureConsumerGroup("caddy"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: certificate assets remain Aegis-only; explicit Caddy TLS bindings will fail: %v\n", err)
	}

	// ── ACME Client (v1.9C) — embedded lego, replaces certbot ──
	acmeClient, err := acme.NewClient(certStoreSvc, cfg.Proxy.Email, cfg.Proxy.ACMEServer, cfg.Runtime.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  acme client: %v (ACME disabled)\n", err)
		acmeClient = nil
	}

	// ── FlowBridge instances (v1.9C-2) — managed data-plane entities ──
	flowbridgeRepo := flowbridge.NewRepository(db)
	flowbridgeSvc := flowbridge.NewService(flowbridgeRepo, logSvc)

	// --- v1.8L: Topology Planner (dimension 2) + Workflow orchestrator ---
	_, controlPort := safety.SplitHostPort(cfg.Server.Addr) // aegis API port, exposed cross-node via the ingress edge
	topoPlanner := topology.NewPlanner(templates.Default(), topology.Dependencies{
		RouteRepo:        routeRepo,
		ServiceRepo:      serviceRepo,
		EndpointResolver: endpointResolver,
		GwLinkRepo:       gwLinkRepo,
		SafetySvc:        safetySvc,
		MasterKey:        masterKey,
		CertStore:        certStoreSvc,
		FlowBridgeRepo:   flowbridgeRepo,
		ControlPort:      controlPort,
	})
	workflow := apply.NewWorkflow(topoPlanner, provRegistry, applyRepo, cfg, logSvc)
	tlsLifecycleSvc := tlslifecycle.New(routeSvc, certStoreSvc, provRegistry)
	tlsObservers := []certstore.AutomaticTLSObserver{
		certstore.NewCaddyTLSObserver(cfg.Proxy.CaddyDataDir),
	}

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
	// WHY: desired state is committed before gateway Apply. Failed applies stay
	// visible as pending and are retried through the canonical apply lock.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if !pendingState.Status().Pending {
				continue
			}
			if _, err := applySvc.TryApply(context.Background()); err != nil && !strings.Contains(err.Error(), "APPLY_LOCKED") {
				fmt.Fprintf(os.Stderr, "pending-apply: retry failed: %v\n", err)
			}
		}
	}()
	// Periodic flowbridge instance health checks. Enabled instances are probed
	// against their control-plane /health every 30s; results are persisted for
	// the UI and route-binding decisions.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Fprintf(os.Stderr, "flowbridge-health: panic in check loop: %v\n", r)
					}
				}()
				instances, err := flowbridgeSvc.List(context.Background())
				if err != nil {
					fmt.Fprintf(os.Stderr, "flowbridge-health: list instances failed: %v\n", err)
					return
				}
				for _, inst := range instances {
					if !inst.Enabled {
						continue
					}
					if _, err := flowbridgeSvc.Check(context.Background(), inst.ID); err != nil {
						fmt.Fprintf(os.Stderr, "flowbridge-health: check %s failed: %v\n", inst.ID, err)
					}
				}
			}()
		}
	}()
	if acmeClient != nil {
		renewalChecker := certstore.NewCertRenewalChecker(certStoreSvc, acmeClient)
		renewalChecker.SetRenewalCoordinator(applySvc)
		go func() {
			// run performs one renewal scan. A panic here must not kill the
			// 12h renewal loop permanently (renewals would silently stop).
			run := func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Fprintf(os.Stderr, "cert-renewal: panic in renewal loop: %v\n", r)
					}
				}()
				// Bound the whole scan: a stuck ACME request must not block
				// renewals forever.
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				defer cancel()
				expiring, err := renewalChecker.Check(ctx, 30)
				if err != nil {
					fmt.Fprintf(os.Stderr, "cert-renewal: scan failed: %v\n", err)
					return
				}
				hasLocalACME := false
				for _, cert := range expiring {
					if cert.Source == certstore.SourceLocalACME && cert.CanRenew {
						hasLocalACME = true
						break
					}
				}
				if !hasLocalACME {
					return
				}
				// Applying first ensures the Caddy HTTP-01 proxy route exists.
				if _, err := applySvc.ForceApply(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "cert-renewal: prepare challenge route: %v\n", err)
					return
				}
				for _, result := range renewalChecker.RenewExpiring(ctx, 30) {
					if result.PendingApply {
						_ = pendingState.MarkPending("certificate renewed but provider reload is pending: " + result.CertID)
					}
					fmt.Fprintf(os.Stderr, "cert-renewal: %s: %s\n", result.CertID, result.Message)
				}
			}
			initial := time.NewTimer(time.Minute)
			defer initial.Stop()
			<-initial.C
			run()
			ticker := time.NewTicker(12 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				run()
			}
		}()
	}
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
	actionSvc.SetCertificateStore(certStoreSvc)
	actionSvc.SetFlowBridgeService(flowbridgeSvc)

	routingPolicyRepo := routingpolicy.NewRepository(db)
	routingPolicySvc := routingpolicy.NewService(routingPolicyRepo)
	routingTableSvc := routingtable.NewService()

	transparentMgr := transparent.NewManager()
	transparentMgr.SetTunnelSecret(cfg.DistNode.Secret)
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
		endpointRepo:   endpointRepo,
		nodeRepo:       nodeRepo,
	}
	workflow.SetPlanAppliedHook(func(plan *topology.TopologyPlan) {
		if plan == nil || plan.ForwardTarget == nil {
			return
		}
		transparentMgr.SetForwardTarget(plan.ForwardTarget.Host, plan.ForwardTarget.Port)
	})
	routeSvc.SetMutationHook(dsHook)
	serviceSvc.SetMutationHook(dsHook)
	endpointSvc.SetMutationHook(dsHook)
	dsHook.syncTransparentRules()

	token.SetAuditLogger(logSvc)
	adminauth.SetAuditLogger(logSvc)

	// --- Service Auth (v1.9A) ---
	serviceAuthRepo := serviceauth.NewRepository(db)
	serviceAuthSvc, err := serviceauth.NewService(serviceauth.Dependencies{
		Repo: serviceAuthRepo,

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
		ProvReg:         provRegistry,
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
	if cfg.DistNode.Enabled != nil && *cfg.DistNode.Enabled {
		distCfg := distnode.Config{
			ID:           cfg.DistNode.ID,
			Name:         cfg.DistNode.Name,
			Addr:         cfg.DistNode.Addr,
			Secret:       cfg.DistNode.Secret,
			Scheme:       "http",
			HealthPath:   "/api/healthz",
			CallPath:     "/api/distnode/v1/call",
			NodeIDHeader: "X-Aegis-Node-ID",
			DefaultRole:  "agent",
		}
		for _, p := range cfg.DistNode.Peers {
			distCfg.Peers = append(distCfg.Peers, distnode.PeerConfig{ID: p.ID, Addr: p.Addr, Secret: p.Secret})
		}
		dn = distnode.New(distCfg)
		handlers.RegisterDistNodeHandlers(dn)
		fmt.Fprintf(os.Stderr, "info: distnode enabled - id=%s addr=%s peers=%d\n", dn.ID, distCfg.Addr, len(distCfg.Peers))

		// Pull the stable service catalog from alive peers. Membership-only peers are
		// already merged for UI display; persisted nodes should use stable node_id values.
		dn.Membership.OnEvent(func(evt distnode.PeerEvent) {
			switch evt.Type {
			case distnode.EventPeerAlive:
				if err := nodeRepo.TouchLiveness(evt.Peer.Info.ID, node.StatusOnline, "", time.Now()); err != nil {
					fmt.Fprintf(os.Stderr, "warning: update peer liveness %s: %v\n", evt.Peer.Info.ID, err)
				}
				go func(peerID string) {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if err := syncClusterCatalogFromPeer(ctx, dn, peerID, nodeRepo, serviceRepo, endpointRepo); err != nil {
						fmt.Fprintf(os.Stderr, "warning: sync catalog from peer %s failed: %v\n", peerID, err)
						return
					}
					fmt.Fprintf(os.Stderr, "info: synced catalog from peer %s\n", peerID)
					dsHook.syncTransparentRules()
				}(evt.Peer.Info.ID)
			case distnode.EventPeerDead:
				if err := nodeRepo.TouchLiveness(evt.Peer.Info.ID, node.StatusOffline, "", time.Now()); err != nil {
					fmt.Fprintf(os.Stderr, "warning: update peer liveness %s: %v\n", evt.Peer.Info.ID, err)
				}
			}
		})

		go dn.Start(context.Background())
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				for _, peer := range dn.Membership.AlivePeers() {
					if peer == nil || peer.Info.ID == "" {
						continue
					}
					if err := nodeRepo.TouchLiveness(peer.Info.ID, node.StatusOnline, "", time.Now()); err != nil {
						fmt.Fprintf(os.Stderr, "warning: peer heartbeat %s: %v\n", peer.Info.ID, err)
					}
				}
				<-ticker.C
			}
		}()
		go syncClusterCatalogFromAlivePeers(context.Background(), dn, nodeRepo, serviceRepo, endpointRepo, dsHook, 5, 3*time.Second)
	} else {
		fmt.Fprintf(os.Stderr, "info: distnode disabled\n")
	}
	httpSvcs := &httpapi.Services{
		DB:              db,
		Config:          cfg,
		Project:         projectSvc,
		Service:         serviceSvc,
		EndpointRepo:    endpointRepo,
		EndpointSvc:     endpointSvc,
		Route:           routeSvc,
		ManagedDomain:   mdSvc,
		Exposure:        exposureSvc,
		Apply:           applySvc,
		Workflow:        workflow,
		Health:          healthSvc,
		Logs:            logSvc,
		Auth:            authMiddleware,
		Action:          actionSvc,
		Space:           spaceSvc,
		AdminAuth:       adminAuthSvc,
		EdgeSvc:         edgeSvc,
		ListenerSvc:     listenerSvc,
		NodeRepo:        nodeRepo,
		NodeSvc:         nodeSvc,
		GatewayInvRepo:  gatewayInvRepo,
		GatewayInvSvc:   gatewayInvSvc,
		TopologySvc:     topologySvc,
		PolicySvc:       routingPolicySvc,
		RoutingTableSvc: routingTableSvc,
		PendingState:    pendingState,
		TraceSvc:        traceSvc,
		GatewayLinkSvc:  gwLinkSvc,
		SafetySvc:       safetySvc,
		RelaySvc:        relaySvc,
		DNSMgmt:         dnsMgmt,
		TransparentMgr:  transparentMgr,
		CredentialSvc:   credSvc,
		ServiceAuthSvc:  serviceAuthSvc,
		EgressSvc:       egressSvc,
		CertStore:       certStoreSvc,
		FlowBridgeSvc:   flowbridgeSvc,
		TLSLifecycle:    tlsLifecycleSvc,
		TLSObservers:    tlsObservers,
		ACMEClient:      acmeClient,
		ProvReg:         provRegistry,
		Version:         Version,
		BuildTime:       BuildTime,
		DistNode:        dn,
		OnShutdown: func() {
			fmt.Fprintf(os.Stderr, "stopping subsystems...\n")
			tcpMgr.Shutdown()
			udpMgr.Shutdown()
			transparentMgr.Shutdown()
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

func handleLightweightCommand(args []string) bool {
	filtered := stripConfigArgs(args)
	if len(filtered) == 0 {
		return false
	}
	if len(filtered) == 1 && filtered[0] == "version" {
		fmt.Printf("Aegis %s (built %s)\n", Version, BuildTime)
		return true
	}
	if isHelpRequest(filtered) {
		executeLightweightHelp(filtered)
		return true
	}
	return false
}

func isHelpRequest(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if args[0] == "help" {
		return true
	}
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "help" {
			return true
		}
	}
	return false
}

func executeLightweightHelp(args []string) {
	rootCmd := cli.NewRootCommand(&cli.Services{
		Config:       config.DefaultConfig(),
		HTTPServices: &httpapi.Services{},
		Version:      Version,
		BuildTime:    BuildTime,
	})
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func stripConfigArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--config" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "--config=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func transparentEdgeAddr(n *node.NodeRecord) string {
	if n == nil || n.PublicIP == "" {
		return ""
	}
	if host, port, err := net.SplitHostPort(n.PublicIP); err == nil && host != "" && port != "" {
		return net.JoinHostPort(host, port)
	}
	return net.JoinHostPort(n.PublicIP, "80")
}

type clusterCatalog struct {
	Nodes     []node.NodeRecord   `json:"nodes"`
	Services  []service.Service   `json:"services"`
	Endpoints []endpoint.Endpoint `json:"endpoints"`
}

func syncClusterCatalogFromPeer(ctx context.Context, dn *distnode.DistNode, peerID string, nodeRepo *node.Repository, serviceRepo *service.Repository, endpointRepo *endpoint.Repository) error {
	if dn == nil || peerID == "" {
		return nil
	}

	var catalog clusterCatalog
	if err := dn.Transport.Call(ctx, peerID, "Aegis.ClusterCatalog", nil, &catalog); err != nil {
		return err
	}

	for i := range catalog.Nodes {
		n := catalog.Nodes[i]
		if n.NodeID == "" || !strings.HasPrefix(n.NodeID, "node_") {
			continue
		}
		n.IsCurrent = false
		n.IsLeader = false
		if err := nodeRepo.UpsertByNodeID(&n); err != nil {
			return fmt.Errorf("upsert node %s: %w", n.NodeID, err)
		}
	}
	for i := range catalog.Services {
		s := catalog.Services[i]
		if err := serviceRepo.Upsert(&s); err != nil {
			return fmt.Errorf("upsert service %s: %w", s.ID, err)
		}
	}
	for i := range catalog.Endpoints {
		ep := catalog.Endpoints[i]
		if ep.ServiceID == "" {
			continue
		}
		if err := endpointRepo.Upsert(&ep); err != nil {
			return fmt.Errorf("upsert endpoint %s: %w", ep.ID, err)
		}
	}
	return nil
}

func syncClusterCatalogFromAlivePeers(ctx context.Context, dn *distnode.DistNode, nodeRepo *node.Repository, serviceRepo *service.Repository, endpointRepo *endpoint.Repository, dsHook *desiredStateHook, attempts int, interval time.Duration) {
	if dn == nil || attempts <= 0 {
		return
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(interval):
			case <-ctx.Done():
				return
			}
		}

		synced := false
		for _, peer := range dn.Membership.AlivePeers() {
			if peer == nil || peer.Info.ID == "" {
				continue
			}
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := syncClusterCatalogFromPeer(callCtx, dn, peer.Info.ID, nodeRepo, serviceRepo, endpointRepo)
			cancel()
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: startup catalog sync from peer %s failed: %v\n", peer.Info.ID, err)
				continue
			}
			fmt.Fprintf(os.Stderr, "info: startup catalog synced from peer %s\n", peer.Info.ID)
			synced = true
		}
		if synced && dsHook != nil {
			dsHook.syncTransparentRules()
		}
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
				TargetEdgeAddr:  transparentEdgeAddr(targetNode),
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
