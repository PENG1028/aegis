package provider

import (
	"context"
	"strings"
	"testing"
	"time"

	"aegis/internal/config"
)

func TestCaddyCommandsTimeOut(t *testing.T) {
	p := &CaddyProvider{
		cfg:            &config.Config{Proxy: config.ProxyConfig{ReloadCommand: "systemctl reload caddy"}},
		binaryPath:     "caddy",
		commandTimeout: 10 * time.Millisecond,
		runCommand: func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{name: "validate", run: func() error { return p.validateConfig("Caddyfile") }},
		{name: "reload", run: p.reload},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started := time.Now()
			err := tc.run()
			if err == nil || !strings.Contains(err.Error(), "timed out") {
				t.Fatalf("expected timeout error, got %v", err)
			}
			if elapsed := time.Since(started); elapsed > time.Second {
				t.Fatalf("timeout took too long: %s", elapsed)
			}
		})
	}
}
