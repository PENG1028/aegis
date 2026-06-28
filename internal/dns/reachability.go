package dns

import (
	"context"
	"log"
	"net"
	"sync"
	"time"
)

// Reachability checks which peer nodes are reachable via private IP.
type Reachability struct {
	mu         sync.RWMutex
	reachable  map[string]bool // node_id → reachable via private IP
	nodeRepo   NodeRepo
	currentID  string

	interval time.Duration
	timeout  time.Duration
}

// NewReachability creates a peer reachability checker.
func NewReachability(nodeRepo NodeRepo, interval time.Duration) *Reachability {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Reachability{
		reachable: make(map[string]bool),
		nodeRepo:  nodeRepo,
		interval:  interval,
		timeout:   3 * time.Second,
	}
}

// Start begins periodic reachability checks. Blocks until ctx is cancelled.
func (rc *Reachability) Start(ctx context.Context) {
	ticker := time.NewTicker(rc.interval)
	defer ticker.Stop()

	// Run once immediately
	rc.checkAll()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rc.checkAll()
		}
	}
}

// IsReachable returns true if the given node's private IP was recently reachable.
func (rc *Reachability) IsReachable(nodeID string) bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.reachable[nodeID]
}

// SetCurrentNodeID sets the current node ID (to skip self-checks).
func (rc *Reachability) SetCurrentNodeID(id string) {
	rc.currentID = id
}

// checkAll probes all peer nodes' private IPs.
func (rc *Reachability) checkAll() {
	nodes, err := rc.nodeRepo.FindAll()
	if err != nil {
		log.Printf("[dns] reachability: find all nodes failed: %v", err)
		return
	}

	var wg sync.WaitGroup
	results := make(map[string]bool, len(nodes))

	for i := range nodes {
		n := &nodes[i]

		// Skip self
		if rc.currentID != "" && n.NodeID == rc.currentID {
			results[n.NodeID] = true // self is always reachable
			continue
		}

		// Skip nodes with no private IP
		if n.PrivateIP == "" {
			results[n.NodeID] = false
			continue
		}

		wg.Add(1)
		go func(nodeID, privateIP string) {
			defer wg.Done()
			reachable := rc.probe(privateIP)
			results[nodeID] = reachable
			if !reachable {
				log.Printf("[dns] reachability: node %s (%s) not reachable via private IP", nodeID, privateIP)
			}
		}(n.NodeID, n.PrivateIP)
	}

	wg.Wait()

	rc.mu.Lock()
	rc.reachable = results
	rc.mu.Unlock()
}

// probe tries a TCP connection to port 80 of the given IP.
func (rc *Reachability) probe(ip string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "80"), rc.timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
