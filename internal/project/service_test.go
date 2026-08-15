package project

import (
	"context"
	"testing"

	"aegis/internal/logs"
)

// TestUpdateProject is a regression test: PATCH /api/projects/{id} was a
// 501 stub ("not implemented yet"), so the UI could never edit a project.
func TestUpdateProject(t *testing.T) {
	db := testDB(t)
	svc := NewAppService(NewRepository(db), logs.NewAppService(logs.NewRepository(db)))

	p, err := svc.CreateProject(context.Background(), CreateProjectInput{
		Name: "old-name", Description: "old desc",
	})
	if err != nil {
		t.Fatal(err)
	}

	newName := "new-name"
	newDesc := "new desc"
	updated, err := svc.UpdateProject(context.Background(), p.ID, UpdateProjectInput{
		Name: &newName, Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if updated.Name != "new-name" || updated.Description != "new desc" {
		t.Fatalf("updated = %+v, want name=new-name desc=new desc", updated)
	}

	// Renaming onto another project's name must be rejected.
	if _, err := svc.CreateProject(context.Background(), CreateProjectInput{Name: "taken"}); err != nil {
		t.Fatal(err)
	}
	dup := "taken"
	if _, err := svc.UpdateProject(context.Background(), p.ID, UpdateProjectInput{Name: &dup}); err == nil {
		t.Fatal("duplicate project name must be rejected")
	}
}
