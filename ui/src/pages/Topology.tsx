import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchTopologyMatrix } from '@/lib/api-bridge';
import { PageHeader, Card, DataTable, StatusBadge, StatCard } from '@/components/shared';
import type { DataTableColumn } from '@/components/shared';
import type { TopologyEdge } from '@/types';

const columns: DataTableColumn<TopologyEdge>[] = [
  { key: 'from_node_name', label: 'From', mono: true },
  { key: 'to_node_name', label: 'To', mono: true },
  {
    key: 'private_reachable',
    label: 'Private',
    render: (row) => row.private_reachable ? <span className="text-[#4cd964]">✓</span> : <span className="text-a-muted">—</span>,
  },
  {
    key: 'public_reachable',
    label: 'Public',
    render: (row) => row.public_reachable ? <span className="text-[#e8b830]">✓</span> : <span className="text-a-muted">—</span>,
  },
  { key: 'preferred_gateway_id', label: 'Preferred GW', mono: true, muted: true, render: (row) => row.preferred_gateway_id || '—' },
  { key: 'gateway_link_id', label: 'GW Link', mono: true, render: (row) => row.gateway_link_id ? <span className="text-a-accent">{row.gateway_link_id}</span> : <span className="text-a-muted">—</span> },
  {
    key: 'status',
    label: '状态',
    render: (row) => <StatusBadge status={row.status} />,
  },
  {
    key: 'last_error',
    label: 'Error',
    muted: true,
    render: (row) => row.last_error || '—',
  },
];

export default function TopologyPage() {
  const navigate = useNavigate();
  const [fromNode, setFromNode] = useState('node-a');
  const [toNode, setToNode] = useState('node-b');

  const { data: matrix, isLoading, error } = useQuery({
    queryKey: ['topology-matrix'],
    queryFn: fetchTopologyMatrix,
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;

  const verifiedCount = matrix?.filter((e) => e.status === 'verified').length || 0;

  return (
    <div>
      <PageHeader title="Topology" helpKey="topology" subtitle="节点间网络拓扑与连通性"  />

      <div className="grid grid-cols-3 gap-3 mb-5">
        <StatCard label="Edges" value={matrix?.length || 0} accent />
        <StatCard label="Verified" value={verifiedCount} success />
        <StatCard label="Missing Links" value={(matrix?.length || 0) - verifiedCount} warn />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
        {/* Topology Matrix */}
        <Card title="Connectivity Matrix" className="md:col-span-2">
          <DataTable columns={columns} data={matrix || []} keyExtractor={(r) => `${r.from_node_id}-${r.to_node_id}`} />
        </Card>

        {/* Path Query Tool */}
        <Card title="路径查询" subtitle="查看两个节点间的 relay 路径">
          <div className="flex gap-2 mb-3">
            <select
              className="flex-1 font-mono text-xs px-2.5 py-1.5 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none"
              value={fromNode}
              onChange={(e) => setFromNode(e.target.value)}
            >
              <option value="node-a">Server A (node-a)</option>
              <option value="node-b">Server B (node-b)</option>
              <option value="node-c">Server C (node-c)</option>
            </select>
            <span className="text-a-muted text-xs self-center">→</span>
            <select
              className="flex-1 font-mono text-xs px-2.5 py-1.5 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none"
              value={toNode}
              onChange={(e) => setToNode(e.target.value)}
            >
              <option value="node-a">Server A (node-a)</option>
              <option value="node-b">Server B (node-b)</option>
              <option value="node-c">Server C (node-c)</option>
            </select>
            <button
              className="inline-flex items-center gap-1 text-xs px-3 py-1.5 rounded-a-md bg-a-accent text-white hover:opacity-90 cursor-pointer border-none font-medium"
              onClick={() => navigate(`/topology/path?from=${fromNode}&to=${toNode}`)}
            >
              查询
            </button>
          </div>
          <div className="text-xs text-a-muted">
            查看 <span className="font-mono">{fromNode}</span> → <span className="font-mono">{toNode}</span> 的中继路径
          </div>
        </Card>

        {/* Legend */}
        <Card title="Edge 状态说明">
          <div className="space-y-2 text-xs">
            <div className="flex items-center gap-2"><StatusBadge status="verified" /> GatewayLink verified, relay path OK</div>
            <div className="flex items-center gap-2"><StatusBadge status="missing_link" /> No GatewayLink configured</div>
            <div className="flex items-center gap-2"><StatusBadge status="unreachable" /> Node not reachable</div>
            <div className="flex items-center gap-2"><StatusBadge status="degraded" /> Partial connectivity</div>
            <div className="text-a-muted mt-2">
              Node public capability ≠ Gateway public accessible ≠ Route public allowed
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
}
