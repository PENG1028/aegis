// ─── Workspace View Types ───
// Composes domain objects into workspace views for the relationship-driven UI.

import type { Route, Gateway, Service, Endpoint, Node } from './index';

// ─── Listener Reference ───
export interface ListenerRef {
  bind_addr: string;
  port: number;
  provider: string;
  purpose: string;
  status: string;
  gateway_id: string;
  node_id: string;
}

// ─── Provider Reference ───
export interface ProviderRef {
  provider_id: string;
  name: string;
  kind: 'caddy' | 'haproxy' | 'aegis';
  status: string;
  node_id?: string;
}

// ─── Chain Health ───
export type ChainHealth = 'healthy' | 'degraded' | 'broken';

// ─── Full Object Chain ───
// Represents the complete path from external request to runtime.
// External Request → Entry Point → Listener → Route → Gateway → Service → Endpoint → Node → Provider
export interface ObjectChain {
  entryPoint: Route | null;
  listener: ListenerRef | null;
  gateway: Gateway | null;
  service: Service | null;
  endpoints: Endpoint[];
  nodes: Node[];
  provider: ProviderRef | null;
  status: ChainHealth;
  error?: string;
}

// ─── Entry Point ───
// Merged concept: Route + Domain + Listener + Exposure
export interface EntryPointSummary {
  route_id: string;
  domain: string;
  protocol: 'http' | 'tcp' | 'udp';
  tls_mode: string;
  listener: ListenerRef | null;
  gateway_id: string | null;
  gateway_name: string | null;
  service_id: string;
  service_name: string;
  endpoints: EndpointSummary[];
  health: 'healthy' | 'degraded' | 'failed' | 'unknown';
  safety: 'safe' | 'warning' | 'blocked' | 'unknown';
  release_state: 'current' | 'pending' | 'applying' | 'drifted' | 'failed';
}

export interface EndpointSummary {
  endpoint_id: string;
  node_id: string;
  node_name: string;
  protocol: string;
  target: string;
  health: 'healthy' | 'unhealthy' | 'unknown';
}

// ─── Anomaly ───
export interface Anomaly {
  id: string;
  severity: 'critical' | 'warning' | 'info';
  title: string;
  description: string;
  affectedObjects: { type: string; id: string; name: string }[];
  workspace: string;
  timestamp: string;
}

// ─── Workspace Health Summary (for Command Center) ───
export interface WorkspaceHealthSummary {
  totalEntries: number;
  healthyEntries: number;
  degradedEntries: number;
  failedEntries: number;
  totalGateways: number;
  activeGateways: number;
  totalNodes: number;
  healthyNodes: number;
  pendingChanges: number;
  anomalies: Anomaly[];
  lastApply: { status: string; version: string; created_at: string } | null;
}

// ─── Pipeline Step ───
// For expressing forwarding logic: Match → Gateway → Target → Forwarding → Fallback → Health
export interface PipelineStep {
  phase: 'match' | 'gateway' | 'target' | 'forwarding' | 'fallback' | 'health';
  label: string;
  status: 'ok' | 'warning' | 'error' | 'pending';
  detail: string;
  objectId?: string;
  objectType?: string;
}

export type ForwardingMode =
  | 'reverse_proxy'
  | 'load_balance'
  | 'relay'
  | 'maintenance'
  | 'transparent_proxy';

// ─── Node Desired vs Actual State ───
export interface NodeStateComparison {
  node_id: string;
  desired_revision: number;
  actual_revision: number;
  desired_hash: string;
  actual_hash: string;
  drifted: boolean;
  drifted_components: string[];
  last_sync_at: string | null;
}
