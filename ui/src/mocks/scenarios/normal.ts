// ─── Scenario 1: Normal ───
// api.proofnote.dev → edge-443 → gateway-main → service-proofnote-api → endpoint-a/node-a + endpoint-b/node-b
// All healthy, no anomalies.

import type { ScenarioData } from './types';

const NOW = '2026-07-02T10:30:00Z';
const HOUR_AGO = '2026-07-02T09:30:00Z';

export const scenarioNormal: ScenarioData = {
  meta: {
    name: '正常链路',
    description: 'api.proofnote.dev 全链路正常，所有节点在线，配置已同步',
  },

  nodes: [
    {
      node_id: 'node-a', name: 'Server A', hostname: 'vps-a',
      public_ip: '43.160.211.232', private_ip: '10.0.0.1',
      roles: ['gateway', 'panel'], status: 'online',
      os: 'Ubuntu 24.04', arch: 'amd64', agent_version: 'v1.8L',
      last_heartbeat_at: NOW,
      capabilities: { gateway_enabled: true, caddy_installed: true, haproxy_installed: false, tls_supported: true, dns_control_available: true, hot_reload_supported: true, edge_mux_supported: true, relay_capable: true, local_gateway_enabled: true },
      desired_revision: 43, applied_revision: 43, sync_status: 'in_sync',
      created_at: '2026-01-01T00:00:00Z', updated_at: NOW,
    },
    {
      node_id: 'node-b', name: 'Server B', hostname: 'vps-b',
      public_ip: '43.159.34.11', private_ip: '10.0.0.2',
      roles: ['gateway', 'backend'], status: 'online',
      os: 'Ubuntu 24.04', arch: 'amd64', agent_version: 'v1.8L',
      last_heartbeat_at: NOW,
      capabilities: { gateway_enabled: true, caddy_installed: true, haproxy_installed: false, tls_supported: true, dns_control_available: false, hot_reload_supported: true, edge_mux_supported: false, relay_capable: true, local_gateway_enabled: true },
      desired_revision: 43, applied_revision: 43, sync_status: 'in_sync',
      created_at: '2026-01-01T00:00:00Z', updated_at: NOW,
    },
    {
      node_id: 'node-c', name: 'Server C', hostname: 'vps-c',
      public_ip: '43.160.50.10', private_ip: '10.0.0.3',
      roles: ['backend'], status: 'online',
      os: 'Ubuntu 24.04', arch: 'amd64', agent_version: 'v1.8L',
      last_heartbeat_at: HOUR_AGO,
      capabilities: { gateway_enabled: false, caddy_installed: false, haproxy_installed: false, tls_supported: false, dns_control_available: false, hot_reload_supported: false, edge_mux_supported: false, relay_capable: false, local_gateway_enabled: false },
      desired_revision: 43, applied_revision: 43, sync_status: 'in_sync',
      created_at: '2026-03-01T00:00:00Z', updated_at: NOW,
    },
  ],

  gateways: [
    {
      gateway_id: 'gateway-main', node_id: 'node-a', node_name: 'Server A',
      name: '主网关', type: 'public', provider: 'caddy',
      bind_addr: '0.0.0.0', host: '43.160.211.232', port: 443,
      scheme: 'https', public_accessible: true, private_accessible: true,
      enabled: true, priority: 10, status: 'active',
      last_verified_at: NOW, last_error: null,
      created_at: '2026-01-01T00:00:00Z', updated_at: NOW,
    },
    {
      gateway_id: 'gateway-edge', node_id: 'node-a', node_name: 'Server A',
      name: '边缘网关', type: 'public', provider: 'caddy',
      bind_addr: '0.0.0.0', host: '43.160.211.232', port: 80,
      scheme: 'http', public_accessible: true, private_accessible: true,
      enabled: true, priority: 5, status: 'active',
      last_verified_at: NOW, last_error: null,
      created_at: '2026-01-01T00:00:00Z', updated_at: NOW,
    },
    {
      gateway_id: 'gateway-private', node_id: 'node-b', node_name: 'Server B',
      name: '私网网关', type: 'private', provider: 'caddy',
      bind_addr: '0.0.0.0', host: '10.0.0.2', port: 80,
      scheme: 'http', public_accessible: false, private_accessible: true,
      enabled: true, priority: 8, status: 'active',
      last_verified_at: NOW, last_error: null,
      created_at: '2026-01-01T00:00:00Z', updated_at: NOW,
    },
  ],

  services: [
    {
      service_id: 'service-proofnote-api', name: 'proofnote-api',
      kind: 'http', scope_id: null, upstream_url: 'http://localhost:3000',
      health_check_url: '/health', status: 'active', health_status: 'healthy',
      routes_count: 1, endpoints_count: 2, latency_ms: 12,
      created_at: '2026-01-01T00:00:00Z', updated_at: NOW,
    },
    {
      service_id: 'service-auth', name: 'auth-service',
      kind: 'http', scope_id: null, upstream_url: 'http://localhost:4000',
      health_check_url: '/health', status: 'active', health_status: 'healthy',
      routes_count: 1, endpoints_count: 2, latency_ms: 8,
      created_at: '2026-01-01T00:00:00Z', updated_at: NOW,
    },
    {
      service_id: 'service-docs', name: 'docs-service',
      kind: 'http', scope_id: null, upstream_url: 'http://localhost:8080',
      health_check_url: '/health', status: 'active', health_status: 'healthy',
      routes_count: 1, endpoints_count: 1, latency_ms: 5,
      created_at: '2026-06-01T00:00:00Z', updated_at: NOW,
    },
  ],

  routes: [
    {
      route_id: 'route-api', domain: 'api.proofnote.dev',
      service_id: 'service-proofnote-api', service_name: 'proofnote-api',
      scope_id: null, tls_mode: 'terminate_local', preserve_host: true,
      public_allowed: true, status: 'active',
      gateway_policy: {
        id: 'policy-api', target_type: 'route', target_id: 'route-api', target_name: 'api.proofnote.dev',
        mode: 'multi', primary_gateway_id: 'gateway-main', fallback_gateway_ids: ['gateway-private'],
        allow_local: true, allow_private: true, allow_public: true,
        require_gateway_link: false, require_relay: false,
        preserve_host: true, tls_mode: 'terminate_local',
        enabled: true, priority: 10,
        created_at: '2026-01-01T00:00:00Z', updated_at: NOW,
      },
      created_at: '2026-01-01T00:00:00Z', updated_at: NOW,
    },
    {
      route_id: 'route-auth', domain: 'auth.proofnote.dev',
      service_id: 'service-auth', service_name: 'auth-service',
      scope_id: null, tls_mode: 'terminate_local', preserve_host: true,
      public_allowed: true, status: 'active',
      gateway_policy: {
        id: 'policy-auth', target_type: 'route', target_id: 'route-auth', target_name: 'auth.proofnote.dev',
        mode: 'fixed', primary_gateway_id: 'gateway-main', fallback_gateway_ids: [],
        allow_local: true, allow_private: true, allow_public: false,
        require_gateway_link: false, require_relay: false,
        preserve_host: true, tls_mode: 'terminate_local',
        enabled: true, priority: 8,
        created_at: '2026-01-01T00:00:00Z', updated_at: NOW,
      },
      created_at: '2026-01-01T00:00:00Z', updated_at: NOW,
    },
    {
      route_id: 'route-docs', domain: 'docs.proofnote.dev',
      service_id: 'service-docs', service_name: 'docs-service',
      scope_id: null, tls_mode: 'http_only', preserve_host: true,
      public_allowed: true, status: 'active',
      gateway_policy: {
        id: 'policy-docs', target_type: 'route', target_id: 'route-docs', target_name: 'docs.proofnote.dev',
        mode: 'fixed', primary_gateway_id: 'gateway-edge', fallback_gateway_ids: [],
        allow_local: true, allow_private: false, allow_public: true,
        require_gateway_link: false, require_relay: false,
        preserve_host: true, tls_mode: 'http_only',
        enabled: true, priority: 5,
        created_at: '2026-06-01T00:00:00Z', updated_at: NOW,
      },
      created_at: '2026-06-01T00:00:00Z', updated_at: NOW,
    },
  ],

  endpoints: [
    {
      endpoint_id: 'endpoint-a', service_id: 'service-proofnote-api', node_id: 'node-a', node_name: 'Server A',
      protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 3000,
      address_type: 'local', relay_eligible: false, health_status: 'healthy',
      latency_ms: 5, last_checked_at: NOW,
    },
    {
      endpoint_id: 'endpoint-b', service_id: 'service-proofnote-api', node_id: 'node-b', node_name: 'Server B',
      protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 3001,
      address_type: 'remote', relay_eligible: true, health_status: 'healthy',
      latency_ms: 18, last_checked_at: NOW,
    },
    {
      endpoint_id: 'endpoint-auth-a', service_id: 'service-auth', node_id: 'node-a', node_name: 'Server A',
      protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 4000,
      address_type: 'local', relay_eligible: false, health_status: 'healthy',
      latency_ms: 3, last_checked_at: NOW,
    },
    {
      endpoint_id: 'endpoint-auth-b', service_id: 'service-auth', node_id: 'node-b', node_name: 'Server B',
      protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 4001,
      address_type: 'remote', relay_eligible: true, health_status: 'healthy',
      latency_ms: 12, last_checked_at: NOW,
    },
    {
      endpoint_id: 'endpoint-docs-a', service_id: 'service-docs', node_id: 'node-c', node_name: 'Server C',
      protocol: 'http', target_local_host: '127.0.0.1', target_local_port: 8080,
      address_type: 'remote', relay_eligible: false, health_status: 'healthy',
      latency_ms: 7, last_checked_at: HOUR_AGO,
    },
  ],

  listeners: [
    { bind_addr: '0.0.0.0', port: 443, provider: 'caddy', purpose: 'https_entry', status: 'active', gateway_id: 'gateway-main', node_id: 'node-a' },
    { bind_addr: '0.0.0.0', port: 80, provider: 'caddy', purpose: 'http_entry', status: 'active', gateway_id: 'gateway-edge', node_id: 'node-a' },
    { bind_addr: '0.0.0.0', port: 80, provider: 'caddy', purpose: 'private_gateway', status: 'active', gateway_id: 'gateway-private', node_id: 'node-b' },
  ],

  gatewayLinks: [
    {
      gateway_link_id: 'link-main-private',
      source_node_id: 'node-a', source_node_name: 'Server A',
      target_node_id: 'node-b', target_node_name: 'Server B',
      status: 'active', created_at: '2026-06-15T08:00:00Z',
      last_verified_at: NOW,
    },
  ],

  policies: [
    // Same as route.gateway_policy for simplicity
  ],

  topologyEdges: [
    {
      from_node_id: 'node-a', from_node_name: 'Server A',
      to_node_id: 'node-b', to_node_name: 'Server B',
      private_reachable: true, public_reachable: true,
      preferred_gateway_id: 'gateway-main', gateway_link_id: 'link-main-private',
      status: 'verified', last_verified_at: NOW, last_error: null,
    },
  ],

  entryPoints: [
    {
      route_id: 'route-api', domain: 'api.proofnote.dev', protocol: 'http',
      tls_mode: 'terminate_local',
      listener: { bind_addr: '0.0.0.0', port: 443, provider: 'caddy', purpose: 'https_entry', status: 'active', gateway_id: 'gateway-main', node_id: 'node-a' },
      gateway_id: 'gateway-main', gateway_name: '主网关',
      service_id: 'service-proofnote-api', service_name: 'proofnote-api',
      endpoints: [
        { endpoint_id: 'endpoint-a', node_id: 'node-a', node_name: 'Server A', protocol: 'http', target: '127.0.0.1:3000', health: 'healthy' },
        { endpoint_id: 'endpoint-b', node_id: 'node-b', node_name: 'Server B', protocol: 'http', target: '127.0.0.1:3001', health: 'healthy' },
      ],
      health: 'healthy', safety: 'safe', release_state: 'current',
    },
    {
      route_id: 'route-auth', domain: 'auth.proofnote.dev', protocol: 'http',
      tls_mode: 'terminate_local',
      listener: { bind_addr: '0.0.0.0', port: 443, provider: 'caddy', purpose: 'https_entry', status: 'active', gateway_id: 'gateway-main', node_id: 'node-a' },
      gateway_id: 'gateway-main', gateway_name: '主网关',
      service_id: 'service-auth', service_name: 'auth-service',
      endpoints: [
        { endpoint_id: 'endpoint-auth-a', node_id: 'node-a', node_name: 'Server A', protocol: 'http', target: '127.0.0.1:4000', health: 'healthy' },
        { endpoint_id: 'endpoint-auth-b', node_id: 'node-b', node_name: 'Server B', protocol: 'http', target: '127.0.0.1:4001', health: 'healthy' },
      ],
      health: 'healthy', safety: 'safe', release_state: 'current',
    },
  ],

  anomalies: [],

  dashboard: {
    nodes_online: 3, nodes_total: 3,
    gateways_online: 3, gateways_total: 3,
    managed_routes: 3, routing_tables_synced: 3, routing_tables_total: 3,
    local_gateway_online: 2, local_gateway_total: 2,
    relay_acceptance: 'pass', secret_runtime: 'verified',
    pending_capabilities: [],
    routes_unavailable: 0, missing_gateway_links: 0, outdated_nodes: 0,
    recent_errors: [],
  },

  syncStatuses: [
    {
      node_id: 'node-a', node_name: 'Server A',
      desired_revision: 43, applied_revision: 43,
      desired_hash: 'abc123', actual_hash: 'abc123',
      status: 'in_sync', last_apply_at: NOW, last_success_at: NOW, last_error: null,
      provider_status: { status: 'ok', message: 'Caddy v2.8.4 running' },
      relay_status: { status: 'ok', message: 'Relay active' },
      gateway_status: { status: 'ok', message: '3 gateways active' },
      diagnostics_status: { status: 'ok', message: 'All checks passed' },
    },
    {
      node_id: 'node-b', node_name: 'Server B',
      desired_revision: 43, applied_revision: 43,
      desired_hash: 'def456', actual_hash: 'def456',
      status: 'in_sync', last_apply_at: NOW, last_success_at: NOW, last_error: null,
      provider_status: { status: 'ok', message: 'Caddy v2.8.4 running' },
      relay_status: { status: 'ok', message: 'Relay active' },
      gateway_status: { status: 'ok', message: '1 gateway active' },
      diagnostics_status: { status: 'ok', message: 'All checks passed' },
    },
    {
      node_id: 'node-c', node_name: 'Server C',
      desired_revision: 43, applied_revision: 43,
      desired_hash: 'ghi789', actual_hash: 'ghi789',
      status: 'in_sync', last_apply_at: NOW, last_success_at: NOW, last_error: null,
      provider_status: { status: 'unknown', message: 'No provider installed' },
      relay_status: { status: 'unknown', message: 'No relay' },
      gateway_status: { status: 'unknown', message: 'No gateway' },
      diagnostics_status: { status: 'ok', message: 'All checks passed' },
    },
  ],

  joinTokens: [
    {
      id: 'token-1', name: 'default-node', token_prefix: 'aegis_n_',
      allowed_roles: ['gateway'], expected_node_name: null,
      expires_at: '2027-01-01T00:00:00Z', allowed_source_cidr: null,
      status: 'active', created_at: '2026-01-01T00:00:00Z', used_at: null,
    },
  ],

  acceptance: {
    labels: [
      { key: 'gateway-link', label: 'Gateway Link 认证', status: 'pass', evidence: 'HMAC-SHA256 双节点验证通过' },
      { key: 'caddy-reload', label: 'Caddy 热重载', status: 'pass', evidence: 'caddy reload 成功，无中断' },
      { key: 'sync', label: '配置同步', status: 'pass', evidence: 'desired == actual，版本 43' },
    ],
    summary: { total_labels: 3, pass_count: 3, pending_count: 0, deferred_count: 0 },
    last_acceptance: {
      command: 'make acceptance',
      http_status: 200,
      response_summary: 'All checks passed',
      selected_candidate: 'gateway-main',
      gateway_link_id: 'link-main-private',
      token_leak_scan: 'clean',
      negative_smoke_result: 'pass',
      docs_link: '/docs/acceptance',
      executed_at: NOW,
    },
    negative_smoke: [],
  },

  dnsStatus: {
    running: true, listen_addr: ':5353', upstream: '1.1.1.1:53',
    enabled: true, local_hits: 1523, upstream_calls: 45,
    managed_count: 3,
  },

  nodeDetails: {},
  gatewayDetails: {},
  serviceDetails: {},
  routeDetails: {},
  endpointDetails: {},
  routingEntries: [],
};
