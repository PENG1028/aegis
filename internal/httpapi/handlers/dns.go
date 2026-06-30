package handlers

import (
	"log"
	"net/http"

	"aegis/internal/config"
	"aegis/internal/dns"
)

// DNSHandler holds dependencies for DNS management endpoints.
type DNSHandler struct {
	Server   *dns.Server
	Resolver *dns.Resolver
	Config   *config.Config
	DNSMgmt  *dns.Manager
}

// DNSStatusResponse is the JSON response for DNS status.
type DNSStatusResponse struct {
	Running      bool                       `json:"running"`
	ListenAddr   string                     `json:"listen_addr"`
	Upstream     string                     `json:"upstream"`
	Enabled      bool                       `json:"enabled"`
	LocalHits    int64                      `json:"local_hits"`
	UpstreamCalls int64                     `json:"upstream_calls"`
	ManagedCount int                        `json:"managed_count"`
	Entries      []dns.ResolvedEntry        `json:"entries,omitempty"`
}

// DNSStatus returns the current DNS server status and stats.
func (h *DNSHandler) DNSStatus(w http.ResponseWriter, r *http.Request) {
	running := false
	var localHits, upstreamCalls int64
	if h.Server != nil {
		running = h.Server.IsRunning()
		localHits, upstreamCalls = h.Server.Stats()
	}

	managedCount := 0
	var entries []dns.ResolvedEntry
	if h.Resolver != nil {
		table := h.Resolver.Table()
		managedCount = len(table)
		if r.URL.Query().Get("detail") == "1" {
			for _, e := range table {
				entries = append(entries, e)
			}
		}
	}

	enabled := false
	listenAddr := ":5353"
	upstream := "1.1.1.1:53"
	if h.Config != nil {
		enabled = h.Config.DNS.Enabled
		listenAddr = h.Config.DNS.ListenAddr
		upstream = h.Config.DNS.Upstream
	}

	writeJSON(w, http.StatusOK, DNSStatusResponse{
		Running:        running,
		ListenAddr:     listenAddr,
		Upstream:       upstream,
		Enabled:        enabled,
		LocalHits:      localHits,
		UpstreamCalls:  upstreamCalls,
		ManagedCount:   managedCount,
		Entries:        entries,
	})
}

// DNSEnable starts the DNS server.
func (h *DNSHandler) DNSEnable(w http.ResponseWriter, r *http.Request) {
	if h.DNSMgmt == nil {
		writeError(w, http.StatusInternalServerError, "DNS management not initialized")
		return
	}
	if err := h.DNSMgmt.Enable(); err != nil {
		log.Printf("[dns] enable failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

// DNSDisable stops the DNS server.
func (h *DNSHandler) DNSDisable(w http.ResponseWriter, r *http.Request) {
	if h.DNSMgmt == nil {
		writeError(w, http.StatusInternalServerError, "DNS management not initialized")
		return
	}
	if err := h.DNSMgmt.Disable(); err != nil {
		log.Printf("[dns] disable failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

// DNSRefresh forces a resolver table refresh.
func (h *DNSHandler) DNSRefresh(w http.ResponseWriter, r *http.Request) {
	if h.Resolver == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "DNS resolver not initialized"})
		return
	}
	if err := h.Resolver.Refresh(); err != nil {
		log.Printf("[dns] refresh failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "refreshed", "count": itoa(len(h.Resolver.Table()))})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
