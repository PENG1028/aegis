package handlers

import (
	"net/http"
	"time"

	"aegis/internal/cluster"
	"aegis/internal/consistency"
)

// ClusterHealth aggregates health across all nodes for 5-10 node diagnosability.
type ClusterHealthResponse struct {
	NodeCount       int                       `json:"node_count"`
	LeaderNodeID    string                    `json:"leader_node_id"`
	SplitBrain      bool                      `json:"split_brain"`
	Nodes           []ClusterNodeHealth       `json:"nodes"`
	OverallHealthy  bool                      `json:"overall_healthy"`
	Issues          []string                  `json:"issues,omitempty"`
}

type ClusterNodeHealth struct {
	NodeID       string `json:"node_id"`
	Hostname     string `json:"hostname"`
	Role         string `json:"role"`
	Status       string `json:"status"`
	IsLeader     bool   `json:"is_leader"`
	SyncStatus   string `json:"sync_status"`
	DesiredRev   int    `json:"desired_revision"`
	AppliedRev   int    `json:"applied_revision"`
	HeartbeatAge string `json:"heartbeat_age,omitempty"`
}

// ClusterHealth returns aggregated health across all nodes.
// Single endpoint to assess cluster state — critical for 5-10 node diagnosis.
func (h *Handlers) ClusterHealth(w http.ResponseWriter, r *http.Request) {
	resp := ClusterHealthResponse{
		Nodes:   []ClusterNodeHealth{},
		Issues:  []string{},
	}

	// 1. Gather all nodes
	nodes, err := h.NodeRepo.FindAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list nodes: "+err.Error())
		return
	}
	resp.NodeCount = len(nodes)

	// 2. Leader check
	leaderSvc := cluster.NewLeaderService(h.NodeRepo)
	leader, _ := leaderSvc.GetLeader()
	if leader != nil {
		resp.LeaderNodeID = leader.NodeID
	}

	// 3. Split-brain detection
	if err := leaderSvc.EnsureSingleLeader(); err != nil {
		resp.SplitBrain = true
		resp.Issues = append(resp.Issues, "SPLIT_BRAIN: "+err.Error())
	}

	// 4. Per-node health
	now := time.Now()
	overallHealthy := true
	for _, n := range nodes {
		nh := ClusterNodeHealth{
			NodeID:   n.NodeID,
			Hostname: n.Hostname,
			Role:     n.Role,
			Status:   n.Status,
			IsLeader: n.IsLeader,
		}

		// Sync status
		if h.NodeStateSvc != nil {
			sync, err := h.NodeStateSvc.GetSyncStatus(n.NodeID)
			if err == nil && sync != nil {
				nh.SyncStatus = sync.Status
				nh.DesiredRev = sync.DesiredRevision
				nh.AppliedRev = sync.AppliedRevision
				if sync.Status != "in_sync" && sync.Status != "no_desired_state" {
					overallHealthy = false
					if sync.LastError != "" {
						resp.Issues = append(resp.Issues,
							n.NodeID+": "+sync.Status+" — "+sync.LastError)
					}
				}
			}
		}

		// Offline detection: heartbeat > 60s
		if !n.LastHeartbeatAt.IsZero() && now.Sub(n.LastHeartbeatAt) > 60*time.Second {
			nh.Status = "offline"
			nh.HeartbeatAge = now.Sub(n.LastHeartbeatAt).Round(time.Second).String()
			overallHealthy = false
			resp.Issues = append(resp.Issues, n.NodeID+": node is offline (heartbeat "+nh.HeartbeatAge+" ago)")
		}

		resp.Nodes = append(resp.Nodes, nh)
	}

	// 5. Overall consistency
	conflictReport := consistency.Check(h.NodeRepo, leaderSvc)
	if conflictReport.HasIssues() {
		overallHealthy = false
		resp.Issues = append(resp.Issues, conflictReport.Summary())
	}

	resp.OverallHealthy = overallHealthy

	// If no issues, provide a clean summary
	if len(resp.Issues) == 0 {
		resp.Issues = nil // JSON null instead of []
	}

	writeJSON(w, http.StatusOK, resp)
}
