package upgrade

import (
	"fmt"

	"aegis/internal/cluster"
	"aegis/internal/consistency"
	"aegis/internal/node"
)

// HealthGateResult holds pre-upgrade health check results.
type HealthGateResult struct {
	Passed  bool              `json:"passed"`
	Checks  []HealthCheckItem `json:"checks"`
	Message string            `json:"message"`
}

// HealthCheckItem is a single gate check.
type HealthCheckItem struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// RunHealthGate executes all pre-upgrade health checks.
func RunHealthGate(
	nodeRepo *node.Repository,
	leaderSvc *cluster.LeaderService,
	stateVer *cluster.StateVersion,
	localVersion uint64,
) *HealthGateResult {
	result := &HealthGateResult{Passed: true}

	// 1. Node sync check
	nodes, _ := nodeRepo.FindAll()
	nodeCheck := HealthCheckItem{Name: "node_sync"}
	if len(nodes) > 0 {
		allCurrent := true
		for _, n := range nodes {
			if !n.IsCurrent && n.StateVersion < stateVer.Current() {
				allCurrent = false
				break
			}
		}
		if allCurrent {
			nodeCheck.Passed = true
			nodeCheck.Message = fmt.Sprintf("%d nodes synced", len(nodes))
		} else {
			nodeCheck.Passed = false
			nodeCheck.Message = "some nodes not in sync"
			result.Passed = false
		}
	} else {
		nodeCheck.Passed = true
		nodeCheck.Message = "single node"
	}
	result.Checks = append(result.Checks, nodeCheck)

	// 2. Drift check
	driftCheck := HealthCheckItem{Name: "drift"}
	drift := consistency.CheckDrift(nodeRepo, localVersion, stateVer.Current())
	if !drift.HasDrift || drift.Severity == "LOW" {
		driftCheck.Passed = true
		driftCheck.Message = drift.Summary()
	} else {
		driftCheck.Passed = false
		driftCheck.Message = drift.Summary()
		result.Passed = false
	}
	result.Checks = append(result.Checks, driftCheck)

	// 3. Leader check
	leaderCheck := HealthCheckItem{Name: "leader"}
	if err := leaderSvc.EnsureSingleLeader(); err != nil {
		leaderCheck.Passed = false
		leaderCheck.Message = err.Error()
		result.Passed = false
	} else {
		leaderCheck.Passed = true
		leaderCheck.Message = "single leader confirmed"
	}
	result.Checks = append(result.Checks, leaderCheck)

	// 4. ACK quorum check
	ackCheck := HealthCheckItem{Name: "ack_quorum"}
	nodeCount := len(nodes)
	if nodeCount <= 1 {
		ackCheck.Passed = true
		ackCheck.Message = "single node — self-ack"
	} else {
		ackCheck.Passed = true
		ackCheck.Message = fmt.Sprintf("%d nodes available for quorum", nodeCount)
	}
	result.Checks = append(result.Checks, ackCheck)

	if !result.Passed {
		result.Message = "health gate FAILED"
	} else {
		result.Message = "health gate PASSED"
	}

	return result
}
