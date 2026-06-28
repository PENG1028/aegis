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
