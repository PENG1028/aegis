package project

import (
	"database/sql"
	"testing"
	"time"

	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

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

// insertLegacyProject inserts a project whose description column is NULL,
// which is how rows created by older versions may look in a production DB.
func insertLegacyProject(t *testing.T, db *sql.DB, id, name string) {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		 VALUES (?, ?, NULL, ?, ?, ?)`,
		id, name, "active", now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
}

// TestFindAllWithNullDescription is a regression test: scanning a NULL
// description into a plain string used to fail with
// "converting NULL to string is unsupported", breaking status/listing APIs.
func TestFindAllWithNullDescription(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	insertLegacyProject(t, db, "proj_1", "legacy-a")
	insertLegacyProject(t, db, "proj_2", "legacy-b")

	projects, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll with NULL description: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("got %d projects, want 2", len(projects))
	}
	for _, p := range projects {
		if p.Description != "" {
			t.Errorf("project %s: description = %q, want empty string for NULL", p.Name, p.Description)
		}
	}
}

func TestFindByIDAndNameWithNullDescription(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	insertLegacyProject(t, db, "proj_x", "legacy-x")

	byID, err := repo.FindByID("proj_x")
	if err != nil {
		t.Fatalf("FindByID with NULL description: %v", err)
	}
	if byID == nil {
		t.Fatal("FindByID returned nil project")
	}
	if byID.Description != "" {
		t.Errorf("FindByID description = %q, want empty", byID.Description)
	}

	byName, err := repo.FindByName("legacy-x")
	if err != nil {
		t.Fatalf("FindByName with NULL description: %v", err)
	}
	if byName == nil {
		t.Fatal("FindByName returned nil project")
	}
	if byName.Description != "" {
		t.Errorf("FindByName description = %q, want empty", byName.Description)
	}
}

// TestCreateRoundTrip ensures normal (non-NULL) round trip still works.
func TestCreateRoundTrip(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	now := time.Now()
	p := &Project{
		ID:          "proj_new",
		Name:        "new-project",
		Description: "described",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repo.Create(p); err != nil {
		t.Fatal(err)
	}
	got, err := repo.FindByID("proj_new")
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "described" {
		t.Errorf("description = %q, want %q", got.Description, "described")
	}
}
