/**
 * Aegis API Client
 *
 * In v1, this is a mock-only client that returns static data.
 * When the real API is available, swap the implementation
 * while keeping the same interface.
 */

import { mockNodes, mockNodeDetail } from '@/mocks/nodes';
import { mockGateways, mockGatewayDetail } from '@/mocks/gateways';
import { mockTopologyMatrix, mockTopologyPath } from '@/mocks/topology';
import { mockServices, mockServiceDetail } from '@/mocks/services';
import { mockRoutes, mockRouteDetail } from '@/mocks/routes';
import { mockEndpoints, mockEndpointDetail } from '@/mocks/endpoints';
import { mockPolicies } from '@/mocks/policies';
import {
  mockRoutingTable,
  mockRoutingPreview,
  mockRoutingValidate,
} from '@/mocks/routing';
import { mockSyncStatus } from '@/mocks/sync';
import { mockLocalGateway } from '@/mocks/local-gateway';
import { mockAcceptance } from '@/mocks/acceptance';
import { mockJoinTokens } from '@/mocks/join-tokens';
import { mockSettings } from '@/mocks/settings';
import { mockDashboard } from '@/mocks/dashboard';
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

const MOCK_LABEL = '模拟';

function delay(ms = 300): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}

/** Mark mock responses so the UI can show "Mock" badges */
function mock<T>(data: T): T & { _mock?: string } {
  (data as any)._mock = MOCK_LABEL;
  return data as T & { _mock?: string };
}

// ─── Dashboard ───
export async function fetchDashboard(): Promise<DashboardData> {
  await delay();
  return mock(mockDashboard);
}

// ─── Nodes ───
export async function fetchNodes(): Promise<Node[]> {
  await delay();
  return mock(mockNodes);
}
export async function fetchNodeDetail(nodeId: string): Promise<NodeDetail> {
  await delay();
  const detail = mockNodeDetail(nodeId);
  if (!detail) throw new Error('Node not found');
  return mock(detail);
}

// ─── Gateways ───
export async function fetchGateways(): Promise<Gateway[]> {
  await delay();
  return mock(mockGateways);
}
export async function fetchGatewayDetail(id: string): Promise<GatewayDetail> {
  await delay();
  const d = mockGatewayDetail(id);
  if (!d) throw new Error('Gateway not found');
  return mock(d);
}

// ─── Topology ───
export async function fetchTopologyMatrix(): Promise<TopologyEdge[]> {
  await delay();
  return mock(mockTopologyMatrix);
}
export async function fetchTopologyPath(from: string, to: string): Promise<TopologyPathResult> {
  await delay(400);
  return mock(mockTopologyPath(from, to));
}

// ─── Services ───
export async function fetchServices(): Promise<Service[]> {
  await delay();
  return mock(mockServices);
}
export async function fetchServiceDetail(id: string): Promise<ServiceDetail> {
  await delay();
  const d = mockServiceDetail(id);
  if (!d) throw new Error('Service not found');
  return mock(d);
}

// ─── Routes ───
export async function fetchRoutes(): Promise<Route[]> {
  await delay();
  return mock(mockRoutes);
}
export async function fetchRouteDetail(id: string): Promise<RouteDetail> {
  await delay();
  const d = mockRouteDetail(id);
  if (!d) throw new Error('Route not found');
  return mock(d);
}

// ─── Endpoints ───
export async function fetchEndpoints(): Promise<Endpoint[]> {
  await delay();
  return mock(mockEndpoints);
}
export async function fetchEndpointDetail(id: string): Promise<EndpointDetail> {
  await delay();
  const d = mockEndpointDetail(id);
  if (!d) throw new Error('Endpoint not found');
  return mock(d);
}

// ─── Policies ───
export async function fetchPolicies(): Promise<GatewayPolicy[]> {
  await delay();
  return mock(mockPolicies);
}

// ─── Routing Table ───
export async function fetchRoutingTable(nodeId?: string): Promise<RoutingEntry[]> {
  await delay();
  return mock(mockRoutingTable(nodeId));
}
export async function previewRouting(domain: string, fromNode: string): Promise<RoutingPreviewResult> {
  await delay(400);
  return mock(mockRoutingPreview(domain, fromNode));
}
export async function validateRouting(nodeId?: string): Promise<RoutingValidationResult> {
  await delay(500);
  return mock(mockRoutingValidate(nodeId));
}

// ─── Sync ───
export async function fetchSyncStatus(nodeId?: string): Promise<SyncStatus[]> {
  await delay();
  return mock(mockSyncStatus(nodeId));
}

// ─── Local Gateway ───
export async function fetchLocalGatewayStatus(nodeId?: string): Promise<LocalGatewayStatus[]> {
  await delay();
  return mock(mockLocalGateway(nodeId));
}

// ─── Acceptance ───
export async function fetchAcceptance(): Promise<AcceptanceStatus> {
  await delay();
  return mock(mockAcceptance);
}

// ─── Join Tokens ───
export async function fetchJoinTokens(): Promise<JoinToken[]> {
  await delay();
  return mock(mockJoinTokens);
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
export async function revokeJoinToken(id: string): Promise<void> {
  await delay(300);
}

// ─── Settings ───
export async function fetchSettings(): Promise<Record<string, any>> {
  await delay();
  return mock(mockSettings);
}

export async function updateSettings(data: Record<string, any>): Promise<Record<string, any>> {
  await delay(800);
  return {
    status: 'updated',
    message: 'settings saved',
    gateway_domain: data?.managed_domain?.gateway_domain || '',
    config_path: '/etc/aegis/config.yaml',
    caddyfile_regenerated: true,
    caddy_reloaded: true,
    panel_url: data?.managed_domain?.gateway_domain
      ? `https://${data.managed_domain.gateway_domain}`
      : 'http://<server-ip>',
    tls: data?.managed_domain?.gateway_domain
      ? "automatic (Let's Encrypt via Caddy)"
      : 'disabled (no domain configured)',
  };
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
        { name: 'caddy_http', protocol: 'http', status: 'available', message: 'caddy is running', config_path: '/etc/caddy/Caddyfile' },
        { name: 'haproxy_edge_mux', protocol: 'tls_mux', status: 'available', message: 'haproxy is running and configured', config_path: '/etc/haproxy/haproxy.cfg' },
      ],
      count: 2,
    };
  },
  diagnoseAll: async () => {
    await delay();
    return {
      diagnostics: [
        {
          provider: 'caddy_http', installed: true, binary_path: '/usr/bin/caddy', version: 'v2.7.6',
          version_supported: true, config_path: '/etc/caddy/Caddyfile', config_exists: true,
          config_valid: true, service_running: true, listener_ok: true, runtime_verify_ok: true,
          last_error_code: '', last_error_message: '', stderr: '', checked_at: new Date().toISOString(),
        },
        {
          provider: 'haproxy_edge_mux', installed: true, binary_path: '/usr/sbin/haproxy', version: '2.8.5',
          version_supported: true, config_path: '/etc/haproxy/haproxy.cfg', config_exists: true,
          config_valid: true, service_running: true, listener_ok: true, runtime_verify_ok: true,
          last_error_code: '', last_error_message: '', stderr: '', checked_at: new Date().toISOString(),
        },
      ],
      count: 2, issues: 0, healthy: true,
    };
  },
  install: async (provider: string) => {
    await delay(2000);
    return { provider, status: 'installed', message: `${provider} installed and service started` };
  },
  getConfig: async (provider: string) => {
    await delay();
    const configs: Record<string, string> = {
      caddy: '# Caddyfile (auto-generated by Aegis)\n{\n\thttp_port 80\n}\n\nblog.example.com {\n\treverse_proxy 127.0.0.1:3000\n}\n\napi.example.com {\n\treverse_proxy 127.0.0.1:8080\n}',
      haproxy: '# /etc/haproxy/haproxy.cfg (Aegis managed)\nglobal\n\tdaemon\n\tmaxconn 4096\n\ndefaults\n\tmode tcp\n\ttimeout connect 5s\n\ttimeout client 30s\n\ttimeout server 30s',
    };
    return {
      provider,
      config_path: provider === 'caddy' ? '/etc/caddy/Caddyfile' : '/etc/haproxy/haproxy.cfg',
      exists: true,
      content: configs[provider] || '# no config',
    };
  },
  // v1.8K Middleware control
  saveConfig: async (provider: string, _content: string) => {
    await delay();
    return { provider, status: 'saved', config_path: provider === 'caddy' ? '/etc/caddy/Caddyfile' : '/etc/haproxy/haproxy.cfg' };
  },
  reload: async (provider: string) => {
    await delay(500);
    return { provider, action: 'reload', status: 'success', output: 'OK' };
  },
  serviceControl: async (provider: string, action: string) => {
    await delay(1000);
    return { provider, action, status: 'success', running: action !== 'stop' };
  },
  uninstall: async (provider: string) => {
    await delay(2000);
    return { provider, status: 'uninstalled', message: `${provider} removed` };
  },
};

// ─── Exposure (TCP/UDP port exposure) ───
export const exposureApi = {
  list: async () => {
    await delay();
    return { exposures: [] as any[], count: 0 };
  },
  create: async (input: any) => {
    await delay();
    return { id: 'exp_mock1', ...input, status: 'pending', created_at: new Date().toISOString() };
  },
  activate: async (id: string) => {
    await delay();
    return { id, status: 'active' };
  },
  disable: async (id: string) => {
    await delay();
    return { id, status: 'disabled' };
  },
};

// ─── Credential API (encrypted connection strings) ───
export const credentialApi = {
  list: async () => {
    await delay();
    return {
      credentials: [
        {
          id: 'cred_mock1', alias: 'pg-prod', scheme: 'postgres',
          masked_uri: 'postgres://user:***@10.0.0.5:5432/mydb',
          secret_version: 0, secret_created_at: new Date().toISOString(),
          description: '生产数据库', created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
        },
        {
          id: 'cred_mock2', alias: 'redis-cache', scheme: 'redis',
          masked_uri: 'redis://:***@10.0.0.6:6379/0',
          secret_version: 1, secret_created_at: new Date().toISOString(),
          description: '缓存服务器', created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
        },
      ],
      count: 2,
    };
  },
  create: async (alias: string, _connString: string, _desc?: string) => {
    await delay();
    return {
      credential: {
        id: 'cred_' + Math.random().toString(36).slice(2, 8),
        alias, scheme: 'postgres',
        masked_uri: 'postgres://user:***@host:5432/db',
        secret_version: 0, secret_created_at: new Date().toISOString(),
        description: _desc || '', created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
      },
    };
  },
  get: async (id: string) => {
    await delay();
    return { credential: { id, alias: 'pg-prod', scheme: 'postgres', masked_uri: 'postgres://user:***@host:5432/db', secret_version: 0 } };
  },
  delete: async (_id: string) => {
    await delay();
    return { status: 'deleted', message: 'credential deleted' };
  },
  rotate: async (_id: string, _connString: string) => {
    await delay();
    return { credential: { id: _id, alias: 'pg-prod', secret_version: 1 } };
  },
};
