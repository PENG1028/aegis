package maintenance

import (
	"fmt"
	"time"

	"aegis/internal/cluster"
	"aegis/internal/node"
)

// ConflictReport holds detected consistency issues.
type ConflictReport struct {
	MultipleLeaders  bool     `json:"multiple_leaders"`
	Leaders          []string `json:"leaders,omitempty"`
	StaleNodes       []string `json:"stale_nodes,omitempty"`
	NoLeader         bool     `json:"no_leader"`
	Issues           []string `json:"issues"`
}

// Check runs all conflict detection checks.
func Check(nodeRepo *node.Repository, leaderSvc *cluster.LeaderService) *ConflictReport {
	report := &ConflictReport{}

	// 1. Multiple leader detection
	nodes, err := nodeRepo.FindAll()
	if err != nil {
		report.Issues = append(report.Issues, "node list unavailable: "+err.Error())
		return report
	}
	leaders := []string{}
	for i := range nodes {
		if nodes[i].IsLeader {
			leaders = append(leaders, nodes[i].NodeID)
		}
	}
	if len(leaders) > 1 {
		report.MultipleLeaders = true
		report.Leaders = leaders
		report.Issues = append(report.Issues,
			fmt.Sprintf("SPLIT_BRAIN: %d leaders found: %v", len(leaders), leaders))
	}
	if len(leaders) == 0 {
		report.NoLeader = true
		report.Issues = append(report.Issues, "no leader elected")
	}

	// 2. Stale node detection: any node (except the current one) whose
	// last_seen is older than 60s is stale.
	now := time.Now()
	for i := range nodes {
		n := &nodes[i]
		if n.IsCurrent {
			continue
		}
		if n.LastSeen.IsZero() || now.Sub(n.LastSeen) > 60*time.Second {
			report.StaleNodes = append(report.StaleNodes, n.NodeID)
			report.Issues = append(report.Issues,
				fmt.Sprintf("STALE: node %s last seen %s", n.NodeID, n.LastSeen.Format(time.RFC3339)))
		}
	}

	return report
}

// HasIssues returns true if any conflicts were detected.
func (r *ConflictReport) HasIssues() bool {
	return len(r.Issues) > 0
}

// Summary returns a one-line summary.
func (r *ConflictReport) Summary() string {
	if !r.HasIssues() {
		return "cluster consistent"
	}
	return fmt.Sprintf("%d issue(s): %v", len(r.Issues), r.Issues)
}
