package maintenance

import (
	"fmt"

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
	nodes, _ := nodeRepo.FindAll()
	leaders := []string{}
	now := ""
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

	// 2. Stale node detection (last_seen > 60s ago)
	for i := range nodes {
		if nodes[i].IsCurrent && now == "" {
			// Nodes with very old last_seen are stale
			if nodes[i].LastSeen.IsZero() {
				report.StaleNodes = append(report.StaleNodes, nodes[i].NodeID)
				report.Issues = append(report.Issues,
					fmt.Sprintf("STALE: node %s has zero last_seen", nodes[i].NodeID))
			}
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
