package handlers

import (
	"strings"
	"testing"
)

// ── redactGatewaySecrets tests (H6 fix) ──

func TestRedactGatewaySecrets_SingleHeader(t *testing.T) {
	config := `example.com {
    encode gzip
    reverse_proxy http://127.0.0.1:3001 {
        header_up X-Aegis-Gateway-Link "gl_abc123"
        header_up X-Aegis-Gateway-Token "secret-value-here"
    }
}`

	result := redactGatewaySecrets(config)

	if strings.Contains(result, "secret-value-here") {
		t.Error("gateway token must be redacted from config")
	}
	if !strings.Contains(result, "***REDACTED***") {
		t.Error("redacted config must contain ***REDACTED*** placeholder")
	}
	if !strings.Contains(result, "X-Aegis-Gateway-Link") {
		t.Error("non-secret headers should be preserved")
	}
}

func TestRedactGatewaySecrets_MultipleHeaders(t *testing.T) {
	config := `site1.com {
    reverse_proxy http://127.0.0.1:3001 {
        header_up X-Aegis-Gateway-Token "token-one"
    }
}
site2.com {
    reverse_proxy http://127.0.0.1:3002 {
        header_up X-Aegis-Gateway-Token "token-two"
    }
}`

	result := redactGatewaySecrets(config)

	if strings.Contains(result, "token-one") || strings.Contains(result, "token-two") {
		t.Error("all gateway tokens must be redacted")
	}
	if strings.Count(result, "***REDACTED***") != 2 {
		t.Errorf("expected 2 redacted tokens, got %d: %s", strings.Count(result, "***REDACTED***"), result)
	}
}

func TestRedactGatewaySecrets_NoSecretsUnchanged(t *testing.T) {
	config := `example.com {
    encode gzip
    reverse_proxy http://127.0.0.1:3001
}`

	result := redactGatewaySecrets(config)

	if result != config {
		t.Errorf("config without secrets should be unchanged, got %q", result)
	}
}

func TestRedactGatewaySecrets_EmptyConfig(t *testing.T) {
	result := redactGatewaySecrets("")
	if result != "" {
		t.Errorf("empty config should remain empty, got %q", result)
	}
}

func TestRedactGatewaySecrets_HeaderWithSpecialChars(t *testing.T) {
	config := `example.com {
    reverse_proxy http://127.0.0.1:3001 {
        header_up X-Aegis-Gateway-Token "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0"
    }
}`

	result := redactGatewaySecrets(config)

	if strings.Contains(result, "eyJhbGci") {
		t.Error("JWT-like token must be redacted")
	}
	if !strings.Contains(result, "***REDACTED***") {
		t.Error("JWT-like token should be replaced with ***REDACTED***")
	}
}

func TestRedactGatewaySecrets_IndentedVariant(t *testing.T) {
	config := `    example.com {
        encode gzip
        reverse_proxy http://127.0.0.1:3001 {
            header_up X-Aegis-Gateway-Token "secret-indented"
        }
    }`

	result := redactGatewaySecrets(config)

	if strings.Contains(result, "secret-indented") {
		t.Error("indented gateway token must be redacted")
	}
	if !strings.Contains(result, "***REDACTED***") {
		t.Error("indented variant should still redact")
	}
}
