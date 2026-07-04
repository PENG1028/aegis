// Package e2e contains end-to-end integration tests.
//
// Scenario E: Database Backup + Recovery
// Tests BackupManager lifecycle: periodic backup, BackupNow, backup file validity,
// cleanup of old backups, and recovery from a corrupted database.
package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"aegis/internal/core"
	"aegis/internal/store"
)

// TestDBBackup_Recovery verifies database backup creation, cleanup, and recovery.
func TestDBBackup_Recovery(t *testing.T) {
	// Step 1: Create a test database with some data
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "aegis-test.db")
	backupDir := filepath.Join(tmpDir, "backups")

	sqlDB, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer sqlDB.Close() // safety net: ensure DB is closed on any exit path

	if err := store.Initialize(sqlDB); err != nil {
		t.Fatalf("initialize schema: %v", err)
	}

	// Insert test data: several services, routes, and endpoints
	st := store.New(sqlDB)
	defer st.Close() // ensure DB is closed before temp dir cleanup on any exit path

	// Create service records
	serviceCount := 5
	for i := 0; i < serviceCount; i++ {
		svcID := core.NewID("svc")
		_, err := st.DB.Exec(
			`INSERT INTO services (id, project_id, name, kind, env, status, owner_type, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
			svcID, "proj-test", fmt.Sprintf("service-%d", i), "http", "dev", "active", "admin",
		)
		if err != nil {
			t.Fatalf("insert service %d: %v", i, err)
		}
	}

	// Create route records
	for i := 0; i < 3; i++ {
		rtID := core.NewID("rt")
		_, err := st.DB.Exec(
			`INSERT INTO routes (id, domain, path_prefix, service_id, status, owner_type, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
			rtID, fmt.Sprintf("route%d.example.com", i), "/", fmt.Sprintf("svc-%d", i), "active", "admin",
		)
		if err != nil {
			t.Fatalf("insert route %d: %v", i, err)
		}
	}

	// Create endpoint records
	for i := 0; i < 4; i++ {
		epID := core.NewID("ep")
		_, err := st.DB.Exec(
			`INSERT INTO endpoints (id, service_id, type, address, enabled, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
			epID, fmt.Sprintf("svc-%d", i%serviceCount), "local",
			fmt.Sprintf("127.0.0.1:%d", 8080+i), true,
		)
		if err != nil {
			t.Fatalf("insert endpoint %d: %v", i, err)
		}
	}

	// Verify data was inserted
	var svcCount int
	if err := st.DB.QueryRow("SELECT COUNT(*) FROM services").Scan(&svcCount); err != nil {
		t.Fatalf("count services: %v", err)
	}
	if svcCount != serviceCount {
		t.Fatalf("expected %d services, got %d", serviceCount, svcCount)
	}
	t.Logf("inserted test data: %d services, 3 routes, 4 endpoints", svcCount)

	// Step 2: Create BackupManager with interval=1h, keep=3
	bm := store.NewBackupManager(st.DB, dbPath, backupDir, 1, 3)
	if bm == nil {
		t.Fatal("NewBackupManager returned nil — check parameters")
	}

	// Step 3: Call BackupNow() to create a backup
	backup1, err := bm.BackupNow()
	if err != nil {
		t.Fatalf("BackupNow #1: %v", err)
	}
	if backup1 == "" {
		t.Fatal("backup path should not be empty")
	}
	t.Logf("backup #1 created: %s", backup1)

	// Step 4: Verify the backup file exists and is a valid SQLite DB
	if _, err := os.Stat(backup1); os.IsNotExist(err) {
		t.Fatalf("backup file does not exist: %s", backup1)
	}

	// Try opening the backup as a SQLite DB to verify it's valid
	backupDB, err := store.OpenSQLite(backup1)
	if err != nil {
		t.Fatalf("open backup as sqlite: %v", err)
	}

	// Verify data is intact in the backup
	var backupSvcCount int
	if err := backupDB.QueryRow("SELECT COUNT(*) FROM services").Scan(&backupSvcCount); err != nil {
		backupDB.Close()
		t.Fatalf("query backup services: %v", err)
	}
	backupDB.Close()

	if backupSvcCount != serviceCount {
		t.Errorf("backup should contain %d services, got %d", serviceCount, backupSvcCount)
	} else {
		t.Logf("backup verified: contains %d services (valid SQLite database)", backupSvcCount)
	}

	// Step 5: Create more backups via BackupNow() until we have 5
	// Sleep at least 1 second between backups to ensure unique timestamp filenames
	// (VACUUM INTO fails if the output file already exists)
	for i := 2; i <= 5; i++ {
		time.Sleep(1100 * time.Millisecond) // ensure unique second-precision timestamps
		backupPath, err := bm.BackupNow()
		if err != nil {
			t.Fatalf("BackupNow #%d: %v", i, err)
		}
		t.Logf("backup #%d created: %s", i, backupPath)
	}

	// Step 6: Verify cleanup keeps only the most recent 3 backups
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}

	var backupFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "aegis-") && strings.HasSuffix(e.Name(), ".db") {
			backupFiles = append(backupFiles, e.Name())
		}
	}

	t.Logf("backup files in directory (%d total):", len(backupFiles))
	sort.Strings(backupFiles)
	for _, bf := range backupFiles {
		t.Logf("  %s", bf)
	}

	if len(backupFiles) != 3 {
		t.Errorf("expected 3 backups after cleanup (keep=3), got %d", len(backupFiles))
	} else {
		t.Log("cleanup verified: only 3 most recent backups retained")
	}

	// Verify old backups were removed (the first 2 should be gone)
	for _, bf := range backupFiles {
		fullPath := filepath.Join(backupDir, bf)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("retained backup file missing: %s", bf)
		}
	}

	// Step 7: Close the original DB and simulate corruption
	// Close both the store and the underlying sql.DB to release Windows file handles
	_ = st.Close()
	_ = sqlDB.Close()

	// Save the latest backup path before deleting
	latestBackup := filepath.Join(backupDir, backupFiles[len(backupFiles)-1])

	// Delete the original database (simulate corruption)
	if err := os.Remove(dbPath); err != nil {
		t.Fatalf("remove original db: %v", err)
	}
	t.Log("original database deleted (simulated corruption)")

	// Verify original is gone
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatal("original db should be deleted")
	}

	// Step 8: Copy the latest backup to the original location
	backupData, err := os.ReadFile(latestBackup)
	if err != nil {
		t.Fatalf("read latest backup: %v", err)
	}

	if err := os.WriteFile(dbPath, backupData, 0644); err != nil {
		t.Fatalf("write restored db: %v", err)
	}
	t.Logf("restored database from backup: %s -> %s", latestBackup, dbPath)

	// Step 9: Open the restored database and verify data integrity
	restoredDB, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open restored database: %v", err)
	}
	defer restoredDB.Close()

	// Verify service count
	var restoredSvcCount int
	if err := restoredDB.QueryRow("SELECT COUNT(*) FROM services").Scan(&restoredSvcCount); err != nil {
		t.Fatalf("count services in restored db: %v", err)
	}
	if restoredSvcCount != serviceCount {
		t.Errorf("restored database: expected %d services, got %d", serviceCount, restoredSvcCount)
	} else {
		t.Logf("data integrity verified: %d services intact after restore", restoredSvcCount)
	}

	// Verify route count
	var routeCount int
	if err := restoredDB.QueryRow("SELECT COUNT(*) FROM routes").Scan(&routeCount); err != nil {
		t.Fatalf("count routes in restored db: %v", err)
	}
	if routeCount != 3 {
		t.Errorf("restored database: expected 3 routes, got %d", routeCount)
	}

	// Verify endpoint count
	var epCount int
	if err := restoredDB.QueryRow("SELECT COUNT(*) FROM endpoints").Scan(&epCount); err != nil {
		t.Fatalf("count endpoints in restored db: %v", err)
	}
	if epCount != 4 {
		t.Errorf("restored database: expected 4 endpoints, got %d", epCount)
	}

	// Verify the database can be opened and queried — check specific records
	rows, err := restoredDB.Query("SELECT id, name FROM services ORDER BY name")
	if err != nil {
		t.Fatalf("query restored services: %v", err)
	}
	defer rows.Close()

	var svcNames []string
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("scan service row: %v", err)
		}
		svcNames = append(svcNames, name)
		t.Logf("  restored service: id=%s name=%s", id, name)
	}
	if len(svcNames) != serviceCount {
		t.Errorf("expected %d service rows, got %d", serviceCount, len(svcNames))
	}

	t.Log("database backup + recovery test completed successfully")
}

// TestBackupManager_Disabled verifies that NewBackupManager returns nil when disabled.
func TestBackupManager_Disabled(t *testing.T) {
	// Test with various disabled configurations
	tests := []struct {
		name       string
		backupDir  string
		intervalHrs int
		keepCount  int
	}{
		{"empty backup dir", "", 1, 3},
		{"zero keep count", "/tmp", 1, 0},
		{"negative keep count", "/tmp", 1, -1},
		{"zero interval", "/tmp", 0, 3},
		{"negative interval", "/tmp", -1, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := store.NewBackupManager(nil, "", tt.backupDir, tt.intervalHrs, tt.keepCount)
			if bm != nil {
				t.Error("expected nil BackupManager when disabled")
			}
		})
	}
}

// TestBackupManager_StartStop verifies the Start/Stop lifecycle.
func TestBackupManager_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	backupDir := filepath.Join(tmpDir, "backups")

	sqlDB, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer sqlDB.Close()

	if err := store.Initialize(sqlDB); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	// Create BackupManager
	bm := store.NewBackupManager(sqlDB, dbPath, backupDir, 1, 3)
	if bm == nil {
		t.Fatal("backup manager should not be nil")
	}

	// Start the background loop
	bm.Start()
	t.Log("backup manager started")

	// Give it a moment to run the initial backup
	time.Sleep(200 * time.Millisecond)

	// Stop gracefully
	bm.Stop()
	t.Log("backup manager stopped")

	// Verify at least the initial backup was created
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}

	backupCount := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "aegis-") && strings.HasSuffix(e.Name(), ".db") {
			backupCount++
		}
	}
	if backupCount < 1 {
		t.Error("expected at least 1 backup after Start")
	} else {
		t.Logf("background backup created: %d backup(s) found", backupCount)
	}
}

// TestBackupManager_NilSafe verifies that calling methods on a nil BackupManager is safe.
func TestBackupManager_NilSafe(t *testing.T) {
	var bm *store.BackupManager

	// Start should not panic
	bm.Start()
	t.Log("Start on nil: safe")

	// Stop should not panic
	bm.Stop()
	t.Log("Stop on nil: safe")

	// BackupNow should return an error
	_, err := bm.BackupNow()
	if err == nil {
		t.Error("expected error from BackupNow on nil")
	} else {
		t.Logf("BackupNow on nil returns error: %v", err)
	}
}
