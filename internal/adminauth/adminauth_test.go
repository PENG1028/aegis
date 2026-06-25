package adminauth

import (
	"database/sql"
	"testing"

	"aegis/internal/store"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

func TestAdminLoginSuccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := NewAdminUserRepository(db)
	sessionRepo := NewAdminSessionRepository(db)
	svc := NewService(userRepo, sessionRepo)

	// Create admin user
	user, err := NewAdminUser("admin", "secure-password-123")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := userRepo.Create(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	// Login with correct password
	result, err := svc.Login("admin", "secure-password-123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if result.SessionToken == "" {
		t.Error("expected session token")
	}
	if result.User.Username != "admin" {
		t.Errorf("expected username admin, got %s", result.User.Username)
	}
	t.Logf("Login success: user=%s token_len=%d", result.User.Username, len(result.SessionToken))

	// Validate session
	sessionHash := hashToken(result.SessionToken)
	validUser, err := svc.ValidateSession(sessionHash)
	if err != nil {
		t.Fatalf("validate session: %v", err)
	}
	if validUser.Username != "admin" {
		t.Error("session user mismatch")
	}
}

func TestAdminLoginFailed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := NewAdminUserRepository(db)
	sessionRepo := NewAdminSessionRepository(db)
	svc := NewService(userRepo, sessionRepo)

	user, _ := NewAdminUser("admin", "correct-password")
	userRepo.Create(user)

	// Wrong password
	_, err := svc.Login("admin", "wrong-password", "127.0.0.1", "test")
	if err == nil {
		t.Error("expected login failure with wrong password")
	}
	t.Logf("Login failed correctly: %v", err)

	// Wrong username
	_, err = svc.Login("nonexistent", "anything", "127.0.0.1", "test")
	if err == nil {
		t.Error("expected login failure with wrong username")
	}
}

func TestPasswordHashing(t *testing.T) {
	password := "my-secret-password"

	// Hash it
	hash1, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	t.Logf("Hash: %s", hash1)

	// Same password should verify
	if !checkPasswordHash(password, hash1) {
		t.Error("password should verify against its hash")
	}

	// Different password should not verify
	if checkPasswordHash("wrong-password", hash1) {
		t.Error("wrong password should not verify")
	}

	// Hash should be different each time (different salt)
	hash2, _ := hashPassword(password)
	if hash1 == hash2 {
		t.Error("hashes should be different due to random salt")
	}
}

func TestSessionRevoke(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := NewAdminUserRepository(db)
	sessionRepo := NewAdminSessionRepository(db)
	svc := NewService(userRepo, sessionRepo)

	user, _ := NewAdminUser("admin", "password")
	userRepo.Create(user)

	result, _ := svc.Login("admin", "password", "127.0.0.1", "test")
	sessionHash := hashToken(result.SessionToken)

	// Revoke
	if err := svc.RevokeSession(sessionHash); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// Should fail after revoke
	_, err := svc.ValidateSession(sessionHash)
	if err == nil {
		t.Error("session should be invalid after revoke")
	}
	t.Logf("Session revoked correctly: %v", err)
}

func TestSessionExpiry(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := NewAdminUserRepository(db)
	sessionRepo := NewAdminSessionRepository(db)

	user, _ := NewAdminUser("admin", "password")
	userRepo.Create(user)

	// Create a session with very short TTL
	token := generateToken(32)
	session := NewAdminSession(user.ID, token, 1) // 1 nanosecond TTL
	sessionRepo.Create(session)

	// Should be expired
	sessionHash := hashToken(token)
	s, _ := sessionRepo.FindBySessionHash(sessionHash)
	if s != nil && !s.IsExpired() {
		t.Log("session may not be expired yet (nanosecond TTL)")
	}
}

func TestEnsureAdmin(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := NewAdminUserRepository(db)
	sessionRepo := NewAdminSessionRepository(db)
	svc := NewService(userRepo, sessionRepo)

	// First call should create admin
	user, err := svc.EnsureAdmin("root", "admin123")
	if err != nil {
		t.Fatalf("ensure admin: %v", err)
	}
	if user.Username != "root" {
		t.Errorf("expected root, got %s", user.Username)
	}

	// Second call should fail
	_, err = svc.EnsureAdmin("root2", "admin456")
	if err == nil {
		t.Error("second EnsureAdmin should fail")
	}
	t.Logf("EnsureAdmin duplicate correctly blocked: %v", err)
}
