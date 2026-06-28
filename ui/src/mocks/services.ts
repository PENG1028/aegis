import type { Service, ServiceDetail } from '@/types';

export const mockServices: Service[] = [
  {
    service_id: 'svc-api-b',
    name: 'api-b-prod',
    kind: 'http',
    scope_id: 'sc_default',
    upstream_url: 'http://127.0.0.1:18081',
    health_check_url: 'http://127.0.0.1:18081/health',
    status: 'active',
    health_status: 'healthy',
    routes_count: 1,
    endpoints_count: 1,
    latency_ms: 38,
    created_at: '2026-06-20T10:00:00Z',
    updated_at: '2026-06-27T12:00:00Z',
  },
  {
    service_id: 'svc-relay',
    name: 'relay-target',
    kind: 'http',
    scope_id: 'sc_default',
    upstream_url: 'http://127.0.0.1:2724',
    health_check_url: null,
    status: 'active',
    health_status: 'healthy',
    routes_count: 1,
    endpoints_count: 1,
    latency_ms: 12,
    created_at: '2026-06-22T10:00:00Z',
    updated_at: '2026-06-27T12:00:00Z',
  },
  {
    service_id: 'svc-policy',
    name: 'policy-web',
    kind: 'http',
    scope_id: 'sc_default',
    upstream_url: 'http://127.0.0.1:3001',
    health_check_url: 'http://127.0.0.1:3001/health',
    status: 'active',
    health_status: 'healthy',
    routes_count: 1,
    endpoints_count: 1,
    latency_ms: 5,
    created_at: '2026-06-20T10:00:00Z',
    updated_at: '2026-06-26T18:00:00Z',
  },
];

const SERVICE_ENDPOINTS: Record<string, {
  route: { route_id: string; domain: string; status: string };
  endpoint: {
    endpoint_id: string; node_id: string; node_name: string; port: number;
    relay_eligible: boolean; address_type: 'local' | 'remote';
  };
  policy: {
    mode: 'auto' | 'fixed' | 'multi' | 'disabled';
    allow_local: boolean; allow_private: boolean; allow_public: boolean;
    require_gateway_link: boolean; require_relay: boolean;
  };
}> = {
  'svc-api-b': {
    route: { route_id: 'route-api-b', domain: 'api-b.example.com', status: 'active' },
    endpoint: { endpoint_id: 'ep-api-b', node_id: 'node-b', node_name: 'Server B', port: 18081, relay_eligible: true, address_type: 'local' },
    policy: { mode: 'auto', allow_local: true, allow_private: true, allow_public: false, require_gateway_link: true, require_relay: true },
  },
  'svc-relay': {
    route: { route_id: 'route-relay', domain: 'relay.example.com', status: 'active' },
    endpoint: { endpoint_id: 'ep-relay', node_id: 'node-b', node_name: 'Server B', port: 2724, relay_eligible: true, address_type: 'local' },
    policy: { mode: 'fixed', allow_local: false, allow_private: true, allow_public: true, require_gateway_link: true, require_relay: true },
  },
  'svc-policy': {
    route: { route_id: 'route-policy', domain: 'policy.example.com', status: 'active' },
    endpoint: { endpoint_id: 'ep-policy', node_id: 'node-a', node_name: 'Server A', port: 3001, relay_eligible: false, address_type: 'local' },
    policy: { mode: 'disabled', allow_local: false, allow_private: false, allow_public: false, require_gateway_link: false, require_relay: false },
  },
};

export function mockServiceDetail(id: string): ServiceDetail | null {
  const svc = mockServices.find((s) => s.service_id === id);
  if (!svc) return null;
  const se = SERVICE_ENDPOINTS[id];
  if (!se) return null;
  const now = new Date();
  return {
    ...svc,
    routes: [se.route],
    endpoints: [{
      endpoint_id: se.endpoint.endpoint_id,
      service_id: svc.service_id,
      node_id: se.endpoint.node_id,
      node_name: se.endpoint.node_name,
      protocol: 'http',
      target_local_host: '127.0.0.1',
      target_local_port: se.endpoint.port,
      address_type: se.endpoint.address_type,
      relay_eligible: se.endpoint.relay_eligible,
      health_status: 'healthy',
      latency_ms: se.endpoint.port === 18081 ? 3 : se.endpoint.port === 2724 ? 5 : 2,
      last_checked_at: new Date(now.getTime() - 60000).toISOString(),
    }],
    gateway_policy: {
      id: 'pol-svc-' + id,
      target_type: 'service',
      target_id: id,
      target_name: svc.name,
      mode: se.policy.mode,
      primary_gateway_id: null,
      fallback_gateway_ids: [],
      allow_local: se.policy.allow_local,
      allow_private: se.policy.allow_private,
      allow_public: se.policy.allow_public,
      require_gateway_link: se.policy.require_gateway_link,
      require_relay: se.policy.require_relay,
      preserve_host: true,
      tls_mode: 'http_only',
      enabled: se.policy.mode !== 'disabled',
      priority: se.policy.mode === 'fixed' ? 200 : 100,
      created_at: '2026-06-22T10:00:00Z',
      updated_at: '2026-06-27T12:00:00Z',
    },
  };
}
