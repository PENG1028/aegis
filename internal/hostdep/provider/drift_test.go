package provider

import (
	"testing"
)

func TestDetectDrift_Consistent(t *testing.T) {
	plan := &Plan{
		Routes: []RouteSpec{
			{Match: MatchSpec{Host: "a.com"}, Upstream: UpstreamSpec{Target: "127.0.0.1:3000"}},
		},
	}
	snap := &ConfigSnapshot{
		ProviderID: "caddy",
		Routes: []RouteSpec{
			{Match: MatchSpec{Host: "a.com"}, Upstream: UpstreamSpec{Target: "127.0.0.1:3000"}},
		},
	}
	r := DetectDrift(plan, snap)
	if !r.Consistent {
		t.Errorf("expected consistent=true, got missing=%d unexpected=%d changed=%d",
			len(r.Missing), len(r.Unexpected), len(r.Changed))
	}
}

func TestDetectDrift_Missing(t *testing.T) {
	plan := &Plan{
		Routes: []RouteSpec{
			{Match: MatchSpec{Host: "a.com"}, Upstream: UpstreamSpec{Target: "127.0.0.1:3000"}},
			{Match: MatchSpec{Host: "b.com"}, Upstream: UpstreamSpec{Target: "127.0.0.1:4000"}},
		},
	}
	snap := &ConfigSnapshot{
		ProviderID: "caddy",
		Routes: []RouteSpec{
			{Match: MatchSpec{Host: "a.com"}, Upstream: UpstreamSpec{Target: "127.0.0.1:3000"}},
		},
	}
	r := DetectDrift(plan, snap)
	if r.Consistent {
		t.Error("expected consistent=false")
	}
	if len(r.Missing) != 1 || r.Missing[0].Domain != "b.com" {
		t.Errorf("expected 1 missing (b.com), got %v", r.Missing)
	}
}

func TestDetectDrift_Unexpected(t *testing.T) {
	plan := &Plan{
		Routes: []RouteSpec{
			{Match: MatchSpec{Host: "a.com"}, Upstream: UpstreamSpec{Target: "127.0.0.1:3000"}},
		},
	}
	snap := &ConfigSnapshot{
		ProviderID: "caddy",
		Routes: []RouteSpec{
			{Match: MatchSpec{Host: "a.com"}, Upstream: UpstreamSpec{Target: "127.0.0.1:3000"}},
			{Match: MatchSpec{Host: "old.com"}, Upstream: UpstreamSpec{Target: "10.0.0.1:8080"}},
		},
	}
	r := DetectDrift(plan, snap)
	if r.Consistent {
		t.Error("expected consistent=false")
	}
	if len(r.Unexpected) != 1 || r.Unexpected[0].Domain != "old.com" {
		t.Errorf("expected 1 unexpected (old.com), got %v", r.Unexpected)
	}
}

func TestDetectDrift_ChangedTarget(t *testing.T) {
	plan := &Plan{
		Routes: []RouteSpec{
			{Match: MatchSpec{Host: "a.com"}, Upstream: UpstreamSpec{Target: "127.0.0.1:3000"}},
		},
	}
	snap := &ConfigSnapshot{
		ProviderID: "caddy",
		Routes: []RouteSpec{
			{Match: MatchSpec{Host: "a.com"}, Upstream: UpstreamSpec{Target: "10.0.0.1:8080"}},
		},
	}
	r := DetectDrift(plan, snap)
	if r.Consistent {
		t.Error("expected consistent=false")
	}
	if len(r.Changed) != 1 {
		t.Fatalf("expected 1 changed, got %d", len(r.Changed))
	}
	if r.Changed[0].ExpectedTarget != "127.0.0.1:3000" || r.Changed[0].ActualTarget != "10.0.0.1:8080" {
		t.Errorf("unexpected diff: %+v", r.Changed[0])
	}
}

func TestDetectDrift_NoPlan(t *testing.T) {
	snap := &ConfigSnapshot{
		ProviderID: "caddy",
		Routes: []RouteSpec{
			{Match: MatchSpec{Host: "a.com"}, Upstream: UpstreamSpec{Target: "10.0.0.1:8080"}},
		},
	}
	r := DetectDrift(nil, snap)
	if r.Consistent {
		t.Error("expected consistent=false when no plan")
	}
	if len(r.Unexpected) != 1 {
		t.Errorf("expected 1 unexpected, got %d", len(r.Unexpected))
	}
}
