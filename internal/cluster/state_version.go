package cluster

import (
	"database/sql"
	"fmt"
	"sync"
)

// StateVersion tracks the global cluster state version.
// Only the leader increments it; nodes sync to it.
type StateVersion struct {
	mu      sync.RWMutex
	db      *sql.DB
	current uint64
}

// NewStateVersion creates a state version tracker.
func NewStateVersion(db *sql.DB) *StateVersion {
	sv := &StateVersion{db: db}
	sv.load()
	return sv
}

// Current returns the current state version.
func (sv *StateVersion) Current() uint64 {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.current
}

// Increment atomically increments the state version (leader only).
func (sv *StateVersion) Increment() (uint64, error) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	sv.current++
	if err := sv.save(); err != nil {
		sv.current--
		return 0, err
	}
	return sv.current, nil
}

// Set atomically sets the state version (used by nodes syncing from leader).
func (sv *StateVersion) Set(version uint64) error {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	if version < sv.current {
		return fmt.Errorf("cannot decrease state_version from %d to %d", sv.current, version)
	}
	sv.current = version
	return sv.save()
}

// IsBehind returns true if local version is behind the given version.
func (sv *StateVersion) IsBehind(leaderVersion uint64) bool {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.current < leaderVersion
}

func (sv *StateVersion) load() {
	var version uint64
	err := sv.db.QueryRow(`SELECT COALESCE(MAX(value), 0) FROM cluster_state WHERE key = 'state_version'`).Scan(&version)
	if err == nil {
		sv.current = version
	}
}

func (sv *StateVersion) save() error {
	_, err := sv.db.Exec(
		`INSERT OR REPLACE INTO cluster_state (key, value, updated_at) VALUES ('state_version', ?, datetime('now'))`,
		sv.current,
	)
	return err
}
