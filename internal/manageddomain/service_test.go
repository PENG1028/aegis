package manageddomain

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"aegis/internal/core"
	"aegis/internal/logs"
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

// TestDeleteDomain is a regression test: DELETE /api/managed-domains/{id}
// was a 501 stub, so users could never remove a managed domain.
func TestDeleteDomain(t *testing.T) {
	db := testDB(t)
	svc := NewAppService(NewRepository(db), logs.NewAppService(logs.NewRepository(db)))

	now := time.Now()
	md := &ManagedDomain{
		ID:               core.NewID("md"),
		Domain:           "delete-me.example.com",
		ServiceID:        "svc_1",
		OwnerRef:         "owner-x",
		TargetType:       "auth_page",
		TargetRef:        "ref-1",
		VerificationName: "_aegis.delete-me.example.com",
		VerificationValue: "aegis-verify-abc",
		Status:           "pending_verification",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := NewRepository(db).Create(md); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteDomain(context.Background(), md.ID); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}

	got, err := NewRepository(db).FindByID(md.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("managed domain still exists after delete")
	}
}
