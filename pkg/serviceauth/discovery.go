package serviceauth

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// autoDetectAegis probes a list of candidate URLs and returns the first one
// that responds with HTTP 200 on /api/system/status (or /api/service-auth/v1
// health endpoint). Users can short-circuit detection by setting AEGIS_URL.
func autoDetectAegis() (string, error) {
	if url := os.Getenv("AEGIS_URL"); url != "" {
		return url, nil
	}

	// Port 7380 is the Aegis control port — the same port derived from
	// cfg.Server.Addr in production (safety.SplitHostPort). It is the only
	// stable address guaranteed to exist on every node running Aegis.
	// SDKs cannot import internal packages, so this default is fixed here
	// and overridable via AEGIS_URL env var or Config.AegisURL.
	candidates := []string{
		"http://127.0.0.1:7380",
		"http://localhost:7380",
	}

	client := &http.Client{Timeout: 2 * time.Second}
	for _, url := range candidates {
		if resp, err := client.Get(url + "/api/system/status"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return url, nil
			}
		}
		// Also try the service-auth health endpoint.
		if resp, err := client.Get(url + "/api/service-auth/v1/sync?bl_version=0&cat_version=0"); err == nil {
			resp.Body.Close()
			// Any response (200 or 304) means it's a serviceauth server.
			return url, nil
		}
	}

	return "", fmt.Errorf("serviceauth: no aegis/auth server found at %v", candidates)
}

// detectLocalIP returns the best guess at the machine's LAN IP.
func detectLocalIP() string {
	// Try environment variable first.
	if ip := os.Getenv("SERVICE_HOST"); ip != "" {
		return ip
	}

	// Probe by connecting to an external address and reading the local addr.
	conn, err := net.DialTimeout("udp", "8.8.8.8:53", 1*time.Second)
	if err != nil {
		return "127.0.0.1"
	}
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	conn.Close()
	return localAddr.IP.String()
}
