import type { SyncStatus } from '@/types';

export function mockSyncStatus(nodeId?: string): SyncStatus[] {
  const all: SyncStatus[] = [
    {
      node_id: 'node-a',
      node_name: 'Server A',
      desired_revision: 17,
      applied_revision: 17,
      desired_hash: 'd8e5f7a2b3c1',
      actual_hash: 'd8e5f7a2b3c1',
      status: 'in_sync',
      last_apply_at: new Date(Date.now() - 120000).toISOString(),
      last_success_at: new Date(Date.now() - 120000).toISOString(),
      last_error: null,
      provider_status: { status: 'ok', message: 'Caddy config applied' },
      relay_status: { status: 'ok', message: 'Relay handler running' },
      gateway_status: { status: 'ok', message: 'Gateway active' },
      diagnostics_status: { status: 'ok', message: 'All checks passed' },
    },
    {
      node_id: 'node-b',
      node_name: 'Server B',
      desired_revision: 17,
      applied_revision: 17,
      desired_hash: 'd8e5f7a2b3c1',
      actual_hash: 'd8e5f7a2b3c1',
      status: 'in_sync',
      last_apply_at: new Date(Date.now() - 60000).toISOString(),
      last_success_at: new Date(Date.now() - 60000).toISOString(),
      last_error: null,
      provider_status: { status: 'ok', message: 'Caddy config applied' },
      relay_status: { status: 'ok', message: 'Relay handler running' },
      gateway_status: { status: 'ok', message: 'Gateway active' },
      diagnostics_status: { status: 'ok', message: 'All checks passed' },
    },
    {
      node_id: 'node-c',
      node_name: 'Server C (planned)',
      desired_revision: 0,
      applied_revision: 0,
      desired_hash: '',
      actual_hash: '',
      status: 'no_desired_state',
      last_apply_at: null,
      last_success_at: null,
      last_error: 'Node not deployed',
      provider_status: { status: 'unknown', message: 'No provider configured' },
      relay_status: { status: 'unknown', message: 'No relay configured' },
      gateway_status: { status: 'unknown', message: 'No gateway configured' },
      diagnostics_status: { status: 'unknown', message: 'No diagnostics available' },
    },
  ];

  if (nodeId) return all.filter((s) => s.node_id === nodeId);
  return all;
}
