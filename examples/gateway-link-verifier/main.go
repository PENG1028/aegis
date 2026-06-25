// Gateway Link Verifier — example downstream verification service.
//
// This is a REFERENCE IMPLEMENTATION for Server B to verify that incoming
// requests are from a trusted Aegis Gateway (Server A).
//
// It checks static shared tokens (X-Aegis-Gateway-Link + X-Aegis-Gateway-Token).
// It does NOT implement HMAC dynamic signing.
//
// Usage:
//
//	export AEGIS_GATEWAY_LINK_ID="gw_abc123"
//	export AEGIS_GATEWAY_TOKEN="<token-from-aegis>"
//	go run main.go
//
// Or with a custom port:
//
//	PORT=3100 go run main.go
package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	expectedLinkID := os.Getenv("AEGIS_GATEWAY_LINK_ID")
	expectedToken := os.Getenv("AEGIS_GATEWAY_TOKEN")
	port := os.Getenv("PORT")
	if port == "" {
		port = "3100"
	}

	if expectedLinkID == "" || expectedToken == "" {
		fmt.Fprintf(os.Stderr, "ERROR: AEGIS_GATEWAY_LINK_ID and AEGIS_GATEWAY_TOKEN must be set\n")
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		linkID := r.Header.Get("X-Aegis-Gateway-Link")
		token := r.Header.Get("X-Aegis-Gateway-Token")

		// Log received headers for debugging
		fmt.Printf("[verifier] request from %s\n", r.RemoteAddr)
		fmt.Printf("[verifier]   X-Aegis-Gateway-Link: %q\n", linkID)
		fmt.Printf("[verifier]   X-Aegis-Gateway-Token: %q\n", token)

		if linkID == "" {
			http.Error(w, "missing X-Aegis-Gateway-Link header", http.StatusUnauthorized)
			fmt.Printf("[verifier]   → 401: missing link ID\n")
			return
		}
		if token == "" {
			http.Error(w, "missing X-Aegis-Gateway-Token header", http.StatusUnauthorized)
			fmt.Printf("[verifier]   → 401: missing token\n")
			return
		}
		if linkID != expectedLinkID {
			http.Error(w, fmt.Sprintf("unexpected link ID: got %q, expected %q", linkID, expectedLinkID), http.StatusForbidden)
			fmt.Printf("[verifier]   → 403: link ID mismatch\n")
			return
		}
		if token != expectedToken {
			http.Error(w, "token mismatch", http.StatusForbidden)
			fmt.Printf("[verifier]   → 403: token mismatch\n")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","link_id":%q,"forwarded_for":%q}`+"\n", linkID, r.RemoteAddr)
		fmt.Printf("[verifier]   → 200: verified OK\n")
	})

	addr := fmt.Sprintf("127.0.0.1:%s", port)
	fmt.Printf("Gateway Link Verifier starting on %s\n", addr)
	fmt.Printf("  Expected link ID: %s\n", expectedLinkID)
	fmt.Printf("  Expected token:   %s...\n", tokenPreview(expectedToken))
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func tokenPreview(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8] + "..."
}
