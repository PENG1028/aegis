import type { LocalGatewayStatus } from '@/types';

export function mockLocalGateway(nodeId?: string): LocalGatewayStatus[] {
  const all: LocalGatewayStatus[] = [
    {
      node_id: 'node-a',
      node_name: 'Server A',
      bind_addr: '127.0.0.1',
      port: 18080,
      status: 'running',
      routing_table_loaded: true,
      routing_table_revision: 17,
      entries_count: 3,
      cache_status: 'fresh',
      diagnostics: [
        { name: 'node_id', status: 'ok', message: 'node-a' },
        { name: 'bind_addr', status: 'ok', message: '127.0.0.1:18080' },
        { name: 'routing_table', status: 'ok', message: 'Revision 17, 3 entries' },
        { name: 'cache', status: 'ok', message: 'Fresh — last sync 30s ago' },
      ],
      last_error: null,
    },
    {
      node_id: 'node-b',
      node_name: 'Server B',
      bind_addr: '—',
      port: 0,
      status: 'stopped',
      routing_table_loaded: false,
      routing_table_revision: null,
      entries_count: 0,
      cache_status: 'empty',
      diagnostics: [
        { name: 'local_gateway', status: 'warning', message: 'Not enabled on this node' },
      ],
      last_error: 'Local gateway not configured for node-b',
    },
  ];

  if (nodeId) return all.filter((g) => g.node_id === nodeId);
  return all;
}
