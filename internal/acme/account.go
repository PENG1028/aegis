// Package acme provides ACME certificate lifecycle management via the lego library.
// It replaces the external certbot CLI dependency with an embedded ACME client.
//
// Account key persistence: /var/lib/aegis/acme/account.key (ECDSA P-256, 0600)
// The account key is generated once on first use and reused for all subsequent
// operations (obtain, renew). This avoids hitting LE rate limits for new accounts.
package acme

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// LoadOrCreateAccountKey loads an existing ACME account key from disk,
// or generates a new ECDSA P-256 key if none exists.
// The key is stored with 0600 permissions.
func LoadOrCreateAccountKey(dataDir string) (*ecdsa.PrivateKey, error) {
	keyDir := filepath.Join(dataDir, "acme")
	keyPath := filepath.Join(keyDir, "account.key")

	if key, err := loadAccountKey(keyPath); err == nil {
		return key, nil
	}

	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, fmt.Errorf("create acme dir: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate account key: %w", err)
	}

	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal account key: %w", err)
	}

	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: der,
	})

	if err := os.WriteFile(keyPath, pemBlock, 0600); err != nil {
		return nil, fmt.Errorf("write account key: %w", err)
	}

	return key, nil
}

func loadAccountKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in account key")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}
