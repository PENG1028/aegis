// Package provider — L3-L7 capability constants for gateway middleware.
//
// These 30 values form a closed, extensible set that describes what any gateway
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
// (gateway middleware wrapper) may declare. The full set of 30 values is a
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

// ============================================================================
// CapabilityDef — JSON-serializable metadata for the frontend capability matrix
// ============================================================================

// CapabilityDef is a UI-ready description of a single capability.
type CapabilityDef struct {
	Key         string `json:"key"`         // e.g. "listen_tcp"
	Layer       string `json:"layer"`       // "L3" | "L4" | "L5" | "L6" | "L7"
	Label       string `json:"label"`       // Chinese display name
	Description string `json:"description"` // One-line explanation
}

// AllCapabilities returns all 30 capabilities in layer-grouped display order.
// This is the canonical row list for the capability matrix UI.
func AllCapabilities() []CapabilityDef {
	return []CapabilityDef{
		// L3 — Network
		{Key: "route_src_ip", Layer: "L3", Label: "源 IP 路由", Description: "基于客户端源 IP 的流量路由与白名单"},
		{Key: "transparent_proxy", Layer: "L3", Label: "透明代理", Description: "通过 iptables DNAT 拦截出站 TCP 连接并重定向"},

		// L4 — Transport
		{Key: "listen_tcp", Layer: "L4", Label: "TCP 监听", Description: "绑定并监听 TCP 端口"},
		{Key: "listen_udp", Layer: "L4", Label: "UDP 监听", Description: "绑定并监听 UDP 端口（HTTP/3 QUIC 所需）"},
		{Key: "upstream_tcp", Layer: "L4", Label: "TCP 上游", Description: "将流量转发到 TCP 上游地址"},
		{Key: "upstream_udp", Layer: "L4", Label: "UDP 上游", Description: "将数据报转发到 UDP 上游地址"},

		// L5 — Session / TLS
		{Key: "tls_terminate", Layer: "L5", Label: "TLS 终结", Description: "持有证书、解密 TLS 流并读取明文 HTTP 内容"},
		{Key: "tls_passthrough", Layer: "L5", Label: "TLS 直通", Description: "不解密，读取 SNI 后直接转发原始 TLS 流"},
		{Key: "mtls_terminate", Layer: "L5", Label: "mTLS 双向认证", Description: "验证客户端证书后转发"},
		{Key: "tls_masquerade", Layer: "L5", Label: "TLS 伪装卸载", Description: "对外展示 TLS，对内明文转发（边缘 TLS 卸载）"},

		// L6 — Presentation
		{Key: "sni_preread", Layer: "L6", Label: "SNI 预读", Description: "不解密即读取 TLS ClientHello 中的 SNI 域名"},
		{Key: "alpn_match", Layer: "L6", Label: "ALPN 匹配", Description: "基于 TLS ALPN 协商协议（h2/http1.1）路由"},
		{Key: "proto_detect", Layer: "L6", Label: "协议检测", Description: "自动检测应用层协议（HTTP/1.1 vs HTTP/2）"},
		{Key: "ocsp_stapling", Layer: "L6", Label: "OCSP 装订", Description: "在 TLS 握手时附装 OCSP 响应，加速证书吊销检查"},

		// L7 — Application protocols
		{Key: "http1", Layer: "L7", Label: "HTTP/1.1", Description: "HTTP/1.1 协议支持"},
		{Key: "http2", Layer: "L7", Label: "HTTP/2", Description: "HTTP/2（h2 over TLS）协议支持"},
		{Key: "http3", Layer: "L7", Label: "HTTP/3", Description: "HTTP/3 over QUIC（UDP 传输）支持"},
		{Key: "websocket", Layer: "L7", Label: "WebSocket", Description: "WebSocket 长连接升级支持"},
		{Key: "grpc", Layer: "L7", Label: "gRPC", Description: "gRPC（HTTP/2 + Protobuf + Trailers）支持"},
		{Key: "sse", Layer: "L7", Label: "Server-Sent Events", Description: "SSE 持久连接与流式推送支持"},
		{Key: "raw_tcp", Layer: "L7", Label: "TCP 原始流", Description: "不解释内容的透明 TCP 流转发"},
		{Key: "raw_udp", Layer: "L7", Label: "UDP 原始数据报", Description: "不解释内容的透明 UDP 数据报转发"},

		// L7 — Routing
		{Key: "route_host", Layer: "L7", Label: "域名路由", Description: "基于 HTTP Host 头的虚拟主机路由"},
		{Key: "route_path", Layer: "L7", Label: "路径路由", Description: "基于 URL 路径前缀的匹配路由"},

		// L7 — Operational
		{Key: "auto_cert", Layer: "L7", Label: "自动证书", Description: "通过 ACME 自动获取和续期 TLS 证书"},
		{Key: "health_check", Layer: "L7", Label: "健康检查", Description: "主动探测上游后端健康状态并摘除故障节点"},
		{Key: "load_balance", Layer: "L7", Label: "负载均衡", Description: "在多个上游后端之间分配请求"},
		{Key: "rate_limit", Layer: "L7", Label: "速率限制", Description: "基于客户端/IP/路由限制请求速率"},
		{Key: "hot_reload", Layer: "L7", Label: "热重载", Description: "不中断现有连接即可重载配置"},
		{Key: "validate_config", Layer: "L7", Label: "配置校验", Description: "在应用前对配置文件进行语法验证"},
	}
}

// TheoreticalMaxCapabilities returns the full set of capabilities a gateway type
// COULD theoretically support (regardless of current implementation status).
// The frontend uses this to compute △ = theoretical - actual.
func TheoreticalMaxCapabilities(gt GatewayType) []Capability {
	switch gt {
	case TypeHTTPTerm:
		// Caddy-like: full L7 HTTP gateway
		return []Capability{
			CapRouteSrcIP,
			CapListenTCP, CapListenUDP, CapUpstreamTCP, CapUpstreamUDP,
			CapTLSTerminate, CapTLSMasquerade, CapMTLSTerminate,
			CapALPNMatch, CapProtoDetect, CapOCSPStapling,
			CapHTTP1, CapHTTP2, CapHTTP3, CapWebSocket, CapGRPC, CapSSE,
			CapRouteHost, CapRoutePath,
			CapAutoCert, CapHealthCheck, CapLoadBalance, CapRateLimit, CapHotReload, CapValidateConfig,
		}
	case TypeSNIPass:
		// HAProxy-like: TLS SNI + TCP stream + some HTTP
		return []Capability{
			CapRouteSrcIP,
			CapListenTCP, CapUpstreamTCP,
			CapTLSPassthrough, CapTLSTerminate, CapMTLSTerminate,
			CapSNIPreread, CapALPNMatch, CapProtoDetect, CapOCSPStapling,
			CapHTTP1, CapHTTP2, CapRawTCP,
			CapRouteHost, CapHealthCheck, CapLoadBalance, CapRateLimit, CapHotReload, CapValidateConfig,
		}
	case TypeTCPForward:
		return []Capability{
			CapRouteSrcIP,
			CapListenTCP, CapUpstreamTCP,
			CapTLSPassthrough, CapSNIPreread,
			CapRawTCP, CapHealthCheck, CapHotReload,
		}
	case TypeUDPForward:
		return []Capability{
			CapListenUDP, CapUpstreamUDP,
			CapRawUDP, CapHealthCheck,
		}
	case TypeTransparent:
		return []Capability{
			CapRouteSrcIP, CapTransparentProxy,
			CapListenTCP, CapUpstreamTCP,
			CapRawTCP,
		}
	default:
		return nil
	}
}
