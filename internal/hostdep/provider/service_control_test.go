package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aegis/internal/config"
)

func TestRuntimeProvidersImplementModeSwitchContracts(t *testing.T) {
	providers := []Provider{
		NewCaddyProvider(&config.Config{Proxy: config.ProxyConfig{CaddyBinary: "caddy"}}),
		NewHAProxyProvider("", "", ""),
	}
	for _, runtimeProvider := range providers {
		if _, ok := runtimeProvider.(ConfigStager); !ok {
			t.Fatalf("provider %s cannot stage mode-switch configuration", runtimeProvider.State().ID)
		}
		if _, ok := runtimeProvider.(ServiceController); !ok {
			t.Fatalf("provider %s cannot control its service", runtimeProvider.State().ID)
		}
	}
}

func TestRuntimeProviderServiceCommands(t *testing.T) {
	tests := []struct {
		name    string
		service string
		action  string
		build   func(providerCommandRunner) ServiceController
		invoke  func(ServiceController) error
	}{
		{name: "start caddy", service: "caddy", action: "start", build: testCaddyController, invoke: func(c ServiceController) error { return c.Start() }},
		{name: "stop caddy", service: "caddy", action: "stop", build: testCaddyController, invoke: func(c ServiceController) error { return c.Stop() }},
		{name: "restart caddy", service: "caddy", action: "restart", build: testCaddyController, invoke: func(c ServiceController) error { return c.Restart() }},
		{name: "start haproxy", service: "haproxy", action: "start", build: testHAProxyController, invoke: func(c ServiceController) error { return c.Start() }},
		{name: "stop haproxy", service: "haproxy", action: "stop", build: testHAProxyController, invoke: func(c ServiceController) error { return c.Stop() }},
		{name: "restart haproxy", service: "haproxy", action: "restart", build: testHAProxyController, invoke: func(c ServiceController) error { return c.Restart() }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var command string
			runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
				command = strings.Join(append([]string{name}, args...), " ")
				return nil, nil
			}
			if err := tc.invoke(tc.build(runner)); err != nil {
				t.Fatal(err)
			}
			want := "systemctl " + tc.action + " " + tc.service
			if command != want {
				t.Fatalf("command = %q, want %q", command, want)
			}
		})
	}
}

func TestRuntimeProviderServiceCommandReportsOutputAndTimeout(t *testing.T) {
	t.Run("output", func(t *testing.T) {
		controller := testHAProxyController(func(context.Context, string, ...string) ([]byte, error) {
			return []byte("unit failed"), errors.New("exit status 1")
		})
		err := controller.Start()
		if err == nil || !strings.Contains(err.Error(), "systemctl start haproxy failed") || !strings.Contains(err.Error(), "unit failed") {
			t.Fatalf("unexpected service error: %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		controller := &HAProxyProvider{
			commandTimeout: 10 * time.Millisecond,
			runCommand: func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}
		started := time.Now()
		err := controller.Stop()
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("expected timeout error, got %v", err)
		}
		if elapsed := time.Since(started); elapsed > time.Second {
			t.Fatalf("timeout took too long: %s", elapsed)
		}
	})
}

func testCaddyController(runner providerCommandRunner) ServiceController {
	return &CaddyProvider{
		cfg: &config.Config{}, commandTimeout: time.Second, runCommand: runner,
	}
}

func testHAProxyController(runner providerCommandRunner) ServiceController {
	return &HAProxyProvider{commandTimeout: time.Second, runCommand: runner}
}
