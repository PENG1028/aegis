import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchNodes } from '@/lib/api-bridge';
import { PageHeader, Card, DataTable, StatusBadge, CapabilityBadge } from '@/components/shared';
import type { DataTableColumn } from '@/components/shared';
import type { Node } from '@/types';
import { fmtRel } from '@/lib/utils';

const columns: DataTableColumn<Node>[] = [
  {
    key: 'node_id',
    label: 'Node ID',
    mono: true,
    render: (row) => (
      <button
        className="text-a-accent font-mono text-xs bg-transparent border-none cursor-pointer p-0 hover:underline"
        onClick={() => window._navigate?.(`/nodes/${row.node_id}`)}
      >
        {row.node_id}
      </button>
    ),
  },
  { key: 'name', label: '名称' },
  { key: 'hostname', label: 'Hostname', mono: true, muted: true },
  { key: 'public_ip', label: 'Public IP', mono: true },
  {
    key: 'roles',
    label: 'Roles',
    render: (row) => (
      <div className="flex gap-1 flex-wrap">
        {row.roles.map((r) => (
          <span key={r} className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-a-border/40 text-a-muted">
            {r}
          </span>
        ))}
      </div>
    ),
  },
  {
    key: 'status',
    label: '状态',
    render: (row) => <StatusBadge status={row.status} />,
  },
  {
    key: 'sync_status',
    label: 'Sync',
    render: (row) => <StatusBadge status={row.sync_status} />,
  },
  {
    key: 'last_heartbeat_at',
    label: '心跳',
    mono: true,
    muted: true,
    render: (row) => fmtRel(row.last_heartbeat_at),
  },
  {
    key: 'capabilities',
    label: 'Capabilities',
    render: (row) => (
      <div className="flex gap-1 flex-wrap max-w-[200px]">
        {Object.entries(row.capabilities)
          .filter(([, v]) => v)
          .slice(0, 3)
          .map(([k]) => (
            <CapabilityBadge key={k} name={k} enabled />
          ))}
      </div>
    ),
  },
];

export default function NodesPage() {
  const navigate = useNavigate();
  const { data, isLoading, error } = useQuery({
    queryKey: ['nodes'],
    queryFn: fetchNodes,
  });

  // Expose for detail page navigation
  (window as any)._navigate = navigate;

  if (isLoading) {
    return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  }

  if (error) {
    return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;
  }

  return (
    <div>
      <PageHeader title="Nodes" helpKey="nodes" subtitle={`${data?.length || 0} 个节点`}  />
      <Card>
        <DataTable columns={columns} data={data || []} keyExtractor={(r) => r.node_id} />
      </Card>
    </div>
  );
}
