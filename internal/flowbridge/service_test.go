package flowbridge

import (
	"context"
	"strings"
	"testing"
)

type stubProbe struct {
	status  string
	latency int64
	message string
}

func (s *stubProbe) Check(_ context.Context, _ *Instance) (string, int64, string) {
	return s.status, s.latency, s.message
}

func TestCreateValidatesInput(t *testing.T) {
	db := testDB(t)
	svc := NewService(NewRepository(db), nil)
	base := CreateInstanceInput{Name: "edge", MachineIP: "10.0.0.5", DataPlanePort: 8080, ControlAddress: "10.0.0.5:9090"}

	cases := []struct {
		name string
		mut  func(*CreateInstanceInput)
	}{
		{"empty name", func(in *CreateInstanceInput) { in.Name = "" }},
		{"long name", func(in *CreateInstanceInput) { in.Name = strings.Repeat("x", 101) }},
		{"invalid IP", func(in *CreateInstanceInput) { in.MachineIP = "not-an-ip" }},
		{"zero port", func(in *CreateInstanceInput) { in.DataPlanePort = 0 }},
		{"port overflow", func(in *CreateInstanceInput) { in.DataPlanePort = 70000 }},
		{"empty control", func(in *CreateInstanceInput) { in.ControlAddress = "" }},
		{"bad control address", func(in *CreateInstanceInput) { in.ControlAddress = "10.0.0.5" }},
	}
	for _, tc := range cases {
		in := base
		tc.mut(&in)
		if _, err := svc.Create(context.Background(), in, "", "admin", "", ""); err == nil {
			t.Errorf("%s: expected validation error", tc.name)
		}
	}
}

func TestCreateRunsInitialCheck(t *testing.T) {
	db := testDB(t)
	svc := NewService(NewRepository(db), nil)
	svc.SetProbe(&stubProbe{status: HealthHealthy, latency: 12, message: "ok"})

	inst, err := svc.Create(context.Background(),
		CreateInstanceInput{Name: "edge", MachineIP: "10.0.0.5", DataPlanePort: 8080, ControlAddress: "10.0.0.5:9090"},
		"", "admin", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if inst.LastHealthStatus != HealthHealthy || inst.LastHealthLatency != 12 {
		t.Fatalf("initial check not persisted: %+v", inst)
	}
}

func TestSetEnabledTogglesAndCheckPersists(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	svc := NewService(repo, nil)
	svc.SetProbe(&stubProbe{status: HealthUnhealthy, latency: 99, message: "down"})
	if err := repo.Create(testInstance("fb_1")); err != nil {
		t.Fatal(err)
	}

	inst, err := svc.SetEnabled(context.Background(), "fb_1", false)
	if err != nil || inst.Enabled {
		t.Fatalf("disable failed: %+v err=%v", inst, err)
	}
	if inst, _ := svc.Get(context.Background(), "fb_1"); inst.Enabled {
		t.Fatal("disabled state not persisted")
	}

	checked, err := svc.Check(context.Background(), "fb_1")
	if err != nil {
		t.Fatal(err)
	}
	if checked.LastHealthStatus != HealthUnhealthy || checked.LastHealthLatency != 99 {
		t.Fatalf("check result not persisted: %+v", checked)
	}
}

func TestUpdatePartialAndValidation(t *testing.T) {
	db := testDB(t)
	svc := NewService(NewRepository(db), nil)
	svc.SetProbe(&stubProbe{status: HealthHealthy, latency: 1, message: "ok"})
	created, err := svc.Create(context.Background(),
		CreateInstanceInput{Name: "edge", MachineIP: "10.0.0.5", DataPlanePort: 8080, ControlAddress: "10.0.0.5:9090"},
		"", "admin", "", "")
	if err != nil {
		t.Fatal(err)
	}

	name := "renamed"
	inst, err := svc.Update(context.Background(), created.ID, UpdateInstanceInput{Name: &name})
	if err != nil || inst.Name != "renamed" {
		t.Fatalf("update failed: %+v err=%v", inst, err)
	}

	bad := "bad"
	if _, err := svc.Update(context.Background(), created.ID, UpdateInstanceInput{MachineIP: &bad}); err == nil {
		t.Fatal("expected invalid IP rejection on update")
	}
	if _, err := svc.Get(context.Background(), "fb_missing"); err == nil {
		t.Fatal("expected not-found error")
	}
}
