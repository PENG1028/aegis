import type { Node, NodeDetail } from '@/types';
import { mockGateways } from './gateways';

export const mockNodes: Node[] = [
  {
    node_id: 'node-a',
    name: 'Server A',
    hostname: 'VM-0-4-ubuntu',
    public_ip: '<SERVER_A_IP>',
    private_ip: '10.0.1.4',
    roles: ['control_plane', 'gateway'],
    status: 'online',
    os: 'Ubuntu 22.04',
    arch: 'amd64',
    agent_version: 'v1.8C',
    last_heartbeat_at: new Date(Date.now() - 8000).toISOString(),
    capabilities: {
      gateway_enabled: true,
      caddy_installed: true,
      haproxy_installed: false,
      tls_supported: false,
      dns_control_available: false,
      hot_reload_supported: true,
      edge_mux_supported: false,
      relay_capable: true,
      local_gateway_enabled: true,
    },
    desired_revision: 17,
    applied_revision: 17,
    sync_status: 'in_sync',
    created_at: '2026-06-20T10:00:00Z',
    updated_at: '2026-06-27T12:00:00Z',
  },
  {
    node_id: 'node-b',
    name: 'Server B',
    hostname: 'VM-0-11-ubuntu',
    public_ip: '<SERVER_B_IP>',
    private_ip: '10.0.1.11',
    roles: ['gateway', 'relay_target'],
    status: 'online',
    os: 'Ubuntu 22.04',
    arch: 'amd64',
    agent_version: 'v1.8C',
    last_heartbeat_at: new Date(Date.now() - 15000).toISOString(),
    capabilities: {
      gateway_enabled: true,
      caddy_installed: true,
      haproxy_installed: false,
      tls_supported: false,
      dns_control_available: false,
      hot_reload_supported: true,
      edge_mux_supported: false,
      relay_capable: true,
      local_gateway_enabled: false,
    },
    desired_revision: 17,
    applied_revision: 17,
    sync_status: 'in_sync',
    created_at: '2026-06-20T10:00:00Z',
    updated_at: '2026-06-27T12:00:00Z',
  },
  {
    node_id: 'node-c',
    name: 'Server C (planned)',
    hostname: '—',
    public_ip: '—',
    private_ip: '—',
    roles: ['gateway'],
    status: 'unknown',
    os: '—',
    arch: '—',
    agent_version: '—',
    last_heartbeat_at: null,
    capabilities: {
      gateway_enabled: false,
      caddy_installed: false,
      haproxy_installed: false,
      tls_supported: false,
      dns_control_available: false,
      hot_reload_supported: false,
      edge_mux_supported: false,
      relay_capable: false,
      local_gateway_enabled: false,
    },
    desired_revision: 0,
    applied_revision: 0,
    sync_status: 'no_desired_state',
    created_at: '2026-06-25T10:00:00Z',
    updated_at: '2026-06-25T10:00:00Z',
  },
];

export function mockNodeDetail(nodeId: string): NodeDetail | null {
  const node = mockNodes.find((n) => n.node_id === nodeId);
  if (!node) return null;

  return {
    ...node,
    gateways: mockGateways.filter((g) => g.node_id === node.node_id),
    sync: {
      status: node.sync_status,
      desired_revision: node.desired_revision,
      applied_revision: node.applied_revision,
      desired_hash: 'd8e5f7a2b3c1',
      actual_hash: 'd8e5f7a2b3c1',
      last_apply_at: new Date(Date.now() - 120000).toISOString(),
      last_success_at: new Date(Date.now() - 120000).toISOString(),
      last_error: null,
    },
    local_gateway: node.node_id === 'node-a'
      ? {
          bind_addr: '127.0.0.1',
          port: 18080,
          status: 'running',
          routing_table_loaded: true,
          routing_table_revision: 17,
          entries_count: 3,
          cache_status: 'fresh',
          last_error: null,
        }
      : undefined,
    routing_table_entries: 3,
    last_error: null,
    diagnostics: computeDiagnostics(node),
  };
}

function computeDiagnostics(node: Node): NodeDetail['diagnostics'] {
  const hbAge = node.last_heartbeat_at
    ? `${Math.round((Date.now() - new Date(node.last_heartbeat_at).getTime()) / 1000)}s`
    : 'never';
  return [
    { name: 'Node ID', status: 'ok', message: node.node_id },
    { name: 'Connectivity', status: node.status === 'online' ? 'ok' : 'error', message: `Heartbeat received ${hbAge} ago` },
    { name: 'Routing Table', status: node.sync_status === 'in_sync' ? 'ok' : 'warning', message: `Revision ${node.applied_revision}, status: ${node.sync_status}` },
    { name: 'Local Gateway', status: node.node_id === 'node-a' ? 'ok' : 'warning', message: node.node_id === 'node-a' ? 'Running on :18080' : 'Not enabled' },
  ];
}
