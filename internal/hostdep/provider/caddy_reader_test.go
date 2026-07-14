package provider

import (
	"context"
	"os"
	"testing"
)

func TestCaddyReader_ParseSimpleSite(t *testing.T) {
	content := `example.com {
    reverse_proxy 127.0.0.1:3000
}`
	f, _ := os.CreateTemp("", "caddyfile-*.test")
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	r := NewCaddyReader(f.Name())
	snap, err := r.ReadConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if len(snap.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(snap.Routes))
	}
	if snap.Routes[0].Match.Host != "example.com" {
		t.Errorf("expected host=example.com, got %s", snap.Routes[0].Match.Host)
	}
	if snap.Routes[0].Upstream.Target != "127.0.0.1:3000" {
		t.Errorf("expected target=127.0.0.1:3000, got %s", snap.Routes[0].Upstream.Target)
	}
}

func TestCaddyReader_ParseWithPath(t *testing.T) {
	content := `app.example.com {
    handle /api/* {
        reverse_proxy 127.0.0.1:8080
    }
    handle {
        reverse_proxy 127.0.0.1:3000
    }
}`
	f, _ := os.CreateTemp("", "caddyfile-*.test")
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	r := NewCaddyReader(f.Name())
	snap, err := r.ReadConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if len(snap.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(snap.Routes))
	}
	if snap.Routes[0].Match.Path != "/api/*" && snap.Routes[1].Match.Path != "/api/*" {
		t.Error("expected one route with path /api/*")
	}
}

func TestCaddyReader_ParseUnmanagedBlock(t *testing.T) {
	content := `{
    email admin@example.com
}

weird-custom-stuff {
    import /etc/caddy/custom.conf
}`
	f, _ := os.CreateTemp("", "caddyfile-*.test")
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	r := NewCaddyReader(f.Name())
	snap, err := r.ReadConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	// Global block ({ email ... }) should be skipped, no unmanaged
	// weird-custom-stuff should be in unmanaged
	if len(snap.Unmanaged) != 1 {
		t.Fatalf("expected 1 unmanaged block, got %d", len(snap.Unmanaged))
	}
}

func TestCaddyReader_NoConfigFile(t *testing.T) {
	r := NewCaddyReader("/nonexistent/Caddyfile")
	snap, err := r.ReadConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadConfig on missing file: %v", err)
	}
	if len(snap.Routes) != 0 {
		t.Error("expected 0 routes for missing config")
	}
}
