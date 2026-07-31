package scripts

// Structural tests for scripts/update.sh. These tests guard against regressions
// in the A2 (stop-before-backup) and A3 (real rollback) fixes by asserting the
// script's source-level invariants.
//
// They are not behavioural — they do not run update.sh. The behavioural check
// happens during deployment in tests/e2e/ and during real Server A updates.
// What these tests do catch is a future PR reverting the script by accident,
// since any revert will fail these invariants.

import (
	"os"
	"strings"
	"testing"
)

func readUpdateScript(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../scripts/update.sh")
	if err != nil {
		t.Fatalf("read scripts/update.sh: %v", err)
	}
	return string(data)
}

// TestUpdateScript_StopsBeforeBackingUpDatabase locks in A2.
//
// The original bug: update.sh did `cp aegis.db` while Aegis was still running.
// With WAL mode (internal/store/sqlite.go:25), uncheckpointed transactions in
// the -wal sidecar were silently dropped from the backup, so the rollback
// target was inconsistent with the live DB.
//
// The fix: stop the service, then copy. The order must hold under any future
// refactor of the script.
func TestUpdateScript_StopsBeforeBackingUpDatabase(t *testing.T) {
	script := readUpdateScript(t)

	// Find the byte offset of "Stopping Aegis gracefully" — this marks the
	// new "stop service first" step.
	stopIdx := strings.Index(script, "Stopping Aegis gracefully")
	if stopIdx < 0 {
		t.Fatal("did not find 'Stopping Aegis gracefully' — A2 fix may have been reverted")
	}

	// Find the byte offset of the first DB cp. The backup line uses a quoted
	// template string, so we look for the unique prefix that only appears in
	// the backup step.
	dbCpIdx := strings.Index(script, "sudo cp ${DATA_DIR}/aegis.db ${BACKUP_DIR}/aegis.${TIMESTAMP}.db")
	if dbCpIdx < 0 {
		t.Fatal("did not find the DB backup cp line — backup step may have been removed")
	}

	if stopIdx >= dbCpIdx {
		t.Errorf("A2 regression: DB backup (offset %d) appears BEFORE service stop (offset %d). "+
			"With WAL mode this leaves the rollback target inconsistent with the live DB.",
			dbCpIdx, stopIdx)
	}
}

// TestUpdateScript_RollbackActuallyCopies locks in A3.
//
// The original bug: when health check failed, update.sh printed `echo`
// commands for the operator to run manually. The service stayed on the broken
// binary until a human intervened.
//
// The fix: a do_rollback function that does the actual `sudo cp ... &&
// sudo systemctl start aegis` and re-checks /api/healthz.
func TestUpdateScript_RollbackActuallyCopies(t *testing.T) {
	script := readUpdateScript(t)

	if !strings.Contains(script, "do_rollback()") {
		t.Fatal("A3 regression: do_rollback() function is missing — update.sh only echoes the rollback commands")
	}

	// Locate do_rollback body — between `do_rollback() {` and the matching `}`.
	start := strings.Index(script, "do_rollback() {")
	if start < 0 {
		t.Fatal("A3 regression: do_rollback() function header not found")
	}
	// Walk to the end of the function by finding the closing brace.
	// The body is short and uses 2-space indentation; closing `}` is unindented.
	body := script[start:]
	endRel := strings.Index(body, "\n}\n")
	if endRel < 0 {
		// tolerate trailing newline
		endRel = strings.Index(body, "\n}")
	}
	if endRel < 0 {
		t.Fatal("could not locate end of do_rollback function")
	}
	body = body[:endRel]

	// The body must actually invoke the cp — not just echo it. The real
	// invocation uses double-quoted SSH command (${SSH} "sudo cp ..."), while
	// the fake "just echo the command" form puts it inside single quotes
	// inside an echo statement. Distinguish the two by matching the surrounding
	// double quotes.
	if !strings.Contains(body, `${SSH} "sudo cp ${BACKUP_DIR}/aegis.${TIMESTAMP} ${BINARY_PATH}`) {
		t.Errorf("A3 regression: do_rollback does not actually invoke 'sudo cp ${BACKUP_DIR}/aegis.${TIMESTAMP} ${BINARY_PATH}'. "+
			"It must restore the binary, not just print the command.")
	}

	// And it must restart the service.
	if !strings.Contains(body, "sudo systemctl start aegis") {
		t.Errorf("A3 regression: do_rollback does not restart aegis after restoring the binary.")
	}
}

// TestUpdateScript_RollbackIsCalledFromHealthFail guards the wiring between
// the health check failure paths and the rollback function.
//
// Without this, do_rollback can exist as dead code while the old `echo` paths
// remain in the failure branches.
func TestUpdateScript_RollbackIsCalledFromHealthFail(t *testing.T) {
	script := readUpdateScript(t)

	// After do_rollback definition, the two health-fail branches (systemd
	// status and API retries) must invoke do_rollback instead of just
	// printing the command.
	rollBackIdx := strings.Index(script, "do_rollback() {")
	if rollBackIdx < 0 {
		t.Fatal("do_rollback() header not found")
	}
	tail := script[rollBackIdx:]

	// Count how many distinct places invoke do_rollback. Expect at least 2:
	// the systemd-status-fail path and the API-healthz-fail path.
	count := strings.Count(tail, "do_rollback")
	// subtract the declaration itself (one occurrence in the function header)
	if count-1 < 2 {
		t.Errorf("expected do_rollback to be called from at least 2 failure paths (systemd, API), "+
			"found %d call site(s) after declaration", count-1)
	}

	// And the script must NOT contain the old "Rollback command:" echo path —
	// any leftover echo-style rollback printout means we still have a path
	// that pretends to roll back without doing anything.
	if strings.Contains(tail, "  Rollback command:") {
		t.Errorf("A3 regression: script still contains the old 'Rollback command:' echo block. "+
			"All rollback paths must invoke do_rollback(), not just print commands.")
	}
}
