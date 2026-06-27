// v1.8C-8 Real Two-node Gateway Runner.
// Cross-compiled for linux/amd64 and deployed to Server A.
// Starts the local gateway with a routing table pointing to Server B relay.
package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"os/signal"
	"syscall"
	"time"

	"aegis/internal/localgateway"
	"aegis/internal/noderuntime"
)

func main() {
	serverB := os.Getenv("SERVER_B")
	if serverB == "" {
		serverB = "<SERVER_B_IP>"
	}
	domain := os.Getenv("TEST_DOMAIN")
	if domain == "" {
		domain = "api-b.example.com"
	}
	routeID := os.Getenv("ROUTE_ID")
	if routeID == "" {
		routeID = "route-api-b"
	}
	gwLinkID := os.Getenv("GATEWAY_LINK_ID")
	if gwLinkID == "" {
		gwLinkID = "gl-a-b"
	}
	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "18080"
	}

	fmt.Println("============================================================")
	fmt.Println("  v1.8C-8 Real Two-node Gateway Runner")
	fmt.Println("============================================================")
	fmt.Printf("  Server B:      %s\n", serverB)
	fmt.Printf("  Domain:        %s\n", domain)
	fmt.Printf("  Route ID:      %s\n", routeID)
	fmt.Printf("  GatewayLink:   %s\n", gwLinkID)
	fmt.Printf("  Gateway Port:  %s\n", port)
	fmt.Println("============================================================")
	fmt.Println()

	// 1. Secret provider (InMemory for now, token set via env)
	secretProvider := noderuntime.NewInMemorySecretProvider()
	token := os.Getenv("GATEWAY_TOKEN")
	if token != "" {
		secretProvider.AddSecret(gwLinkID, token)
		fmt.Println("[1] Secret provider initialized with token from env")
	} else {
		fmt.Println("[1] Secret provider initialized (NO TOKEN SET)")
	}

	// 2. Routing table resolver
	relayURL := fmt.Sprintf("http://%s:80", serverB)
	resolver := &simpleResolver{
		decisions: map[string]*localgateway.RoutingDecision{
			domain: {
				Domain:  domain,
				Status:  "available",
				RouteID: routeID,
				SelectedCandidate: &localgateway.CandidateEntry{
					Mode:          "private_gateway",
					GatewayURL:    relayURL,
					GatewayLinkID: gwLinkID,
					Priority:      1,
				},
			},
		},
	}
	fmt.Printf("[2] Routing table: %s -> %s/__aegis/relay\n", domain, relayURL)

	// 3. Local gateway
	portInt := 18080
	if p, err := strconv.Atoi(port); err == nil && p > 0 {
		portInt = p
	}
	cfg := localgateway.DefaultConfig()
	cfg.Port = portInt
	cfg.NodeID = "node-a"

	fmt.Print("[3] Starting local HTTP gateway ... ")
	gateway := localgateway.NewGateway(cfg, resolver, secretProvider)
	gateway.WireGatewayStatus()

	if err := gateway.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}
	defer gateway.Stop()
	time.Sleep(100 * time.Millisecond)

	gwAddr := fmt.Sprintf("http://127.0.0.1:%d", portInt)
	fmt.Printf("OK (%s)\n", gwAddr)
	fmt.Println()

	// 4. Print status
	gs := gateway.Status()
	fmt.Printf("  Gateway Status: %s\n", gs.Status)
	fmt.Printf("  Enabled: %v\n", gs.Enabled)
	fmt.Printf("  Port: %d\n", gs.Port)

	// 5. Health check
	resp, err := http.Get(gwAddr + "/__aegis/local/health")
	if err == nil {
		fmt.Printf("  Health: HTTP %d\n", resp.StatusCode)
		resp.Body.Close()
	} else {
		fmt.Printf("  Health: %v\n", err)
	}

	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("  READY")
	fmt.Println("============================================================")
	fmt.Println()
	fmt.Println("  Run curl commands from another terminal:")
	fmt.Println()
	fmt.Printf("    curl -i -H \"Host: %s\" %s/health\n", domain, gwAddr)
	fmt.Println()
	fmt.Println("  Press Ctrl+C to stop")
	fmt.Println("============================================================")

	// Wait for signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println()
	fmt.Println("Shutting down...")
}

type simpleResolver struct {
	decisions map[string]*localgateway.RoutingDecision
}

func (r *simpleResolver) Resolve(domain string) *localgateway.RoutingDecision {
	if d, ok := r.decisions[domain]; ok {
		return d
	}
	return &localgateway.RoutingDecision{
		Domain: domain,
		Status: "unavailable",
	}
}

func (r *simpleResolver) GetRoutingTableStatus() localgateway.RoutingTableInfo {
	return localgateway.RoutingTableInfo{
		Loaded:  true,
		Entries: len(r.decisions),
	}
}

// Ensure simpleResolver implements RoutingTableStatusProvider
var _ localgateway.RoutingTableStatusProvider = (*simpleResolver)(nil)
