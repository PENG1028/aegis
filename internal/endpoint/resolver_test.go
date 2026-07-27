package endpoint

import (
	"testing"
)

func TestNormalizeAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"127.0.0.1:3001", "http://127.0.0.1:3001"},
		{"http://127.0.0.1:3001", "http://127.0.0.1:3001"},
		{"https://127.0.0.1:443", "https://127.0.0.1:443"},
		{"10.0.0.5:8080", "http://10.0.0.5:8080"},
		{"1.2.3.4:3001", "http://1.2.3.4:3001"},
	}

	for _, tt := range tests {
		result := NormalizeAddress(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeAddress(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestParseHostPort(t *testing.T) {
	tests := []struct {
		addr     string
		host     string
		port     string
		hasError bool
	}{
		{"http://127.0.0.1:3001", "127.0.0.1", "3001", false},
		{"https://10.0.0.5:443", "10.0.0.5", "443", false},
		{"1.2.3.4:8080", "1.2.3.4", "8080", false},
		{"http://example.com", "example.com", "80", false},
		{"https://example.com", "example.com", "443", false},
	}

	for _, tt := range tests {
		host, port, err := parseHostPort(tt.addr)
		if tt.hasError && err == nil {
			t.Errorf("parseHostPort(%q) expected error", tt.addr)
		}
		if !tt.hasError {
			if err != nil {
				t.Errorf("parseHostPort(%q) unexpected error: %v", tt.addr, err)
			}
			if host != tt.host || port != tt.port {
				t.Errorf("parseHostPort(%q) = (%q, %q), want (%q, %q)", tt.addr, host, port, tt.host, tt.port)
			}
		}
	}
}

func TestEndpointPriority(t *testing.T) {
	ep1 := Endpoint{Type: "local"}
	ep2 := Endpoint{Type: "private"}
	ep3 := Endpoint{Type: "public"}

	if ep1.Priority() >= ep2.Priority() {
		t.Error("local should have lower priority than private")
	}
	if ep2.Priority() >= ep3.Priority() {
		t.Error("private should have lower priority than public")
	}
}
