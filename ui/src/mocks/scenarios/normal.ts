// ─── Scenario 1: Normal ───
// All healthy, no anomalies. Single source of truth for ALL mock data.
//
// Gateway IDs are shared between list pages and chain resolver:
//   gw_local_a, gw_public_a, gw_public_b, gw_private_a, gw_private_b

import type { ScenarioData } from './types';

const NOW = '2026-07-02T10:30:00Z';
const HOUR_AGO = '2026-07-02T09:30:00Z';

export const scenarioNormal: ScenarioData = {
  meta: {
    name: '正常链路',
    description: '5 网关全链路正常，3 节点在线，配置已同步',
  },

  // ═══ Nodes ═══
  nodes: [
    {
      node_id: 'node-a', name: 'Server A', hostname: 'VM-0-4-ubuntu',
      public_ip: '<SERVER_A_IP>', private_ip: '10.0.1.4',
      roles: ['control_plane', 'gateway'], status: 'online',
      os: 'Ubuntu 22.04', arch: 'amd64', agent_version: 'v1.8C',
      last_heartbeat_at: new Date(Date.now() - 8000).toISOString(),
      capabilities: { gateway_enabled: true, caddy_installed: true, haproxy_installed: false, tls_supported: true, dns_control_available: true, hot_reload_supported: true, edge_mux_supported: true, relay_capable: true, local_gateway_enabled: true },
      desired_revision: 17, applied_revision: 17, sync_status: 'in_sync',
      created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
    },
    {
      node_id: 'node-b', name: 'Server B', hostname: 'VM-0-11-ubuntu',
      public_ip: '<SERVER_B_IP>', private_ip: '10.0.1.11',
      roles: ['gateway', 'backend'], status: 'online',
      os: 'Ubuntu 22.04', arch: 'amd64', agent_version: 'v1.8C',
      last_heartbeat_at: new Date(Date.now() - 15000).toISOString(),
      capabilities: { gateway_enabled: true, caddy_installed: true, haproxy_installed: false, tls_supported: false, dns_control_available: false, hot_reload_supported: true, edge_mux_supported: false, relay_capable: true, local_gateway_enabled: false },
      desired_revision: 17, applied_revision: 17, sync_status: 'in_sync',
      created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
    },
    {
      node_id: 'node-c', name: 'Server C', hostname: '—',
      public_ip: '—', private_ip: '—',
      roles: ['gateway'], status: 'unknown',
      os: '—', arch: '—', agent_version: '—',
      last_heartbeat_at: null,
      capabilities: { gateway_enabled: false, caddy_installed: false, haproxy_installed: false, tls_supported: false, dns_control_available: false, hot_reload_supported: false, edge_mux_supported: false, relay_capable: false, local_gateway_enabled: false },
      desired_revision: 0, applied_revision: 0, sync_status: 'no_desired_state',
      created_at: '2026-06-25T10:00:00Z', updated_at: '2026-06-25T10:00:00Z',
    },
  ],

  // ═══ Gateways ═══
  gateways: [
    {
      gateway_id: 'gw_local_a', node_id: 'node-a', node_name: 'Server A',
      name: 'A Local Gateway', type: 'local', provider: 'aegis',
      bind_addr: '127.0.0.1', host: '127.0.0.1', port: 18080,
      scheme: 'http', public_accessible: false, private_accessible: true,
      enabled: true, priority: 100, status: 'active',
      last_verified_at: new Date(Date.now() - 30000).toISOString(), last_error: null,
      created_at: '2026-06-22T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
    },
    {
      gateway_id: 'gw_public_a', node_id: 'node-a', node_name: 'Server A',
      name: 'A Public Gateway', type: 'public', provider: 'caddy',
      bind_addr: '0.0.0.0', host: '<SERVER_A_IP>', port: 80,
      scheme: 'http', public_accessible: true, private_accessible: true,
      enabled: true, priority: 50, status: 'active',
      last_verified_at: new Date(Date.now() - 60000).toISOString(), last_error: null,
      created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
    },
    {
      gateway_id: 'gw_public_b', node_id: 'node-b', node_name: 'Server B',
      name: 'B Public Gateway', type: 'public', provider: 'caddy',
      bind_addr: '0.0.0.0', host: '<SERVER_B_IP>', port: 80,
      scheme: 'http', public_accessible: true, private_accessible: true,
      enabled: true, priority: 50, status: 'active',
      last_verified_at: new Date(Date.now() - 45000).toISOString(), last_error: null,
      created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
    },
    {
      gateway_id: 'gw_private_a', node_id: 'node-a', node_name: 'Server A',
      name: 'A Private Gateway', type: 'private', provider: 'caddy',
      bind_addr: '10.0.1.4', host: '10.0.1.4', port: 80,
      scheme: 'http', public_accessible: false, private_accessible: true,
      enabled: true, priority: 80, status: 'active',
      last_verified_at: new Date(Date.now() - 120000).toISOString(), last_error: null,
      created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
    },
    {
      gateway_id: 'gw_private_b', node_id: 'node-b', node_name: 'Server B',
      name: 'B Private Gateway', type: 'private', provider: 'caddy',
      bind_addr: '10.0.1.11', host: '10.0.1.11', port: 80,
      scheme: 'http', public_accessible: false, private_accessible: true,
      enabled: true, priority: 80, status: 'active',
      last_verified_at: new Date(Date.now() - 60000).toISOString(), last_error: null,
      created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
    },
  ],

  // ═══ Services ═══
  services: [
    {
      service_id: 'svc-api-b', name: 'api-b-prod', kind: 'http', scope_id: null,
      upstream_url: 'http://127.0.0.1:18081', health_check_url: 'http://127.0.0.1:18081/health',
      status: 'active', health_status: 'healthy', routes_count: 1, endpoints_count: 1,
      latency_ms: 38, created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
    },
    {
      service_id: 'svc-relay', name: 'relay-target', kind: 'http', scope_id: null,
      upstream_url: 'http://127.0.0.1:2724', health_check_url: null,
      status: 'active', health_status: 'healthy', routes_count: 1, endpoints_count: 1,
      latency_ms: 12, created_at: '2026-06-22T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
    },
    {
      service_id: 'svc-policy', name: 'policy-web', kind: 'http', scope_id: null,
      upstream_url: 'http://127.0.0.1:3001', health_check_url: 'http://127.0.0.1:3001/health',
      status: 'active', health_status: 'healthy', routes_count: 1, endpoints_count: 1,
      latency_ms: 5, created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-26T18:00:00Z',
    },
  ],

  // ═══ Routes ═══
  routes: [
    {
      route_id: 'route-api-b', domain: 'api-b.example.com', service_id: 'svc-api-b',
      service_name: 'api-b-prod', scope_id: null, tls_mode: 'http_only',
      preserve_host: true, public_allowed: false, status: 'active',
      gateway_policy: {
        id: 'policy-api-b', target_type: 'route', target_id: 'route-api-b', target_name: 'api-b.example.com',
        mode: 'auto', primary_gateway_id: 'gw_private_b', fallback_gateway_ids: [],
        allow_local: true, allow_private: true, allow_public: false,
        require_gateway_link: true, require_relay: false,
        preserve_host: true, tls_mode: 'http_only',
        enabled: true, priority: 10,
        created_at: '2026-06-20T10:00:00Z', updated_at: NOW,
      },
      created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
    },
    {
      route_id: 'route-relay', domain: 'relay.example.com', service_id: 'svc-relay',
      service_name: 'relay-target', scope_id: null, tls_mode: 'http_only',
      preserve_host: true, public_allowed: true, status: 'active',
      gateway_policy: {
        id: 'policy-relay', target_type: 'route', target_id: 'route-relay', target_name: 'relay.example.com',
        mode: 'fixed', primary_gateway_id: 'gw_public_b', fallback_gateway_ids: [],
        allow_local: false, allow_private: true, allow_public: true,
        require_gateway_link: true, require_relay: true,
        preserve_host: true, tls_mode: 'http_only',
        enabled: true, priority: 8,
        created_at: '2026-06-22T10:00:00Z', updated_at: NOW,
      },
      created_at: '2026-06-22T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
    },
    {
      route_id: 'route-policy', domain: 'policy.example.com', service_id: 'svc-policy',
      service_name: 'policy-web', scope_id: null, tls_mode: 'http_only',
      preserve_host: false, public_allowed: true, status: 'active',
      gateway_policy: {
        id: 'policy-policy', target_type: 'route', target_id: 'route-policy', target_name: 'policy.example.com',
        mode: 'disabled', primary_gateway_id: null, fallback_gateway_ids: [],
        allow_local: false, allow_private: false, allow_public: false,
        require_gateway_link: false, require_relay: false,
        preserve_host: true, tls_mode: 'http_only',
        enabled: false, priority: 0,
        created_at: '2026-06-20T10:00:00Z', updated_at: NOW,
      },
      created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-26T18:00:00Z',
    },
  ],

  // ═══ Endpoints ═══
  endpoints: [
    {
      endpoint_id: 'ep-api-b', service_id: 'svc-api-b', node_id: 'node-b', node_name: 'Server B',
      protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 18081,
      address_type: 'local', relay_eligible: true, health_status: 'healthy',
      latency_ms: 3, last_checked_at: new Date(Date.now() - 60000).toISOString(),
    },
    {
      endpoint_id: 'ep-relay', service_id: 'svc-relay', node_id: 'node-b', node_name: 'Server B',
      protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 2724,
      address_type: 'local', relay_eligible: true, health_status: 'healthy',
      latency_ms: 5, last_checked_at: new Date(Date.now() - 30000).toISOString(),
    },
    {
      endpoint_id: 'ep-policy', service_id: 'svc-policy', node_id: 'node-a', node_name: 'Server A',
      protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 3001,
      address_type: 'local', relay_eligible: false, health_status: 'healthy',
      latency_ms: 2, last_checked_at: new Date(Date.now() - 45000).toISOString(),
    },
  ],

  // ═══ Listeners (derived from gateways) ═══
  listeners: [
    { bind_addr: '0.0.0.0', port: 80, provider: 'caddy', purpose: 'public_gateway', status: 'active', gateway_id: 'gw_public_a', node_id: 'node-a' },
    { bind_addr: '0.0.0.0', port: 80, provider: 'caddy', purpose: 'public_gateway', status: 'active', gateway_id: 'gw_public_b', node_id: 'node-b' },
    { bind_addr: '10.0.1.4', port: 80, provider: 'caddy', purpose: 'private_gateway', status: 'active', gateway_id: 'gw_private_a', node_id: 'node-a' },
    { bind_addr: '10.0.1.11', port: 80, provider: 'caddy', purpose: 'private_gateway', status: 'active', gateway_id: 'gw_private_b', node_id: 'node-b' },
    { bind_addr: '127.0.0.1', port: 18080, provider: 'aegis', purpose: 'local_gateway', status: 'active', gateway_id: 'gw_local_a', node_id: 'node-a' },
  ],

  // ═══ Gateway Links ═══
  gatewayLinks: [
    {
      gateway_link_id: 'gl-a-b', source_node_id: 'node-a', source_node_name: 'Server A',
      target_node_id: 'node-b', target_node_name: 'Server B',
      status: 'active', created_at: '2026-06-15T08:00:00Z', last_verified_at: NOW,
    },
  ],

  // ═══ Policies ═══
  policies: [
    // Derived from route gateway_policy fields above
  ],

  // ═══ Topology Edges ═══
  topologyEdges: [
    {
      from_node_id: 'node-a', from_node_name: 'Server A',
      to_node_id: 'node-b', to_node_name: 'Server B',
      private_reachable: true, public_reachable: true,
      preferred_gateway_id: 'gw_public_a', gateway_link_id: 'gl-a-b',
      status: 'verified', last_verified_at: NOW, last_error: null,
    },
  ],

  // ═══ Entry Points ═══
  entryPoints: [
    {
      route_id: 'route-api-b', domain: 'api-b.example.com', protocol: 'http',
      tls_mode: 'http_only',
      listener: { bind_addr: '10.0.1.11', port: 80, provider: 'caddy', purpose: 'private_gateway', status: 'active', gateway_id: 'gw_private_b', node_id: 'node-b' },
      gateway_id: 'gw_private_b', gateway_name: 'B Private Gateway',
      service_id: 'svc-api-b', service_name: 'api-b-prod',
      endpoints: [
        { endpoint_id: 'ep-api-b', node_id: 'node-b', node_name: 'Server B', protocol: 'http', target: '127.0.0.1:18081', health: 'healthy' },
      ],
      health: 'healthy', safety: 'safe', release_state: 'current',
    },
    {
      route_id: 'route-relay', domain: 'relay.example.com', protocol: 'http',
      tls_mode: 'http_only',
      listener: { bind_addr: '0.0.0.0', port: 80, provider: 'caddy', purpose: 'public_gateway', status: 'active', gateway_id: 'gw_public_b', node_id: 'node-b' },
      gateway_id: 'gw_public_b', gateway_name: 'B Public Gateway',
      service_id: 'svc-relay', service_name: 'relay-target',
      endpoints: [
        { endpoint_id: 'ep-relay', node_id: 'node-b', node_name: 'Server B', protocol: 'http', target: '127.0.0.1:2724', health: 'healthy' },
      ],
      health: 'healthy', safety: 'safe', release_state: 'current',
    },
    {
      route_id: 'route-policy', domain: 'policy.example.com', protocol: 'http',
      tls_mode: 'http_only',
      listener: { bind_addr: '0.0.0.0', port: 80, provider: 'caddy', purpose: 'public_gateway', status: 'active', gateway_id: 'gw_public_a', node_id: 'node-a' },
      gateway_id: 'gw_public_a', gateway_name: 'A Public Gateway',
      service_id: 'svc-policy', service_name: 'policy-web',
      endpoints: [
        { endpoint_id: 'ep-policy', node_id: 'node-a', node_name: 'Server A', protocol: 'http', target: '127.0.0.1:3001', health: 'healthy' },
      ],
      health: 'healthy', safety: 'safe', release_state: 'current',
    },
  ],

  anomalies: [],

  // ═══ Dashboard ═══
  dashboard: {
    nodes_online: 2, nodes_total: 3,
    gateways_online: 5, gateways_total: 5,
    managed_routes: 3, routing_tables_synced: 2, routing_tables_total: 3,
    local_gateway_online: 1, local_gateway_total: 1,
    relay_acceptance: 'pass', secret_runtime: 'verified',
    pending_capabilities: ['node-c: no heartbeat — plan deployment'],
    routes_unavailable: 1, missing_gateway_links: 0, outdated_nodes: 0,
    recent_errors: [],
  },

  // ═══ Sync Statuses ═══
  syncStatuses: [
    {
      node_id: 'node-a', node_name: 'Server A',
      desired_revision: 17, applied_revision: 17,
      desired_hash: 'd8e5f7a2b3c1', actual_hash: 'd8e5f7a2b3c1',
      status: 'in_sync', last_apply_at: NOW, last_success_at: NOW, last_error: null,
      provider_status: { status: 'ok', message: 'Caddy v2.7.6 running' },
      relay_status: { status: 'ok', message: 'Relay active' },
      gateway_status: { status: 'ok', message: '3 gateways active' },
      diagnostics_status: { status: 'ok', message: 'All checks passed' },
    },
    {
      node_id: 'node-b', node_name: 'Server B',
      desired_revision: 17, applied_revision: 17,
      desired_hash: 'd8e5f7a2b3c1', actual_hash: 'd8e5f7a2b3c1',
      status: 'in_sync', last_apply_at: NOW, last_success_at: NOW, last_error: null,
      provider_status: { status: 'ok', message: 'Caddy v2.7.6 running' },
      relay_status: { status: 'ok', message: 'Relay active' },
      gateway_status: { status: 'ok', message: '2 gateways active' },
      diagnostics_status: { status: 'ok', message: 'All checks passed' },
    },
    {
      node_id: 'node-c', node_name: 'Server C',
      desired_revision: 0, applied_revision: 0,
      desired_hash: '', actual_hash: '',
      status: 'no_desired_state', last_apply_at: null, last_success_at: null, last_error: null,
      provider_status: { status: 'unknown', message: 'No provider' },
      relay_status: { status: 'unknown', message: 'No relay' },
      gateway_status: { status: 'unknown', message: 'No gateway' },
      diagnostics_status: { status: 'unknown', message: 'Node never contacted' },
    },
  ],

  // ═══ Join Tokens ═══
  joinTokens: [
    {
      id: 'token-1', name: 'default-node', token_prefix: 'aegis_n_',
      allowed_roles: ['gateway'], expected_node_name: null,
      expires_at: '2027-01-01T00:00:00Z', allowed_source_cidr: null,
      status: 'active', created_at: '2026-01-01T00:00:00Z', used_at: null,
    },
  ],

  // ═══ Acceptance ═══
  acceptance: {
    labels: [
      { key: 'gateway-link', label: 'Gateway Link 认证', status: 'pass', evidence: 'HMAC-SHA256 双节点验证通过' },
      { key: 'caddy-reload', label: 'Caddy 热重载', status: 'pass', evidence: 'caddy reload 成功' },
      { key: 'sync', label: '配置同步', status: 'pass', evidence: 'desired == actual，v17' },
    ],
    summary: { total_labels: 3, pass_count: 3, pending_count: 0, deferred_count: 0 },
    last_acceptance: null,
    negative_smoke: [],
  },

  // ═══ DNS Status ═══
  dnsStatus: {
    running: true, listen_addr: ':5353', upstream: '1.1.1.1:53',
    enabled: true, local_hits: 1523, upstream_calls: 45,
    managed_count: 3,
  },

  // ═══ Detail Views ═══

  gatewayDetails: {
    'gw_local_a': {
      gateway_id: 'gw_local_a', node_id: 'node-a', node_name: 'Server A',
      name: 'A Local Gateway', type: 'local', provider: 'aegis',
      bind_addr: '127.0.0.1', host: '127.0.0.1', port: 18080,
      scheme: 'http', public_accessible: false, private_accessible: true,
      enabled: true, priority: 100, status: 'active',
      last_verified_at: new Date(Date.now() - 30000).toISOString(), last_error: null,
      created_at: '2026-06-22T10:00:00Z', updated_at: NOW,
      routes_served: 1,
      gateway_links: [],
    },
    'gw_public_a': {
      gateway_id: 'gw_public_a', node_id: 'node-a', node_name: 'Server A',
      name: 'A Public Gateway', type: 'public', provider: 'caddy',
      bind_addr: '0.0.0.0', host: '<SERVER_A_IP>', port: 80,
      scheme: 'http', public_accessible: true, private_accessible: true,
      enabled: true, priority: 50, status: 'active',
      last_verified_at: new Date(Date.now() - 60000).toISOString(), last_error: null,
      created_at: '2026-06-20T10:00:00Z', updated_at: NOW,
      routes_served: 1,
      gateway_links: [{ gateway_link_id: 'gl-a-b', source_node_id: 'node-a', target_node_id: 'node-b', status: 'active' }],
    },
    'gw_public_b': {
      gateway_id: 'gw_public_b', node_id: 'node-b', node_name: 'Server B',
      name: 'B Public Gateway', type: 'public', provider: 'caddy',
      bind_addr: '0.0.0.0', host: '<SERVER_B_IP>', port: 80,
      scheme: 'http', public_accessible: true, private_accessible: true,
      enabled: true, priority: 50, status: 'active',
      last_verified_at: new Date(Date.now() - 45000).toISOString(), last_error: null,
      created_at: '2026-06-20T10:00:00Z', updated_at: NOW,
      routes_served: 2,
      gateway_links: [{ gateway_link_id: 'gl-a-b', source_node_id: 'node-a', target_node_id: 'node-b', status: 'active' }],
    },
    'gw_private_a': {
      gateway_id: 'gw_private_a', node_id: 'node-a', node_name: 'Server A',
      name: 'A Private Gateway', type: 'private', provider: 'caddy',
      bind_addr: '10.0.1.4', host: '10.0.1.4', port: 80,
      scheme: 'http', public_accessible: false, private_accessible: true,
      enabled: true, priority: 80, status: 'active',
      last_verified_at: new Date(Date.now() - 120000).toISOString(), last_error: null,
      created_at: '2026-06-20T10:00:00Z', updated_at: NOW,
      routes_served: 0,
      gateway_links: [],
    },
    'gw_private_b': {
      gateway_id: 'gw_private_b', node_id: 'node-b', node_name: 'Server B',
      name: 'B Private Gateway', type: 'private', provider: 'caddy',
      bind_addr: '10.0.1.11', host: '10.0.1.11', port: 80,
      scheme: 'http', public_accessible: false, private_accessible: true,
      enabled: true, priority: 80, status: 'active',
      last_verified_at: new Date(Date.now() - 60000).toISOString(), last_error: null,
      created_at: '2026-06-20T10:00:00Z', updated_at: NOW,
      routes_served: 1,
      gateway_links: [],
    },
  },

  nodeDetails: {
    'node-a': {
      node_id: 'node-a', name: 'Server A', hostname: 'VM-0-4-ubuntu',
      public_ip: '<SERVER_A_IP>', private_ip: '10.0.1.4',
      roles: ['control_plane', 'gateway'], status: 'online',
      os: 'Ubuntu 22.04', arch: 'amd64', agent_version: 'v1.8C',
      last_heartbeat_at: new Date(Date.now() - 8000).toISOString(),
      capabilities: { gateway_enabled: true, caddy_installed: true, haproxy_installed: false, tls_supported: true, dns_control_available: true, hot_reload_supported: true, edge_mux_supported: true, relay_capable: true, local_gateway_enabled: true },
      desired_revision: 17, applied_revision: 17, sync_status: 'in_sync',
      created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
      gateways: [
        { gateway_id: 'gw_local_a', node_id: 'node-a', node_name: 'Server A', name: 'A Local Gateway', type: 'local', provider: 'aegis', bind_addr: '127.0.0.1', host: '127.0.0.1', port: 18080, scheme: 'http', public_accessible: false, private_accessible: true, enabled: true, priority: 100, status: 'active', last_verified_at: new Date(Date.now() - 30000).toISOString(), last_error: null, created_at: '2026-06-22T10:00:00Z', updated_at: NOW },
        { gateway_id: 'gw_public_a', node_id: 'node-a', node_name: 'Server A', name: 'A Public Gateway', type: 'public', provider: 'caddy', bind_addr: '0.0.0.0', host: '<SERVER_A_IP>', port: 80, scheme: 'http', public_accessible: true, private_accessible: true, enabled: true, priority: 50, status: 'active', last_verified_at: new Date(Date.now() - 60000).toISOString(), last_error: null, created_at: '2026-06-20T10:00:00Z', updated_at: NOW },
        { gateway_id: 'gw_private_a', node_id: 'node-a', node_name: 'Server A', name: 'A Private Gateway', type: 'private', provider: 'caddy', bind_addr: '10.0.1.4', host: '10.0.1.4', port: 80, scheme: 'http', public_accessible: false, private_accessible: true, enabled: true, priority: 80, status: 'active', last_verified_at: new Date(Date.now() - 120000).toISOString(), last_error: null, created_at: '2026-06-20T10:00:00Z', updated_at: NOW },
      ],
      sync: {
        status: 'in_sync', desired_revision: 17, applied_revision: 17,
        desired_hash: 'd8e5f7a2b3c1', actual_hash: 'd8e5f7a2b3c1',
        last_apply_at: new Date(Date.now() - 120000).toISOString(),
        last_success_at: new Date(Date.now() - 120000).toISOString(),
        last_error: null,
      },
      routing_table_entries: 3, last_error: null,
      diagnostics: [
        { name: 'heartbeat', status: 'ok', message: '心跳正常 — 8s 前' },
        { name: 'sync', status: 'ok', message: '配置已同步 (v17)' },
        { name: 'gateways', status: 'ok', message: '3 个网关全部活跃' },
      ],
    },
    'node-b': {
      node_id: 'node-b', name: 'Server B', hostname: 'VM-0-11-ubuntu',
      public_ip: '<SERVER_B_IP>', private_ip: '10.0.1.11',
      roles: ['gateway', 'backend'], status: 'online',
      os: 'Ubuntu 22.04', arch: 'amd64', agent_version: 'v1.8C',
      last_heartbeat_at: new Date(Date.now() - 15000).toISOString(),
      capabilities: { gateway_enabled: true, caddy_installed: true, haproxy_installed: false, tls_supported: false, dns_control_available: false, hot_reload_supported: true, edge_mux_supported: false, relay_capable: true, local_gateway_enabled: false },
      desired_revision: 17, applied_revision: 17, sync_status: 'in_sync',
      created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-27T12:00:00Z',
      gateways: [
        { gateway_id: 'gw_public_b', node_id: 'node-b', node_name: 'Server B', name: 'B Public Gateway', type: 'public', provider: 'caddy', bind_addr: '0.0.0.0', host: '<SERVER_B_IP>', port: 80, scheme: 'http', public_accessible: true, private_accessible: true, enabled: true, priority: 50, status: 'active', last_verified_at: new Date(Date.now() - 45000).toISOString(), last_error: null, created_at: '2026-06-20T10:00:00Z', updated_at: NOW },
        { gateway_id: 'gw_private_b', node_id: 'node-b', node_name: 'Server B', name: 'B Private Gateway', type: 'private', provider: 'caddy', bind_addr: '10.0.1.11', host: '10.0.1.11', port: 80, scheme: 'http', public_accessible: false, private_accessible: true, enabled: true, priority: 80, status: 'active', last_verified_at: new Date(Date.now() - 60000).toISOString(), last_error: null, created_at: '2026-06-20T10:00:00Z', updated_at: NOW },
      ],
      sync: {
        status: 'in_sync', desired_revision: 17, applied_revision: 17,
        desired_hash: 'd8e5f7a2b3c1', actual_hash: 'd8e5f7a2b3c1',
        last_apply_at: new Date(Date.now() - 150000).toISOString(),
        last_success_at: new Date(Date.now() - 150000).toISOString(),
        last_error: null,
      },
      routing_table_entries: 2, last_error: null,
      diagnostics: [
        { name: 'heartbeat', status: 'ok', message: '心跳正常 — 15s 前' },
        { name: 'sync', status: 'ok', message: '配置已同步 (v17)' },
        { name: 'gateways', status: 'ok', message: '2 个网关全部活跃' },
      ],
    },
    'node-c': {
      node_id: 'node-c', name: 'Server C', hostname: '—',
      public_ip: '—', private_ip: '—',
      roles: ['gateway'], status: 'unknown',
      os: '—', arch: '—', agent_version: '—',
      last_heartbeat_at: null,
      capabilities: { gateway_enabled: false, caddy_installed: false, haproxy_installed: false, tls_supported: false, dns_control_available: false, hot_reload_supported: false, edge_mux_supported: false, relay_capable: false, local_gateway_enabled: false },
      desired_revision: 0, applied_revision: 0, sync_status: 'no_desired_state',
      created_at: '2026-06-25T10:00:00Z', updated_at: '2026-06-25T10:00:00Z',
      gateways: [],
      sync: {
        status: 'no_desired_state', desired_revision: 0, applied_revision: 0,
        desired_hash: '', actual_hash: '',
        last_apply_at: null, last_success_at: null, last_error: null,
      },
      routing_table_entries: 0, last_error: null,
      diagnostics: [
        { name: 'heartbeat', status: 'error', message: '从未收到心跳' },
        { name: 'sync', status: 'warning', message: '无期望状态' },
      ],
    },
  },

  serviceDetails: {
    'svc-api-b': {
      service_id: 'svc-api-b', name: 'api-b-prod', kind: 'http', scope_id: null,
      upstream_url: 'http://127.0.0.1:18081', health_check_url: 'http://127.0.0.1:18081/health',
      status: 'active', health_status: 'healthy', routes_count: 1, endpoints_count: 1,
      latency_ms: 38, created_at: '2026-06-20T10:00:00Z', updated_at: NOW,
      routes: [{ route_id: 'route-api-b', domain: 'api-b.example.com', status: 'active' }],
      endpoints: [{
        endpoint_id: 'ep-api-b', service_id: 'svc-api-b', node_id: 'node-b', node_name: 'Server B',
        protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 18081,
        address_type: 'local', relay_eligible: true, health_status: 'healthy',
        latency_ms: 3, last_checked_at: new Date(Date.now() - 60000).toISOString(),
      }],
      gateway_policy: {
        id: 'pol-svc-svc-api-b', target_type: 'service', target_id: 'svc-api-b', target_name: 'api-b-prod',
        mode: 'auto', primary_gateway_id: 'gw_private_b', fallback_gateway_ids: [],
        allow_local: true, allow_private: true, allow_public: false,
        require_gateway_link: true, require_relay: false,
        preserve_host: true, tls_mode: 'http_only', enabled: true, priority: 10,
        created_at: '2026-06-22T10:00:00Z', updated_at: NOW,
      },
    },
    'svc-relay': {
      service_id: 'svc-relay', name: 'relay-target', kind: 'http', scope_id: null,
      upstream_url: 'http://127.0.0.1:2724', health_check_url: null,
      status: 'active', health_status: 'healthy', routes_count: 1, endpoints_count: 1,
      latency_ms: 12, created_at: '2026-06-22T10:00:00Z', updated_at: NOW,
      routes: [{ route_id: 'route-relay', domain: 'relay.example.com', status: 'active' }],
      endpoints: [{
        endpoint_id: 'ep-relay', service_id: 'svc-relay', node_id: 'node-b', node_name: 'Server B',
        protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 2724,
        address_type: 'local', relay_eligible: true, health_status: 'healthy',
        latency_ms: 5, last_checked_at: new Date(Date.now() - 30000).toISOString(),
      }],
      gateway_policy: {
        id: 'pol-svc-svc-relay', target_type: 'service', target_id: 'svc-relay', target_name: 'relay-target',
        mode: 'fixed', primary_gateway_id: 'gw_public_b', fallback_gateway_ids: [],
        allow_local: false, allow_private: true, allow_public: true,
        require_gateway_link: true, require_relay: true,
        preserve_host: true, tls_mode: 'http_only', enabled: true, priority: 8,
        created_at: '2026-06-22T10:00:00Z', updated_at: NOW,
      },
    },
    'svc-policy': {
      service_id: 'svc-policy', name: 'policy-web', kind: 'http', scope_id: null,
      upstream_url: 'http://127.0.0.1:3001', health_check_url: 'http://127.0.0.1:3001/health',
      status: 'active', health_status: 'healthy', routes_count: 1, endpoints_count: 1,
      latency_ms: 5, created_at: '2026-06-20T10:00:00Z', updated_at: NOW,
      routes: [{ route_id: 'route-policy', domain: 'policy.example.com', status: 'active' }],
      endpoints: [{
        endpoint_id: 'ep-policy', service_id: 'svc-policy', node_id: 'node-a', node_name: 'Server A',
        protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 3001,
        address_type: 'local', relay_eligible: false, health_status: 'healthy',
        latency_ms: 2, last_checked_at: new Date(Date.now() - 45000).toISOString(),
      }],
      gateway_policy: {
        id: 'pol-svc-svc-policy', target_type: 'service', target_id: 'svc-policy', target_name: 'policy-web',
        mode: 'disabled', primary_gateway_id: null, fallback_gateway_ids: [],
        allow_local: false, allow_private: false, allow_public: false,
        require_gateway_link: false, require_relay: false,
        preserve_host: true, tls_mode: 'http_only', enabled: false, priority: 0,
        created_at: '2026-06-22T10:00:00Z', updated_at: NOW,
      },
    },
  },

  routeDetails: {
    'route-api-b': {
      route_id: 'route-api-b', domain: 'api-b.example.com', service_id: 'svc-api-b', service_name: 'api-b-prod',
      scope_id: null, tls_mode: 'http_only', preserve_host: true, public_allowed: false, status: 'active',
      gateway_policy: null,
      created_at: '2026-06-20T10:00:00Z', updated_at: NOW,
      endpoint: {
        endpoint_id: 'ep-api-b', service_id: 'svc-api-b', node_id: 'node-b', node_name: 'Server B',
        protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 18081,
        address_type: 'local', relay_eligible: true, health_status: 'healthy',
        latency_ms: 3, last_checked_at: new Date(Date.now() - 60000).toISOString(),
        routes: ['route-api-b'],
      },
      policy_summary: 'mode: auto, primary: gw_private_b, require_gateway_link: true',
      routing_status: 'available',
    },
    'route-relay': {
      route_id: 'route-relay', domain: 'relay.example.com', service_id: 'svc-relay', service_name: 'relay-target',
      scope_id: null, tls_mode: 'http_only', preserve_host: true, public_allowed: true, status: 'active',
      gateway_policy: null,
      created_at: '2026-06-22T10:00:00Z', updated_at: NOW,
      endpoint: {
        endpoint_id: 'ep-relay', service_id: 'svc-relay', node_id: 'node-b', node_name: 'Server B',
        protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 2724,
        address_type: 'local', relay_eligible: true, health_status: 'healthy',
        latency_ms: 5, last_checked_at: new Date(Date.now() - 30000).toISOString(),
        routes: ['route-relay'],
      },
      policy_summary: 'mode: fixed, primary: gw_public_b, require_relay: true',
      routing_status: 'available',
    },
    'route-policy': {
      route_id: 'route-policy', domain: 'policy.example.com', service_id: 'svc-policy', service_name: 'policy-web',
      scope_id: null, tls_mode: 'http_only', preserve_host: false, public_allowed: true, status: 'active',
      gateway_policy: null,
      created_at: '2026-06-20T10:00:00Z', updated_at: NOW,
      endpoint: {
        endpoint_id: 'ep-policy', service_id: 'svc-policy', node_id: 'node-a', node_name: 'Server A',
        protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 3001,
        address_type: 'local', relay_eligible: false, health_status: 'healthy',
        latency_ms: 2, last_checked_at: new Date(Date.now() - 45000).toISOString(),
        routes: ['route-policy'],
      },
      policy_summary: 'mode: disabled — no routing entry generated',
      routing_status: 'unavailable',
    },
  },

  endpointDetails: {
    'ep-api-b': {
      endpoint_id: 'ep-api-b', service_id: 'svc-api-b', node_id: 'node-b', node_name: 'Server B',
      protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 18081,
      address_type: 'local', relay_eligible: true, health_status: 'healthy',
      latency_ms: 3, last_checked_at: new Date(Date.now() - 60000).toISOString(),
      routes: ['route-api-b'],
    },
    'ep-relay': {
      endpoint_id: 'ep-relay', service_id: 'svc-relay', node_id: 'node-b', node_name: 'Server B',
      protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 2724,
      address_type: 'local', relay_eligible: true, health_status: 'healthy',
      latency_ms: 5, last_checked_at: new Date(Date.now() - 30000).toISOString(),
      routes: ['route-relay'],
    },
    'ep-policy': {
      endpoint_id: 'ep-policy', service_id: 'svc-policy', node_id: 'node-a', node_name: 'Server A',
      protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 3001,
      address_type: 'local', relay_eligible: false, health_status: 'healthy',
      latency_ms: 2, last_checked_at: new Date(Date.now() - 45000).toISOString(),
      routes: ['route-policy'],
    },
  },

  routingEntries: [],
};
