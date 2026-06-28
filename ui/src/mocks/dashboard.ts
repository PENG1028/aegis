import type { DashboardData } from '@/types';

/** All 5 gateways have status 'active'. */
const ALL_GATEWAYS_ACTIVE = 5;
/** Topology has 2 missing links (A→C, B→C). */
const MISSING_LINKS = 2;
/** Node-c is undeployed — no routing table, no local gateway. */
const DEPLOYED_NODES = 2;

export const mockDashboard: DashboardData = {
  nodes_online: 2,
  nodes_total: 3,
  gateways_online: ALL_GATEWAYS_ACTIVE,
  gateways_total: ALL_GATEWAYS_ACTIVE,
  managed_routes: 3,
  routing_tables_synced: DEPLOYED_NODES,
  routing_tables_total: DEPLOYED_NODES,
  local_gateway_online: 1,
  local_gateway_total: DEPLOYED_NODES,
  relay_acceptance: 'real_two_node_local_gateway_verified',
  secret_runtime: 'code_verified',
  pending_capabilities: [
    'real_secret_runtime_deploy_pending',
    'real_three_node_pending',
    'https_deferred',
    'raw_tcp_deferred',
  ],
  routes_unavailable: 0,
  missing_gateway_links: MISSING_LINKS,
  outdated_nodes: 0,
  recent_errors: [],
};
