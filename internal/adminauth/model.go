package adminauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"aegis/internal/id"
)

// AdminUser represents the single administrator account.
type AdminUser struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AdminSession represents an active admin login session.
type AdminSession struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	SessionHash string    `json:"-"`
	ExpiresAt   time.Time `json:"expires_at"`
	RevokedAt   time.Time `json:"revoked_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

// IsExpired returns true if the session has expired.
func (s *AdminSession) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsRevoked returns true if the session has been revoked.
func (s *AdminSession) IsRevoked() bool {
	return !s.RevokedAt.IsZero()
}

// hashPassword creates a salted SHA-256 hash of a password.
// Format: $sha256$<hex-salt>$<hex-hash>
func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(password))
	hash := hex.EncodeToString(h.Sum(nil))
	saltHex := hex.EncodeToString(salt)
	return fmt.Sprintf("$sha256$%s$%s", saltHex, hash), nil
}

// checkPassword verifies a password against a salted SHA-256 hash.
func checkPasswordHash(password, storedHash string) bool {
	parts := splitHash(storedHash)
	if len(parts) != 3 || parts[0] != "$sha256$" {
		return false
	}
	salt, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(password))
	expected := hex.EncodeToString(h.Sum(nil))
	return expected == parts[2]
}

// splitHash splits a hash string formatted as $sha256$<salt>$<hash>
func splitHash(hash string) []string {
	// Format: $sha256$<salt>$<hash>
	if len(hash) < 9 || hash[:8] != "$sha256$" {
		return nil
	}
	rest := hash[8:]
	var parts []string
	parts = append(parts, "$sha256$")
	// Find first $ after position 8
	for i := 0; i < len(rest); i++ {
		if rest[i] == '$' {
			parts = append(parts, rest[:i], rest[i+1:])
			return parts
		}
	}
	return nil
}

// NewAdminUser creates a new admin user with a hashed password.
func NewAdminUser(username, password string) (*AdminUser, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &AdminUser{
		ID:           id.New("admin"),
		Username:     username,
		PasswordHash: hash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// CheckPassword verifies a password against the stored hash.
func (u *AdminUser) CheckPassword(password string) bool {
	return checkPasswordHash(password, u.PasswordHash)
}

// NewAdminSession creates a new session for a user.
func NewAdminSession(userID string, sessionToken string, ttl time.Duration) *AdminSession {
	now := time.Now()
	return &AdminSession{
		ID:          id.New("asess"),
		UserID:      userID,
		SessionHash: hashToken(sessionToken),
		ExpiresAt:   now.Add(ttl),
		CreatedAt:   now,
		LastSeenAt:  now,
	}
}

// hashToken creates a SHA-256 hash of a token for storage.
func hashToken(token string) string {
	return hashTokenString(token)
}

// hashTokenString computes SHA-256 of a string (shared by model and middleware).
func hashTokenString(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
