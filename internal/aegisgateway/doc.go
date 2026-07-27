// Package aegisgateway is the Aegis capability gateway.
//
// It is intentionally separate from internal/gateway:
//   - internal/gateway models traffic relay, GatewayLink, and network path selection.
//   - internal/aegisgateway exposes Aegis capabilities and service calls to
//     authenticated services.
//
// This package owns capability-facing request routing. It depends on small
// interfaces for service discovery, node lookup, and remote node calls so the
// authserver/serviceauth core does not grow Aegis-specific transport logic.
package aegisgateway
