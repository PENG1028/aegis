/**
 * Aegis API Client — Mock Implementation
 *
 * ALL mock functions delegate to the active scenario (getScenario()),
 * ensuring data consistency between list pages, detail pages, and chain resolver.
 *
 * When VITE_USE_MOCK=false, api-bridge.ts uses real-api-client.ts instead.
 */

import { getScenario } from '@/mocks';
import type {
  Node, NodeDetail, Gateway, GatewayDetail,
  TopologyEdge, TopologyPathResult,
  Service, ServiceDetail,
  Route, RouteDetail,
  Endpoint, EndpointDetail,
  GatewayPolicy, RoutingEntry, RoutingPreviewResult, RoutingValidationResult,
  SyncStatus, LocalGatewayStatus, AcceptanceStatus,
  JoinToken, DashboardData,
} from '@/types';

function delay(ms = 200): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}

function mock<T>(data: T): T & { _mock?: string } {
  (data as any)._mock = '模拟';
  return data as T & { _mock?: string };
}

// ─── Dashboard ───
export async function fetchDashboard(): Promise<DashboardData> {
  await delay();
  return mock(getScenario().dashboard);
}

// ─── Nodes ───
export async function fetchNodes(): Promise<Node[]> {
  await delay();
  return mock(getScenario().nodes);
}
export async function fetchNodeDetail(nodeId: string): Promise<NodeDetail> {
  await delay();
  const d = getScenario().nodeDetails[nodeId];
  if (!d) throw new Error('Node not found');
  return mock(d);
}

// ─── Gateways ───
export async function fetchGateways(): Promise<Gateway[]> {
  await delay();
  return mock(getScenario().gateways);
}
export async function fetchGatewayDetail(id: string): Promise<GatewayDetail> {
  await delay();
  const d = getScenario().gatewayDetails[id];
  if (!d) throw new Error('Gateway not found');
  return mock(d);
}

// ─── Topology ───
export async function fetchTopologyMatrix(): Promise<TopologyEdge[]> {
  await delay();
  return mock(getScenario().topologyEdges);
}
export async function fetchTopologyPath(from: string, to: string): Promise<TopologyPathResult> {
  await delay(400);
  const edges = getScenario().topologyEdges;
  const edge = edges.find(e => e.from_node_id === from && e.to_node_id === to);
  const nodes = getScenario().nodes;
  const fromNode = nodes.find(n => n.node_id === from);
  const toNode = nodes.find(n => n.node_id === to);
  return mock({
    from_node: from,
    to_node: to,
    path: edge ? [
      { hop: 0, node_id: from, node_name: fromNode?.name || from, via: 'local', gateway_url: null, gateway_link_id: null, status: 'ok' as const, reason: null },
      { hop: 1, node_id: to, node_name: toNode?.name || to, via: 'private_gateway', gateway_url: null, gateway_link_id: edge.gateway_link_id || null, status: edge.status as any, reason: edge.last_error || null },
    ] : [],
    reachable: !!edge,
    total_hops: edge ? 2 : 0,
    summary: edge?.status || 'unknown',
  });
}

// ─── Services ───
export async function fetchServices(): Promise<Service[]> {
  await delay();
  return mock(getScenario().services);
}
export async function fetchServiceDetail(id: string): Promise<ServiceDetail> {
  await delay();
  const d = getScenario().serviceDetails[id];
  if (!d) throw new Error('Service not found');
  return mock(d);
}

// ─── Routes ───
export async function fetchRoutes(): Promise<Route[]> {
  await delay();
  return mock(getScenario().routes);
}
export async function fetchRouteDetail(id: string): Promise<RouteDetail> {
  await delay();
  const d = getScenario().routeDetails[id];
  if (!d) throw new Error('Route not found');
  return mock(d);
}

// ─── Endpoints ───
export async function fetchEndpoints(): Promise<Endpoint[]> {
  await delay();
  return mock(getScenario().endpoints);
}
export async function fetchEndpointDetail(id: string): Promise<EndpointDetail> {
  await delay();
  const d = getScenario().endpointDetails[id];
  if (!d) throw new Error('Endpoint not found');
  return mock(d);
}

// ─── Policies ───
export async function fetchPolicies(): Promise<GatewayPolicy[]> {
  await delay();
  return mock(getScenario().policies);
}

// ─── Routing Table ───
export async function fetchRoutingTable(_nodeId?: string): Promise<RoutingEntry[]> {
  await delay();
  return mock(getScenario().routingEntries);
}
export async function previewRouting(_domain: string, _fromNode: string): Promise<RoutingPreviewResult> {
  await delay(400);
  return mock({ domain: _domain, from_node_id: _fromNode, from_node_name: _fromNode, entries: [], available: false, summary: 'no routing data (mock)', unavailable_reason: null });
}
export async function validateRouting(_nodeId?: string): Promise<RoutingValidationResult> {
  await delay(500);
  return mock({ valid: true, node_id: _nodeId || null, total_entries: 0, errors: [], warnings: [], valid_count: 0, error_count: 0, warning_count: 0 });
}

// ─── Sync ───
export async function fetchSyncStatus(_nodeId?: string): Promise<SyncStatus[]> {
  await delay();
  const all = getScenario().syncStatuses;
  if (_nodeId) return mock(all.filter(s => s.node_id === _nodeId));
  return mock(all);
}

// ─── Local Gateway ───
export async function fetchLocalGatewayStatus(_nodeId?: string): Promise<LocalGatewayStatus[]> {
  await delay();
  const nodes = getScenario().nodes;
  const gateways = getScenario().gateways;
  const localGws = gateways.filter(g => g.type === 'local');
  return mock(localGws.map(gw => {
    const node = nodes.find(n => n.node_id === gw.node_id);
    return {
      node_id: gw.node_id,
      node_name: node?.name || gw.node_id,
      bind_addr: gw.bind_addr,
      port: gw.port,
      status: gw.enabled ? 'running' as const : 'stopped' as const,
      routing_table_loaded: true,
      routing_table_revision: node?.applied_revision || 0,
      entries_count: 3,
      cache_status: 'fresh' as const,
      diagnostics: [],
      last_error: gw.last_error,
    };
  }));
}

// ─── Acceptance ───
export async function fetchAcceptance(): Promise<AcceptanceStatus> {
  await delay();
  return mock(getScenario().acceptance);
}

// ─── Join Tokens ───
export async function fetchJoinTokens(): Promise<JoinToken[]> {
  await delay();
  return mock(getScenario().joinTokens);
}
export async function createJoinToken(data: Partial<JoinToken>): Promise<JoinToken & { rawToken?: string }> {
  await delay(500);
  return mock({
    ...data,
    id: 'jt_' + Date.now().toString(36),
    createdAt: new Date().toISOString(),
    expiresAt: new Date(Date.now() + 86400000).toISOString(),
    status: 'active',
    rawToken: 'aegis_join_' + Math.random().toString(36).slice(2, 14),
  } as any);
}
export async function revokeJoinToken(_id: string): Promise<void> {
  await delay(300);
}

// ─── Settings ───
export async function fetchSettings(): Promise<Record<string, any>> {
  await delay();
  return mock({
    bind_addr: '0.0.0.0',
    port: 7380,
    data_dir: '/data/aegis',
    tls_enabled: false,
    managed_domain: { gateway_domain: '', tls_email: '' },
  });
}
export async function updateSettings(_data: Record<string, any>): Promise<Record<string, any>> {
  await delay(800);
  return mock({ status: 'updated', message: 'settings saved (mock)' });
}

// ─── Transparent Proxy (v1.8F) ───
export const transparentApi = {
  listRules: async () => {
    await delay();
    return { rules: [], count: 0, message: 'transparent proxy not configured' };
  },
  deleteRule: async (_id: string) => {
    await delay();
    return { status: 'removed', rule_id: _id };
  },
};

// ─── Cluster Health (v1.8G) ───
export const clusterHealthApi = {
  get: async () => {
    await delay();
    return {
      node_count: 3, leader_node_id: 'node-a', split_brain: false,
      nodes: [
        { node_id: 'node-a', hostname: 'vps1', role: 'worker', status: 'online', is_leader: true, sync_status: 'in_sync', desired_revision: 5, applied_revision: 5 },
        { node_id: 'node-b', hostname: 'vps2', role: 'worker', status: 'online', is_leader: false, sync_status: 'in_sync', desired_revision: 5, applied_revision: 5 },
        { node_id: 'node-c', hostname: 'vps3', role: 'worker', status: 'offline', is_leader: false, sync_status: 'out_of_sync', desired_revision: 5, applied_revision: 3, heartbeat_age: '2m30s' },
      ],
      overall_healthy: false,
      issues: ['node-c: node is offline (heartbeat 2m30s ago)'],
    };
  },
};

// ─── Port Conflict Detection (v1.8G) ───
export const portCheckApi = {
  scan: async () => {
    await delay();
    return { conflicts: [], total: 0 };
  },
};

// ─── System Health (v1.8G) ───
export const systemHealthApi = {
  get: async () => {
    await delay();
    return {
      sqlite_ok: true, sqlite_size_bytes: 716800,
      disk_free_bytes: 50_000_000_000, disk_total_bytes: 100_000_000_000,
      memory_used_mb: 512, memory_total_mb: 2048,
      go_version: 'go1.22', goroutines: 42, uptime_seconds: 86400,
    };
  },
};

// ─── Health Check Actions ───
export const healthCheckApi = {
  checkAll: async () => {
    await delay();
    return { results: [], count: 0 };
  },
  getLatest: async () => {
    await delay();
    return { healthy_endpoints: 3, unhealthy_endpoints: 0, unknown_endpoints: 0 };
  },
};

// ─── Provider / Middleware (v1.7S + v1.8H) ───
export const providerApi = {
  list: async () => {
    await delay();
    return {
      providers: [
        {
          provider_id: 'caddy', provider_name: 'Caddy HTTP', gateway_type: 'http_terminate',
          detected: true, binary_path: '/usr/bin/caddy', version: 'v2.8.4',
          config_path: '/etc/caddy/Caddyfile', config_valid: true,
          service_running: true, running: true, status_message: '运行中',
          listening_ports: [80, 443],
          can_install: true,
          capabilities: {
            provider_id: 'caddy', provider_name: 'Caddy', gateway_type: 'http_terminate',
            match_keys: ['host_path'],
            protocols: ['http/1.1', 'http/2', 'http/3', 'websocket', 'grpc', 'sse'],
            auto_tls: true, load_balance: true, health_check: false, rate_limit: true,
            config_import: true, sni_passthrough: false, can_install: true,
          },
          diagnostic: {
            provider: 'caddy', installed: true, binary_path: '/usr/bin/caddy',
            version: 'v2.8.4', version_supported: true,
            config_path: '/etc/caddy/Caddyfile', config_exists: true, config_valid: true,
            service_running: true, listener_ok: true, runtime_verify_ok: true,
            last_error_code: '', last_error_message: '', stderr: '', checked_at: new Date().toISOString(),
          },
        },
        {
          provider_id: 'haproxy_edge_mux', provider_name: 'HAProxy EdgeMux', gateway_type: 'sni_passthrough',
          detected: true, binary_path: '/usr/sbin/haproxy', version: '2.8.5',
          config_path: '/etc/haproxy/haproxy.cfg', config_valid: true,
          service_running: true, running: true, status_message: '运行中 · EdgeMux 模式',
          listening_ports: [443],
          can_install: true,
          capabilities: {
            provider_id: 'haproxy_edge_mux', provider_name: 'HAProxy EdgeMux', gateway_type: 'sni_passthrough',
            match_keys: ['sni'],
            protocols: ['tcp'],
            auto_tls: false, load_balance: false, health_check: true, rate_limit: false,
            config_import: false, sni_passthrough: true, can_install: true,
          },
          diagnostic: {
            provider: 'haproxy_edge_mux', installed: true, binary_path: '/usr/sbin/haproxy',
            version: '2.8.5', version_supported: true,
            config_path: '/etc/haproxy/haproxy.cfg', config_exists: true, config_valid: true,
            service_running: true, listener_ok: true, runtime_verify_ok: true,
            last_error_code: '', last_error_message: '', stderr: '', checked_at: new Date().toISOString(),
          },
        },
        {
          provider_id: 'haproxy_tcp', provider_name: 'HAProxy TCP', gateway_type: 'tcp_forward',
          detected: true, binary_path: '/usr/sbin/haproxy', version: '2.8.5',
          config_path: '/etc/haproxy/haproxy.cfg', config_valid: true,
          service_running: true, running: true, status_message: '共享 HAProxy 二进制',
          listening_ports: [],
          can_install: false, // shared binary
          capabilities: {
            provider_id: 'haproxy_tcp', provider_name: 'HAProxy TCP', gateway_type: 'tcp_forward',
            match_keys: ['port'],
            protocols: ['tcp'],
            auto_tls: false, load_balance: false, health_check: true, rate_limit: false,
            config_import: false, sni_passthrough: false, can_install: false,
          },
          diagnostic: null,
        },
        {
          provider_id: 'aegis_tcp', provider_name: 'Aegis TCP Manager', gateway_type: 'tcp_forward',
          detected: true, binary_path: '', version: 'built-in',
          config_path: '', config_valid: null,
          service_running: null, running: true, status_message: '内置于 Aegis，无需额外安装',
          listening_ports: [],
          can_install: false,
          capabilities: {
            provider_id: 'aegis_tcp', provider_name: 'Aegis TCP Manager', gateway_type: 'tcp_forward',
            match_keys: ['port'],
            protocols: ['tcp', 'unix'],
            auto_tls: false, load_balance: false, health_check: false, rate_limit: false,
            config_import: false, sni_passthrough: false, can_install: false,
          },
          diagnostic: null,
        },
        {
          provider_id: 'aegis_udp', provider_name: 'Aegis UDP Manager', gateway_type: 'udp_forward',
          detected: true, binary_path: '', version: 'built-in',
          config_path: '', config_valid: null,
          service_running: null, running: true, status_message: '内置于 Aegis，无需额外安装',
          listening_ports: [],
          can_install: false,
          capabilities: {
            provider_id: 'aegis_udp', provider_name: 'Aegis UDP Manager', gateway_type: 'udp_forward',
            match_keys: ['port'],
            protocols: ['udp', 'unix'],
            auto_tls: false, load_balance: false, health_check: false, rate_limit: false,
            config_import: false, sni_passthrough: false, can_install: false,
          },
          diagnostic: null,
        },
      ],
      count: 5,
    };
  },
  diagnoseAll: async () => {
    await delay();
    return {
      diagnostics: [
        { provider: 'caddy_http', installed: true, binary_path: '/usr/bin/caddy', version: 'v2.7.6', version_supported: true, config_path: '/etc/caddy/Caddyfile', config_exists: true, config_valid: true, service_running: true, listener_ok: true, runtime_verify_ok: true, last_error_code: '', last_error_message: '', stderr: '', checked_at: new Date().toISOString() },
        { provider: 'haproxy_edge_mux', installed: true, binary_path: '/usr/sbin/haproxy', version: '2.8.5', version_supported: true, config_path: '/etc/haproxy/haproxy.cfg', config_exists: true, config_valid: true, service_running: true, listener_ok: true, runtime_verify_ok: true, last_error_code: '', last_error_message: '', stderr: '', checked_at: new Date().toISOString() },
      ],
      count: 2, issues: 0, healthy: true,
    };
  },
  install: async (provider: string) => { await delay(2000); return { provider, status: 'installed', message: `${provider} installed and service started` }; },
  getConfig: async (provider: string) => {
    await delay();
    const configs: Record<string, string> = {
      caddy: '# Caddyfile (auto-generated by Aegis)\n{\n\thttp_port 80\n}\n\nblog.example.com {\n\treverse_proxy 127.0.0.1:3000\n}\n\napi.example.com {\n\treverse_proxy 127.0.0.1:8080\n}',
      haproxy: '# /etc/haproxy/haproxy.cfg (Aegis managed)\nglobal\n\tdaemon\n\tmaxconn 4096\n\ndefaults\n\tmode tcp\n\ttimeout connect 5s\n\ttimeout client 30s\n\ttimeout server 30s',
    };
    return { provider, config_path: provider === 'caddy' ? '/etc/caddy/Caddyfile' : '/etc/haproxy/haproxy.cfg', exists: true, content: configs[provider] || '# no config' };
  },
  saveConfig: async (provider: string, _content: string) => { await delay(); return { provider, status: 'saved', config_path: provider === 'caddy' ? '/etc/caddy/Caddyfile' : '/etc/haproxy/haproxy.cfg' }; },
  reload: async (provider: string) => { await delay(500); return { provider, action: 'reload', status: 'success', output: 'OK' }; },
  serviceControl: async (provider: string, action: string) => { await delay(1000); return { provider, action, status: 'success', running: action !== 'stop' }; },
  uninstall: async (provider: string) => { await delay(2000); return { provider, status: 'uninstalled', message: `${provider} removed` }; },
  portPolicy: async () => {
    await delay(100);
    // Return current port policy — defaults to legacy in mock
    return {
      mode: 'legacy',
      bindings: [
        { port: 80, owner: 'caddy', protocol: 'tcp', purpose: 'http', status: 'active' },
        { port: 443, owner: 'caddy', protocol: 'tcp', purpose: 'https', status: 'active' },
      ],
    };
  },
};

// ─── Exposure (TCP/UDP port exposure) ───
export const exposureApi = {
  list: async () => { await delay(); return { exposures: [] as any[], count: 0 }; },
  create: async (input: any) => { await delay(); return { id: 'exp_mock1', ...input, status: 'pending', created_at: new Date().toISOString() }; },
  activate: async (id: string) => { await delay(); return { id, status: 'active' }; },
  disable: async (id: string) => { await delay(); return { id, status: 'disabled' }; },
};

// ─── Credential API ───
export const credentialApi = {
  list: async () => {
    await delay();
    return { credentials: [
      { id: 'cred_mock1', alias: 'pg-prod', scheme: 'postgres', masked_uri: 'postgres://user:***@10.0.0.5:5432/mydb', secret_version: 0, secret_created_at: new Date().toISOString(), description: '生产数据库', created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
      { id: 'cred_mock2', alias: 'redis-cache', scheme: 'redis', masked_uri: 'redis://:***@10.0.0.6:6379/0', secret_version: 1, secret_created_at: new Date().toISOString(), description: '缓存服务器', created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
    ], count: 2 };
  },
  create: async (alias: string, _connString: string, _desc?: string) => { await delay(); return { credential: { id: 'cred_' + Math.random().toString(36).slice(2, 8), alias, scheme: 'postgres', masked_uri: 'postgres://user:***@host:5432/db', secret_version: 0, secret_created_at: new Date().toISOString(), description: _desc || '', created_at: new Date().toISOString(), updated_at: new Date().toISOString() } }; },
  get: async (id: string) => { await delay(); return { credential: { id, alias: 'pg-prod', scheme: 'postgres', masked_uri: 'postgres://user:***@host:5432/db', secret_version: 0 } }; },
  delete: async (_id: string) => { await delay(); return { status: 'deleted', message: 'credential deleted' }; },
  rotate: async (_id: string, _connString: string) => { await delay(); return { credential: { id: _id, alias: 'pg-prod', secret_version: 1 } }; },
};
