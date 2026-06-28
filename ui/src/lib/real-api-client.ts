/**
 * Real API Client — connects to the Aegis Go backend.
 *
 * All JSON keys from the backend are snake_case, matching our TypeScript interfaces
 * directly (no key transformation needed).
 *
 * Auth: admin cookie session (aegis_admin_session), set via login and
 * automatically included in subsequent requests.
 *
 * Usage:
 *   import { api } from '@/lib/real-api-client';
 *   const nodes = await api.getNodes();
 */

import { API_CONFIG, apiUrl } from './api-config';
import type {
  Node, NodeDetail, Gateway, GatewayDetail,
  TopologyEdge, TopologyPathResult, TopologyPathHop,
  Service, ServiceDetail, RouteSummary,
  Route, RouteDetail,
  Endpoint, EndpointDetail,
  GatewayPolicy,
  RoutingEntry, RoutingCandidate, RoutingPreviewResult, RoutingValidationResult,
  SyncStatus, SyncComponentStatus,
  LocalGatewayStatus, LocalGatewayDiagnostic,
  AcceptanceStatus, VerificationLabel, AcceptanceSummary, AcceptanceRun, AcceptanceTest,
  JoinToken, DashboardData, DashboardError,
  NodeCapabilities, NodeDiagnostic, SyncStatusDetail, LocalGatewayRuntimeStatus,
} from '@/types';

// ─── Error class ───

export class ApiError extends Error {
  status: number;
  code?: string;
  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

// ─── Base fetch wrapper ───

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  opts?: { timeout?: number },
): Promise<T> {
  const url = apiUrl(path);
  const timeout = opts?.timeout ?? API_CONFIG.timeout;

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);

  try {
    const res = await fetch(url, {
      method,
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
      credentials: API_CONFIG.credentials,
      body: body != null ? JSON.stringify(body) : undefined,
      signal: controller.signal,
    });

    clearTimeout(timer);

    if (!res.ok) {
      let msg = `HTTP ${res.status}`;
      try {
        const err = await res.json() as { error?: string | { code?: string; message?: string } };
        if (typeof err.error === 'string') {
          msg = err.error;
        } else if (err.error && typeof err.error === 'object') {
          msg = err.error.message || msg;
        }
      } catch { /* ignore parse failures */ }
      throw new ApiError(msg, res.status);
    }

    // 204 No Content
    if (res.status === 204) return undefined as T;

    return await res.json() as T;
  } catch (err) {
    clearTimeout(timer);
    if (err instanceof ApiError) throw err;
    if ((err as Error).name === 'AbortError') {
      throw new ApiError('请求超时', 408);
    }
    throw new ApiError((err as Error).message || '网络错误', 0);
  }
}

function get<T>(path: string, opts?: { timeout?: number }): Promise<T> {
  return request<T>('GET', path, undefined, opts);
}

function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>('POST', path, body);
}

function patch<T>(path: string, body?: unknown): Promise<T> {
  return request<T>('PATCH', path, body);
}

function del<T = void>(path: string): Promise<T> {
  return request<T>('DELETE', path);
}

// ─── Auth ───

export interface AuthUser {
  id: string;
  username: string;
}

export interface LoginResult {
  user: AuthUser;
  expires_at: string;
}

export const auth = {
  login: (username: string, password: string): Promise<LoginResult> =>
    post<LoginResult>('/api/admin/v1/auth/login', { username, password }),

  logout: (): Promise<{ message: string }> =>
    post('/api/admin/v1/auth/logout'),

  me: (): Promise<{ user: AuthUser }> =>
    get('/api/admin/v1/auth/me'),
};

// ─── System / Overview ───

interface SystemOverviewResponse {
  node_count: number;
  leader_node: string;
  route_count: number;
  edge_rule_count: number;
  service_count: number;
  space_count: number;
  last_apply: {
    status: string;
    version: string;
    created_at: string;
  } | null;
}

interface SystemStatusResponse {
  name: string;
  version: string;
  server_time: string;
  proxy: {
    provider: string;
    config_path: string;
    validate_available: boolean;
    reload_command_configured: boolean;
  };
  store: {
    sqlite_path: string;
    schema_version: string;
  };
  counts: {
    projects: number;
    services: number;
    endpoints: string;
    routes: number;
    managed_domains: number;
  };
  health: {
    healthy_endpoints: number;
    unhealthy_endpoints: number;
    unknown_endpoints: number;
  };
  pending_apply: {
    pending: boolean;
    since: string;
    reason: string;
  };
  last_apply: {
    status: string;
    version: string;
    created_at: string;
  } | null;
}

export const system = {
  overview: (): Promise<SystemOverviewResponse> =>
    get('/api/admin/v1/system/overview'),

  status: (): Promise<SystemStatusResponse> =>
    get('/api/system/status'),

  doctor: (): Promise<{ message: string }> =>
    post('/api/admin/v1/system/doctor'),

  apply: (): Promise<{ message: string; routes: number; warnings: number }> =>
    post('/api/admin/v1/system/apply'),
};

// ─── Dashboard ───
// Build dashboard from system overview + node/gateway counts

export async function fetchDashboard(): Promise<DashboardData> {
  const [ov, st] = await Promise.all([
    system.overview(),
    system.status().catch(() => null),
  ]);

  // Fetch gateways and nodes for richer data
  const [nodesRes, gatewaysRes] = await Promise.all([
    get<{ nodes: any[]; count: number }>('/api/admin/v1/nodes').catch(() => ({ nodes: [], count: 0 })),
    get<{ gateways: any[]; count: number }>('/api/admin/v1/gateways').catch(() => ({ gateways: [], count: 0 })),
  ]);

  const nodes = nodesRes.nodes || [];
  const gateways = gatewaysRes.gateways || [];
  const nodesOnline = nodes.filter((n: any) => n.status === 'online').length;
  const gatewaysOnline = gateways.filter((g: any) => g.status === 'online' || g.status === 'active').length;

  // Build pending capabilities from nodes with issues
  const pendingCapabilities: string[] = [];
  for (const n of nodes) {
    if (n.last_error) pendingCapabilities.push(`${n.node_id}: ${n.last_error}`);
  }

  return {
    nodes_online: nodesOnline,
    nodes_total: nodes.length,
    gateways_online: gatewaysOnline,
    gateways_total: gateways.length,
    managed_routes: ov.route_count,
    routing_tables_synced: 0,
    routing_tables_total: 0,
    local_gateway_online: 0,
    local_gateway_total: 0,
    relay_acceptance: st?.version || '—',
    secret_runtime: '—',
    pending_capabilities: pendingCapabilities.slice(0, 5),
    routes_unavailable: 0,
    missing_gateway_links: 0,
    outdated_nodes: 0,
    recent_errors: nodes
      .filter((n: any) => n.last_error)
      .map((n: any) => ({
        node_id: n.node_id,
        node_name: n.name || n.node_id,
        error: n.last_error,
        last_seen: n.last_heartbeat_at || n.updated_at,
      })),
  };
}

// ─── Nodes ───

interface NodeListResponse {
  nodes: any[];
  count: number;
}

interface NodeDetailResponse {
  node: any;
}

function mapNode(raw: any): Node {
  return {
    node_id: raw.node_id,
    name: raw.name || raw.hostname || raw.node_id,
    hostname: raw.hostname || '',
    public_ip: raw.public_ip || '',
    private_ip: raw.private_ip || raw.local_ip || '',
    roles: raw.role ? [raw.role] : [],
    status: raw.status || 'unknown',
    os: raw.os || '',
    arch: raw.arch || '',
    agent_version: raw.agent_version || '',
    last_heartbeat_at: raw.last_heartbeat_at || null,
    capabilities: (raw.capabilities || {}) as NodeCapabilities,
    desired_revision: raw.desired_revision || 0,
    applied_revision: raw.applied_revision || 0,
    sync_status: (raw.sync_status || 'unknown') as any,
    created_at: raw.created_at || '',
    updated_at: raw.updated_at || '',
  };
}

function mapCapabilities(caps: Record<string, boolean>): NodeCapabilities {
  return {
    gateway_enabled: caps.gateway_enabled ?? caps.GatewayEnabled ?? false,
    caddy_installed: caps.caddy_installed ?? caps.CaddyInstalled ?? false,
    haproxy_installed: caps.haproxy_installed ?? caps.HAProxyInstalled ?? false,
    tls_supported: caps.tls_supported ?? caps.TLSSupported ?? false,
    dns_control_available: caps.dns_control_available ?? false,
    hot_reload_supported: caps.hot_reload_supported ?? false,
    edge_mux_supported: caps.edge_mux_supported ?? false,
    relay_capable: caps.relay_capable ?? false,
    local_gateway_enabled: caps.local_gateway_enabled ?? false,
  };
}

export const nodeApi = {
  list: (): Promise<NodeListResponse> =>
    get('/api/admin/v1/nodes'),

  get: (id: string): Promise<NodeDetailResponse> =>
    get(`/api/admin/v1/nodes/${id}`),

  health: (id: string): Promise<any> =>
    get(`/api/admin/v1/nodes/${id}/health`),

  capabilities: (id: string): Promise<any> =>
    get(`/api/admin/v1/nodes/${id}/capabilities`),

  refreshCapabilities: (id: string): Promise<any> =>
    post(`/api/admin/v1/nodes/${id}/refresh-capabilities`),

  gateways: (id: string): Promise<{ gateways: any[]; count: number }> =>
    get(`/api/admin/v1/nodes/${id}/gateways`),

  syncStatus: (id: string): Promise<any> =>
    get(`/api/admin/v1/nodes/${id}/sync-status`),

  routingTable: (id: string): Promise<any> =>
    get(`/api/admin/v1/nodes/${id}/routing-table`),

  generateRoutingTable: (id: string): Promise<any> =>
    post(`/api/admin/v1/nodes/${id}/routing-table/generate`),
};

export async function fetchNodes(): Promise<Node[]> {
  const res = await nodeApi.list();
  return (res.nodes || []).map(mapNode);
}

export async function fetchNodeDetail(nodeId: string): Promise<NodeDetail> {
  const [nodeRes, gatewaysRes, syncRes] = await Promise.all([
    nodeApi.get(nodeId),
    nodeApi.gateways(nodeId).catch(() => ({ gateways: [] })),
    nodeApi.syncStatus(nodeId).catch(() => null),
  ]);

  const raw = nodeRes.node;
  const node = mapNode(raw);

  // Map synced state
  const syncDetail: SyncStatusDetail = syncRes ? {
    status: syncRes.status || 'unknown',
    desired_revision: syncRes.desired_revision ?? node.desired_revision,
    applied_revision: syncRes.applied_revision ?? node.applied_revision,
    desired_hash: syncRes.desired_hash || '',
    actual_hash: syncRes.actual_hash || '',
    last_apply_at: syncRes.last_apply_at || null,
    last_success_at: syncRes.last_success_at || null,
    last_error: syncRes.last_error || null,
  } : {
    status: 'unknown',
    desired_revision: node.desired_revision,
    applied_revision: node.applied_revision,
    desired_hash: '',
    actual_hash: '',
    last_apply_at: null,
    last_success_at: null,
    last_error: raw.last_error || null,
  };

  const gateways: Gateway[] = (gatewaysRes.gateways || []).map(mapGateway);

  return {
    ...node,
    gateways,
    sync: syncDetail,
    routing_table_entries: 0,
    last_error: raw.last_error || null,
    diagnostics: buildDiagnostics(raw),
  };
}

function buildDiagnostics(raw: any): NodeDiagnostic[] {
  const diag: NodeDiagnostic[] = [];
  if (raw.status === 'online') {
    diag.push({ name: 'heartbeat', status: 'ok', message: '节点可达' });
  } else if (raw.last_heartbeat_at) {
    diag.push({ name: 'heartbeat', status: 'warning', message: '上次心跳: ' + raw.last_heartbeat_at });
  } else {
    diag.push({ name: 'heartbeat', status: 'error', message: '未收到心跳' });
  }
  if (raw.last_error) {
    diag.push({ name: 'last_error', status: 'error', message: raw.last_error });
  }
  return diag;
}

// ─── Gateways ───

function mapGateway(raw: any): Gateway {
  return {
    gateway_id: raw.gateway_id || raw.id,
    node_id: raw.node_id,
    node_name: raw.node_name || '',
    name: raw.name || raw.gateway_id || raw.id,
    type: raw.type || 'local',
    provider: raw.provider || 'aegis',
    bind_addr: raw.bind_addr || '0.0.0.0',
    host: raw.host || '',
    port: raw.port || 0,
    scheme: raw.scheme || 'http',
    public_accessible: !!raw.public_accessible,
    private_accessible: !!raw.private_accessible,
    enabled: raw.enabled !== false,
    priority: raw.priority ?? 50,
    status: raw.status || 'unknown',
    last_verified_at: raw.last_verified_at || null,
    last_error: raw.last_error || null,
    created_at: raw.created_at || '',
    updated_at: raw.updated_at || '',
  };
}

export const gatewayApi = {
  list: (nodeId?: string): Promise<{ gateways: any[]; count: number }> =>
    get(`/api/admin/v1/gateways${nodeId ? '?node_id=' + encodeURIComponent(nodeId) : ''}`),

  get: (id: string): Promise<any> =>
    get(`/api/admin/v1/gateways/${id}`),

  update: (id: string, data: any): Promise<any> =>
    patch(`/api/admin/v1/gateways/${id}`, data),
};

export async function fetchGateways(): Promise<Gateway[]> {
  const res = await gatewayApi.list();
  return (res.gateways || []).map(mapGateway);
}

export async function fetchGatewayDetail(id: string): Promise<GatewayDetail> {
  const raw = await gatewayApi.get(id);
  const gw = mapGateway(raw);
  return {
    ...gw,
    routes_served: raw.routes_served ?? 0,
    gateway_links: raw.gateway_links || [],
  };
}

// ─── Topology ───

export const topologyApi = {
  matrix: (): Promise<{ matrix: any[]; count: number }> =>
    get('/api/admin/v1/topology/matrix'),

  path: (from: string, to: string): Promise<any> =>
    get(`/api/admin/v1/topology/path?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`),
};

export async function fetchTopologyMatrix(): Promise<TopologyEdge[]> {
  const res = await topologyApi.matrix();
  return (res.matrix || []).map((raw: any) => ({
    from_node_id: raw.from_node_id,
    from_node_name: raw.from_node_name || raw.from_node_id,
    to_node_id: raw.to_node_id,
    to_node_name: raw.to_node_name || raw.to_node_id,
    private_reachable: !!raw.private_reachable,
    public_reachable: !!raw.public_reachable,
    preferred_gateway_id: raw.preferred_gateway_id || null,
    gateway_link_id: raw.gateway_link_id || null,
    status: raw.status || 'unknown',
    last_verified_at: raw.last_verified_at || null,
    last_error: raw.last_error || null,
  }));
}

export async function fetchTopologyPath(from: string, to: string): Promise<TopologyPathResult> {
  const raw = await topologyApi.path(from, to);
  const hops: TopologyPathHop[] = [
    {
      hop: 0,
      node_id: raw.from_node_id,
      node_name: raw.from_node_id,
      via: 'local',
      gateway_url: null,
      gateway_link_id: null,
      status: 'ok',
      reason: null,
    },
  ];

  if (raw.preferred_gateway_id) {
    hops.push({
      hop: 1,
      node_id: raw.to_node_id,
      node_name: raw.to_node_id,
      via: raw.public_reachable ? 'public_gateway' : 'private_gateway',
      gateway_url: null,
      gateway_link_id: raw.gateway_link_id || null,
      status: raw.public_reachable || raw.private_reachable ? 'ok' : 'blocked',
      reason: raw.last_error || null,
    });
  }

  return {
    from_node: raw.from_node_id,
    to_node: raw.to_node_id,
    path: hops,
    reachable: raw.public_reachable || raw.private_reachable || false,
    total_hops: hops.length,
    summary: raw.status || 'unknown',
  };
}

// ─── Services ───

function mapService(raw: any): Service {
  return {
    service_id: raw.id || raw.service_id,
    name: raw.name || raw.id,
    kind: raw.kind || 'http',
    scope_id: raw.space_id || raw.scope_id || null,
    upstream_url: raw.upstream_url || null,
    health_check_url: raw.health_check_url || null,
    status: raw.status || 'unknown',
    health_status: raw.health_status || 'unknown',
    routes_count: raw.routes_count ?? 0,
    endpoints_count: raw.endpoints_count ?? 0,
    latency_ms: raw.latency_ms ?? null,
    created_at: raw.created_at || '',
    updated_at: raw.updated_at || '',
  };
}

export const serviceApi = {
  list: (): Promise<{ services: any[]; count: number }> =>
    get('/api/admin/v1/services'),

  get: (id: string): Promise<any> =>
    get(`/api/v1/services/${id}`).catch(() =>
      get(`/api/admin/v1/services/${id}`)),

  getPolicy: (id: string): Promise<any> =>
    get(`/api/admin/v1/services/${id}/gateway-policy`),

  setPolicy: (id: string, policy: any): Promise<any> =>
    patch(`/api/admin/v1/services/${id}/gateway-policy`, policy),
};

export async function fetchServices(): Promise<Service[]> {
  const res = await serviceApi.list();
  return (res.services || []).map(mapService);
}

export async function fetchServiceDetail(id: string): Promise<ServiceDetail> {
  const raw = await serviceApi.get(id);
  const svc = mapService(raw);

  // Fetch endpoints for this service
  const epsRes = await get<{ gateways?: any[] } | any[]>(`/api/v1/services/${id}/endpoints`)
    .catch(() => []);

  const endpoints: Endpoint[] = (Array.isArray(epsRes) ? epsRes : []).map((ep: any) => ({
    endpoint_id: ep.id || ep.endpoint_id,
    service_id: id,
    node_id: ep.node_id || '',
    node_name: ep.node_name || '',
    protocol: 'http',
    target_local_host: ep.address ? ep.address.split(':')[0] : '',
    target_local_port: ep.address ? parseInt(ep.address.split(':')[1] || '0') : 0,
    address_type: ep.type === 'remote' ? 'remote' : 'local',
    relay_eligible: ep.type !== 'public',
    health_status: ep.enabled ? 'healthy' : 'unknown',
    latency_ms: null,
    last_checked_at: null,
  }));

  // Fetch routes for this service
  const routesRes = await get<{ routes: any[]; count: number }>('/api/admin/v1/routes')
    .catch(() => ({ routes: [], count: 0 }));
  const routes: RouteSummary[] = (routesRes.routes || [])
    .filter((r: any) => r.service_id === id)
    .map((r: any) => ({
      route_id: r.id || r.route_id,
      domain: r.domain || '',
      status: r.status || 'unknown',
    }));

  // Fetch gateway policy
  const policy = await serviceApi.getPolicy(id).catch(() => null);

  return {
    ...svc,
    routes,
    endpoints,
    gateway_policy: policy ? mapPolicy(policy) : null,
  };
}

// ─── Routes ───

function mapRoute(raw: any): Route {
  return {
    route_id: raw.id || raw.route_id,
    domain: raw.domain || '',
    service_id: raw.service_id || '',
    service_name: raw.service_name || '',
    scope_id: raw.space_id || raw.scope_id || null,
    tls_mode: raw.tls_enabled ? 'terminate_local' : 'http_only',
    preserve_host: !raw.strip_prefix,
    public_allowed: raw.public_allowed ?? true,
    status: raw.status || 'unknown',
    gateway_policy: null,
    created_at: raw.created_at || '',
    updated_at: raw.updated_at || '',
  };
}

export const routeApi = {
  list: (): Promise<{ routes: any[]; count: number }> =>
    get('/api/admin/v1/routes'),

  get: (id: string): Promise<any> =>
    get(`/api/v1/routes/${id}`).catch(() =>
      get(`/api/admin/v1/routes/${id}`)),

  getPolicy: (id: string): Promise<any> =>
    get(`/api/admin/v1/routes/${id}/gateway-policy`),
};

export async function fetchRoutes(): Promise<Route[]> {
  const res = await routeApi.list();
  return (res.routes || []).map(mapRoute);
}

export async function fetchRouteDetail(id: string): Promise<RouteDetail> {
  const raw = await routeApi.get(id);
  const route = mapRoute(raw);

  // Fetch gateway policy
  const policy = await routeApi.getPolicy(id).catch(() => null);

  return {
    ...route,
    endpoint: null,
    policy_summary: policy?.mode || 'auto',
    routing_status: raw.status === 'active' ? 'available'
      : raw.maintenance_enabled ? 'unavailable'
      : 'unknown',
    gateway_policy: policy ? mapPolicy(policy) : null,
  };
}

// ─── Endpoints ───

export async function fetchEndpoints(): Promise<Endpoint[]> {
  // Gather endpoints from services
  const svcRes = await serviceApi.list().catch(() => ({ services: [], count: 0 }));
  const endpoints: Endpoint[] = [];

  for (const svc of (svcRes.services || [])) {
    const eps = await get<any[]>(`/api/v1/services/${svc.id}/endpoints`).catch(() => []);
    for (const ep of (Array.isArray(eps) ? eps : [])) {
      endpoints.push({
        endpoint_id: ep.id || ep.endpoint_id,
        service_id: svc.id,
        node_id: ep.node_id || '',
        node_name: ep.node_name || '',
        protocol: 'http',
        target_local_host: ep.address ? ep.address.split(':')[0] : '',
        target_local_port: ep.address ? parseInt(ep.address.split(':')[1] || '0') : 0,
        address_type: ep.type === 'remote' ? 'remote' : 'local',
        relay_eligible: ep.type !== 'public',
        health_status: ep.enabled ? 'healthy' : 'unknown',
        latency_ms: null,
        last_checked_at: null,
      });
    }
  }

  return endpoints;
}

export async function fetchEndpointDetail(id: string): Promise<EndpointDetail> {
  // Try to find from services
  const all = await fetchEndpoints();
  const ep = all.find((e) => e.endpoint_id === id);
  if (!ep) throw new ApiError(`端点 ${id} 未找到`, 404);

  return {
    ...ep,
    routes: [],
  };
}

// ─── Policies ───

function mapPolicy(raw: any): GatewayPolicy {
  return {
    id: raw.policy_id || raw.id || '',
    target_type: raw.service_id ? 'service' as const : 'route' as const,
    target_id: raw.service_id || raw.route_id || '',
    target_name: '',
    mode: raw.mode || 'auto',
    primary_gateway_id: raw.primary_gateway_id || null,
    fallback_gateway_ids: raw.fallback_gateway_ids || [],
    allow_local: !!raw.allow_local,
    allow_private: !!raw.allow_private,
    allow_public: !!raw.allow_public,
    require_gateway_link: !!raw.require_gateway_link,
    require_relay: !!raw.require_relay,
    preserve_host: !!raw.preserve_host,
    tls_mode: raw.tls_mode || 'http_only',
    enabled: raw.enabled !== false,
    priority: raw.priority ?? 0,
    created_at: raw.created_at || '',
    updated_at: raw.updated_at || '',
  };
}

export const policyApi = {
  listServicePolicies: (): Promise<any[]> =>
    get('/api/admin/v1/services').then(async (res: any) => {
      const policies: any[] = [];
      for (const svc of (res.services || []).slice(0, 20)) {
        const p = await get(`/api/admin/v1/services/${svc.id}/gateway-policy`).catch(() => null);
        if (p) policies.push(p);
      }
      return policies;
    }),

  listRoutePolicies: (): Promise<any[]> =>
    get('/api/admin/v1/routes').then(async (res: any) => {
      const policies: any[] = [];
      for (const r of (res.routes || []).slice(0, 20)) {
        const p = await get(`/api/admin/v1/routes/${r.id}/gateway-policy`).catch(() => null);
        if (p) policies.push(p);
      }
      return policies;
    }),
};

export async function fetchPolicies(): Promise<GatewayPolicy[]> {
  const [svcPolicies, routePolicies] = await Promise.all([
    policyApi.listServicePolicies(),
    policyApi.listRoutePolicies(),
  ]);

  const all = [...svcPolicies, ...routePolicies];
  return all.map(mapPolicy);
}

// ─── Routing Table ───

export const routingApi = {
  preview: (domain: string, fromNode: string): Promise<any> =>
    get(`/api/admin/v1/routing/preview?domain=${encodeURIComponent(domain)}&from_node_id=${encodeURIComponent(fromNode)}`),

  validate: (nodeId: string): Promise<any> =>
    get(`/api/admin/v1/routing/validate?from_node_id=${encodeURIComponent(nodeId)}`),
};

export async function fetchRoutingTable(nodeId?: string): Promise<RoutingEntry[]> {
  if (!nodeId) {
    const nodes = await fetchNodes();
    if (nodes.length === 0) return [];
    nodeId = nodes[0].node_id;
  }

  const raw = await nodeApi.routingTable(nodeId).catch(() => null);
  if (!raw || !raw.entries) return [];

  return (raw.entries || []).map((e: any) => ({
    domain: e.domain || '',
    route_id: e.route_id || '',
    service_id: e.service_id || '',
    endpoint_id: e.endpoint_id || '',
    from_node_id: nodeId || '',
    target_node_id: e.target_node_id || '',
    protocol: e.protocol || 'http',
    target_local_host: e.target_local_host || '',
    target_local_port: e.target_local_port || 0,
    policy_mode: e.policy_mode || 'auto',
    candidates: (e.candidates || []).map((c: any) => ({
      mode: c.mode || 'local_gateway',
      gateway_id: c.gateway_id || null,
      gateway_url: c.gateway_url || null,
      priority: c.priority ?? 0,
      requires_gateway_link: !!c.requires_gateway_link,
      gateway_link_id: c.gateway_link_id || null,
    })),
    status: e.status || 'unknown',
    unavailable_reason: e.unavailable_reason || null,
  }));
}

export async function previewRouting(domain: string, fromNode: string): Promise<RoutingPreviewResult> {
  const raw = await routingApi.preview(domain, fromNode);

  return {
    domain: raw.domain || domain,
    from_node_id: raw.from_node_id || fromNode,
    from_node_name: raw.from_node_name || fromNode,
    entries: (raw.entries || []).map((e: any) => ({
      domain: e.domain || '',
      route_id: e.route_id || '',
      service_id: e.service_id || '',
      endpoint_id: e.endpoint_id || '',
      from_node_id: fromNode,
      target_node_id: e.target_node_id || '',
      protocol: e.protocol || 'http',
      target_local_host: e.target_local_host || '',
      target_local_port: e.target_local_port || 0,
      policy_mode: e.policy_mode || 'auto',
      candidates: (e.candidates || []).map((c: any) => ({
        mode: c.mode || 'local_gateway',
        gateway_id: c.gateway_id || null,
        gateway_url: c.gateway_url || null,
        priority: c.priority ?? 0,
        requires_gateway_link: !!c.requires_gateway_link,
        gateway_link_id: c.gateway_link_id || null,
      })),
      status: e.status || 'unknown',
      unavailable_reason: e.unavailable_reason || null,
    })),
    available: (raw.entries || []).length > 0,
    summary: raw.summary || '',
    unavailable_reason: raw.unavailable_reason || null,
  };
}

export async function validateRouting(nodeId?: string): Promise<RoutingValidationResult> {
  if (!nodeId) {
    const nodes = await fetchNodes();
    if (nodes.length === 0) {
      return { valid: true, node_id: null, total_entries: 0, errors: [], warnings: [], valid_count: 0, error_count: 0, warning_count: 0 };
    }
    nodeId = nodes[0].node_id;
  }

  const raw = await routingApi.validate(nodeId).catch(() => null);
  if (!raw) {
    return { valid: true, node_id: nodeId, total_entries: 0, errors: [], warnings: [], valid_count: 0, error_count: 0, warning_count: 0 };
  }

  return {
    valid: raw.is_valid ?? raw.valid ?? false,
    node_id: raw.node_id || nodeId,
    total_entries: raw.entry_count ?? raw.total_entries ?? 0,
    errors: (raw.errors || []).map((e: any) => ({
      domain: e.domain || '',
      code: e.code || '',
      message: e.message || '',
    })),
    warnings: (raw.warnings || []).map((w: any) => ({
      domain: w.domain || '',
      code: w.code || '',
      message: w.message || '',
    })),
    valid_count: raw.valid_count ?? 0,
    error_count: raw.error_count ?? 0,
    warning_count: raw.warning_count ?? 0,
  };
}

// ─── Sync Status ───

export const syncApi = {
  getNodeSync: (nodeId: string): Promise<any> =>
    get(`/api/admin/v1/nodes/${nodeId}/sync-status`),
};

export async function fetchSyncStatus(nodeId?: string): Promise<SyncStatus[]> {
  const nodes = await fetchNodes();

  if (nodeId) {
    const node = nodes.find((n) => n.node_id === nodeId);
    const sync = await syncApi.getNodeSync(nodeId).catch(() => null);
    return [{
      node_id: nodeId,
      node_name: node?.name || nodeId,
      desired_revision: sync?.desired_revision ?? 0,
      applied_revision: sync?.applied_revision ?? 0,
      desired_hash: sync?.desired_hash || '',
      actual_hash: sync?.actual_hash || '',
      status: sync?.status || 'unknown',
      last_apply_at: sync?.last_apply_at || null,
      last_success_at: sync?.last_success_at || null,
      last_error: sync?.last_error || null,
      provider_status: { status: 'unknown', message: '' },
      relay_status: { status: 'unknown', message: '' },
      gateway_status: { status: 'unknown', message: '' },
      diagnostics_status: { status: 'unknown', message: '' },
    }];
  }

  const results = await Promise.all(
    nodes.map(async (n) => {
      const sync = await syncApi.getNodeSync(n.node_id).catch(() => null);
      return {
        node_id: n.node_id,
        node_name: n.name,
        desired_revision: sync?.desired_revision ?? 0,
        applied_revision: sync?.applied_revision ?? 0,
        desired_hash: sync?.desired_hash || '',
        actual_hash: sync?.actual_hash || '',
        status: sync?.status || 'unknown',
        last_apply_at: sync?.last_apply_at || null,
        last_success_at: sync?.last_success_at || null,
        last_error: sync?.last_error || null,
        provider_status: { status: 'unknown' as const, message: '' },
        relay_status: { status: 'unknown' as const, message: '' },
        gateway_status: { status: 'unknown' as const, message: '' },
        diagnostics_status: { status: 'unknown' as const, message: '' },
      };
    }),
  );

  return results;
}

// ─── Local Gateway ───
// Backend doesn't have a direct local-gateway API yet, derive from nodes/gateways

export async function fetchLocalGatewayStatus(nodeId?: string): Promise<LocalGatewayStatus[]> {
  const nodes = await fetchNodes();
  const gateways = await fetchGateways();

  const localGws = gateways.filter((g) => g.type === 'local');
  const results: LocalGatewayStatus[] = [];

  for (const gw of localGws) {
    const node = nodes.find((n) => n.node_id === gw.node_id);
    if (nodeId && gw.node_id !== nodeId) continue;

    results.push({
      node_id: gw.node_id,
      node_name: node?.name || gw.node_id,
      bind_addr: gw.bind_addr,
      port: gw.port,
      status: gw.enabled ? 'running' : 'stopped',
      routing_table_loaded: false,
      routing_table_revision: null,
      entries_count: 0,
      cache_status: 'empty',
      diagnostics: [],
      last_error: gw.last_error,
    });
  }

  return results;
}

// ─── Acceptance ───
// Backend doesn't have a consolidated acceptance endpoint; build from verification status

export async function fetchAcceptance(): Promise<AcceptanceStatus> {
  const [statusRes, safetyRes] = await Promise.all([
    system.status().catch(() => null),
    get<any[]>('/api/admin/v1/routes/safety').catch(() => []),
  ]);

  const labels: VerificationLabel[] = [
    {
      key: 'two_node_verified',
      label: '双节点已验证',
      status: safetyRes && safetyRes.length > 0 ? 'pass' : 'pending',
      evidence: safetyRes ? `${safetyRes.length} 条路由已检查` : '无数据',
    },
    {
      key: 'local_gateway_verified',
      label: '本地网关',
      status: statusRes?.proxy?.provider ? 'pass' : 'pending',
      evidence: `提供者: ${statusRes?.proxy?.provider || '—'}`,
    },
    {
      key: 'secret_runtime_code_verified',
      label: '密钥运行时代码',
      status: 'deferred',
      evidence: '延期至 v1.9',
    },
    {
      key: 'https_deferred',
      label: 'HTTPS',
      status: 'deferred',
      evidence: '延期至 v1.9',
    },
    {
      key: 'raw_tcp_deferred',
      label: 'Raw TCP',
      status: 'deferred',
      evidence: '延期至 v2.0',
    },
  ];

  return {
    labels,
    summary: {
      total_labels: labels.length,
      pass_count: labels.filter((l) => l.status === 'pass').length,
      pending_count: labels.filter((l) => l.status === 'pending').length,
      deferred_count: labels.filter((l) => l.status === 'deferred').length,
    },
    last_acceptance: null,
    negative_smoke: [],
  };
}

// ─── Join Tokens ───

export const joinTokenApi = {
  list: (): Promise<{ join_tokens: any[]; count: number }> =>
    get('/api/admin/v1/node-join-tokens'),

  create: (data: any): Promise<any> =>
    post('/api/admin/v1/node-join-tokens', data),

  revoke: (id: string): Promise<any> =>
    post(`/api/admin/v1/node-join-tokens/${id}/revoke`),
};

export async function fetchJoinTokens(): Promise<JoinToken[]> {
  const res = await joinTokenApi.list();
  return (res.join_tokens || []).map((raw: any) => ({
    id: raw.id,
    name: raw.name || raw.id,
    token_prefix: raw.token_redacted ? (raw.raw_join_token || '').slice(0, 12) + '…' : '',
    allowed_roles: raw.allowed_roles || [],
    expected_node_name: raw.expected_node_name || null,
    expires_at: raw.expires_at || '',
    allowed_source_cidr: raw.allowed_source_cidr || null,
    status: raw.revoked_at ? 'revoked' : (raw.used_at ? 'revoked' : 'active'),
    created_at: raw.created_at || '',
    used_at: raw.used_at || null,
  }));
}

export async function createJoinToken(data: Partial<JoinToken>): Promise<JoinToken & { rawToken?: string }> {
  const raw = await joinTokenApi.create({
    name: data.name || 'new-token',
    allowed_roles: data.allowed_roles || ['gateway'],
    expected_node_name: data.expected_node_name || null,
    allowed_source_cidr: data.allowed_source_cidr || null,
  });

  return {
    id: raw.id,
    name: raw.name,
    token_prefix: (raw.raw_join_token || '').slice(0, 12) + '…',
    allowed_roles: data.allowed_roles || ['gateway'],
    expected_node_name: data.expected_node_name || null,
    expires_at: raw.expires_at || '',
    allowed_source_cidr: data.allowed_source_cidr || null,
    status: 'active',
    created_at: raw.created_at || '',
    used_at: null,
    rawToken: raw.raw_join_token,
  };
}

export async function revokeJoinToken(id: string): Promise<void> {
  await joinTokenApi.revoke(id);
}

// ─── Settings ───

export async function fetchSettings(): Promise<Record<string, any>> {
  const [settings, statusRes] = await Promise.all([
    get<any>('/api/admin/v1/settings').catch(() => ({})),
    system.status().catch(() => null),
  ]);

  return {
    admin: {
      username: 'admin',
      session_timeout: '24h',
      auth_mode: 'password + cookie',
    },
    node_identity: {
      current_node_id: '—',
      private_ip: '—',
      public_ip: '—',
    },
    gateway_defaults: {
      default_listener: '0.0.0.0:80',
      default_provider: statusRes?.proxy?.provider || 'caddy',
      gateway_mode: 'edge_mux + caddy',
    },
    relay_defaults: {
      default_mode: 'public_gateway',
      max_hop: 1,
      target_suppressed: true,
    },
    safety_defaults: {
      warn_mode: 'log only',
      block_mode: 'disabled',
      auto_detect_public_target: true,
    },
    logging: {
      log_level: 'info',
      operation_log_retention: '30d',
      audit_log_retention: '90d',
    },
    secret_key: {
      key_path: '/etc/aegis/secret.key',
      key_format: 'PEM',
      key_rotation: 'manual',
    },
    ...settings,
  };
}

// ─── DNS (v1.8E) ───

import type { DnsStatus } from '@/types';

export const dnsApi = {
  status: (detail?: boolean) =>
    get<DnsStatus>(`/api/admin/v1/dns/status${detail ? '?detail=1' : ''}`),

  enable: () =>
    post<Record<string, any>>('/api/admin/v1/dns/enable'),

  disable: () =>
    post<Record<string, any>>('/api/admin/v1/dns/disable'),

  refresh: () =>
    post<Record<string, any>>('/api/admin/v1/dns/refresh'),
};

// ─── Trace (v1.8A) ───

export interface TraceStep {
  order: number;
  component: string;
  name: string;
  status: string;
  detail: string;
  address?: string;
}

export interface TraceResult {
  input: string;
  input_type: string;
  trace_status: string;
  steps: TraceStep[];
  final_target?: {
    host: string;
    port: number;
    protocol: string;
    reachable?: boolean;
  };
  warnings?: string[];
  errors?: string[];
  gateway_link?: {
    link_id: string;
    enabled: boolean;
    token_version: number;
    verification_mode: string;
  };
}

export const traceApi = {
  byDomain: (domain: string): Promise<TraceResult> =>
    get(`/api/admin/v1/trace/domain/${encodeURIComponent(domain)}`),

  byRoute: (routeId: string): Promise<TraceResult> =>
    get(`/api/admin/v1/trace/route/${encodeURIComponent(routeId)}`),

  bySNI: (sni: string): Promise<TraceResult> =>
    get(`/api/admin/v1/trace/sni/${encodeURIComponent(sni)}`),

  egress: (domain: string, fromNode: string): Promise<any> =>
    get(`/api/admin/v1/trace/egress?domain=${encodeURIComponent(domain)}&from_node=${encodeURIComponent(fromNode)}`),
};

// ─── Safety (v1.8A) ───

export const safetyApi = {
  checkAllRoutes: (): Promise<any[]> =>
    get('/api/admin/v1/routes/safety'),

  checkRoute: (id: string): Promise<any> =>
    get(`/api/admin/v1/routes/${id}/safety`),

  traceEgress: (domain: string, fromNode: string): Promise<any> =>
    get(`/api/admin/v1/trace/egress?domain=${encodeURIComponent(domain)}&from_node=${encodeURIComponent(fromNode)}`),
};

// ─── Relay (v1.8B) ───

export const relayApi = {
  resolve: (domain: string, fromNode: string): Promise<any> =>
    get(`/api/admin/v1/relay/resolve?domain=${encodeURIComponent(domain)}&from_node=${encodeURIComponent(fromNode)}`),
};

// ─── Gateway Links (v1.7AB) ───

export const gatewayLinkApi = {
  list: (): Promise<any[]> =>
    get('/api/admin/v1/gateway-links'),

  get: (id: string): Promise<any> =>
    get(`/api/admin/v1/gateway-links/${id}`),

  create: (data: any): Promise<any> =>
    post('/api/admin/v1/gateway-links', data),

  delete: (id: string): Promise<any> =>
    del(`/api/admin/v1/gateway-links/${id}`),

  rotate: (id: string): Promise<any> =>
    post(`/api/admin/v1/gateway-links/${id}/rotate`),
};

// ─── Providers (v1.7S) ───

export const providerApi = {
  list: (): Promise<{ providers: any[]; count: number }> =>
    get('/api/admin/v1/providers'),

  diagnoseAll: (): Promise<any> =>
    post('/api/admin/v1/providers/diagnose'),
};

// ─── Admin operations ───

export const adminApi = {
  // Scopes
  listScopes: (): Promise<{ spaces: any[]; count: number }> =>
    get('/api/admin/v1/scopes'),

  createScope: (data: any): Promise<any> =>
    post('/api/admin/v1/scopes', data),

  // API Keys
  listApiKeys: (scopeId?: string): Promise<{ api_keys: any[]; count: number }> =>
    get(`/api/admin/v1/api-keys${scopeId ? '?scope_id=' + encodeURIComponent(scopeId) : ''}`),

  createApiKey: (scopeId: string, name: string): Promise<any> =>
    post(`/api/admin/v1/scopes/${scopeId}/api-keys`, { name }),

  revokeApiKey: (id: string): Promise<any> =>
    post(`/api/admin/v1/api-keys/${id}/revoke`),

  rotateApiKey: (id: string): Promise<any> =>
    post(`/api/admin/v1/api-keys/${id}/rotate`),

  // Logs
  listOperations: (): Promise<{ operations: any[]; count: number }> =>
    get('/api/admin/v1/operations'),

  listApplyLogs: (): Promise<{ apply_logs: any[]; count: number }> =>
    get('/api/admin/v1/apply-logs'),

  listAuditLogs: (): Promise<{ audit_logs: any[]; count: number }> =>
    get('/api/admin/v1/audit-logs'),

  listNodeEvents: (): Promise<{ node_events: any[]; count: number }> =>
    get('/api/admin/v1/node-events'),

  // Edge Rules
  listEdgeRules: (): Promise<{ edge_rules: any[]; count: number }> =>
    get('/api/admin/v1/edge-rules'),

  // Config / Apply
  configCurrent: (): Promise<any> =>
    get('/api/v1/config/current'),

  configPreview: (): Promise<any> =>
    get('/api/v1/config/preview'),

  configDiff: (): Promise<any> =>
    get('/api/v1/config/diff'),

  applyHistory: (): Promise<any[]> =>
    get('/api/v1/apply/history'),

  applyConfig: (): Promise<any> =>
    post('/api/v1/config/apply'),

  dryRun: (): Promise<any> =>
    post('/api/v1/config/dry-run'),

  rollback: (): Promise<any> =>
    post('/api/v1/config/rollback'),

  // Actions
  bindHTTPDomain: (data: any): Promise<any> =>
    post('/api/v1/actions/bind-http-domain', data),

  bindTLSBackend: (data: any): Promise<any> =>
    post('/api/v1/actions/bind-tls-backend', data),

  updateTarget: (data: any): Promise<any> =>
    patch('/api/v1/actions/update-target', data),

  // Diagnostics
  exportDiagnostics: (): Promise<any> =>
    get('/api/admin/v1/diagnostics/export'),

  // v1.8D Import — Caddyfile
  importCaddyPreview: (): Promise<any> =>
    get('/api/admin/v1/import/caddy/preview'),

  importCaddyConfirm: (routes: any[]): Promise<any> =>
    post('/api/admin/v1/import/caddy/confirm', { routes }),
};

// ─── Listeners ───
// Derived from gateways

export async function fetchListeners(): Promise<any[]> {
  const gateways = await fetchGateways();
  return gateways.map((gw) => ({
    bind_ip: gw.bind_addr,
    port: gw.port,
    provider: gw.provider,
    purpose: gw.type === 'local' ? 'local_gateway' : gw.type === 'private' ? 'private_gateway' : 'public_gateway',
    status: gw.enabled ? 'active' : 'disabled',
    gateway_id: gw.gateway_id,
    node_id: gw.node_id,
  }));
}
