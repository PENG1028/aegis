package store

import (
	"path/filepath"
	"testing"
	"time"
)

// TestBackupManager_StopIsIdempotent covers the production pattern from
// cmd/aegis/main.go where Stop() is invoked once via OnShutdown (signal
// handler) and again via defer (LIFO at function return). The first
// close(stopCh) succeeds; without sync.Once protection the second call
// panics on the closed channel.
func TestBackupManager_StopIsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	db, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mgr := NewBackupManager(db, dbPath, tmp, 1, 5)
	if mgr == nil {
		t.Fatal("NewBackupManager returned nil")
	}
	mgr.Start()

	// Let the loop run briefly so it has actually entered the select.
	time.Sleep(50 * time.Millisecond)

	// First Stop — should close stopCh and block on <-doneCh.
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		mgr.Stop()
	}()
	select {
	case <-firstDone:
	case <-time.After(3 * time.Second):
		t.Fatal("first Stop timed out")
	}

	// Subsequent Stops must NOT panic and must NOT block.
	for i := 2; i <= 4; i++ {
		done := make(chan struct{})
		var recovered any
		go func() {
			defer close(done)
			defer func() { recovered = recover() }()
			mgr.Stop()
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatalf("Stop call %d timed out", i)
		}
		if recovered != nil {
			t.Errorf("Stop call %d panicked: %v", i, recovered)
		}
	}
}

// TestBackupManager_DeferChainSurvivesDuplicateStop mirrors the defer order
// in cmd/aegis/main.go:
//
//	defer db.Close()         // line 127 — registered FIRST  → runs LAST (LIFO)
//	defer backupMgr.Stop()   // line 133 — registered SECOND → runs FIRST
//
// OnShutdown calls Stop() explicitly. When the function returns, the deferred
// Stop runs second. Without sync.Once, the second close(stopCh) panics; the
// panic propagates out of main() with a non-zero exit, which systemd treats as
// a crash. This test asserts that no panic reaches the test runner.
//
// Note: Go defers do continue executing after a panic, so db.Close() still
// runs even when a defer panics. The user-visible problem is the panic itself
// (exit code, log noise, systemd Restart=on-failure), not the loss of
// db.Close().
func TestBackupManager_DeferChainSurvivesDuplicateStop(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	db, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	dbClosed := false

	// Mimic defer db.Close() (registered first, runs last in LIFO).
	defer func() {
		if !dbClosed {
			t.Error("db.Close() did not run — backupMgr.Stop() panicked in the defer chain")
		}
	}()
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("db close error: %v", err)
		}
		dbClosed = true
	}()

	mgr := NewBackupManager(db, dbPath, tmp, 1, 5)
	if mgr != nil {
		mgr.Start()
		// Mimic defer backupMgr.Stop() (registered second, runs first).
		defer mgr.Stop()
	}

	// Let the loop tick at least once.
	time.Sleep(50 * time.Millisecond)

	// First Stop — mimics the OnShutdown callback that fires from the
	// signal handler before cobra returns and the defer chain unwinds.
	mgr.Stop()

	// Function returns here. Defers run LIFO:
	//   1. mgr.Stop() (this defer) — MUST NOT panic; with sync.Once it returns
	//      immediately because the loop has already exited.
	//   2. db close check — verifies the panic did not abort the chain.
	//   3. db.Close() — runs to completion.
}
