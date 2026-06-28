import type { RoutingEntry, RoutingPreviewResult, RoutingValidationResult } from '@/types';

export function mockRoutingTable(nodeId?: string): RoutingEntry[] {
  const entries: RoutingEntry[] = [
    {
      domain: 'api-b.example.com',
      route_id: 'route-api-b',
      service_id: 'svc-api-b',
      endpoint_id: 'ep-api-b',
      from_node_id: 'node-a',
      target_node_id: 'node-b',
      protocol: 'http',
      target_local_host: '127.0.0.1',
      target_local_port: 18081,
      policy_mode: 'auto',
      candidates: [
        {
          mode: 'local_gateway',
          gateway_id: 'gw_local_a',
          gateway_url: 'http://127.0.0.1:18080',
          priority: 100,
          requires_gateway_link: false,
          gateway_link_id: null,
        },
        {
          mode: 'private_gateway',
          gateway_id: 'gw_private_a',
          gateway_url: 'http://10.0.1.4:80',
          priority: 80,
          requires_gateway_link: true,
          gateway_link_id: 'gl-a-b',
        },
        {
          mode: 'public_gateway',
          gateway_id: 'gw_public_a',
          gateway_url: 'http://<SERVER_A_IP>:80',
          priority: 50,
          requires_gateway_link: true,
          gateway_link_id: 'gl-a-b',
        },
      ],
      status: 'available',
      unavailable_reason: null,
    },
    {
      domain: 'relay.example.com',
      route_id: 'route-relay',
      service_id: 'svc-relay',
      endpoint_id: 'ep-relay',
      from_node_id: 'node-a',
      target_node_id: 'node-b',
      protocol: 'http',
      target_local_host: '127.0.0.1',
      target_local_port: 2724,
      policy_mode: 'fixed',
      candidates: [
        {
          mode: 'public_gateway',
          gateway_id: 'gw_public_b',
          gateway_url: 'http://<SERVER_B_IP>:80',
          priority: 50,
          requires_gateway_link: true,
          gateway_link_id: 'gl-a-b',
        },
      ],
      status: 'available',
      unavailable_reason: null,
    },
    {
      domain: 'policy.example.com',
      route_id: 'route-policy',
      service_id: 'svc-policy',
      endpoint_id: 'ep-policy',
      from_node_id: 'node-a',
      target_node_id: 'node-a',
      protocol: 'http',
      target_local_host: '127.0.0.1',
      target_local_port: 3001,
      policy_mode: 'disabled',
      candidates: [],
      status: 'disabled',
      unavailable_reason: 'Policy mode is disabled',
    },
  ];

  if (nodeId) {
    return entries.filter((e) => e.from_node_id === nodeId);
  }
  return entries;
}

export function mockRoutingPreview(domain: string, fromNode: string): RoutingPreviewResult {
  const entries = mockRoutingTable(fromNode).filter((e) => e.domain === domain);
  return {
    domain,
    from_node_id: fromNode,
    from_node_name: fromNode === 'node-a' ? 'Server A' : fromNode === 'node-b' ? 'Server B' : fromNode,
    entries,
    available: entries.some((e) => e.status === 'available'),
    summary: entries.length
      ? entries[0].status === 'available'
        ? `${domain} → ${entries[0].endpoint_id} → ${entries[0].target_local_host}:${entries[0].target_local_port}`
        : `${domain}: ${entries[0].unavailable_reason}`
      : 'No routing entries found for this domain from this node',
    unavailable_reason: entries.length ? entries[0].unavailable_reason : 'No routing entries',
  };
}

export function mockRoutingValidate(nodeId?: string): RoutingValidationResult {
  const entries = mockRoutingTable(nodeId);
  return {
    valid: true,
    node_id: nodeId || null,
    total_entries: entries.length,
    errors: [],
    warnings: [
      {
        domain: 'policy.example.com',
        code: 'POLICY_DISABLED',
        message: 'Policy mode is disabled — no routing entry generated',
      },
    ],
    valid_count: entries.filter((e) => e.status === 'available').length,
    error_count: 0,
    warning_count: 1,
  };
}
