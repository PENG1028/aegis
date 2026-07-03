// Package provider — L3-L7 capability constants for gateway middleware.
//
// These 26 values form a closed, extensible set that describes what any gateway
// middleware (Caddy, HAProxy, Nginx, etc.) CAN do on a network. Every Provider
// declares a []Capability slice. The Planner matches traffic intents against
// available capabilities to select a topology.
//
// Adding a new protocol: add a constant in the appropriate layer group below,
// then add it to whichever Provider implementations support it. No other code
// changes needed — the Planner and templates consume capabilities generically.
//
// Grouping follows the OSI model from L3 (network) to L7 (application):
//
//	L3 — Network (source IP routing, transparent proxy / iptables DNAT)
//	L4 — Transport (TCP/UDP listen + upstream)
//	L5 — Session (TLS mode: terminate / passthrough / mTLS / masquerade)
//	L6 — Presentation (SNI, ALPN, protocol detection, OCSP)
//	L7 — Application (HTTP versions, WebSocket, gRPC, raw streams, routing, certs)
package provider

// Capability is a single, well-defined networking capability that a Provider
// (gateway middleware wrapper) may declare. The full set of 26 values is a
// closed enumeration — every gateway-relevant protocol primitive is covered.
type Capability string

const (
	// ==========================================================================
	// L3 — Network layer
	// ==========================================================================

	// CapRouteSrcIP — can match/route traffic based on the client's source IP address.
	// Used for IP whitelisting, geo-routing, and internal-vs-external traffic separation.
	// Supported by: Caddy (remote_ip matcher), HAProxy (src ACL), Nginx (allow/deny, geo).
	CapRouteSrcIP Capability = "route_src_ip"

	// CapTransparentProxy — can intercept outbound TCP connections via iptables DNAT
	// (SO_ORIGINAL_DST) and redirect them to a local gateway for forwarding.
	// This is NOT a Provider capability — it is declared by Aegis's built-in
	// transparent proxy manager. The Planner uses it to decide whether cross-node
	// transparent forwarding is possible and to compute the ForwardTarget.
	// Requires: Linux, iptables, root/sudo, SO_ORIGINAL_DST kernel support.
	CapTransparentProxy Capability = "transparent_proxy"

	// ==========================================================================
	// L4 — Transport layer
	// ==========================================================================

	// CapListenTCP — can bind and listen on a TCP port.
	// This is the most fundamental gateway capability. Nearly every provider supports it.
	CapListenTCP Capability = "listen_tcp"

	// CapListenUDP — can bind and listen on a UDP port.
	// Required for HTTP/3 (QUIC) and raw UDP forwarding.
	// Supported by: Caddy (h3), Nginx stream, Aegis UDP Manager.
	CapListenUDP Capability = "listen_udp"

	// CapUpstreamTCP — can forward traffic to a TCP upstream (host:port).
	CapUpstreamTCP Capability = "upstream_tcp"

	// CapUpstreamUDP — can forward traffic to a UDP upstream (host:port).
	CapUpstreamUDP Capability = "upstream_udp"

	// ==========================================================================
	// L5 — Session layer (TLS)
	// ==========================================================================

	// CapTLSTerminate — can terminate TLS: hold the certificate, decrypt the stream,
	// and read the plaintext HTTP content. Required for Host/Path routing on HTTPS.
	// Supported by: Caddy, Nginx http, HAProxy (http mode).
	CapTLSTerminate Capability = "tls_terminate"

	// CapTLSPassthrough — can forward a TLS stream without decrypting it.
	// The gateway reads only the ClientHello (SNI), then proxies the raw TCP stream
	// to a backend that holds the certificate.
	// Supported by: HAProxy (mode tcp), Nginx stream_ssl_preread, Caddy layer4.
	CapTLSPassthrough Capability = "tls_passthrough"

	// CapMTLSTerminate — can terminate TLS with mutual (client-certificate) authentication.
	// The gateway verifies the client's certificate before forwarding.
	// Supported by: HAProxy (client-cert verify), Nginx (ssl_client_certificate), Caddy.
	CapMTLSTerminate Capability = "mtls_terminate"

	// CapTLSMasquerade — presents TLS to the outside world but forwards plaintext
	// to the upstream (TLS-offloading at the edge).
	// This is effectively CapTLSTerminate + upstream over cleartext.
	CapTLSMasquerade Capability = "tls_masquerade"

	// ==========================================================================
	// L6 — Presentation layer (protocol detection & negotiation)
	// ==========================================================================

	// CapSNIPreread — can read the TLS Server Name Indication (SNI) field from the
	// ClientHello without terminating TLS. This allows domain-based routing of
	// encrypted traffic at L4.
	// Supported by: HAProxy (req.ssl_sni), Nginx (ssl_preread), Caddy layer4.
	CapSNIPreread Capability = "sni_preread"

	// CapALPNMatch — can read the ALPN (Application-Layer Protocol Negotiation)
	// extension from the TLS handshake and route based on the negotiated protocol.
	// Requires TLS termination. ALPN values: "h2", "http/1.1", "h3".
	// Supported by: Caddy, Nginx, HAProxy (http mode).
	CapALPNMatch Capability = "alpn_match"

	// CapProtoDetect — can auto-detect the application protocol from the stream
	// without explicit ALPN (e.g. distinguish HTTP/1.1 from HTTP/2 via preamble).
	// Supported by: Caddy, HAProxy (http mode).
	CapProtoDetect Capability = "proto_detect"

	// CapOCSPStapling — can staple OCSP responses in the TLS handshake, improving
	// certificate revocation check performance for clients.
	// Supported by: Caddy (built-in), Nginx (ssl_stapling), HAProxy.
	CapOCSPStapling Capability = "ocsp_stapling"

	// ==========================================================================
	// L7 — Application layer (protocols)
	// ==========================================================================

	// CapHTTP1 — HTTP/1.1 support (the baseline web protocol).
	CapHTTP1 Capability = "http1"

	// CapHTTP2 — HTTP/2 support (h2 over TLS, h2c over cleartext).
	// Requires ALPN "h2" negotiation or prior knowledge.
	CapHTTP2 Capability = "http2"

	// CapHTTP3 — HTTP/3 support over QUIC (UDP transport).
	// Requires a UDP listener on port 443.
	// Supported by: Caddy (h3), Nginx (experimental), HAProxy (build-dependent).
	CapHTTP3 Capability = "http3"

	// CapWebSocket — WebSocket upgrade support (HTTP/1.1 Upgrade: websocket).
	// The gateway must handle long-lived bidirectional connections and not apply
	// short HTTP timeouts.
	CapWebSocket Capability = "websocket"

	// CapGRPC — gRPC support (HTTP/2 + protobuf + trailers).
	// The gateway must preserve HTTP/2 trailers and support streaming RPCs.
	CapGRPC Capability = "grpc"

	// CapSSE — Server-Sent Events support (text/event-stream over HTTP/1.1).
	// The gateway must handle persistent connections and not buffer responses.
	CapSSE Capability = "sse"

	// CapRawTCP — raw TCP stream forwarding. The gateway does not inspect or modify
	// the stream content. Used for SSH, MySQL, Redis, custom TCP protocols.
	CapRawTCP Capability = "raw_tcp"

	// CapRawUDP — raw UDP datagram forwarding. The gateway forwards individual
	// datagrams without interpreting them. Used for DNS, syslog, game protocols.
	CapRawUDP Capability = "raw_udp"

	// ==========================================================================
	// L7 — Application layer (routing dimensions)
	// ==========================================================================

	// CapRouteHost — can route requests based on the HTTP Host header.
	// This is the primary routing dimension for virtual-hosting: one port serves
	// unlimited domains differentiated by Host.
	CapRouteHost Capability = "route_host"

	// CapRoutePath — can route requests based on the URL path prefix.
	// Used for path-based API routing: /api → backend-a, /app → backend-b.
	CapRoutePath Capability = "route_path"

	// ==========================================================================
	// L7 — Application layer (operational capabilities)
	// ==========================================================================

	// CapAutoCert — can automatically obtain and renew TLS certificates via ACME
	// (Let's Encrypt / ZeroSSL). This includes HTTP-01 and TLS-ALPN-01 challenge
	// handling, certificate storage, and renewal scheduling.
	// Supported by: Caddy (built-in, best-in-class). Nginx + certbot (external).
	CapAutoCert Capability = "auto_cert"

	// CapHealthCheck — can actively health-check upstream backends (TCP connect,
	// HTTP GET, gRPC health) and stop forwarding to unhealthy ones.
	CapHealthCheck Capability = "health_check"

	// CapLoadBalance — can distribute requests across multiple upstream backends
	// using an algorithm (round-robin, least-conn, hash, random).
	CapLoadBalance Capability = "load_balance"

	// CapRateLimit — can limit the rate of incoming requests per client/IP/route.
	CapRateLimit Capability = "rate_limit"

	// CapHotReload — can reload its configuration without dropping existing
	// connections (graceful reload).
	CapHotReload Capability = "hot_reload"

	// CapValidateConfig — can validate a configuration file for syntax errors
	// before applying it (dry-run / check mode).
	CapValidateConfig Capability = "validate_config"
)

// Layer returns the OSI layer group for a capability (L3-L7).
func (c Capability) Layer() string {
	switch c {
	case CapRouteSrcIP, CapTransparentProxy:
		return "L3"
	case CapListenTCP, CapListenUDP, CapUpstreamTCP, CapUpstreamUDP:
		return "L4"
	case CapTLSTerminate, CapTLSPassthrough, CapMTLSTerminate, CapTLSMasquerade:
		return "L5"
	case CapSNIPreread, CapALPNMatch, CapProtoDetect, CapOCSPStapling:
		return "L6"
	default:
		return "L7"
	}
}

// IsIngress returns true if the capability is relevant to external traffic entry
// (as opposed to internal forwarding or operational concerns).
func (c Capability) IsIngress() bool {
	switch c {
	case CapListenTCP, CapListenUDP,
		CapTLSTerminate, CapTLSPassthrough, CapMTLSTerminate, CapTLSMasquerade,
		CapSNIPreread, CapALPNMatch, CapProtoDetect,
		CapHTTP1, CapHTTP2, CapHTTP3, CapWebSocket, CapGRPC, CapSSE,
		CapRawTCP, CapRawUDP,
		CapRouteHost, CapRoutePath:
		return true
	default:
		return false
	}
}
