import type { JoinToken } from '@/types';

export const mockJoinTokens: JoinToken[] = [
  {
    id: 'jt_001',
    name: 'Server B join',
    token_prefix: 'aegis_join_ab3',
    allowed_roles: ['gateway', 'relay_target'],
    expected_node_name: 'node-b',
    expires_at: new Date(Date.now() + 86400000 * 7).toISOString(),
    allowed_source_cidr: null,
    status: 'active',
    created_at: new Date(Date.now() - 86400000 * 2).toISOString(),
    used_at: new Date(Date.now() - 86400000).toISOString(),
  },
  {
    id: 'jt_002',
    name: 'Future node-c',
    token_prefix: 'aegis_join_xy9',
    allowed_roles: ['gateway'],
    expected_node_name: 'node-c',
    expires_at: new Date(Date.now() + 86400000 * 30).toISOString(),
    allowed_source_cidr: null,
    status: 'active',
    created_at: new Date(Date.now() - 86400000).toISOString(),
    used_at: null,
  },
];
