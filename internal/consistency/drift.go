package consistency

import (
	"aegis/internal/node"
	"fmt"
)

// DriftReport holds drift detection results.
type DriftReport struct {
	HasDrift       bool   `json:"has_drift"`
	Severity       string `json:"severity"` // LOW / MEDIUM / HIGH
	LocalVersion   uint64 `json:"local_version"`
	LeaderVersion  uint64 `json:"leader_version"`
	Details        []string `json:"details"`
}

// CheckDrift compares local state version against the leader version.
func CheckDrift(nodeRepo *node.Repository, localVersion, leaderVersion uint64) *DriftReport {
	report := &DriftReport{
		LocalVersion:  localVersion,
		LeaderVersion: leaderVersion,
	}

	if localVersion == leaderVersion {
		return report // no drift
	}

	report.HasDrift = true
	diff := leaderVersion - localVersion

	switch {
	case diff == 1:
		report.Severity = "LOW"
		report.Details = append(report.Details, "minor version lag (1 version behind)")
	case diff <= 5:
		report.Severity = "MEDIUM"
		report.Details = append(report.Details, fmt.Sprintf("moderate version lag (%d versions behind)", diff))
	default:
		report.Severity = "HIGH"
		report.Details = append(report.Details, fmt.Sprintf("CRITICAL: %d versions behind leader", diff))
	}

	// Check for additional drift indicators
	nodes, _ := nodeRepo.FindAll()
	staleCount := 0
	for i := range nodes {
		if nodes[i].StateVersion < leaderVersion {
			staleCount++
		}
	}
	if staleCount > 1 {
		report.Details = append(report.Details, fmt.Sprintf("%d nodes behind leader", staleCount))
	}

	return report
}

// Summary returns a one-line drift summary.
func (r *DriftReport) Summary() string {
	if !r.HasDrift {
		return "no drift"
	}
	return fmt.Sprintf("DRIFT_DETECTED [%s]: local v%d < leader v%d", r.Severity, r.LocalVersion, r.LeaderVersion)
}
