package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

const (
	// NonceSize is the GCM nonce size (12 bytes for AES-256-GCM).
	NonceSize = 12
)

// Encrypt encrypts plaintext using AES-256-GCM with the master key.
// Returns base64-encoded ciphertext and base64-encoded nonce separately.
func Encrypt(mk *MasterKey, plaintext string) (encryptedB64, nonceB64 string, err error) {
	if mk == nil {
		return "", "", fmt.Errorf("master key is nil")
	}
	if plaintext == "" {
		return "", "", fmt.Errorf("plaintext is empty")
	}

	block, err := aes.NewCipher(mk.Bytes())
	if err != nil {
		return "", "", fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	encryptedB64 = base64.StdEncoding.EncodeToString(ciphertext)
	nonceB64 = base64.StdEncoding.EncodeToString(nonce)
	return encryptedB64, nonceB64, nil
}

// Decrypt decrypts a base64-encoded ciphertext with the given nonce using AES-256-GCM.
// Returns the original plaintext.
func Decrypt(mk *MasterKey, encryptedB64, nonceB64 string) (string, error) {
	if mk == nil {
		return "", fmt.Errorf("master key is nil")
	}
	if encryptedB64 == "" || nonceB64 == "" {
		return "", fmt.Errorf("encrypted data and nonce are required")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedB64)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}

	if len(nonce) != NonceSize {
		return "", fmt.Errorf("invalid nonce length: got %d, expected %d", len(nonce), NonceSize)
	}

	block, err := aes.NewCipher(mk.Bytes())
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptFailed, err)
	}

	return string(plaintext), nil
}

// EncryptToStorage encrypts a secret and returns the combined storage format.
// Format: "base64_nonce:base64_ciphertext"
// This is a convenience wrapper for DB storage as a single column.
func EncryptToStorage(mk *MasterKey, plaintext string) (string, error) {
	enc, nonce, err := Encrypt(mk, plaintext)
	if err != nil {
		return "", err
	}
	return nonce + ":" + enc, nil
}

// DecryptFromStorage decrypts a secret stored in combined format.
// Format: "base64_nonce:base64_ciphertext"
func DecryptFromStorage(mk *MasterKey, storage string) (string, error) {
	if storage == "" {
		return "", fmt.Errorf("storage is empty")
	}

	// Find the colon separator (base64 alphabet does not include ':')
	sepIdx := -1
	for i, c := range storage {
		if c == ':' {
			sepIdx = i
			break
		}
	}
	if sepIdx < 0 || sepIdx+1 >= len(storage) {
		return "", fmt.Errorf("invalid storage format: missing separator")
	}

	nonceB64 := storage[:sepIdx]
	encB64 := storage[sepIdx+1:]
	return Decrypt(mk, encB64, nonceB64)
}

// KeyHash returns a hex-encoded SHA-256 hash of the key material.
// Used for identifying which key was used, without exposing the key itself.
// This is NOT used for encryption — only for tracking which key version is active.
func (mk *MasterKey) KeyHash() string {
	if mk == nil {
		return ""
	}
	sum := sha256.Sum256(mk.key[:])
	return fmt.Sprintf("%x", sum[:8]) // first 8 bytes only, for identification
}
