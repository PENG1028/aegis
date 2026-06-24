package maintenance

import (
	"database/sql"
	"fmt"
	"time"
)

// CleanupStats holds the results of a cleanup run.
type CleanupStats struct {
	OrphanEdgeRules int `json:"orphan_edge_rules"`
	StaleNodes      int `json:"stale_nodes"`
	OldSessions     int `json:"old_sessions"`
	TotalRemoved    int `json:"total_removed"`
}

// RunCleanup performs a full cleanup of stale/orphan data.
func RunCleanup(db *sql.DB) (*CleanupStats, error) {
	stats := &CleanupStats{}

	// 1. Cleanup orphan edge rules (managed_by=http_route but route no longer exists)
	result, err := db.Exec(
		`DELETE FROM edge_mux_rules WHERE managed_by = 'http_route'
		 AND source_ref NOT IN (SELECT id FROM routes WHERE status = 'active')`)
	if err != nil {
		return stats, fmt.Errorf("cleanup edge rules: %w", err)
	}
	if n, _ := result.RowsAffected(); n > 0 {
		stats.OrphanEdgeRules = int(n)
		stats.TotalRemoved += int(n)
	}

	// 2. Cleanup stale nodes (not seen in 7 days)
	result, err = db.Exec(
		`DELETE FROM nodes WHERE is_current = 0 AND last_seen < ?`,
		time.Now().Add(-7*24*time.Hour).Format(time.RFC3339))
	if err != nil {
		return stats, fmt.Errorf("cleanup stale nodes: %w", err)
	}
	if n, _ := result.RowsAffected(); n > 0 {
		stats.StaleNodes = int(n)
		stats.TotalRemoved += int(n)
	}

	// 3. Cleanup old upgrade sessions (>30 days)
	result, err = db.Exec(
		`DELETE FROM upgrade_sessions WHERE start_time < ? AND status != 'running'`,
		time.Now().Add(-30*24*time.Hour).Format(time.RFC3339))
	if err != nil {
		return stats, fmt.Errorf("cleanup old sessions: %w", err)
	}
	if n, _ := result.RowsAffected(); n > 0 {
		stats.OldSessions = int(n)
		stats.TotalRemoved += int(n)
	}

	return stats, nil
}
