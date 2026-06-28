// ─── Core Domain Types ───

/** Node status */
export type NodeStatus = 'online' | 'offline' | 'degraded' | 'unknown';
export type SyncStatusValue = 'in_sync' | 'outdated' | 'no_desired_state' | 'no_actual_state' | 'failed' | 'unknown';

export interface Node {
  node_id: string;
  name: string;
  hostname: string;
  public_ip: string;
  private_ip: string;
  roles: string[];
  status: NodeStatus;
  os: string;
  arch: string;
  agent_version: string;
  last_heartbeat_at: string | null;
  capabilities: NodeCapabilities;
  desired_revision: number;
  applied_revision: number;
  sync_status: SyncStatusValue;
  created_at: string;
  updated_at: string;
}

export interface NodeCapabilities {
  gateway_enabled: boolean;
  caddy_installed: boolean;
  haproxy_installed: boolean;
  tls_supported: boolean;
  dns_control_available: boolean;
  hot_reload_supported: boolean;
  edge_mux_supported: boolean;
  relay_capable: boolean;
  local_gateway_enabled: boolean;
}

export interface NodeDetail extends Node {
  gateways: Gateway[];
  sync: SyncStatusDetail;
  local_gateway?: LocalGatewayRuntimeStatus;
  routing_table_entries: number;
  last_error: string | null;
  diagnostics: NodeDiagnostic[];
}

export interface NodeDiagnostic {
  name: string;
  status: 'ok' | 'warning' | 'error';
  message: string;
}

// ─── Gateway ───
export type GatewayType = 'local' | 'private' | 'public';
export type GatewayProvider = 'caddy' | 'haproxy' | 'aegis';

export interface Gateway {
  gateway_id: string;
  node_id: string;
  node_name?: string;
  name: string;
  type: GatewayType;
  provider: GatewayProvider;
  bind_addr: string;
  host: string;
  port: number;
  scheme: 'http' | 'https';
  public_accessible: boolean;
  private_accessible: boolean;
  enabled: boolean;
  priority: number;
  status: 'active' | 'disabled' | 'error' | 'unknown';
  last_verified_at: string | null;
  last_error: string | null;
  created_at: string;
  updated_at: string;
}

export interface GatewayDetail extends Gateway {
  routes_served: number;
  gateway_links: GatewayLinkRef[];
}

export interface GatewayLinkRef {
  gateway_link_id: string;
  source_node_id: string;
  target_node_id: string;
  status: 'active' | 'inactive' | 'pending';
}

// ─── Topology ───
export interface TopologyEdge {
  from_node_id: string;
  from_node_name: string;
  to_node_id: string;
  to_node_name: string;
  private_reachable: boolean;
  public_reachable: boolean;
  preferred_gateway_id: string | null;
  gateway_link_id: string | null;
  status: TopologyEdgeStatus;
  last_verified_at: string | null;
  last_error: string | null;
}

export type TopologyEdgeStatus =
  | 'verified'
  | 'missing_link'
  | 'unreachable'
  | 'degraded'
  | 'unknown';

export interface TopologyPathResult {
  from_node: string;
  to_node: string;
  path: TopologyPathHop[];
  reachable: boolean;
  total_hops: number;
  summary: string;
}

export interface TopologyPathHop {
  hop: number;
  node_id: string;
  node_name: string;
  via: 'local' | 'private_gateway' | 'public_gateway';
  gateway_url: string | null;
  gateway_link_id: string | null;
  status: 'ok' | 'blocked' | 'unreachable';
  reason: string | null;
}

// ─── Service ───
export interface Service {
  service_id: string;
  name: string;
  kind: string;
  scope_id: string | null;
  upstream_url: string | null;
  health_check_url: string | null;
  status: 'active' | 'disabled' | 'error';
  health_status: 'healthy' | 'unhealthy' | 'unknown';
  routes_count: number;
  endpoints_count: number;
  latency_ms: number | null;
  created_at: string;
  updated_at: string;
}

export interface ServiceDetail extends Service {
  routes: RouteSummary[];
  endpoints: Endpoint[];
  gateway_policy: GatewayPolicy | null;
}

export interface RouteSummary {
  route_id: string;
  domain: string;
  status: string;
}

// ─── Route ───
export interface Route {
  route_id: string;
  domain: string;
  service_id: string;
  service_name?: string;
  scope_id: string | null;
  tls_mode: 'http_only' | 'terminate_local' | 'passthrough_deferred';
  preserve_host: boolean;
  public_allowed: boolean;
  status: 'active' | 'disabled' | 'error';
  gateway_policy: GatewayPolicy | null;
  created_at: string;
  updated_at: string;
}

export interface RouteDetail extends Route {
  endpoint: EndpointDetail | null;
  policy_summary: string;
  routing_status: 'available' | 'unavailable' | 'unknown';
}

// ─── Endpoint ───
export interface Endpoint {
  endpoint_id: string;
  service_id: string;
  node_id: string;
  node_name?: string;
  protocol: string;
  target_local_host: string;
  target_local_port: number;
  address_type: 'local' | 'remote';
  relay_eligible: boolean;
  health_status: 'healthy' | 'unhealthy' | 'unknown';
  latency_ms: number | null;
  last_checked_at: string | null;
}

export interface EndpointDetail extends Endpoint {
  routes: string[];
}

// ─── Gateway Policy ───
export interface GatewayPolicy {
  id: string;
  target_type: 'service' | 'route';
  target_id: string;
  target_name?: string;
  mode: 'auto' | 'fixed' | 'multi' | 'disabled';
  primary_gateway_id: string | null;
  fallback_gateway_ids: string[];
  allow_local: boolean;
  allow_private: boolean;
  allow_public: boolean;
  require_gateway_link: boolean;
  require_relay: boolean;
  preserve_host: boolean;
  tls_mode: 'http_only' | 'terminate_local' | 'passthrough_deferred';
  enabled: boolean;
  priority: number;
  created_at: string;
  updated_at: string;
}

// ─── Routing Table ───
export interface RoutingEntry {
  domain: string;
  route_id: string;
  service_id: string;
  endpoint_id: string;
  from_node_id: string;
  target_node_id: string;
  protocol: string;
  target_local_host: string;
  target_local_port: number;
  policy_mode: string;
  candidates: RoutingCandidate[];
  status: 'available' | 'unavailable' | 'disabled' | 'unknown';
  unavailable_reason: string | null;
}

export interface RoutingCandidate {
  mode: 'local_gateway' | 'private_gateway' | 'public_gateway';
  gateway_id: string | null;
  gateway_url: string | null;
  priority: number;
  requires_gateway_link: boolean;
  gateway_link_id: string | null;
}

export interface RoutingPreviewResult {
  domain: string;
  from_node_id: string;
  from_node_name: string;
  entries: RoutingEntry[];
  available: boolean;
  summary: string;
  unavailable_reason: string | null;
}

export interface RoutingValidationResult {
  valid: boolean;
  node_id: string | null;
  total_entries: number;
  errors: RoutingValidationError[];
  warnings: RoutingValidationWarning[];
  valid_count: number;
  error_count: number;
  warning_count: number;
}

export interface RoutingValidationError {
  domain: string;
  code: string;
  message: string;
}

export interface RoutingValidationWarning {
  domain: string;
  code: string;
  message: string;
}

// ─── Sync ───
export interface SyncStatus {
  node_id: string;
  node_name: string;
  desired_revision: number;
  applied_revision: number;
  desired_hash: string;
  actual_hash: string;
  status: SyncStatusValue;
  last_apply_at: string | null;
  last_success_at: string | null;
  last_error: string | null;
  provider_status: SyncComponentStatus;
  relay_status: SyncComponentStatus;
  gateway_status: SyncComponentStatus;
  diagnostics_status: SyncComponentStatus;
}

export interface SyncStatusDetail {
  status: SyncStatusValue;
  desired_revision: number;
  applied_revision: number;
  desired_hash: string;
  actual_hash: string;
  last_apply_at: string | null;
  last_success_at: string | null;
  last_error: string | null;
}

export interface SyncComponentStatus {
  status: 'ok' | 'error' | 'unknown';
  message: string;
}

// ─── Local Gateway ───
export interface LocalGatewayStatus {
  node_id: string;
  node_name: string;
  bind_addr: string;
  port: number;
  status: 'running' | 'stopped' | 'error' | 'unknown';
  routing_table_loaded: boolean;
  routing_table_revision: number | null;
  entries_count: number;
  cache_status: 'fresh' | 'stale' | 'empty';
  diagnostics: LocalGatewayDiagnostic[];
  last_error: string | null;
}

export interface LocalGatewayRuntimeStatus {
  bind_addr: string;
  port: number;
  status: 'running' | 'stopped' | 'error' | 'unknown';
  routing_table_loaded: boolean;
  routing_table_revision: number | null;
  entries_count: number;
  cache_status: string;
  last_error: string | null;
}

export interface LocalGatewayDiagnostic {
  name: string;
  status: 'ok' | 'warning' | 'error';
  message: string;
}

// ─── Acceptance / Verification ───
export interface AcceptanceStatus {
  labels: VerificationLabel[];
  summary: AcceptanceSummary;
  last_acceptance: AcceptanceRun | null;
  negative_smoke: AcceptanceTest[];
}

export interface VerificationLabel {
  key: string;
  label: string;
  status: 'pass' | 'pending' | 'deferred';
  evidence: string;
}

export interface AcceptanceSummary {
  total_labels: number;
  pass_count: number;
  pending_count: number;
  deferred_count: number;
}

export interface AcceptanceRun {
  command: string;
  http_status: number;
  response_summary: string;
  selected_candidate: string;
  gateway_link_id: string;
  token_leak_scan: 'clean' | 'warning' | 'fail';
  negative_smoke_result: 'pass' | 'partial' | 'fail';
  docs_link: string;
  executed_at: string;
}

export interface AcceptanceTest {
  id: string;
  desc: string;
  expected: string;
  actual: string;
  status: 'pass' | 'fail' | 'partial';
}

// ─── Join Token ───
export interface JoinToken {
  id: string;
  name: string;
  token_prefix: string;
  allowed_roles: string[];
  expected_node_name: string | null;
  expires_at: string;
  allowed_source_cidr: string | null;
  status: 'active' | 'revoked' | 'expired';
  created_at: string;
  used_at: string | null;
}

// ─── Dashboard ───
export interface DashboardData {
  nodes_online: number;
  nodes_total: number;
  gateways_online: number;
  gateways_total: number;
  managed_routes: number;
  routing_tables_synced: number;
  routing_tables_total: number;
  local_gateway_online: number;
  local_gateway_total: number;
  relay_acceptance: string;
  secret_runtime: string;
  pending_capabilities: string[];
  routes_unavailable: number;
  missing_gateway_links: number;
  outdated_nodes: number;
  recent_errors: DashboardError[];
}

export interface DashboardError {
  node_id: string;
  node_name: string;
  error: string;
  last_seen: string;
}

// ─── DNS (v1.8E) ───

export interface DnsResolvedEntry {
  domain: string;
  target_ip: string;
  target_node: string;
  node_ip: string;
  public_ip: string;
  is_local: boolean;
  route_id: string;
  service_id: string;
  endpoint_id: string;
}

export interface DnsStatus {
  running: boolean;
  listen_addr: string;
  upstream: string;
  enabled: boolean;
  local_hits: number;
  upstream_calls: number;
  managed_count: number;
  entries?: DnsResolvedEntry[];
}
