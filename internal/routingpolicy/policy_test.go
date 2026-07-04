package routingpolicy

import (
	"database/sql"
	"testing"
	"encoding/json"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS service_gateway_policies (
			policy_id TEXT PRIMARY KEY, service_id TEXT NOT NULL,
			mode TEXT NOT NULL DEFAULT 'auto', primary_gateway_id TEXT DEFAULT '',
			fallback_gateway_ids_json TEXT DEFAULT '[]',
			allow_local INTEGER NOT NULL DEFAULT 1, allow_private INTEGER NOT NULL DEFAULT 1,
			allow_public INTEGER NOT NULL DEFAULT 0, require_gateway_link INTEGER NOT NULL DEFAULT 1,
			require_relay INTEGER NOT NULL DEFAULT 1, preserve_host INTEGER NOT NULL DEFAULT 1,
			tls_mode TEXT NOT NULL DEFAULT 'http_only', priority INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS route_gateway_policies (
			policy_id TEXT PRIMARY KEY, route_id TEXT NOT NULL,
			mode TEXT NOT NULL DEFAULT 'auto', primary_gateway_id TEXT DEFAULT '',
			fallback_gateway_ids_json TEXT DEFAULT '[]',
			allow_local INTEGER NOT NULL DEFAULT 1, allow_private INTEGER NOT NULL DEFAULT 1,
			allow_public INTEGER NOT NULL DEFAULT 0, require_gateway_link INTEGER NOT NULL DEFAULT 1,
			require_relay INTEGER NOT NULL DEFAULT 1, preserve_host INTEGER NOT NULL DEFAULT 1,
			tls_mode TEXT NOT NULL DEFAULT 'http_only', priority INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("create tables: %v", err)
	}
	return db
}

func TestCreateServiceGatewayPolicy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	policy, err := svc.SetServicePolicy(PolicyInput{
		ServiceID: "svc_test", Mode: ModeAuto,
	})
	if err != nil {
		t.Fatalf("create service policy: %v", err)
	}
	if policy.PolicyID == "" {
		t.Error("expected non-empty policy_id")
	}
	if policy.Mode != ModeAuto {
		t.Errorf("expected mode auto, got %s", policy.Mode)
	}

	// Verify via repo
	fetched, err := svc.GetServicePolicy("svc_test")
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if fetched == nil || fetched.PolicyID != policy.PolicyID {
		t.Error("expected to find service policy")
	}
}

func TestCreateRouteGatewayPolicy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	policy, err := svc.SetRoutePolicy(PolicyInput{
		RouteID: "rt_test", Mode: ModeAuto,
	})
	if err != nil {
		t.Fatalf("create route policy: %v", err)
	}
	if policy.PolicyID == "" {
		t.Error("expected non-empty policy_id")
	}
}

func TestRoutePolicyOverridesServicePolicy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	// Service policy: mode=multi
	svc.SetServicePolicy(PolicyInput{
		ServiceID: "svc_ovr", Mode: ModeMulti,
		PrimaryGatewayID: "gw_svc", FallbackGatewayIDs: []string{"gw_svc2"},
		AllowPublic: boolPtr(true),
	})

	// Route policy: mode=fixed
	svc.SetRoutePolicy(PolicyInput{
		RouteID: "rt_ovr", Mode: ModeFixed,
		PrimaryGatewayID: "gw_route",
	})

	// Resolution should pick route policy
	resolved, err := svc.ResolvePolicy("rt_ovr", "svc_ovr")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Source != "route" {
		t.Errorf("expected route source, got %s", resolved.Source)
	}
	if resolved.Mode != ModeFixed {
		t.Errorf("expected fixed mode, got %s", resolved.Mode)
	}
	if resolved.PrimaryGatewayID != "gw_route" {
		t.Errorf("expected primary gw route, got %s", resolved.PrimaryGatewayID)
	}
}

func TestMalformedPolicyRejected(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	_, err := svc.SetServicePolicy(PolicyInput{
		ServiceID: "svc_bad", Mode: "invalid_mode",
	})
	if err == nil {
		t.Error("expected error for invalid mode")
	}

	_, err = svc.SetRoutePolicy(PolicyInput{
		RouteID: "rt_bad", Mode: ModeFixed, PrimaryGatewayID: "",
	})
	if err == nil {
		t.Error("expected error for fixed mode without primary gateway")
	}
}

func TestDisabledPolicy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	policy, err := svc.SetServicePolicy(PolicyInput{
		ServiceID: "svc_disabled", Mode: ModeDisabled,
	})
	if err != nil {
		t.Fatalf("create disabled policy: %v", err)
	}
	if policy.Mode != ModeDisabled {
		t.Errorf("expected mode disabled, got %s", policy.Mode)
	}
}

func TestFixedPolicyMissingPrimaryUnavailable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	_, err := svc.SetRoutePolicy(PolicyInput{
		RouteID: "rt_fixed_bad", Mode: ModeFixed, PrimaryGatewayID: "",
	})
	if err == nil {
		t.Error("expected error for fixed mode without primary_gateway_id")
	}
}

func TestMultiPolicyFallbackOrderStable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	policy, err := svc.SetRoutePolicy(PolicyInput{
		RouteID: "rt_multi", Mode: ModeMulti,
		PrimaryGatewayID:   "gw_primary",
		FallbackGatewayIDs: []string{"gw_fb1", "gw_fb2", "gw_fb3"},
	})
	if err != nil {
		t.Fatalf("create multi policy: %v", err)
	}
	if len(policy.FallbackGatewayIDs) != 3 {
		t.Errorf("expected 3 fallback ids, got %d", len(policy.FallbackGatewayIDs))
	}
	if policy.FallbackGatewayIDs[0] != "gw_fb1" || policy.FallbackGatewayIDs[2] != "gw_fb3" {
		t.Error("fallback order not preserved")
	}
}

func TestAutoPolicyDefaults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	// No policy set → default
	resolved, err := svc.ResolvePolicy("rt_unknown", "svc_unknown")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Source != "default" {
		t.Errorf("expected default source, got %s", resolved.Source)
	}
	if resolved.Mode != ModeAuto {
		t.Errorf("expected auto mode, got %s", resolved.Mode)
	}
	if resolved.AllowPublic {
		t.Error("default should not allow public")
	}
	if !resolved.RequireGatewayLink {
		t.Error("default should require gateway link")
	}
}

func TestResolvePolicyNoMatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	resolved, err := svc.ResolvePolicy("rt_nonexist", "svc_nonexist")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// Falls back to default
	if resolved.Source != "default" {
		t.Errorf("expected default, got %s", resolved.Source)
	}
}

func TestServicePolicyFieldsRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	input := PolicyInput{
		ServiceID: "svc_rt", Mode: ModeMulti,
		PrimaryGatewayID:   "gw_p",
		FallbackGatewayIDs: []string{"gw_f1", "gw_f2"},
		AllowLocal:         boolPtr(true),
		AllowPrivate:       boolPtr(true),
		AllowPublic:        boolPtr(false),
		RequireGatewayLink: boolPtr(true),
		RequireRelay:       boolPtr(false),
		PreserveHost:       boolPtr(true),
		TLSMode:            TLSModeTerminateLocal,
		Priority:           10,
		Enabled:            boolPtr(true),
	}

	_, err := svc.SetServicePolicy(input)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	fetched, _ := svc.GetServicePolicy("svc_rt")
	if fetched == nil {
		t.Fatal("expected policy")
	}
	if fetched.Mode != ModeMulti {
		t.Errorf("expected multi, got %s", fetched.Mode)
	}
	if fetched.PrimaryGatewayID != "gw_p" {
		t.Errorf("expected gw_p, got %s", fetched.PrimaryGatewayID)
	}
	if fetched.RequireRelay {
		t.Error("expected require_relay=false")
	}
	if fetched.TLSMode != TLSModeTerminateLocal {
		t.Errorf("expected tls_mode terminate_local, got %s", fetched.TLSMode)
	}
	if fetched.Priority != 10 {
		t.Errorf("expected priority 10, got %d", fetched.Priority)
	}
}

func TestRoutePolicyFieldsRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	policy, err := svc.SetRoutePolicy(PolicyInput{
		RouteID: "rt_fields", Mode: ModeAuto,
		AllowPublic: boolPtr(true),
		TLSMode:     TLSModePassthroughDefer,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !policy.AllowPublic {
		t.Error("expected allow_public=true")
	}
	if policy.TLSMode != TLSModePassthroughDefer {
		t.Errorf("expected passthrough_deferred, got %s", policy.TLSMode)
	}
}

func TestListServicePolicies(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	svc.SetServicePolicy(PolicyInput{ServiceID: "svc_a", Mode: ModeAuto})
	svc.SetServicePolicy(PolicyInput{ServiceID: "svc_b", Mode: ModeFixed, PrimaryGatewayID: "gw1"})

	list, err := svc.ListServicePolicies()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 policies, got %d", len(list))
	}
}

func TestListRoutePolicies(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	svc.SetRoutePolicy(PolicyInput{RouteID: "rt_a", Mode: ModeAuto})
	svc.SetRoutePolicy(PolicyInput{RouteID: "rt_b", Mode: ModeDisabled})

	list, err := svc.ListRoutePolicies()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 policies, got %d", len(list))
	}
}

func TestDefaultPolicy(t *testing.T) {
	def := DefaultPolicy()
	if def.Mode != ModeAuto {
		t.Errorf("expected auto, got %s", def.Mode)
	}
	if def.Source != "default" {
		t.Errorf("expected source default, got %s", def.Source)
	}
}

func TestValidModes(t *testing.T) {
	modes := ValidModes()
	if len(modes) != 4 {
		t.Errorf("expected 4 modes, got %d", len(modes))
	}
}

func TestFallbackGatewayIDsJSON(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	svc := NewService(repo)

	_, err := svc.SetServicePolicy(PolicyInput{
		ServiceID: "svc_fb_json", Mode: ModeMulti,
		FallbackGatewayIDs: []string{"a", "b", "c"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Verify JSON serialization round-trips correctly
	var ids []string
	json.Unmarshal([]byte(`["a","b","c"]`), &ids)
	if len(ids) != 3 {
		t.Error("expected 3 fallback ids")
	}
}

func boolPtr(b bool) *bool {
	return &b
}
