package config

import (
	"os"
	"path/filepath"
)

// EnsureDirs creates all required directories from the config.
func EnsureDirs(cfg *Config) error {
	dirs := []string{
		filepath.Dir(cfg.Proxy.CaddyfilePath),
		cfg.Proxy.BackupDir,
		filepath.Dir(cfg.Store.SQLitePath),
		cfg.Runtime.ConfigDir,
		cfg.Runtime.DataDir,
	}

	for _, dir := range dirs {
		if dir == "" || dir == "." {
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// FindCaddyBinary checks if caddy binary exists in PATH.
func FindCaddyBinary() (string, bool) {
	// Common names to check
	names := []string{"caddy", "caddy.exe"}
	for _, name := range names {
		if path, err := lookPath(name); err == nil {
			return path, true
		}
	}
	return "", false
}

// lookPath is a simple PATH lookup.
func lookPath(file string) (string, error) {
	// Check if file exists as-is (absolute or relative)
	if _, err := os.Stat(file); err == nil {
		return file, nil
	}

	// Search PATH
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			dir = "."
		}
		fullPath := filepath.Join(dir, file)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}
	return "", os.ErrNotExist
}
