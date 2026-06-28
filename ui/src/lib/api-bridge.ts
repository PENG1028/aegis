/**
 * API Bridge — switches between mock and real API client.
 *
 * All pages import from here instead of directly from api-client or real-api-client.
 * Toggle with VITE_USE_MOCK env var or API_CONFIG.useMock.
 */

import { API_CONFIG } from './api-config';
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
  auth as realAuth,
  system as realSystem,
  safetyApi,
  traceApi,
  relayApi,
  gatewayApi,
  gatewayLinkApi,
  nodeApi,
  providerApi,
  adminApi,
  fetchListeners as realFetchListeners,
  dnsApi as realDnsApi,
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

// ─── API service exports (always real - new pages) ───
export { safetyApi, traceApi, relayApi, gatewayApi, gatewayLinkApi, nodeApi, providerApi, adminApi };
