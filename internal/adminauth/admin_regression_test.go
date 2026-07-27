package adminauth

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"aegis/internal/store"
)

// v1.7Y Bug 2: EnsureAdmin never called on fresh database.
func TestEnsureAdminCreatesDefaultAdmin(t *testing.T) {
	tmpFile := tmpPath(t)
	defer os.Remove(tmpFile)

	db := setupRegressionDB(t, tmpFile)
	defer db.Close()

	repo := NewAdminUserRepository(db)
	sessionRepo := NewAdminSessionRepository(db)
	svc := NewService(repo, sessionRepo)

	user, err := svc.EnsureAdmin("admin", "admin")
	if err != nil {
		t.Fatalf("Bug 2 regression: first EnsureAdmin failed: %v", err)
	}
	if user == nil {
		t.Fatal("Bug 2 regression: EnsureAdmin returned nil user")
	}
	if user.Username != "admin" {
		t.Errorf("expected username=admin, got %s", user.Username)
	}
	if !user.CheckPassword("admin") {
		t.Error("expected CheckPassword('admin') to be true")
	}
	t.Logf("Bug 2 regression PASS: EnsureAdmin created admin user id=%s", user.ID)
}

func TestEnsureAdminIdempotent(t *testing.T) {
	tmpFile := tmpPath(t)
	defer os.Remove(tmpFile)

	db := setupRegressionDB(t, tmpFile)
	defer db.Close()

	repo := NewAdminUserRepository(db)
	sessionRepo := NewAdminSessionRepository(db)
	svc := NewService(repo, sessionRepo)

	_, err := svc.EnsureAdmin("admin", "admin")
	if err != nil {
		t.Fatalf("first EnsureAdmin failed: %v", err)
	}

	user, err := svc.EnsureAdmin("admin", "admin")
	if err == nil {
		t.Error("Bug 2 regression: second EnsureAdmin should fail (admin already exists)")
	}
	if user != nil {
		t.Error("expected nil user on second call")
	}

	count, err := repo.Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 admin, got %d", count)
	}
	t.Log("Bug 2 regression PASS: EnsureAdmin is idempotent")
}

func TestAdminLoginAfterEnsureAdmin(t *testing.T) {
	tmpFile := tmpPath(t)
	defer os.Remove(tmpFile)

	db := setupRegressionDB(t, tmpFile)
	defer db.Close()

	repo := NewAdminUserRepository(db)
	sessionRepo := NewAdminSessionRepository(db)
	svc := NewService(repo, sessionRepo)

	svc.EnsureAdmin("admin", "admin")

	result, err := svc.Login("admin", "admin", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("Bug 2 regression: Login after EnsureAdmin failed: %v", err)
	}
	if result.User.Username != "admin" {
		t.Errorf("expected admin user, got %s", result.User.Username)
	}
	if result.SessionToken == "" {
		t.Error("expected non-empty session token")
	}
	t.Logf("Bug 2 regression PASS: Login succeeds (session=%s)", result.SessionToken[:12])
}

func TestAdminLoginWrongPassword(t *testing.T) {
	tmpFile := tmpPath(t)
	defer os.Remove(tmpFile)

	db := setupRegressionDB(t, tmpFile)
	defer db.Close()

	repo := NewAdminUserRepository(db)
	sessionRepo := NewAdminSessionRepository(db)
	svc := NewService(repo, sessionRepo)

	svc.EnsureAdmin("admin", "admin")

	_, err := svc.Login("admin", "wrongpassword", "127.0.0.1", "test")
	if err == nil {
		t.Error("Bug 2 regression: Login with wrong password should fail")
	}
	t.Log("Bug 2 regression PASS: Wrong password correctly rejected")
}

// Bug 3: Middleware order — cookie auth must work with AdminContext.
func TestAdminContextInjection(t *testing.T) {
	ac := &AdminContext{
		UserID:   "admin-test",
		Username: "test-admin",
	}

	ctx := WithAdminContext(context.Background(), ac)
	retrieved := GetAdminContext(ctx)

	if retrieved == nil {
		t.Fatal("Bug 3 regression: AdminContext should be retrievable")
	}
	if retrieved.UserID != "admin-test" {
		t.Errorf("expected UserID=admin-test, got %s", retrieved.UserID)
	}
	if retrieved.Username != "test-admin" {
		t.Errorf("expected Username=test-admin, got %s", retrieved.Username)
	}
	t.Log("Bug 3 regression PASS: AdminContext injection + retrieval works")
}

func TestGetAdminContextReturnsNilWhenNotSet(t *testing.T) {
	ac := GetAdminContext(context.Background())
	if ac != nil {
		t.Error("expected nil AdminContext when not set")
	}
	t.Log("Bug 3 regression PASS: nil context returns nil AdminContext")
}

// Helpers
func tmpPath(t *testing.T) string {
	f, err := os.CreateTemp("", "aegis-admin-regression-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	f.Close()
	os.Remove(f.Name())
	return f.Name()
}

func setupRegressionDB(t *testing.T, path string) *sql.DB {
	db, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := store.Initialize(db); err != nil {
		db.Close()
		t.Fatalf("init db: %v", err)
	}
	return db
}
