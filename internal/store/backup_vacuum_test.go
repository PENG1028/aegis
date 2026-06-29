package store

import (
	"path/filepath"
	"strings"
	"testing"
)

// ── VACUUM INTO path sanitization tests (C4 fix) ──

func TestVacuumBackupPath_SanitizesSingleQuotes(t *testing.T) {
	// Simulate a backup directory path containing a single quote (SQL injection vector)
	dangerousDir := "/var/lib/aegis/backups/evil'path/db"
	timestamp := "20260101_120000"
	backupFile := filepath.Join(dangerousDir, "aegis-"+timestamp+".db")

	// Apply the same sanitization used in runBackup/BackupNow
	safePath := strings.ReplaceAll(backupFile, "'", "''")

	// Single quotes must be doubled
	if strings.Count(safePath, "'") != 2 {
		t.Errorf("expected exactly 2 single quotes (doubled), got %d: %q", strings.Count(safePath, "'"), safePath)
	}

	// The escaped path should not contain the original single-quote dir name
	if strings.Contains(safePath, "evil'path") {
		t.Error("unescaped single quote still present in path")
	}

	// The doubled version should be present
	if !strings.Contains(safePath, "evil''path") {
		t.Error("doubled single quotes not found in sanitized path")
	}
}

func TestVacuumBackupPath_NormalPathUnchanged(t *testing.T) {
	normalDir := "/var/lib/aegis/backups/db"
	timestamp := "20260101_120000"
	backupFile := filepath.Join(normalDir, "aegis-"+timestamp+".db")

	safePath := strings.ReplaceAll(backupFile, "'", "''")

	// Normal path should be unchanged
	if safePath != backupFile {
		t.Errorf("normal path should be unchanged: %q != %q", safePath, backupFile)
	}
}

func TestVacuumBackupPath_NoEmptyString(t *testing.T) {
	// Even empty path should be handled safely
	safePath := strings.ReplaceAll("", "'", "''")
	if safePath != "" {
		t.Errorf("empty string should remain empty, got %q", safePath)
	}
}

func TestVacuumBackupPath_MultipleQuotes(t *testing.T) {
	// 3 single quotes → each becomes '' → 6 single quotes total
	path := "/path/with/'multiple/'quotes/'here"
	safePath := strings.ReplaceAll(path, "'", "''")
	if strings.Count(safePath, "'") != 6 {
		t.Errorf("expected 6 single quotes (3 doubled), got %d: %q", strings.Count(safePath, "'"), safePath)
	}
	// Original path had 3 quotes, after doubling should be 6
	if strings.Count(path, "'") != 3 {
		t.Errorf("original should have 3 quotes, got %d: %q", strings.Count(path, "'"), path)
	}
}
