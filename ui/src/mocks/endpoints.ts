import type { Endpoint, EndpointDetail } from '@/types';

export const mockEndpoints: Endpoint[] = [
  {
    endpoint_id: 'ep-api-b',
    service_id: 'svc-api-b',
    node_id: 'node-b',
    node_name: 'Server B',
    protocol: 'http',
    target_local_host: '127.0.0.1',
    target_local_port: 18081,
    address_type: 'local',
    relay_eligible: true,
    health_status: 'healthy',
    latency_ms: 3,
    last_checked_at: new Date(Date.now() - 60000).toISOString(),
  },
  {
    endpoint_id: 'ep-relay',
    service_id: 'svc-relay',
    node_id: 'node-b',
    node_name: 'Server B',
    protocol: 'http',
    target_local_host: '127.0.0.1',
    target_local_port: 2724,
    address_type: 'local',
    relay_eligible: true,
    health_status: 'healthy',
    latency_ms: 5,
    last_checked_at: new Date(Date.now() - 30000).toISOString(),
  },
  {
    endpoint_id: 'ep-policy',
    service_id: 'svc-policy',
    node_id: 'node-a',
    node_name: 'Server A',
    protocol: 'http',
    target_local_host: '127.0.0.1',
    target_local_port: 3001,
    address_type: 'local',
    relay_eligible: false,
    health_status: 'healthy',
    latency_ms: 2,
    last_checked_at: new Date(Date.now() - 45000).toISOString(),
  },
];

const EP_ROUTES: Record<string, string[]> = {
  'ep-api-b': ['route-api-b'],
  'ep-relay': ['route-relay'],
  'ep-policy': ['route-policy'],
};

export function mockEndpointDetail(id: string): EndpointDetail | null {
  const ep = mockEndpoints.find((e) => e.endpoint_id === id);
  if (!ep) return null;
  return {
    ...ep,
    routes: EP_ROUTES[id] || [],
  };
}
