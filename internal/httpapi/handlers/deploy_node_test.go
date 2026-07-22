package handlers

import (
	"strings"
	"testing"

	"aegis/internal/config"
	"aegis/internal/deploy"
	"aegis/internal/distnode/onboarding"

	"gopkg.in/yaml.v3"
)

func TestRenderNodeServeConfigUsesControlProvider(t *testing.T) {
	out, err := renderNodeServeConfig(config.ProxyConfig{Provider: "haproxy"}, "node-b", "adm-test", "secret-test", "node-a", "43.159.34.11:80")
	if err != nil {
		t.Fatalf("renderNodeServeConfig: %v", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("unmarshal rendered config: %v\n%s", err, out)
	}

	if cfg.Proxy.Provider != "haproxy" {
		t.Fatalf("provider = %q, want haproxy", cfg.Proxy.Provider)
	}
	if cfg.Proxy.CaddyfilePath != "/etc/haproxy/haproxy.cfg" {
		t.Fatalf("config path = %q, want /etc/haproxy/haproxy.cfg", cfg.Proxy.CaddyfilePath)
	}
	if !strings.Contains(cfg.Proxy.ValidateCommand, "haproxy -c") {
		t.Fatalf("validate command = %q, want haproxy validation", cfg.Proxy.ValidateCommand)
	}
	if cfg.DistNode.Peers[0].Addr != "43.159.34.11:80" {
		t.Fatalf("peer addr = %q, want control edge", cfg.DistNode.Peers[0].Addr)
	}
}

func TestNodeProxyConfigIgnoresDevelopmentPaths(t *testing.T) {
	proxy := nodeProxyConfig(config.ProxyConfig{
		Provider:      "caddy",
		CaddyfilePath: ".aegis/Caddyfile",
		BackupDir:     ".aegis/backups",
	})

	if proxy.CaddyfilePath != "/etc/caddy/Caddyfile" {
		t.Fatalf("config path = %q, want production caddy path", proxy.CaddyfilePath)
	}
	if proxy.BackupDir != "/var/lib/aegis/backups" {
		t.Fatalf("backup dir = %q, want production backup path", proxy.BackupDir)
	}
}

func TestDeployResponseFromEnsurePreservesLegacyFields(t *testing.T) {
	resp := deployResponseFromEnsure(&onboarding.EnsureResult{
		Success: true,
		NodeID:  "node-b",
		Message: "ok",
	})

	if !resp.Success {
		t.Fatal("Success = false, want true")
	}
	if resp.NodeID != "node-b" {
		t.Fatalf("NodeID = %q, want node-b", resp.NodeID)
	}
	if resp.Message != "ok" {
		t.Fatalf("Message = %q, want ok", resp.Message)
	}
}

func TestNativeSSHDoesNotRequireSystemSSHBinary(t *testing.T) {
	if !isSSHAvailable() {
		t.Fatal("native SSH deployment should not depend on an external ssh binary")
	}
}

func TestResolveControlPeerRejectsLocalhostCurrentMode(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.DistNode.ID = "node_control"

	_, err := h.resolveControlPeer(DeployNodeRequest{TargetIP: "43.160.211.232"}, "localhost:7380")
	if err == nil {
		t.Fatal("resolveControlPeer returned nil error for localhost current mode")
	}
	if !strings.Contains(err.Error(), "push_only") {
		t.Fatalf("error = %q, want push_only guidance", err.Error())
	}
}

func TestResolveControlPeerPushOnlyUsesExplicitControl(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.DistNode.ID = "node_laptop"

	peer, err := h.resolveControlPeer(DeployNodeRequest{
		ControllerMode:  controllerModePushOnly,
		ControlNodeID:   "control-a",
		ControlEdgeAddr: "43.159.34.11",
		ControlSecret:   "cluster-secret",
		TargetIP:        "43.160.211.232",
	}, "localhost:7380")
	if err != nil {
		t.Fatalf("resolveControlPeer: %v", err)
	}
	if !peer.PushOnly {
		t.Fatal("PushOnly = false, want true")
	}
	if peer.NodeID != "node_control-a" {
		t.Fatalf("NodeID = %q, want node_control-a", peer.NodeID)
	}
	if peer.EdgeAddr != "43.159.34.11:80" {
		t.Fatalf("EdgeAddr = %q, want 43.159.34.11:80", peer.EdgeAddr)
	}
	if peer.Secret != "cluster-secret" {
		t.Fatalf("Secret = %q, want cluster-secret", peer.Secret)
	}
}

func TestResolveControlPeerPushOnlyRequiresControlNodeID(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.DistNode.ID = "node_laptop"

	_, err := h.resolveControlPeer(DeployNodeRequest{
		ControllerMode:  controllerModePushOnly,
		ControlEdgeAddr: "43.159.34.11:80",
		TargetIP:        "43.160.211.232",
	}, "localhost:7380")
	if err == nil {
		t.Fatal("resolveControlPeer returned nil error without control_node_id")
	}
	if !strings.Contains(err.Error(), "control_node_id") {
		t.Fatalf("error = %q, want control_node_id guidance", err.Error())
	}
}

func TestResolveControlPeerPushOnlyRequiresControlSecret(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.DistNode.ID = "node_laptop"

	_, err := h.resolveControlPeer(DeployNodeRequest{
		ControllerMode:  controllerModePushOnly,
		ControlNodeID:   "node_control",
		ControlEdgeAddr: "43.159.34.11:80",
		TargetIP:        "43.160.211.232",
	}, "localhost:7380")
	if err == nil {
		t.Fatal("resolveControlPeer returned nil error without control_secret")
	}
	if !strings.Contains(err.Error(), "control_secret") {
		t.Fatalf("error = %q, want control_secret guidance", err.Error())
	}
}

func TestResolveControlPeerCurrentUsesRequestHost(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.DistNode.ID = "node_control"
	h.Config.DistNode.Secret = "current-secret"

	peer, err := h.resolveControlPeer(DeployNodeRequest{TargetIP: "43.160.211.232"}, "43.159.34.11:7380")
	if err != nil {
		t.Fatalf("resolveControlPeer: %v", err)
	}
	if peer.PushOnly {
		t.Fatal("PushOnly = true, want false")
	}
	if peer.NodeID != "node_control" {
		t.Fatalf("NodeID = %q, want node_control", peer.NodeID)
	}
	if peer.EdgeAddr != "43.159.34.11:80" {
		t.Fatalf("EdgeAddr = %q, want 43.159.34.11:80", peer.EdgeAddr)
	}
	if peer.Secret != "current-secret" {
		t.Fatalf("Secret = %q, want current-secret", peer.Secret)
	}
}

func TestBuildDeployPlanForCleanTargetShowsArtifactAndFiles(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.Proxy.Provider = "haproxy"
	h.Config.DistNode.ID = "node_control"
	h.Config.DistNode.Secret = "secret"

	plan := h.buildDeployPlan(
		DeployNodeRequest{TargetIP: "43.160.211.232", ControllerMode: controllerModeCurrent},
		controlPeer{NodeID: "node_control", EdgeAddr: "43.159.34.11:80", Secret: "secret"},
		&deploy.PreflightReport{
			Host:  &deploy.HostInfo{OS: "linux", Arch: "x86_64"},
			Aegis: &deploy.BinaryInfo{Found: false},
			Providers: map[string]*deploy.BinaryInfo{
				"haproxy": {Found: true, Running: true, ConfigPath: "/etc/haproxy/haproxy.cfg"},
			},
		},
	)

	if plan.Action != "deploy" {
		t.Fatalf("Action = %q, want deploy", plan.Action)
	}
	if plan.Artifact.URL != "https://raw.githubusercontent.com/PENG1028/aegis/8dc738d91c93d7a577476e26eca5ebc1383461b1/aegis-linux-amd64" {
		t.Fatalf("Artifact.URL = %q, want default raw binary URL", plan.Artifact.URL)
	}
	if plan.Artifact.SHA256 != "bfabbb48612da5ebbd12325fd73fe435ab465969f9479055a9cada6acb6756f8" {
		t.Fatalf("Artifact.SHA256 = %q, want default checksum", plan.Artifact.SHA256)
	}
	if plan.Provider.Status != "ready" {
		t.Fatalf("Provider.Status = %q, want ready", plan.Provider.Status)
	}
	if len(plan.Files) == 0 || plan.Files[0].Path != "/usr/local/bin/aegis" {
		t.Fatalf("Files = %#v, want binary install path", plan.Files)
	}
}

func TestBuildDeployPlanForExistingRunningAegisUsesJoin(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.Proxy.Provider = "caddy"

	plan := h.buildDeployPlan(
		DeployNodeRequest{TargetIP: "43.160.211.232", ControllerMode: controllerModeCurrent},
		controlPeer{NodeID: "node_control", EdgeAddr: "43.159.34.11:80", Secret: "secret"},
		&deploy.PreflightReport{
			Host:   &deploy.HostInfo{OS: "linux", Arch: "x86_64"},
			Aegis:  &deploy.BinaryInfo{Found: true, Running: true},
			Config: &deploy.ConfigInfo{Found: true, Path: "/etc/aegis/config.yaml"},
			Providers: map[string]*deploy.BinaryInfo{
				"caddy": {Found: true, Running: true, ConfigPath: "/etc/caddy/Caddyfile"},
			},
		},
	)

	if plan.Action != "join" {
		t.Fatalf("Action = %q, want join", plan.Action)
	}
	if len(plan.Files) != 1 || plan.Files[0].Action != "update_distnode_block" {
		t.Fatalf("Files = %#v, want distnode-only config update", plan.Files)
	}
}

func TestBuildDeployPlanReportsMissingProvider(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.Proxy.Provider = "haproxy"

	plan := h.buildDeployPlan(
		DeployNodeRequest{TargetIP: "43.160.211.232", ControllerMode: controllerModeCurrent},
		controlPeer{NodeID: "node_control", EdgeAddr: "43.159.34.11:80", Secret: "secret"},
		&deploy.PreflightReport{
			Host:  &deploy.HostInfo{OS: "linux", Arch: "x86_64"},
			Aegis: &deploy.BinaryInfo{Found: false},
			Providers: map[string]*deploy.BinaryInfo{
				"haproxy": {Found: false},
			},
		},
	)

	if plan.Provider.Status != "provider_missing" {
		t.Fatalf("Provider.Status = %q, want provider_missing", plan.Provider.Status)
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("Warnings is empty, want provider warning")
	}
	if plan.CanProceed {
		t.Fatal("CanProceed = true, want false when provider install requires confirmation")
	}
	if len(plan.RepairActions) != 1 {
		t.Fatalf("RepairActions len = %d, want 1", len(plan.RepairActions))
	}
	repair := plan.RepairActions[0]
	if repair.Name != "install_provider" {
		t.Fatalf("repair.Name = %q, want install_provider", repair.Name)
	}
	if !repair.RequiresConfirmation {
		t.Fatal("repair.RequiresConfirmation = false, want true")
	}
	if len(repair.Commands) == 0 || !strings.Contains(strings.Join(repair.Commands, "\n"), "haproxy") {
		t.Fatalf("repair.Commands = %#v, want haproxy install commands", repair.Commands)
	}
}

func TestBuildDeployPlanBlocksUnexpectedPortOwner(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.Proxy.Provider = "haproxy"

	plan := h.buildDeployPlan(
		DeployNodeRequest{TargetIP: "43.160.211.232", ControllerMode: controllerModeCurrent},
		controlPeer{NodeID: "node_control", EdgeAddr: "43.159.34.11:80", Secret: "secret"},
		&deploy.PreflightReport{
			Host:  &deploy.HostInfo{OS: "linux", Arch: "x86_64"},
			Aegis: &deploy.BinaryInfo{Found: false},
			Providers: map[string]*deploy.BinaryInfo{
				"haproxy": {Found: true, Running: true},
			},
			Ports: []deploy.PortInfo{{Port: 80, Process: "nginx", Listen: "0.0.0.0:80"}},
		},
	)

	if plan.Provider.Status != "port_conflict" {
		t.Fatalf("Provider.Status = %q, want port_conflict", plan.Provider.Status)
	}
	if plan.CanProceed {
		t.Fatal("CanProceed = true, want false when 80/443 has an unexpected owner")
	}
	if len(plan.RepairActions) != 1 || plan.RepairActions[0].Name != "resolve_port_conflict" {
		t.Fatalf("RepairActions = %#v, want resolve_port_conflict", plan.RepairActions)
	}
}

func TestBuildDeployPlanProviderStoppedHasAutomaticRepair(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.Proxy.Provider = "caddy"

	plan := h.buildDeployPlan(
		DeployNodeRequest{TargetIP: "43.160.211.232", ControllerMode: controllerModeCurrent},
		controlPeer{NodeID: "node_control", EdgeAddr: "43.159.34.11:80", Secret: "secret"},
		&deploy.PreflightReport{
			Host:  &deploy.HostInfo{OS: "linux", Arch: "x86_64"},
			Aegis: &deploy.BinaryInfo{Found: false},
			Providers: map[string]*deploy.BinaryInfo{
				"caddy": {Found: true, Running: false},
			},
		},
	)

	if plan.Provider.Status != "provider_stopped" {
		t.Fatalf("Provider.Status = %q, want provider_stopped", plan.Provider.Status)
	}
	if !plan.CanProceed {
		t.Fatal("CanProceed = false, want true for automatic provider start repair")
	}
	if len(plan.RepairActions) != 1 {
		t.Fatalf("RepairActions len = %d, want 1", len(plan.RepairActions))
	}
	repair := plan.RepairActions[0]
	if repair.Name != "start_provider" {
		t.Fatalf("repair.Name = %q, want start_provider", repair.Name)
	}
	if !repair.Automatic || repair.RequiresConfirmation {
		t.Fatalf("repair automatic/confirmation = %v/%v, want true/false", repair.Automatic, repair.RequiresConfirmation)
	}
	if len(repair.Commands) != 1 || !strings.Contains(repair.Commands[0], "systemctl enable --now caddy") {
		t.Fatalf("repair.Commands = %#v, want caddy start command", repair.Commands)
	}
}

func TestSelectExecutableRepairAllowsStartProvider(t *testing.T) {
	repair, err := selectExecutableRepair(&DeployPlan{RepairActions: []DeployPlanRepairAction{
		{Name: "start_provider", Automatic: true, Status: "available", Commands: []string{"sudo systemctl enable --now caddy"}},
	}}, "start_provider", false)
	if err != nil {
		t.Fatalf("selectExecutableRepair: %v", err)
	}
	if repair.Name != "start_provider" {
		t.Fatalf("repair.Name = %q, want start_provider", repair.Name)
	}
}

func TestSelectExecutableRepairRejectsInstallProvider(t *testing.T) {
	_, err := selectExecutableRepair(&DeployPlan{RepairActions: []DeployPlanRepairAction{
		{Name: "install_provider", Automatic: false, RequiresConfirmation: true, Status: "manual_required", Commands: []string{"sudo apt-get install -y haproxy"}},
	}}, "install_provider", true)
	if err == nil {
		t.Fatal("selectExecutableRepair succeeded, want rejection")
	}
	if !strings.Contains(err.Error(), "only executes start_provider") {
		t.Fatalf("error = %q, want start_provider whitelist message", err.Error())
	}
}

func TestSelectExecutableRepairRejectsMissingAction(t *testing.T) {
	_, err := selectExecutableRepair(&DeployPlan{}, "start_provider", false)
	if err == nil {
		t.Fatal("selectExecutableRepair succeeded, want missing action error")
	}
	if !strings.Contains(err.Error(), "not present") {
		t.Fatalf("error = %q, want not present guidance", err.Error())
	}
}
