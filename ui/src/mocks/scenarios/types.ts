// ─── Mock Scenario Types ───

import type {
  Node, NodeDetail, Gateway, GatewayDetail,
  TopologyEdge, TopologyPathResult,
  Service, ServiceDetail,
  Route, RouteDetail,
  Endpoint, EndpointDetail,
  GatewayPolicy, RoutingEntry,
  SyncStatus, AcceptanceStatus,
  JoinToken, DashboardData, DnsStatus,
} from '@/types';
import type { EntryPointSummary, Anomaly, ListenerRef } from '@/types/workspace';

export type ScenarioId = 'normal' | 'endpoint-failure' | 'pending-release' | 'node-drift' | 'gateway-link-anomaly';

export interface ScenarioMeta {
  name: string;
  description: string;
}

export interface ScenarioData {
  meta: ScenarioMeta;

  // Core domain collections
  nodes: Node[];
  gateways: Gateway[];
  services: Service[];
  routes: Route[];
  endpoints: Endpoint[];
  gatewayLinks: any[];
  policies: GatewayPolicy[];
  topologyEdges: TopologyEdge[];

  // Derived data
  entryPoints: EntryPointSummary[];
  anomalies: Anomaly[];
  dashboard: DashboardData;
  syncStatuses: SyncStatus[];
  joinTokens: JoinToken[];
  acceptance: AcceptanceStatus;
  dnsStatus: DnsStatus;

  // Detail views (keyed by ID)
  nodeDetails: Record<string, NodeDetail>;
  gatewayDetails: Record<string, GatewayDetail>;
  serviceDetails: Record<string, ServiceDetail>;
  routeDetails: Record<string, RouteDetail>;
  endpointDetails: Record<string, EndpointDetail>;

  // Listeners (from gateways)
  listeners: ListenerRef[];

  // Routing
  routingEntries: RoutingEntry[];
}
