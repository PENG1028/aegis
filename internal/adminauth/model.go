package adminauth

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"aegis/internal/core"

	"golang.org/x/crypto/bcrypt"
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

// hashPassword creates a bcrypt hash of a password.
// Uses cost factor 12 (~250ms per hash on modern hardware).
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// checkPasswordHash verifies a password against a bcrypt hash.
func checkPasswordHash(password, storedHash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
	return err == nil
}

// NewAdminUser creates a new admin user with a hashed password.
func NewAdminUser(username, password string) (*AdminUser, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &AdminUser{
		ID:           core.NewID("admin"),
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
		ID:          core.NewID("asess"),
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
