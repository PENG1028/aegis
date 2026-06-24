package cluster

import (
	"fmt"
	"sync"
	"time"
)

// ACKTracker tracks node acknowledgements for state pushes.
type ACKTracker struct {
	mu           sync.Mutex
	nodeCount    int
	acks         map[string]bool // nodeID → acked
	pushVersion  uint64
	pushTime     time.Time
}

// NewACKTracker creates an ACK tracker for a state push.
func NewACKTracker(nodeCount int, version uint64) *ACKTracker {
	return &ACKTracker{
		nodeCount:   nodeCount,
		acks:        make(map[string]bool),
		pushVersion: version,
		pushTime:    time.Now(),
	}
}

// RecordACK records an acknowledgement from a node.
func (t *ACKTracker) RecordACK(nodeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.acks[nodeID] = true
}

// HasQuorum returns true if enough nodes have ACKed.
// Single node → self-ack (always true).
// Multi node → majority ACK required.
func (t *ACKTracker) HasQuorum() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.nodeCount <= 1 {
		return true // single node always has quorum
	}

	acked := 0
	for range t.acks {
		acked++
	}
	majority := (t.nodeCount / 2) + 1
	return acked >= majority
}

// ACKCount returns the number of ACKs received.
func (t *ACKTracker) ACKCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.acks)
}

// WaitForQuorum waits for quorum or timeout.
func (t *ACKTracker) WaitForQuorum(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if t.HasQuorum() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("ACK timeout: got %d/%d ACKs for version %d", t.ACKCount(), t.nodeCount, t.pushVersion)
}

// PushResult summarizes the push outcome.
type PushResult struct {
	Version    uint64 `json:"version"`
	ACKsNeeded int    `json:"acks_needed"`
	ACKsGot    int    `json:"acks_got"`
	Success    bool   `json:"success"`
	Message    string `json:"message"`
}
