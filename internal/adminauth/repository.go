package adminauth

import (
	"database/sql"
	"fmt"
	"time"
)

// AdminUserRepository provides database access for admin users.
type AdminUserRepository struct {
	DB *sql.DB
}

// NewAdminUserRepository creates a new admin user repository.
func NewAdminUserRepository(db *sql.DB) *AdminUserRepository {
	return &AdminUserRepository{DB: db}
}

// Create inserts a new admin user.
func (r *AdminUserRepository) Create(u *AdminUser) error {
	_, err := r.DB.Exec(
		`INSERT INTO admin_users (id, username, password_hash, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.PasswordHash,
		u.CreatedAt.Format(time.RFC3339),
		u.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert admin_user: %w", err)
	}
	return nil
}

// FindByUsername returns an admin user by username.
func (r *AdminUserRepository) FindByUsername(username string) (*AdminUser, error) {
	var u AdminUser
	var createdAt, updatedAt string
	err := r.DB.QueryRow(
		`SELECT id, username, password_hash, created_at, updated_at
		 FROM admin_users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query admin_user by username: %w", err)
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	u.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &u, nil
}

// FindByID returns an admin user by ID.
func (r *AdminUserRepository) FindByID(id string) (*AdminUser, error) {
	var u AdminUser
	var createdAt, updatedAt string
	err := r.DB.QueryRow(
		`SELECT id, username, password_hash, created_at, updated_at
		 FROM admin_users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query admin_user by id: %w", err)
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	u.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &u, nil
}

// Count returns the number of admin users.
func (r *AdminUserRepository) Count() (int, error) {
	var count int
	err := r.DB.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&count)
	return count, err
}

// AdminSessionRepository provides database access for admin sessions.
type AdminSessionRepository struct {
	DB *sql.DB
}

// NewAdminSessionRepository creates a new admin session repository.
func NewAdminSessionRepository(db *sql.DB) *AdminSessionRepository {
	return &AdminSessionRepository{DB: db}
}

// Create inserts a new session.
func (r *AdminSessionRepository) Create(s *AdminSession) error {
	revokedAt := ""
	if !s.RevokedAt.IsZero() {
		revokedAt = s.RevokedAt.Format(time.RFC3339)
	}
	_, err := r.DB.Exec(
		`INSERT INTO admin_sessions (id, user_id, session_hash, expires_at, revoked_at, created_at, last_seen_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.UserID, s.SessionHash,
		s.ExpiresAt.Format(time.RFC3339),
		revokedAt,
		s.CreatedAt.Format(time.RFC3339),
		s.LastSeenAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert admin_session: %w", err)
	}
	return nil
}

// FindBySessionHash returns a session by its hash.
func (r *AdminSessionRepository) FindBySessionHash(hash string) (*AdminSession, error) {
	var s AdminSession
	var expiresAt, revokedAt, createdAt, lastSeenAt string
	err := r.DB.QueryRow(
		`SELECT id, user_id, session_hash, expires_at, revoked_at, created_at, last_seen_at
		 FROM admin_sessions WHERE session_hash = ?`, hash,
	).Scan(&s.ID, &s.UserID, &s.SessionHash, &expiresAt, &revokedAt, &createdAt, &lastSeenAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query admin_session by hash: %w", err)
	}
	s.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	if revokedAt != "" {
		s.RevokedAt, _ = time.Parse(time.RFC3339, revokedAt)
	}
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.LastSeenAt, _ = time.Parse(time.RFC3339, lastSeenAt)
	return &s, nil
}

// Revoke marks a session as revoked.
func (r *AdminSessionRepository) Revoke(sessionHash string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.DB.Exec(
		`UPDATE admin_sessions SET revoked_at = ? WHERE session_hash = ?`,
		now, sessionHash,
	)
	return err
}

// TouchLastSeen updates the last_seen_at timestamp.
func (r *AdminSessionRepository) TouchLastSeen(sessionHash string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.DB.Exec(
		`UPDATE admin_sessions SET last_seen_at = ? WHERE session_hash = ?`,
		now, sessionHash,
	)
	return err
}

// DeleteExpired removes expired sessions.
func (r *AdminSessionRepository) DeleteExpired() (int64, error) {
	result, err := r.DB.Exec(
		`DELETE FROM admin_sessions WHERE expires_at < ? AND revoked_at = ''`,
		time.Now().Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
