/**
 * API Bridge — switches between mock and real API client.
 *
 * All pages import from here instead of directly from api-client or real-api-client.
 * Toggle with VITE_USE_MOCK env var or API_CONFIG.useMock.
 */

import { API_CONFIG } from './api-config';
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

// ─── Mock Client ───
import {
  fetchDashboard as mockDashboard,
  fetchNodes as mockFetchNodes,
  fetchNodeDetail as mockFetchNodeDetail,
  fetchGateways as mockFetchGateways,
  fetchGatewayDetail as mockFetchGatewayDetail,
  fetchTopologyMatrix as mockFetchTopologyMatrix,
  fetchTopologyPath as mockFetchTopologyPath,
  fetchServices as mockFetchServices,
  fetchServiceDetail as mockFetchServiceDetail,
  fetchRoutes as mockFetchRoutes,
  fetchRouteDetail as mockFetchRouteDetail,
  fetchEndpoints as mockFetchEndpoints,
  fetchEndpointDetail as mockFetchEndpointDetail,
  fetchPolicies as mockFetchPolicies,
  fetchRoutingTable as mockFetchRoutingTable,
  previewRouting as mockPreviewRouting,
  validateRouting as mockValidateRouting,
  fetchSyncStatus as mockFetchSyncStatus,
  fetchLocalGatewayStatus as mockFetchLocalGatewayStatus,
  fetchAcceptance as mockFetchAcceptance,
  fetchJoinTokens as mockFetchJoinTokens,
  createJoinToken as mockCreateJoinToken,
  revokeJoinToken as mockRevokeJoinToken,
  fetchSettings as mockFetchSettings,
  updateSettings as mockUpdateSettings,
} from './api-client';

// ─── Real Client ───
import {
  fetchDashboard as realDashboard,
  fetchNodes as realFetchNodes,
  fetchNodeDetail as realFetchNodeDetail,
  fetchGateways as realFetchGateways,
  fetchGatewayDetail as realFetchGatewayDetail,
  fetchTopologyMatrix as realFetchTopologyMatrix,
  fetchTopologyPath as realFetchTopologyPath,
  fetchServices as realFetchServices,
  fetchServiceDetail as realFetchServiceDetail,
  fetchRoutes as realFetchRoutes,
  fetchRouteDetail as realFetchRouteDetail,
  fetchEndpoints as realFetchEndpoints,
  fetchEndpointDetail as realFetchEndpointDetail,
  fetchPolicies as realFetchPolicies,
  fetchRoutingTable as realFetchRoutingTable,
  previewRouting as realPreviewRouting,
  validateRouting as realValidateRouting,
  fetchSyncStatus as realFetchSyncStatus,
  fetchLocalGatewayStatus as realFetchLocalGatewayStatus,
  fetchAcceptance as realFetchAcceptance,
  fetchJoinTokens as realFetchJoinTokens,
  createJoinToken as realCreateJoinToken,
  revokeJoinToken as realRevokeJoinToken,
  fetchSettings as realFetchSettings,
  updateSettings as realUpdateSettings,
  auth as realAuth,
  system as realSystem,
  safetyApi as realSafetyApi,
  traceApi as realTraceApi,
  relayApi as realRelayApi,
  gatewayApi as realGatewayApi,
  gatewayLinkApi as realGatewayLinkApi,
  nodeApi as realNodeApi,
  providerApi as realProviderApi,
  exposureApi as realExposureApi,
  adminApi as realAdminApi,
  fetchListeners as realFetchListeners,
  dnsApi as realDnsApi,
  transparentApi as realTransparentApi,
  clusterHealthApi as realClusterHealthApi,
  portCheckApi as realPortCheckApi,
  systemHealthApi as realSystemHealthApi,
  healthCheckApi as realHealthCheckApi,
} from './real-api-client';

import type { auth as AuthType, system as SystemType } from './real-api-client';

// ─── Auto-select based on config ───

const useMock = API_CONFIG.useMock;

// ─── Data fetching exports ───

export const fetchDashboard: typeof realDashboard = useMock ? mockDashboard : realDashboard;
export const fetchNodes: typeof realFetchNodes = useMock ? mockFetchNodes : realFetchNodes;
export const fetchNodeDetail: typeof realFetchNodeDetail = useMock ? mockFetchNodeDetail : realFetchNodeDetail;
export const fetchGateways: typeof realFetchGateways = useMock ? mockFetchGateways : realFetchGateways;
export const fetchGatewayDetail: typeof realFetchGatewayDetail = useMock ? mockFetchGatewayDetail : realFetchGatewayDetail;
export const fetchTopologyMatrix: typeof realFetchTopologyMatrix = useMock ? mockFetchTopologyMatrix : realFetchTopologyMatrix;
export const fetchTopologyPath: typeof realFetchTopologyPath = useMock ? mockFetchTopologyPath : realFetchTopologyPath;
export const fetchServices: typeof realFetchServices = useMock ? mockFetchServices : realFetchServices;
export const fetchServiceDetail: typeof realFetchServiceDetail = useMock ? mockFetchServiceDetail : realFetchServiceDetail;
export const fetchRoutes: typeof realFetchRoutes = useMock ? mockFetchRoutes : realFetchRoutes;
export const fetchRouteDetail: typeof realFetchRouteDetail = useMock ? mockFetchRouteDetail : realFetchRouteDetail;
export const fetchEndpoints: typeof realFetchEndpoints = useMock ? mockFetchEndpoints : realFetchEndpoints;
export const fetchEndpointDetail: typeof realFetchEndpointDetail = useMock ? mockFetchEndpointDetail : realFetchEndpointDetail;
export const fetchPolicies: typeof realFetchPolicies = useMock ? mockFetchPolicies : realFetchPolicies;
export const fetchRoutingTable: typeof realFetchRoutingTable = useMock ? mockFetchRoutingTable : realFetchRoutingTable;
export const previewRouting: typeof realPreviewRouting = useMock ? mockPreviewRouting : realPreviewRouting;
export const validateRouting: typeof realValidateRouting = useMock ? mockValidateRouting : realValidateRouting;
export const fetchSyncStatus: typeof realFetchSyncStatus = useMock ? mockFetchSyncStatus : realFetchSyncStatus;
export const fetchLocalGatewayStatus: typeof realFetchLocalGatewayStatus = useMock ? mockFetchLocalGatewayStatus : realFetchLocalGatewayStatus;
export const fetchAcceptance: typeof realFetchAcceptance = useMock ? mockFetchAcceptance : realFetchAcceptance;
export const fetchJoinTokens: typeof realFetchJoinTokens = useMock ? mockFetchJoinTokens : realFetchJoinTokens;
export const createJoinToken: typeof realCreateJoinToken = useMock ? mockCreateJoinToken : realCreateJoinToken;
export const revokeJoinToken: typeof realRevokeJoinToken = useMock ? mockRevokeJoinToken : realRevokeJoinToken;
export const fetchSettings: typeof realFetchSettings = useMock ? mockFetchSettings : realFetchSettings;
export const updateSettings: typeof realUpdateSettings = useMock ? mockUpdateSettings : realUpdateSettings;
export const fetchListeners: typeof realFetchListeners = useMock
  ? async () => {
      const g = await import('@/mocks/gateways');
      return g.mockGateways.map((gw: any) => ({
        bind_ip: gw.bind_addr,
        port: gw.port,
        provider: gw.provider,
        purpose: gw.type === 'local' ? 'local_gateway' : gw.type === 'private' ? 'private_gateway' : 'public_gateway',
        status: gw.enabled ? 'active' : 'disabled',
        gateway_id: gw.gateway_id,
        node_id: gw.node_id,
      }));
    }
  : realFetchListeners;

// ─── DNS (v1.8E) ───
export const dnsApi = useMock
  ? {
      status: async () => ({
        running: false,
        listen_addr: ':5353',
        upstream: '1.1.1.1:53',
        enabled: false,
        local_hits: 0,
        upstream_calls: 0,
        managed_count: 0,
      }),
      enable: async () => ({}),
      disable: async () => ({}),
      refresh: async () => ({}),
    }
  : realDnsApi;

// ─── Transparent Proxy (v1.8F) ───
import { transparentApi as mockTransparentApi } from './api-client';
export const transparentApi = useMock ? mockTransparentApi : realTransparentApi;

// ─── Cluster Health (v1.8G) ───
import { clusterHealthApi as mockClusterHealthApi } from './api-client';
export const clusterHealthApi = useMock ? mockClusterHealthApi : realClusterHealthApi;

// ─── Port Conflict Detection (v1.8G) ───
import { portCheckApi as mockPortCheckApi } from './api-client';
export const portCheckApi = useMock ? mockPortCheckApi : realPortCheckApi;

// ─── System Health (v1.8G) ───
import { systemHealthApi as mockSystemHealthApi } from './api-client';
export const systemHealthApi = useMock ? mockSystemHealthApi : realSystemHealthApi;

// ─── Health Check Actions ───
import { healthCheckApi as mockHealthCheckApi } from './api-client';
export const healthCheckApi = useMock ? mockHealthCheckApi : realHealthCheckApi;

// ─── Auth & System exports ───
export const auth: typeof realAuth = useMock
  ? {
      login: async (u: string, p: string) => {
        await new Promise(r => setTimeout(r, 300));
        if (u === 'admin' && p === 'admin') {
          return { user: { id: '1', username: 'admin' }, expires_at: new Date(Date.now() + 86400000).toISOString() };
        }
        throw Object.assign(new Error('无效凭据'), { status: 401 });
      },
      logout: async () => { await new Promise(r => setTimeout(r, 200)); return { message: '已登出' }; },
      me: async () => ({ user: { id: '1', username: 'admin' } }),
      changePassword: async (_current: string, _newPassword: string) => {
        await new Promise(r => setTimeout(r, 300));
        return { message: '密码已修改' };
      },
    }
  : realAuth;

export const system: typeof realSystem = useMock
  ? {
      overview: async () => ({
        node_count: 3,
        leader_node: 'node-a',
        route_count: 3,
        edge_rule_count: 2,
        service_count: 3,
        space_count: 1,
        last_apply: { status: 'success', version: 'v17', created_at: new Date().toISOString() },
      }),
      status: async () => ({
        name: 'aegis', version: '1.8C', server_time: new Date().toISOString(),
        proxy: { provider: 'caddy', config_path: '/etc/caddy/Caddyfile', validate_available: true, reload_command_configured: false },
        store: { sqlite_path: '/data/aegis.db', schema_version: '023' },
        counts: { projects: 2, services: 3, endpoints: 'n/a', routes: 3, managed_domains: 0 },
        health: { healthy_endpoints: 3, unhealthy_endpoints: 0, unknown_endpoints: 0 },
        pending_apply: { pending: false, since: '', reason: '' },
        last_apply: { status: 'success', version: 'v17', created_at: new Date().toISOString() },
      }),
      doctor: async () => ({ message: '诊断已触发' }),
      apply: async () => ({ message: '应用已完成', routes: 3, warnings: 0 }),
    }
  : realSystem;

// ─── Provider / Middleware API (v1.7S + v1.8H) ───
import { providerApi as mockProviderApi } from './api-client';
export const providerApi = useMock ? mockProviderApi : realProviderApi;

// ─── Exposure API (TCP/UDP port exposure) ───
import { exposureApi as mockExposureApi } from './api-client';
export const exposureApi = useMock ? mockExposureApi : realExposureApi;

// ─── Credential API (encrypted connection strings) ───
import { credentialApi as mockCredentialApi } from './api-client';
import { credentialApi as realCredentialApi } from './real-api-client';
export const credentialApi = useMock ? mockCredentialApi : realCredentialApi;

// ─── API service exports (with mock fallback for VITE_USE_MOCK) ───

// Safety API mock
const mockSafetyApi = {
  checkAllRoutes: async () => [],
  checkRoute: async (_id: string) => null,
  traceEgress: async (_domain: string, _fromNode: string) => ({ steps: [], trace_status: 'ok' }),
};
export const safetyApi = useMock ? mockSafetyApi : realSafetyApi;

// Trace API mock
const mockTraceApi = {
  byDomain: async (domain: string) => ({ input: domain, input_type: 'domain', trace_status: 'ok', steps: [] }),
  byRoute: async (routeId: string) => ({ input: routeId, input_type: 'route', trace_status: 'ok', steps: [] }),
  bySNI: async (sni: string) => ({ input: sni, input_type: 'sni', trace_status: 'ok', steps: [] }),
  egress: async (domain: string, fromNode: string) => ({ domain, from_node: fromNode, trace_status: 'ok', steps: [] }),
};
export const traceApi = useMock ? mockTraceApi : realTraceApi;

// Relay API mock
const mockRelayApi = {
  resolve: async (domain: string, _fromNode: string) => ({ domain, path: [], reachable: false, summary: 'mock' }),
};
export const relayApi = useMock ? mockRelayApi : realRelayApi;

// Gateway API mock
const mockGatewayApi = {
  list: async () => ({ gateways: [], count: 0 }),
  get: async (id: string) => ({ gateway_id: id }),
  update: async (_id: string, _data: any) => ({}),
};
export const gatewayApi = useMock ? mockGatewayApi : realGatewayApi;

// Gateway Link API mock
const mockGatewayLinkApi = {
  list: async () => [],
  get: async (id: string) => ({ id }),
  create: async (data: any) => ({ id: 'mock-' + Date.now(), ...data }),
  delete: async (_id: string) => ({}),
  rotate: async (_id: string) => ({ secret: 'mock-secret-' + Date.now() }),
};
export const gatewayLinkApi = useMock ? mockGatewayLinkApi : realGatewayLinkApi;

// Node API mock
const mockNodeApi = {
  list: async () => ({ nodes: [], count: 0 }),
  get: async (id: string) => ({ node: { node_id: id } }),
  health: async (_id: string) => ({}),
  capabilities: async (_id: string) => ({}),
  refreshCapabilities: async (_id: string) => ({}),
  gateways: async (_id: string) => ({ gateways: [], count: 0 }),
  syncStatus: async (_id: string) => ({}),
  routingTable: async (_id: string) => ({ entries: [] }),
  generateRoutingTable: async (_id: string) => ({}),
};
export const nodeApi = useMock ? mockNodeApi : realNodeApi;

// Admin API mock
const mockAdminApi = {
  listScopes: async () => ({ spaces: [], count: 0 }),
  createScope: async (data: any) => ({ id: 'mock-scope-' + Date.now(), ...data }),
  listApiKeys: async () => ({ api_keys: [], count: 0 }),
  createApiKey: async (scopeId: string, name: string) => ({ id: 'mock-key-' + Date.now(), scope_id: scopeId, name }),
  revokeApiKey: async (_id: string) => ({}),
  rotateApiKey: async (_id: string) => ({ token: 'mock-rotated-' + Date.now() }),
  listOperations: async () => ({ operations: [], count: 0 }),
  listApplyLogs: async () => ({ apply_logs: [], count: 0 }),
  listAuditLogs: async () => ({ audit_logs: [], count: 0 }),
  listNodeEvents: async () => ({ node_events: [], count: 0 }),
  listEdgeRules: async () => ({ edge_rules: [], count: 0 }),
  configCurrent: async () => ({}),
  configPreview: async () => ({}),
  configDiff: async () => ({}),
  applyHistory: async () => [],
  applyConfig: async () => ({ message: 'applied (mock)', routes: 0, warnings: 0 }),
  dryRun: async () => ({ message: 'dry-run (mock)', routes: 0, warnings: 0 }),
  rollback: async () => ({ message: 'rollback (mock)' }),
  bindHTTPDomain: async (data: any) => ({ domain: data.domain, status: 'created (mock)' }),
  bindTLSBackend: async (data: any) => ({ domain: data.domain, status: 'created (mock)' }),
  updateTarget: async (data: any) => ({ status: 'updated (mock)' }),
  exportDiagnostics: async () => ({}),
  importCaddyPreview: async () => {
    // Return scenario routes as preview when in mock mode
    const scenario = getScenario();
    if (scenario?.routes?.length) {
      const routes = scenario.routes.map((r: any) => ({
        domain: r.domain,
        upstream_url: `http://127.0.0.1:${r.service_id === 'service-proofnote-api' ? '3000' : r.service_id === 'service-auth' ? '4000' : '8080'}`,
        tls_enabled: r.tls_mode !== 'http_only',
        path_prefix: '',
        source_file: '/etc/caddy/Caddyfile',
        source_line: 1,
      }));
      return { routes, count: routes.length, caddy_path: '/etc/caddy/Caddyfile' };
    }
    return { routes: [], count: 0 };
  },
  importCaddyConfirm: async (routes: any[]) => ({ imported: routes.length, count: routes.length }),
};
export const adminApi = useMock ? mockAdminApi : realAdminApi;
