package secrets

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	// KeySize is the required AES-256 key length.
	KeySize = 32

	// EnvVar is the environment variable for the master key.
	EnvVar = "AEGIS_SECRET_KEY"

	// DefaultKeyPath is the default file path for the master key.
	DefaultKeyPath = "/etc/aegis/secret.key"
)

var (
	ErrKeyNotFound      = errors.New("master key not found: set AEGIS_SECRET_KEY env or provide key file")
	ErrKeyInvalidLength = fmt.Errorf("master key must be %d bytes (%d hex chars)", KeySize, KeySize*2)
	ErrKeyFilePerms     = errors.New("master key file permissions must be 0600 or 0640")
	ErrDecryptFailed    = errors.New("decryption failed — wrong key or corrupted data")
)

// MasterKey is an AES-256 key loaded from environment, file, or config.
// It is immutable once created.
type MasterKey struct {
	key    [KeySize]byte
	source string // description of where the key came from (for error messages)
}

// Bytes returns the key bytes as a slice.
// Callers MUST NOT modify the returned slice.
func (mk *MasterKey) Bytes() []byte {
	return mk.key[:]
}

// Source returns a human-readable description of where key was loaded from.
func (mk *MasterKey) Source() string {
	return mk.source
}

// KeyMaterial returns a copy of the key bytes.
func (mk *MasterKey) KeyMaterial() []byte {
	b := make([]byte, KeySize)
	copy(b, mk.key[:])
	return b
}

// LoadMasterKey loads the master key from the first available source:
//  1. AEGIS_SECRET_KEY environment variable (hex-encoded 32 bytes → 64 hex chars)
//  2. /etc/aegis/secret.key file (hex-encoded 32 bytes, permissions 0600)
//  3. If both are missing and devMode is true, generates a temp key for testing
//
// Returns an error if no key source is found and devMode is false.
func LoadMasterKey(devMode bool) (*MasterKey, error) {
	// 1. Check environment variable
	if envKey := os.Getenv(EnvVar); envKey != "" {
		key, err := parseKey(envKey)
		if err != nil {
			return nil, fmt.Errorf("AEGIS_SECRET_KEY: %w", err)
		}
		mk := &MasterKey{source: fmt.Sprintf("env:%s", EnvVar)}
		copy(mk.key[:], key)
		return mk, nil
	}

	// 2. Check default key file
	if key, err := loadKeyFile(DefaultKeyPath); err == nil {
		mk := &MasterKey{source: DefaultKeyPath}
		copy(mk.key[:], key)
		return mk, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("load key file %s: %w", DefaultKeyPath, err)
	}

	// 3. Dev mode: generate ephemeral key
	if devMode {
		mk, err := generateEphemeralKey("dev-mode-generated")
		if err != nil {
			return nil, fmt.Errorf("generate dev key: %w", err)
		}
		return mk, nil
	}

	return nil, ErrKeyNotFound
}

// LoadMasterKeyFromFile loads the master key from a specific file path.
func LoadMasterKeyFromFile(path string) (*MasterKey, error) {
	key, err := loadKeyFile(path)
	if err != nil {
		return nil, err
	}
	mk := &MasterKey{source: path}
	copy(mk.key[:], key)
	return mk, nil
}

// generateEphemeralKey creates a temporary master key (for dev/test mode only).
func generateEphemeralKey(source string) (*MasterKey, error) {
	mk := &MasterKey{source: source}
	_, err := rand.Read(mk.key[:])
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return mk, nil
}

// loadKeyFile reads a hex-encoded 32-byte key from a file.
// Validates file permissions are 0600 or 0640.
func loadKeyFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Check file permissions
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	perm := info.Mode().Perm()
	if perm != 0600 && perm != 0640 {
		return nil, fmt.Errorf("%w: got %o", ErrKeyFilePerms, perm)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	// Trim whitespace (newlines, spaces)
	clean := make([]byte, 0, len(data))
	for _, b := range data {
		if b != '\n' && b != '\r' && b != ' ' && b != '\t' {
			clean = append(clean, b)
		}
	}

	return parseKey(string(clean))
}

// parseKey parses a hex-encoded key string.
func parseKey(s string) ([]byte, error) {
	expectedLen := KeySize * 2 // 64 hex chars for 32 bytes
	if len(s) != expectedLen {
		return nil, ErrKeyInvalidLength
	}

	key, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode hex: %w", err)
	}
	if len(key) != KeySize {
		return nil, ErrKeyInvalidLength
	}
	return key, nil
}

// GenerateKeyString generates a new random 64-hex-char key string.
// Useful for initial setup: aegis init can output this.
func GenerateKeyString() (string, error) {
	b := make([]byte, KeySize)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ValidateKeyPath checks key file path exists and has correct format.
// Returns (exists, is_valid, error).
func ValidateKeyPath(path string) (bool, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}
	perm := info.Mode().Perm()
	if perm != 0600 && perm != 0640 {
		return true, false, fmt.Errorf("%w: got %o", ErrKeyFilePerms, perm)
	}
	return true, true, nil
}

// DevMasterKey returns a master key for testing.
// This must only be used in test files.
func DevMasterKey() *MasterKey {
	key := make([]byte, KeySize)
	rand.Read(key)
	mk := &MasterKey{source: "test-dev-key"}
	copy(mk.key[:], key)
	return mk
}

// MustDevKey is like DevMasterKey but panics on failure (for test helpers).
func MustDevKey(t interface{ Fatalf(string, ...interface{}) }) *MasterKey {
	mk := DevMasterKey()
	if mk == nil {
		t.Fatalf("failed to create dev master key")
	}
	return mk
}
