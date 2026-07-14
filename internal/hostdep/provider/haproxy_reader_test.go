package provider

import (
	"context"
	"os"
	"testing"
)

func TestHAProxyReader_ParseSNIRoutes(t *testing.T) {
	content := `global
    log stdout format raw local0

defaults
    log global
    mode tcp

frontend fe_tls_443
    bind 0.0.0.0:443
    mode tcp
    tcp-request inspect-delay 5s
    tcp-request content accept if { req_ssl_hello_type 1 }
    use_backend be_app_example_com if { req.ssl_sni -i app.example.com }
    use_backend be_db_example_com if { req.ssl_sni -i db.example.com }
    default_backend be_reject

backend be_app_example_com
    mode tcp
    server target 10.0.0.1:3000 check

backend be_db_example_com
    mode tcp
    server target 10.0.0.1:5432 check

backend be_reject
    mode tcp
    tcp-request content reject
`
	f, _ := os.CreateTemp("", "haproxy-*.cfg")
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	reader := NewHAProxyReader(f.Name(), "/nonexistent/tcp.cfg")
	snap, err := reader.ReadConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if len(snap.Routes) != 2 {
		t.Fatalf("expected 2 SNI routes, got %d", len(snap.Routes))
	}
	hasApp := snap.Routes[0].Match.SNI == "app.example.com" || snap.Routes[1].Match.SNI == "app.example.com"
	hasDB := snap.Routes[0].Match.SNI == "db.example.com" || snap.Routes[1].Match.SNI == "db.example.com"
	if !hasApp || !hasDB {
		t.Errorf("expected routes for app.example.com and db.example.com, got %+v", snap.Routes)
	}
	if snap.Routes[0].TLSMode != "passthrough" && snap.Routes[1].TLSMode != "passthrough" {
		t.Error("expected TLSMode=passthrough for SNI route")
	}
}

func TestHAProxyReader_NoConfigFile(t *testing.T) {
	reader := NewHAProxyReader("/nonexistent/haproxy.cfg", "/nonexistent/tcp.cfg")
	snap, err := reader.ReadConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadConfig on missing file: %v", err)
	}
	if len(snap.Routes) != 0 {
		t.Error("expected 0 routes for missing config")
	}
}

func TestHAProxyReader_UnmanagedBlock(t *testing.T) {
	content := `global
    log stdout format raw local0

# Aegis-generated SNI routing
frontend fe_tls_443
    bind 0.0.0.0:443
    use_backend be_app if { req.ssl_sni -i app.example.com }
    default_backend be_reject

backend be_app
    server target 10.0.0.1:3000 check

backend be_reject
    tcp-request content reject

# This is a hand-written config that Aegis can't parse
custom-rule something
    unknown-parameter value
`
	f, _ := os.CreateTemp("", "haproxy-*.cfg")
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	r := NewHAProxyReader(f.Name(), "/nonexistent/tcp.cfg")
	snap, err := r.ReadConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if len(snap.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(snap.Routes))
	}
	if len(snap.Unmanaged) == 0 {
		t.Error("expected at least 1 unmanaged block for the hand-written config")
	}
}
