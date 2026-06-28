import { useQuery } from '@tanstack/react-query';
import { fetchEndpoints } from '@/lib/api-bridge';
import { PageHeader, Card, DataTable, StatusBadge } from '@/components/shared';
import type { DataTableColumn } from '@/components/shared';
import type { Endpoint } from '@/types';
import { fmtRel } from '@/lib/utils';

const columns: DataTableColumn<Endpoint>[] = [
  { key: 'endpoint_id', label: 'Endpoint ID', mono: true, render: (row) => <span className="text-a-accent font-mono text-xs">{row.endpoint_id}</span> },
  { key: 'service_id', label: 'Service', mono: true, muted: true },
  { key: 'node_name', label: 'Node', muted: true },
  { key: 'protocol', label: '协议', render: (row) => <span className="font-mono text-[11px] px-2 py-0.5 rounded bg-a-border/40 text-a-muted">{row.protocol}</span> },
  {
    key: 'target_local_host',
    label: 'Target',
    mono: true,
    render: (row) => `${row.target_local_host}:${row.target_local_port}`,
  },
  {
    key: 'address_type',
    label: '类型',
    render: (row) => <StatusBadge status={row.address_type} />,
  },
  {
    key: 'relay_eligible',
    label: 'Relay',
    render: (row) => row.relay_eligible ? <span className="text-[#4cd964]">✓</span> : '—',
  },
  {
    key: 'health_status',
    label: '健康',
    render: (row) => <StatusBadge status={row.health_status} />,
  },
  {
    key: 'latency_ms',
    label: '延迟',
    mono: true,
    muted: true,
    render: (row) => row.latency_ms ? `${row.latency_ms}ms` : '—',
  },
  {
    key: 'last_checked_at',
    label: '检查',
    mono: true,
    muted: true,
    render: (row) => fmtRel(row.last_checked_at),
  },
];

export default function EndpointsPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['endpoints'],
    queryFn: fetchEndpoints,
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;

  return (
    <div>
      <PageHeader title="Endpoints" helpKey="endpoints" subtitle={`${data?.length || 0} 个 endpoint`}  />
      <Card>
        <DataTable columns={columns} data={data || []} keyExtractor={(r) => r.endpoint_id} />
      </Card>
    </div>
  );
}
