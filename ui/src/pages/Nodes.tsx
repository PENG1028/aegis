import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchNodes } from '@/lib/api-bridge';
import { PageHeader, Card, DataTable, StatusBadge, CapabilityBadge, Btn } from '@/components/shared';
import type { DataTableColumn } from '@/components/shared';
import type { Node } from '@/types';
import { fmtRel } from '@/lib/utils';
import DeployNodeDialog from '@/components/deploy/DeployNodeDialog';

const columns: DataTableColumn<Node>[] = [
  {
    key: 'node_id',
    label: '节点 ID',
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
  { key: 'hostname', label: '主机名', mono: true, muted: true },
  { key: 'public_ip', label: '公网 IP', mono: true },
  {
    key: 'roles',
    label: '角色',
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
    label: '同步',
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
    label: '能力',
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
  const [showDeploy, setShowDeploy] = useState(false);
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['nodes'],
    queryFn: fetchNodes,
  });

  // Expose for detail page navigation
  (window as any)._navigate = navigate;

  if (isLoading) {
    return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  }

  if (error) {
    return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message} <Btn sm className="ml-2" onClick={() => refetch()}>重试</Btn></div>;
  }

  return (
    <div>
      <PageHeader
        title="节点"
        helpKey="nodes"
        subtitle={`${data?.length || 0} 个节点`}
        actions={<Btn variant="primary" onClick={() => setShowDeploy(true)}>+ 部署节点</Btn>}
      />
      <Card>
        <DataTable columns={columns} data={data || []} keyExtractor={(r) => r.node_id} />
      </Card>

      <DeployNodeDialog open={showDeploy} onClose={() => setShowDeploy(false)} />
    </div>
  );
}
