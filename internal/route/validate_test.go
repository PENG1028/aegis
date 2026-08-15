package route

import (
	"database/sql"
	"testing"
	"time"

	"aegis/internal/core"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

// TestValidatePathPrefixRejectsQueryAndFragment is a regression test: paths
// containing "?" or "#" passed validation and were rendered verbatim into
// `handle /api?x=1/*` — a Caddy matcher that can never match, silently
// killing the route (Caddy validate does not complain).
func TestValidatePathPrefixRejectsQueryAndFragment(t *testing.T) {
	for _, p := range []string{"/api?x=1", "/api#frag", "/api?v=1#f", "?x"} {
		if err := ValidatePathPrefix(p); err == nil {
			t.Errorf("ValidatePathPrefix(%q) accepted, want error", p)
		}
	}
}

// TestValidatePathPrefixAcceptsNormalPaths keeps valid prefixes working.
func TestValidatePathPrefixAcceptsNormalPaths(t *testing.T) {
	for _, p := range []string{"", "/", "/api", "/api/v1", "/api/*"} {
		if err := ValidatePathPrefix(p); err != nil {
			t.Errorf("ValidatePathPrefix(%q) = %v, want nil", p, err)
		}
	}
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestCheckDuplicatePathNormalizesTrailingSlash is a regression test:
// "/api" and "/api/" both render to the same `handle /api/*` matcher, so the
// second route silently shadows the first. The duplicate check must treat
// them as the same path.
func TestCheckDuplicatePathNormalizesTrailingSlash(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	now := time.Now()
	rt := &Route{
		ID:        core.NewID("rt"),
		Domain:    "dup.example.com",
		PathPrefix: "/api",
		ServiceID: "svc_1",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.Create(rt); err != nil {
		t.Fatal(err)
	}

	if err := repo.CheckDuplicatePath("dup.example.com", "/api/", ""); err == nil {
		t.Fatal("'/api/' must be rejected as duplicate of '/api' (same rendered matcher)")
	}
	if err := repo.CheckDuplicatePath("dup.example.com", "/api", ""); err == nil {
		t.Fatal("exact duplicate must be rejected")
	}
	if err := repo.CheckDuplicatePath("dup.example.com", "/api/v2", ""); err != nil {
		t.Fatalf("different path rejected: %v", err)
	}
}
