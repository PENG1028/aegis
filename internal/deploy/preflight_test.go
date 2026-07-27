package deploy

import (
	"context"
	"strings"
	"testing"
)

type fakePreflightExecutor struct {
	commands []string
}

func (e *fakePreflightExecutor) Run(ctx context.Context, command string) *RunResult {
	e.commands = append(e.commands, command)
	if strings.HasPrefix(command, "cat > /tmp/aegis-preflight.sh") {
		return &RunResult{ExitCode: 0}
	}
	if command == "sh /tmp/aegis-preflight.sh" {
		return &RunResult{ExitCode: 0, Stdout: `{
  "host": {"os":"linux","arch":"x86_64"},
  "aegis": {"found":true,"path":"/usr/local/bin/aegis","version":"aegis test","running":true,"service":"aegis"},
  "providers": {"haproxy":{"found":true,"path":"/usr/sbin/haproxy","version":"2.8","running":true,"service":"haproxy"}},
  "config": {"found":true,"path":"/etc/aegis/config.yaml"},
  "ports": [{"port":80,"process":"haproxy","listen":"0.0.0.0:80"}]
}`}
	}
	return &RunResult{ExitCode: 127, Stderr: "unexpected command"}
}

func (e *fakePreflightExecutor) Close() error { return nil }

func TestPreflightConnectionUsesExistingConnection(t *testing.T) {
	exec := &fakePreflightExecutor{}
	report, err := PreflightConnection(context.Background(), &Connection{Executor: exec})
	if err != nil {
		t.Fatalf("PreflightConnection: %v", err)
	}

	if report == nil || report.Aegis == nil || !report.Aegis.Found || !report.Aegis.Running {
		t.Fatalf("unexpected aegis report: %#v", report)
	}
	if report.Host == nil || report.Host.OS != "linux" || report.Host.Arch != "x86_64" {
		t.Fatalf("host = %#v, want linux/x86_64", report.Host)
	}
	if got := report.Providers["haproxy"]; got == nil || !got.Found {
		t.Fatalf("haproxy provider not parsed: %#v", report.Providers)
	}
	if len(report.Ports) != 1 || report.Ports[0].Port != 80 {
		t.Fatalf("ports = %#v, want port 80", report.Ports)
	}
	if len(exec.commands) != 2 {
		t.Fatalf("commands = %#v, want script write and execute", exec.commands)
	}
}
