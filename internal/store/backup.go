package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// BackupManager periodically backs up the SQLite database using VACUUM INTO.
// Runs on a configurable interval with automatic cleanup of old backups.
type BackupManager struct {
	db        *sql.DB
	dbPath    string
	backupDir string
	interval  time.Duration
	keepCount int

	mu     sync.Mutex
	stopCh chan struct{}
	doneCh chan struct{}
}

// NewBackupManager creates a new backup manager.
// If backupDir is empty or keepCount <= 0, backups are disabled.
func NewBackupManager(db *sql.DB, dbPath, backupDir string, intervalHrs, keepCount int) *BackupManager {
	if backupDir == "" || keepCount <= 0 || intervalHrs <= 0 {
		return nil
	}
	return &BackupManager{
		db:        db,
		dbPath:    dbPath,
		backupDir: backupDir,
		interval:  time.Duration(intervalHrs) * time.Hour,
		keepCount: keepCount,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// Start begins periodic backups in a background goroutine.
// Runs one backup immediately, then on each interval tick.
func (b *BackupManager) Start() {
	if b == nil {
		return
	}
	go b.loop()
}

// Stop gracefully stops the backup loop.
func (b *BackupManager) Stop() {
	if b == nil {
		return
	}
	close(b.stopCh)
	<-b.doneCh
}

func (b *BackupManager) loop() {
	defer close(b.doneCh)

	// Run first backup immediately
	b.runBackup()

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.runBackup()
		}
	}
}

func (b *BackupManager) runBackup() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Ensure backup directory exists
	if err := os.MkdirAll(b.backupDir, 0700); err != nil {
		log.Printf("[backup] create dir %s: %v", b.backupDir, err)
		return
	}

	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(b.backupDir, fmt.Sprintf("aegis-%s.db", timestamp))

	// Sanitize path for SQLite string literal: double any single quotes.
	// VACUUM INTO does not support parameterized queries via Go drivers,
	// so we must escape the path manually.
	safePath := strings.ReplaceAll(backupFile, "'", "''")

	// VACUUM INTO creates a clean, defragmented copy
	_, err := b.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", safePath))
	if err != nil {
		log.Printf("[backup] VACUUM INTO %s failed: %v", safePath, err)
		return
	}

	log.Printf("[backup] database backed up to %s", backupFile)

	// Clean up old backups
	b.cleanup()
}

func (b *BackupManager) cleanup() {
	entries, err := os.ReadDir(b.backupDir)
	if err != nil {
		return
	}

	var backups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "aegis-") && strings.HasSuffix(e.Name(), ".db") {
			backups = append(backups, e.Name())
		}
	}

	if len(backups) <= b.keepCount {
		return
	}

	// Sort by name (timestamp) ascending, delete oldest first
	sort.Strings(backups)
	toDelete := len(backups) - b.keepCount

	for i := 0; i < toDelete; i++ {
		path := filepath.Join(b.backupDir, backups[i])
		if err := os.Remove(path); err != nil {
			log.Printf("[backup] cleanup %s: %v", path, err)
		} else {
			log.Printf("[backup] removed old backup %s", backups[i])
		}
	}
}

// BackupNow performs an immediate backup and returns the file path.
// Useful for pre-upgrade or manual backup triggers.
func (b *BackupManager) BackupNow() (string, error) {
	if b == nil {
		return "", fmt.Errorf("backup manager not configured")
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := os.MkdirAll(b.backupDir, 0700); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(b.backupDir, fmt.Sprintf("aegis-%s.db", timestamp))

	// Sanitize path for SQLite string literal (same rationale as runBackup).
	safePath := strings.ReplaceAll(backupFile, "'", "''")

	_, err := b.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", safePath))
	if err != nil {
		return "", fmt.Errorf("VACUUM INTO: %w", err)
	}

	log.Printf("[backup] manual backup created: %s", backupFile)
	b.cleanup()
	return backupFile, nil
}
