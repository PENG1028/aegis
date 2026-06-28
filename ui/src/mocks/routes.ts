import type { Route, RouteDetail } from '@/types';

export const mockRoutes: Route[] = [
  {
    route_id: 'route-api-b',
    domain: 'api-b.example.com',
    service_id: 'svc-api-b',
    service_name: 'api-b-prod',
    scope_id: 'sc_default',
    tls_mode: 'http_only',
    preserve_host: true,
    public_allowed: false,
    status: 'active',
    gateway_policy: null,
    created_at: '2026-06-20T10:00:00Z',
    updated_at: '2026-06-27T12:00:00Z',
  },
  {
    route_id: 'route-relay',
    domain: 'relay.example.com',
    service_id: 'svc-relay',
    service_name: 'relay-target',
    scope_id: 'sc_default',
    tls_mode: 'http_only',
    preserve_host: true,
    public_allowed: true,
    status: 'active',
    gateway_policy: null,
    created_at: '2026-06-22T10:00:00Z',
    updated_at: '2026-06-27T12:00:00Z',
  },
  {
    route_id: 'route-policy',
    domain: 'policy.example.com',
    service_id: 'svc-policy',
    service_name: 'policy-web',
    scope_id: 'sc_default',
    tls_mode: 'http_only',
    preserve_host: false,
    public_allowed: true,
    status: 'active',
    gateway_policy: null,
    created_at: '2026-06-20T10:00:00Z',
    updated_at: '2026-06-26T18:00:00Z',
  },
];

const ROUTE_ENDPOINTS: Record<string, {
  endpoint_id: string; node_id: string; node_name: string; port: number;
  relay_eligible: boolean; policy_summary: string;
}> = {
  'route-api-b': {
    endpoint_id: 'ep-api-b', node_id: 'node-b', node_name: 'Server B', port: 18081,
    relay_eligible: true, policy_summary: 'mode: auto, require_gateway_link: true, require_relay: true',
  },
  'route-relay': {
    endpoint_id: 'ep-relay', node_id: 'node-b', node_name: 'Server B', port: 2724,
    relay_eligible: true, policy_summary: 'mode: fixed, primary: gw_public_b, require_relay: true',
  },
  'route-policy': {
    endpoint_id: 'ep-policy', node_id: 'node-a', node_name: 'Server A', port: 3001,
    relay_eligible: false, policy_summary: 'mode: disabled — no routing entry generated',
  },
};

export function mockRouteDetail(id: string): RouteDetail | null {
  const route = mockRoutes.find((r) => r.route_id === id);
  if (!route) return null;
  const re = ROUTE_ENDPOINTS[id];
  if (!re) return null;
  return {
    ...route,
    endpoint: {
      endpoint_id: re.endpoint_id,
      service_id: route.service_id,
      node_id: re.node_id,
      node_name: re.node_name,
      protocol: 'http',
      target_local_host: '127.0.0.1',
      target_local_port: re.port,
      address_type: 'local',
      relay_eligible: re.relay_eligible,
      health_status: 'healthy',
      latency_ms: re.port === 18081 ? 3 : re.port === 2724 ? 5 : 2,
      last_checked_at: new Date(Date.now() - 60000).toISOString(),
      routes: [id],
    },
    policy_summary: re.policy_summary,
    routing_status: id === 'route-policy' ? 'unavailable' : 'available',
  };
}
