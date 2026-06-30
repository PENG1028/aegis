package credential

import "time"

// Credential holds an encrypted connection string identified by alias.
//
// Security:
//   - EncryptedConnString is AES-256-GCM ciphertext (never plaintext in DB)
//   - SecretNonce is the GCM nonce
//   - SecretVersion is incremented on rotation
//   - json:"-" tags prevent accidental API exposure
type Credential struct {
	ID                   string    `json:"id"`
	Alias                string    `json:"alias"`
	EncryptedConnString  string    `json:"-"` // base64_nonce:base64_ciphertext
	SecretVersion        int       `json:"secret_version"`
	SecretCreatedAt      string    `json:"secret_created_at,omitempty"`
	SecretRotatedAt      string    `json:"secret_rotated_at,omitempty"`
	Scheme               string    `json:"scheme"`   // detected from URI: postgres, mysql, etc.
	MaskedURI            string    `json:"masked_uri"` // password masked version for display
	Description          string    `json:"description"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
