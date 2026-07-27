// Package transparent provides transparent IP:port interception for distributed
// control planes and apps that communicate via direct IP:port connections.
//
// Use case: A management platform on Machine A connects to 192.168.1.100:9100
// (Machine B's private IP + app port). With transparent interception:
//
//  1. iptables OUTPUT DNAT redirects 192.168.1.100:9100 → 127.0.0.1:<local_port>
//  2. Aegis transparent proxy on <local_port> accepts the connection
//  3. Proxy reads SO_ORIGINAL_DST to learn the original destination (192.168.1.100:9100)
//  4. Proxy looks up: 192.168.1.100:9100 → Service X on Node Y
//  5. Proxy forwards the TCP stream through Aegis relay/gateway link
//  6. Remote Aegis forwards to the actual backend (127.0.0.1:9100)
//
// This enables unified port 80/443 management without requiring every
// distributed console to be configured with domain names.
//
// Security: Rules are only added for IPs/ports registered in Aegis's
// endpoint database. The SO_ORIGINAL_DST check prevents spoofing.
// iptables rules are cleaned up on shutdown.
package transparent
