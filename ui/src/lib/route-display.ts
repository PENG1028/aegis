// Route display utility — single source of truth for route type, TLS mode, health, and port labels.
// Replace all scattered composition/tls_enabled checks across the frontend with these functions.

export interface RouteDisplay {
  typeLabel: string;    // "HTTPS Route", "UDP 隧道", "TLS Passthrough", etc.
  tlsLabel: string;     // "HTTPS", "HTTP", "TLS", "SNI", "QUIC"
  tlsActive: boolean;   // whether TLS is enabled
  isHTTP: boolean;      // whether this is an HTTP-based route (shows domain, cert, etc.)
  port: number;         // default port (80 for HTTP, 443 for HTTPS/TLS)
  compositionKey: string; // raw composition key from route
}

export function routeDisplay(route: {
  composition?: string;
  tls_enabled?: boolean;
  kind?: string;
}): RouteDisplay {
  const comp = route.composition || '';
  const tls = route.tls_enabled ?? false;
  const kind = route.kind || '';

  // Non-HTTP service kinds override everything
  if (kind === 'udp' || kind === 'tunnel') {
    return {
      typeLabel: 'UDP 隧道',
      tlsLabel: '加密',
      tlsActive: true,
      isHTTP: false,
      port: 0,
      compositionKey: comp,
    };
  }
  if (kind === 'tcp') {
    return {
      typeLabel: 'TCP 转发',
      tlsLabel: tls ? 'TLS' : 'TCP',
      tlsActive: tls,
      isHTTP: false,
      port: 0,
      compositionKey: comp,
    };
  }

  // HTTP-ish compositions
  switch (comp) {
    case 'http_route':
      return { typeLabel: 'HTTP Route', tlsLabel: 'HTTP', tlsActive: false, isHTTP: true, port: 80, compositionKey: comp };
    case 'https_route':
      return { typeLabel: 'HTTPS Route', tlsLabel: 'HTTPS', tlsActive: true, isHTTP: true, port: 443, compositionKey: comp };
    case 'tls_passthrough':
      return { typeLabel: 'TLS Passthrough', tlsLabel: 'SNI', tlsActive: true, isHTTP: false, port: 443, compositionKey: comp };
    case 'http3':
      return { typeLabel: 'HTTP/3', tlsLabel: 'QUIC', tlsActive: true, isHTTP: true, port: 443, compositionKey: comp };
    case 'raw_tcp':
      return { typeLabel: 'Raw TCP Forward', tlsLabel: tls ? 'TLS' : 'TCP', tlsActive: tls, isHTTP: false, port: 0, compositionKey: comp };
    case 'raw_udp':
      return { typeLabel: 'Raw UDP Forward', tlsLabel: 'UDP', tlsActive: false, isHTTP: false, port: 0, compositionKey: comp };
    default:
      break;
  }

  // Fallback: use tls_enabled
  return {
    typeLabel: tls ? 'HTTPS Route' : 'HTTP Route',
    tlsLabel: tls ? 'HTTPS' : 'HTTP',
    tlsActive: tls,
    isHTTP: true,
    port: tls ? 443 : 80,
    compositionKey: comp,
  };
}

// TLS mode string for backend compatibility
export function tlsMode(route: {
  composition?: string;
  tls_enabled?: boolean;
}): string {
  const comp = route.composition || '';
  if (comp === 'tls_passthrough') return 'passthrough_deferred';
  if (comp === 'raw_tcp' || comp === 'raw_udp') return 'terminate_local';
  return route.tls_enabled ? 'terminate_local' : 'http_only';
}
