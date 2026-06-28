import type { TopologyEdge, TopologyPathResult } from '@/types';

export const mockTopologyMatrix: TopologyEdge[] = [
  {
    from_node_id: 'node-a',
    from_node_name: 'Server A',
    to_node_id: 'node-b',
    to_node_name: 'Server B',
    private_reachable: true,
    public_reachable: true,
    preferred_gateway_id: 'gw_public_a',
    gateway_link_id: 'gl-a-b',
    status: 'verified',
    last_verified_at: new Date(Date.now() - 120000).toISOString(),
    last_error: null,
  },
  {
    from_node_id: 'node-b',
    from_node_name: 'Server B',
    to_node_id: 'node-a',
    to_node_name: 'Server A',
    private_reachable: true,
    public_reachable: true,
    preferred_gateway_id: 'gw_public_b',
    gateway_link_id: 'gl-a-b',
    status: 'verified',
    last_verified_at: new Date(Date.now() - 120000).toISOString(),
    last_error: null,
  },
  {
    from_node_id: 'node-a',
    from_node_name: 'Server A',
    to_node_id: 'node-c',
    to_node_name: 'Server C (planned)',
    private_reachable: false,
    public_reachable: false,
    preferred_gateway_id: null,
    gateway_link_id: null,
    status: 'missing_link',
    last_verified_at: null,
    last_error: 'Node not deployed',
  },
  {
    from_node_id: 'node-b',
    from_node_name: 'Server B',
    to_node_id: 'node-c',
    to_node_name: 'Server C (planned)',
    private_reachable: false,
    public_reachable: false,
    preferred_gateway_id: null,
    gateway_link_id: null,
    status: 'missing_link',
    last_verified_at: null,
    last_error: 'Node not deployed',
  },
];

export function mockTopologyPath(from: string, to: string): TopologyPathResult {
  if (from === 'node-a' && to === 'node-b') {
    return {
      from_node: 'node-a',
      to_node: 'node-b',
      path: [
        {
          hop: 0,
          node_id: 'node-a',
          node_name: 'Server A',
          via: 'local',
          gateway_url: null,
          gateway_link_id: null,
          status: 'ok',
          reason: 'Local gateway on :18080',
        },
        {
          hop: 1,
          node_id: 'node-b',
          node_name: 'Server B',
          via: 'public_gateway',
          gateway_url: 'http://43.159.34.11:80',
          gateway_link_id: 'gl-a-b',
          status: 'ok',
          reason: 'POST /__aegis/relay → Caddy → relay handler → target',
        },
      ],
      reachable: true,
      total_hops: 1,
      summary: 'Node A → Node B via public gateway :80, GatewayLink gl-a-b verified',
    };
  }
  return {
    from_node: from,
    to_node: to,
    path: [],
    reachable: false,
    total_hops: 0,
    summary: 'No path available',
  };
}
